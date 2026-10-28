package xyz.solanes.gregal

import android.app.Notification
import android.app.NotificationChannel
import android.app.NotificationManager
import android.app.PendingIntent
import android.app.Person
import android.app.Service
import android.content.Context
import android.content.Intent
import android.content.pm.PackageManager
import android.content.pm.ServiceInfo
import android.graphics.drawable.Icon
import android.os.Build
import android.os.IBinder

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

    fun workStarted(context: Context) {
        val start = synchronized(lock) {
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

    fun workFinished(context: Context, hasAnswer: Boolean, userText: String = "", answer: String = "") {
        val stopNow = synchronized(lock) {
            wantRunning = false
            // Si encara arrenca, no fem stopService (abans de startForeground
            // seria ForegroundServiceDidNotStartInTimeException): el servei ho
            // veurà a onStartCommand i pararà ell mateix.
            if (state == 2) { state = 0; true } else false
        }
        if (stopNow) {
            try {
                context.stopService(Intent(context, GregalWorkService::class.java))
            } catch (_: Exception) {
            }
        }
        if (!hasAnswer || AppVisibility.foreground || !canNotify(context)) return
        ensureChannel(context)
        val b = Notification.Builder(context, CHANNEL_ID)
            .setSmallIcon(R.drawable.ic_stat_gregal)
            .setContentTitle(text(context, "app.notifications.readyTitle"))
            .setContentIntent(openApp(context, SCREEN_CHAT))
            .setAutoCancel(true)
            .setVisibility(Notification.VISIBILITY_PRIVATE)
        val excerpt = answer.trim().take(280)
        if (excerpt.isNotEmpty() && Build.VERSION.SDK_INT >= Build.VERSION_CODES.P) {
            // Estil conversa quan està desbloquejat; a la pantalla bloquejada
            // segueix sent genèric (PRIVATE): ni resposta ni dades del projecte.
            val me = Person.Builder().setName(text(context, "app.notifications.you")).build()
            val gregal = Person.Builder().setName("Gregal").build()
            val now = System.currentTimeMillis()
            b.style = Notification.MessagingStyle(gregal)
                .addMessage(userText.trim().take(160).ifEmpty { "…" }, now, me)
                .addMessage(excerpt, now, gregal)
        } else {
            b.setContentText(text(context, "app.notifications.readyBody"))
        }
        try {
            b.setLargeIcon(Icon.createWithResource(context, R.mipmap.ic_gregal))
        } catch (_: Exception) {
        }
        try {
            context.getSystemService(NotificationManager::class.java).notify(READY_ID, b.build())
        } catch (_: Exception) {
            // Permís revocat entre la comprovació i l'avís: no és motiu per caure.
        }
    }

    /**
     * L'agent espera una aprovació o una resposta. Amb l'app en segon pla
     * (un torn llarg mentre fas una altra cosa) no ho veuries: sense avís,
     * el torn es quedaria aturat fins que caduqués la pregunta.
     */
    fun needsYou(context: Context) {
        if (AppVisibility.foreground || !canNotify(context)) return
        ensureChannel(context)
        val n = Notification.Builder(context, CHANNEL_ID)
            .setSmallIcon(R.drawable.ic_stat_gregal)
            .setContentTitle(text(context, "app.notifications.needsYouTitle"))
            .setContentText(text(context, "app.notifications.needsYouBody"))
            .setContentIntent(openApp(context, SCREEN_CHAT))
            .setAutoCancel(true)
            .setVisibility(Notification.VISIBILITY_PRIVATE)
            .build()
        try {
            context.getSystemService(NotificationManager::class.java).notify(NEEDS_YOU_ID, n)
        } catch (_: Exception) {
        }
    }

    internal fun working(context: Context): Notification = Notification.Builder(context, CHANNEL_ID)
        .setSmallIcon(R.drawable.ic_stat_gregal)
        .setContentTitle(text(context, "app.notifications.workingTitle"))
        .setContentText(text(context, "app.notifications.workingBody"))
        .setContentIntent(openApp(context, SCREEN_CHAT))
        .setUsesChronometer(true)
        .setOngoing(true)
        .setVisibility(Notification.VISIBILITY_PRIVATE)
        .build()

    /**
     * Tocar l'avís ha d'obrir la pantalla que l'ha provocat. Abans anava sempre
     * a la portada, i qui rebia «ja està» acabava a la llista sense la
     * conversa que demanava.
     */
    private fun openApp(context: Context, screen: String = ""): PendingIntent = PendingIntent.getActivity(
        context, screen.hashCode() and 0xffff,
        Intent(context, MainActivity::class.java).apply {
            flags = Intent.FLAG_ACTIVITY_SINGLE_TOP or Intent.FLAG_ACTIVITY_CLEAR_TOP
            if (screen.isNotEmpty()) putExtra(EXTRA_SCREEN, screen)
        },
        PendingIntent.FLAG_UPDATE_CURRENT or PendingIntent.FLAG_IMMUTABLE,
    )

    /** Pantalla que obre l'avís: el xat, que és on viu la conversa. */
    const val SCREEN_CHAT = "chat"
    const val EXTRA_SCREEN = "xyz.solanes.gregal.SCREEN"

    private fun canNotify(context: Context): Boolean =
        Build.VERSION.SDK_INT < 33 ||
            context.checkSelfPermission(android.Manifest.permission.POST_NOTIFICATIONS) ==
            PackageManager.PERMISSION_GRANTED
}
