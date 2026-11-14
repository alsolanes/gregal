package xyz.solanes.gregal

import android.app.Notification
import android.app.NotificationChannel
import android.app.NotificationManager
import android.app.PendingIntent

import android.app.Service
import android.content.Context
import android.content.Intent
import android.content.pm.PackageManager
import android.content.pm.ServiceInfo
import android.graphics.drawable.Icon
import android.os.Build
import android.os.IBinder
import android.net.Uri

/** Estat mínim per no avisar si l'usuari ja està llegint el streaming. */
object AppVisibility {
    @Volatile var foreground: Boolean = false
}

/**
 * Manté el procés prioritari mentre el servidor envia SSE. Sense aquest servei,
 * Android pot matar la connexió quan l'app queda en segon pla.
 */
class GregalWorkService : Service() {
    override fun onStartCommand(intent: Intent?, flags: Int, startId: Int): Int {
        // startForeground ha d'anar primer i sempre: cada startForegroundService
        // ho exigeix, fins i tot si mentrestant ja ens han demanat parar.
        try {
            GregalNotifications.ensureChannel(this)
            val n = GregalNotifications.working(this)
            if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.Q) {
                startForeground(GregalNotifications.WORK_ID, n, ServiceInfo.FOREGROUND_SERVICE_TYPE_DATA_SYNC)
            } else {
                startForeground(GregalNotifications.WORK_ID, n)
            }
        } catch (_: Exception) {
            GregalNotifications.serviceGone()
            stopSelf()
            return START_NOT_STICKY
        }
        if (!GregalNotifications.serviceForegrounded()) {
            // La feina ja havia acabat abans que arribéssim: parem ara, ja
            // complert el contracte de startForeground.
            stopWork()
        }
        return START_NOT_STICKY
    }

    private fun stopWork() {
        try {
            stopForeground(STOP_FOREGROUND_REMOVE)
        } catch (_: Exception) {
        }
        GregalNotifications.serviceGone()
        stopSelf()
    }

    override fun onDestroy() {
        GregalNotifications.serviceGone()
        super.onDestroy()
    }

    override fun onBind(intent: Intent?): IBinder? = null
}

/** Notificacions locals: no hi ha cap token ni text del projecte a la pantalla bloquejada. */
object GregalNotifications {
    const val CHANNEL_ID = "gregal_answers"
    const val WORK_ID = 431
    private const val READY_ID = 432
    private const val NEEDS_YOU_ID = 433

    private fun text(context: Context, key: String): String = tr(Prefs(context).idioma, key)

    fun ensureChannel(context: Context) {
        if (Build.VERSION.SDK_INT < Build.VERSION_CODES.O) return
        val channel = NotificationChannel(
            CHANNEL_ID, text(context, "app.notifications.channel"), NotificationManager.IMPORTANCE_DEFAULT
        ).apply {
            description = text(context, "app.notifications.channelDescription")
        }
        context.getSystemService(NotificationManager::class.java).createNotificationChannel(channel)
    }

    // Estat del servei de feina. Les crides arriben del fil principal, però el
    // servei també hi escriu: tot plegat sota el mateix lock.
    private val lock = Any()
    private var wantRunning = false
    private data class WorkKey(val origin: String, val target: String)
    private data class Work(val count: Int = 1, val name: String = "", val stage: String = "", val since: Long = System.currentTimeMillis())
    private val activeWork = linkedMapOf<WorkKey, Work>()
    private var state = 0           // 0 aturat, 1 arrencant, 2 en primer pla
    private var startingSince = 0L

    /**
     * Cridat des de onStartCommand: marca el servei en primer pla i torna si
     * encara cal (si no, el servei para tot sol just després de startForeground).
     */
    internal fun serviceForegrounded(): Boolean = synchronized(lock) {
        state = 2
        wantRunning
    }

    internal fun serviceGone() = synchronized(lock) { state = 0 }

    /** Pair starts/finishes with the same target. Empty target preserves legacy callers. */
    fun workStarted(context: Context, target: String = "", displayName: String = "", origin: String = "") {
        // Amb el push funcionant no cal mantenir el procés viu ni una
        // notificació fixa a la barra mentre treballa: el servidor avisa quan
        // el torn s'acaba o quan es queda esperant el teu permís (winds_torn).
        // Sense push (servidor sense Firebase, app vella) es queda com abans.
        if (VentsPush.active(context)) return
        val key = WorkKey(NotificationOrigins.effective(context, origin), target)
        val start = synchronized(lock) {
            val previous = activeWork[key]
            activeWork[key] = previous?.copy(count = previous.count + 1) ?: Work(name = displayName.take(60))
            wantRunning = true
            // Si una arrencada s'ha perdut (el sistema no ha cridat mai
            // onStartCommand), no ens quedem encallats en «arrencant».
            val stale = state == 1 && System.currentTimeMillis() - startingSince > 15_000
            if (state == 0 || stale) {
                state = 1
                startingSince = System.currentTimeMillis()
                true
            } else false
        }
        if (!start) return
        try {
            ensureChannel(context)
            val intent = Intent(context, GregalWorkService::class.java)
            if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) context.startForegroundService(intent)
            else context.startService(intent)
        } catch (_: Exception) {
            // API 31+: ForegroundServiceStartNotAllowedException si l'app és en
            // segon pla. Sense servei la resposta segueix arribant; només perdem
            // la prioritat del procés. Mai una caiguda.
            synchronized(lock) { state = 0 }
        }
    }

    /** Already-localized coarse stage, never tokens or raw tool arguments. No repeated alerts. */
    fun workUpdated(context: Context, target: String, stage: String, origin: String = "") {
        val key = WorkKey(NotificationOrigins.effective(context, origin), target)
        synchronized(lock) {
            val work = activeWork[key] ?: return
            val compact = stage.replace(Regex("\\s+"), " ").trim().take(100)
            if (compact.isBlank() || work.stage == compact) return
            activeWork[key] = work.copy(stage = compact)
            refreshWorking(context)
        }
    }

    private fun refreshWorking(context: Context) {
        if (state != 2 || !wantRunning) return
        try { context.getSystemService(NotificationManager::class.java).notify(WORK_ID, working(context)) } catch (_: Exception) { }
    }

    fun workFinished(
        context: Context, hasAnswer: Boolean, userText: String = "", answer: String = "",
        target: String = "", displayName: String = "", partial: Boolean = false, error: String = "",
        origin: String = "",
    ) {
        val source = NotificationOrigins.effective(context, origin)
        val key = WorkKey(source, target)
        val stopNow = synchronized(lock) {
            activeWork[key]?.let { work ->
                if (work.count > 1) activeWork[key] = work.copy(count = work.count - 1) else activeWork.remove(key)
            }
            wantRunning = activeWork.isNotEmpty()
            if (wantRunning) refreshWorking(context)
            // Si encara arrenca, no fem stopService (abans de startForeground
            // seria ForegroundServiceDidNotStartInTimeException): el servei ho
            // veurà a onStartCommand i pararà ell mateix.
            if (!wantRunning && state == 2) { state = 0; true } else false
        }
        if (stopNow) {
            try {
                context.stopService(Intent(context, GregalWorkService::class.java))
            } catch (_: Exception) {
            }
        }
        if ((!hasAnswer && !partial && error.isBlank()) || AppVisibility.foreground || !canNotify(context)) return
        ensureChannel(context)
        val titleKey = when {
            error.isNotBlank() -> "app.notifications.failedTitle"
            partial -> "app.notifications.partialTitle"
            target.isNotBlank() -> "app.notifications.replyTitle"
            else -> "app.notifications.readyTitle"
        }
        val bodyKey = when {
            error.isNotBlank() -> "app.notifications.failedBody"
            partial -> "app.notifications.partialBody"
            else -> "app.notifications.readyBody"
        }
        val title = text(context, titleKey)
        val b = Notification.Builder(context, CHANNEL_ID)
            .setSmallIcon(R.drawable.ic_stat_gregal)
            .setContentTitle(NotificationOrigins.label(context, source) + " · " + if (displayName.isBlank()) title else "${displayName.take(60)} · $title")
            .setContentIntent(openApp(context, SCREEN_CHAT, target, source))
            .setAutoCancel(true)
            .setVisibility(Notification.VISIBILITY_PRIVATE)
            .setPublicVersion(publicNotice(context, title))
        val excerpt = answer.trim().take(280)
        if (excerpt.isNotEmpty() && !partial && error.isBlank()) {
            // MessagingStyle replaces EXTRA_TITLE with the speaker's name,
            // hiding the actual outcome. Keep status explicit in all layouts.
            b.setContentText(excerpt.lineSequence().firstOrNull().orEmpty())
                .setStyle(Notification.BigTextStyle().bigText(excerpt))
        } else {
            b.setContentText(text(context, bodyKey))
            if (partial && error.isBlank() && excerpt.isNotEmpty()) b.setStyle(Notification.BigTextStyle()
                .bigText(text(context, bodyKey) + "\n" + excerpt))
        }
        try {
            b.setLargeIcon(Icon.createWithResource(context, R.mipmap.ic_gregal))
        } catch (_: Exception) {
        }
        try {
            val manager = context.getSystemService(NotificationManager::class.java)
            val identity = NotificationNavigation.identity(source, target.ifBlank { "screen:$SCREEN_CHAT" })
            manager.notify("gregal-work:$identity", identity.hashCode(), b.build())
            // Marcat per no avisar dues vegades si el servidor també envia el
            // push del torn (VentsPush.onMessage el mira).
            lastTurnOrigin = source
            lastTurnAt = System.currentTimeMillis()
        } catch (_: Exception) {
            // Permís revocat entre la comprovació i l'avís: no és motiu per caure.
        }
    }

    /**
     * L'agent espera una aprovació o una resposta. Amb l'app en segon pla
     * (un torn llarg mentre fas una altra cosa) no ho veuries: sense avís,
     * el torn es quedaria aturat fins que caduqués la pregunta.
     */
    fun needsYou(context: Context, origin: String = "") {
        val source = NotificationOrigins.effective(context, origin)
        if (AppVisibility.foreground || !canNotify(context)) return
        ensureChannel(context)
        val n = Notification.Builder(context, CHANNEL_ID)
            .setSmallIcon(R.drawable.ic_stat_gregal)
            .setContentTitle(NotificationOrigins.label(context, source) + " · " + text(context, "app.notifications.needsYouTitle"))
            .setContentText(text(context, "app.notifications.needsYouBody"))
            .setContentIntent(openApp(context, SCREEN_CHAT, origin = source))
            .setAutoCancel(true)
            .setVisibility(Notification.VISIBILITY_PRIVATE)
            .setPublicVersion(publicNotice(context))
            .build()
        try {
            context.getSystemService(NotificationManager::class.java).notify("gregal-needs:$source", NEEDS_YOU_ID, n)
        } catch (_: Exception) {
        }
    }

    internal fun working(context: Context): Notification = synchronized(lock) {
        val key = activeWork.keys.firstOrNull()
        val work = activeWork[key]
        val source = key?.origin ?: NotificationOrigins.effective(context, "")
        val title = text(context, "app.notifications.workingTitle")
        Notification.Builder(context, CHANNEL_ID)
            .setSmallIcon(R.drawable.ic_stat_gregal)
            .setContentTitle(NotificationOrigins.label(context, source) + " · " + if (work?.name.isNullOrBlank()) title else "${work?.name} · $title")
            .setContentText(work?.stage?.ifBlank { null } ?: text(context, "app.notifications.workingBody"))
            .setContentIntent(openApp(context, SCREEN_CHAT, key?.target.orEmpty(), source))
            .setWhen(work?.since ?: System.currentTimeMillis())
            .setUsesChronometer(true)
            .setOnlyAlertOnce(true)
            .setGroup("gregal_work_silent")
            .setGroupAlertBehavior(Notification.GROUP_ALERT_SUMMARY)
            .setOngoing(true)
            .setVisibility(Notification.VISIBILITY_PRIVATE)
            .setPublicVersion(publicNotice(context, title))
            .build()
    }

    /**
     * Tocar l'avís ha d'obrir la pantalla que l'ha provocat. Abans anava sempre
     * a la portada, i qui rebia «ja està» acabava a la llista sense la
     * conversa que demanava.
     */
    private fun publicNotice(context: Context, title: String = "Gregal"): Notification = Notification.Builder(context, CHANNEL_ID)
        .setSmallIcon(R.drawable.ic_stat_gregal)
        .setContentTitle("Gregal")
        .build()

    private fun openApp(context: Context, screen: String = "", target: String = "", origin: String = ""): PendingIntent =
        NotificationNavigation.pending(context, origin, target.ifBlank { "screen:$screen" })

    /** Pantalla que obre l'avís: el xat, que és on viu la conversa. */
    const val SCREEN_CHAT = "chat"
    const val EXTRA_SCREEN = "xyz.solanes.gregal.SCREEN"

    // Últim avís de «torn acabat» que ha sortit d'aquí (l'app amb el fil obert
    // en segon pla). El push del servidor el consulta per no duplicar-lo.
    @Volatile private var lastTurnAt = 0L
    @Volatile private var lastTurnOrigin = ""

    /** Cert si aquest origen ja ha avisat d'un torn fa menys de [windowMs]. */
    fun turnJustNotified(origin: String, now: Long, windowMs: Long = 120_000L): Boolean =
        origin.isNotBlank() && origin == lastTurnOrigin && lastTurnAt > 0 && now - lastTurnAt in 0 until windowMs

    private fun canNotify(context: Context): Boolean =
        Build.VERSION.SDK_INT < 33 ||
            context.checkSelfPermission(android.Manifest.permission.POST_NOTIFICATIONS) ==
            PackageManager.PERMISSION_GRANTED
}
