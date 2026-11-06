package xyz.solanes.gregal

import android.app.AlarmManager
import android.app.Notification
import android.app.NotificationChannel
import android.app.NotificationManager
import android.app.PendingIntent
import android.app.Person
import android.app.Service
import android.content.BroadcastReceiver
import android.content.Context
import android.content.Intent
import android.content.pm.PackageManager
import android.content.pm.ServiceInfo
import android.os.Build
import android.os.IBinder
import android.os.PowerManager
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.cancel
import kotlinx.coroutines.launch

/*
 * Vigilant dels vents en mode servidor. El servidor programa els seus jobs
 * (cor, recordatoris) i apunta cada veu proactiva a /api/winds/novetats;
 * aquí preguntem cada 15 min (alarma inexacta —exacta si l'usuari l'ha
 * permesa—, que també sona en Doze) i si hi ha veus noves avisem amb
 * notificació local. En mode local no cal: els jobs ja els dispara l'app
 * (JobAlarms) i el seu resultat ja avisa.
 *
 * Tocar l'avís obre el xat directe amb el vent.
 */

/** Lògica pura del vigilant: quines veus són noves i què cal desar. */
object WindWatchLogic {
    /** Hores de passada a recordar: les properes primer (32 com a molt), mai les passades. */
    fun keepDue(times: List<Long>, now: Long): List<Long> =
        times.filter { it + WindWatch.DUE_GRACE_MS > now }.distinct().sorted().take(32)

    /**
     * Agrupa per vent les veus amb ts posterior a l'últim vist; torna
     * (perVent, nouTs): l'última veu de cada vent i el ts a desar.
     * Buides o antigues no hi entren.
     */
    fun noves(novetats: List<WindNovetat>, desDe: Long): Pair<Map<String, WindNovetat>, Long> {
        var ts = desDe
        val perVent = linkedMapOf<String, WindNovetat>()
        for (n in novetats.sortedBy { it.ts }) {
            if (n.ts > ts) ts = n.ts
            if (n.ts > desDe && n.text.isNotBlank() && n.wind.isNotBlank()) perVent[n.wind] = n
        }
        return perVent to ts
    }
}

object WindWatch {
    const val ACTION_CHECK = "xyz.solanes.gregal.WIND_WATCH"
    private const val PREFS = "gregal_winds_watch"
    private const val KEY_TS = "last_ts"
    const val INTERVAL_MS = 15 * 60_000L
    /** Amb una feina en marxa al servidor: l'avís d'acabada no ha d'esperar 15 minuts. */
    const val SOON_MS = 3 * 60_000L

    private fun prefs(ctx: Context) = ctx.getSharedPreferences(PREFS, Context.MODE_PRIVATE)

    /**
     * Cursor per servidor desat: canviar de servidor no hereta l'última
     * marca de l'altre. Sense servidor desat, clau única (mai s'usa).
     */
    private fun originKey(ctx: Context): String {
        val id = NotificationOrigins.remote(Prefs(ctx))?.id
        return if (id.isNullOrBlank()) KEY_TS else KEY_TS + "_" + id
    }

    fun lastTs(ctx: Context): Long = prefs(ctx).getLong(originKey(ctx), 0L)
    fun saveTs(ctx: Context, ts: Long) { prefs(ctx).edit().putLong(originKey(ctx), ts).apply() }

    /**
     * El vigilant segueix el servidor desat encara que ara usis el mòbil:
     * cada avís porta el seu origen i obre la instància que toca.
     */
    fun wanted(ctx: Context): Boolean = NotificationOrigins.remote(Prefs(ctx)) != null

    private const val KEY_DUE = "due"
    /** Marge després de l'hora de l'avís: el servidor el fa sonar i el desa. */
    const val DUE_GRACE_MS = 30_000L
    const val ACTION_LAG_MS = 120_000L

    /** Avisos pendents (ms), els que encara no han passat. */
    fun dueTimes(ctx: Context, now: Long = System.currentTimeMillis()): List<Long> =
        prefs(ctx).getStringSet(KEY_DUE, emptySet()).orEmpty().mapNotNull { it.toLongOrNull() }
            .filter { it + DUE_GRACE_MS > now }.sorted()

    /**
     * Un vent ha programat avisos al servidor: la propera passada es fa just
     * després de l'hora, no d'aquí a 15 minuts (un «d'aquí a 2 minuts» arribava tard).
     */
    fun expect(ctx: Context, times: List<Long>) {
        if (times.isEmpty()) return
        val now = System.currentTimeMillis()
        // Un avís amb feina (cerca web) acaba uns segons després de l'hora: una segona
        // passada 2 minuts després el recull sense esperar els 15 minuts.
        val all = times.flatMap { listOf(it, it + ACTION_LAG_MS) }
        val keep = WindWatchLogic.keepDue(dueTimes(ctx, now) + all, now)
        prefs(ctx).edit().putStringSet(KEY_DUE, keep.map { it.toString() }.toSet()).apply()
        schedule(ctx)
    }

    /** Quan toca la passada: el primer avís pendent si arriba abans que l'interval. */
    fun nextCheckDelay(delayMs: Long, due: List<Long>, now: Long): Long {
        val first = due.firstOrNull { it + DUE_GRACE_MS > now } ?: return delayMs
        return minOf(delayMs, maxOf(5_000L, first + DUE_GRACE_MS - now))
    }

    /** Programa la propera passada (exacta si es pot, si no inexacta). */
    fun schedule(ctx: Context, delayMs: Long = INTERVAL_MS) {
        if (!wanted(ctx)) return
        val am = ctx.getSystemService(AlarmManager::class.java)
        val now = System.currentTimeMillis()
        val at = now + nextCheckDelay(delayMs, dueTimes(ctx, now), now)
        try {
            if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.S && am.canScheduleExactAlarms())
                am.setExactAndAllowWhileIdle(AlarmManager.RTC_WAKEUP, at, pending(ctx))
            else am.setAndAllowWhileIdle(AlarmManager.RTC_WAKEUP, at, pending(ctx))
        } catch (_: SecurityException) {
            try { am.setAndAllowWhileIdle(AlarmManager.RTC_WAKEUP, at, pending(ctx)) } catch (_: Exception) {}
        }
    }

    fun cancel(ctx: Context) {
        try { ctx.getSystemService(AlarmManager::class.java).cancel(pending(ctx)) } catch (_: Exception) {}
    }

    private fun pending(ctx: Context): PendingIntent = PendingIntent.getBroadcast(
        ctx, 0,
        Intent(ctx, WindWatchReceiver::class.java).setAction(ACTION_CHECK),
        PendingIntent.FLAG_UPDATE_CURRENT or PendingIntent.FLAG_IMMUTABLE,
    )
}

/** Sona la passada: el servei fa la feina en primer pla. */
class WindWatchReceiver : BroadcastReceiver() {
    override fun onReceive(context: Context, intent: Intent) {
        if (intent.action != WindWatch.ACTION_CHECK) return
        WindWatchService.start(context)
    }
}

/** Reinici o canvi de permís: tornar a armar la passada. */
class WindWatchBootReceiver : BroadcastReceiver() {
    override fun onReceive(context: Context, intent: Intent) {
        when (intent.action) {
            Intent.ACTION_BOOT_COMPLETED, Intent.ACTION_MY_PACKAGE_REPLACED,
            AlarmManager.ACTION_SCHEDULE_EXACT_ALARM_PERMISSION_STATE_CHANGED ->
                WindWatch.schedule(context)
        }
    }
}

/** Una passada del vigilant: novetats noves → avís; rearma sempre. */
class WindWatchService : Service() {
    private val scope = CoroutineScope(SupervisorJob() + Dispatchers.IO)

    override fun onStartCommand(intent: Intent?, flags: Int, startId: Int): Int {
        try {
            WindNotifications.ensureChannel(this)
            val n = WindNotifications.running(this)
            if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.Q)
                startForeground(WindNotifications.RUNNING_ID, n, ServiceInfo.FOREGROUND_SERVICE_TYPE_DATA_SYNC)
            else startForeground(WindNotifications.RUNNING_ID, n)
        } catch (_: Exception) {
            stopSelf()
            return START_NOT_STICKY
        }
        scope.launch {
            try { check() } finally {
                try { stopForeground(STOP_FOREGROUND_REMOVE) } catch (_: Exception) {}
                stopSelf()
            }
        }
        return START_NOT_STICKY
    }

    private fun check() {
        try {
            if (!WindWatch.wanted(applicationContext)) return
            val p = Prefs(applicationContext)
            if (p.host.isBlank() || p.token.isBlank()) return
            val wake = getSystemService(PowerManager::class.java)
                .newWakeLock(PowerManager.PARTIAL_WAKE_LOCK, "gregal:windwatch")
            wake.acquire(60_000L)
            try {
                val api = GregalApi(baseUrlOf(p.host, p.port), p.token)
                // Invitacions a xats compartits: avisen una sola vegada (SharedChats.kt).
                SharedInviteWatch.check(this, api, notify = !AppVisibility.foreground)
                val desDe = WindWatch.lastTs(this)
                val store = VentsOffline.savedServer(applicationContext)
                val now = System.currentTimeMillis()
                val noves = try { api.windNovetats(desDe) } catch (_: Exception) {
                    // El servidor no respon: els recordatoris que ja toquen surten del mòbil.
                    if (store != null) mirrorDue(store, p, now)
                    return
                }
                val shown = store?.shown().orEmpty()
                if (store != null) {
                    store.saveReminders(OfflineLogic.pruneOnline(store.reminders(), now))
                    store.saveShown(OfflineLogic.pruneShown(shown, now))
                }
                // El cursor avança amb totes; només avisen les que el mòbil encara no ha mostrat.
                val nouTs = WindWatchLogic.noves(noves, desDe).second
                val perVent = WindWatchLogic.noves(OfflineLogic.notYetShown(noves, shown), desDe).first
                WindWatch.saveTs(this, nouTs)
                // Primera passada en aquest servidor/compte: només marca on som.
                // Avisar de tot el que ja hi havia seria una ràfega de notificacions velles.
                if (desDe == 0L) return
                aviat = try { api.windTasks().any { it.running } } catch (_: Exception) { false }
                if (perVent.isEmpty() || AppVisibility.foreground) return
                if (!WindNotifications.canNotify(this)) return
                val source = NotificationOrigins.remote(p)?.id.orEmpty()
                for ((_, n) in perVent) WindNotifications.veu(this, n, source)
            } finally {
                if (wake.isHeld) wake.release()
            }
        } finally {
            try { WindWatch.schedule(applicationContext, if (aviat) WindWatch.SOON_MS else WindWatch.INTERVAL_MS) } catch (_: Exception) {}
        }
    }

    /**
     * Sense servidor a l'hora d'un recordatori: l'avís surt igualment, amb
     * el text que el vent va desar. L'id queda apuntat perquè, quan el
     * servidor torni i el dispari, no soni dues vegades.
     */
    private fun mirrorDue(store: VentsOfflineStore, p: Prefs, now: Long) {
        val shown = store.shown()
        val due = OfflineLogic.dueOffline(store.reminders(), shown, now)
        if (due.isEmpty() || !WindNotifications.canNotify(this)) return
        val source = NotificationOrigins.remote(p)?.id.orEmpty()
        for (r in due) WindNotifications.veu(this, WindNovetat(r.at / 1000, r.wind, r.name, "remind", r.text, r.id), source)
        store.saveShown(shown + due.associate { it.id to now })
    }

    /** Hi ha feina en marxa: la propera passada, en 3 minuts. */
    private var aviat = false

    override fun onBind(intent: Intent?): IBinder? = null

    companion object {
        fun start(ctx: Context) {
            val i = Intent(ctx, WindWatchService::class.java)
            try {
                if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) ctx.startForegroundService(i)
                else ctx.startService(i)
            } catch (_: Exception) {
                // Sense permís de primer pla en segon pla: rearma i prou.
                try { WindWatch.schedule(ctx) } catch (_: Exception) {}
            }
        }
    }
}

/** Avisos dels vents: canal propi per silenciar-los a part. */
object WindNotifications {
    const val CHANNEL_ID = "gregal_vents"
    const val RUNNING_ID = 451
    const val EXTRA_WIND = "xyz.solanes.gregal.WIND_ID"
    const val EXTRA_ROT = "xyz.solanes.gregal.ROT_ID"

    private fun text(ctx: Context, key: String, vararg a: Any): String = tr(Prefs(ctx).idioma, key, *a)

    fun ensureChannel(ctx: Context) {
        if (Build.VERSION.SDK_INT < Build.VERSION_CODES.O) return
        val ch = NotificationChannel(CHANNEL_ID, text(ctx, "app.vents.notif.channel"), NotificationManager.IMPORTANCE_DEFAULT)
            .apply { description = text(ctx, "app.vents.notif.channelDescription") }
        ctx.getSystemService(NotificationManager::class.java).createNotificationChannel(ch)
    }

    internal fun running(ctx: Context): Notification = Notification.Builder(ctx, CHANNEL_ID)
        .setSmallIcon(R.drawable.ic_stat_gregal)
        .setContentTitle(text(ctx, "app.vents.notif.running"))
        .setUsesChronometer(true)
        .setOngoing(true)
        .setContentIntent(open(ctx, ""))
        .setVisibility(Notification.VISIBILITY_PRIVATE)
        .build()

    /** Una veu nova d'un vent: nom + resum; tocar-la obre el seu xat a la instància que la va provocar. */
    fun veu(ctx: Context, n: WindNovetat, origin: String = "") {
        if (Build.VERSION.SDK_INT >= 33 &&
            ctx.checkSelfPermission(android.Manifest.permission.POST_NOTIFICATIONS) != PackageManager.PERMISSION_GRANTED) return
        ensureChannel(ctx)
        val source = NotificationOrigins.effective(ctx, origin)
        val nom = n.name.ifBlank { text(ctx, "app.vents.notif.wind") }
        val cos = JobSchedule.excerpt(n.text, 600)
        val rot = n.source.removePrefix("feina:").takeIf { n.source.startsWith("feina:") }.orEmpty()
        val target = if (rot.isNotEmpty()) "rot:$rot" else "wind:${n.wind}"
        val builder = Notification.Builder(ctx, CHANNEL_ID)
            .setSmallIcon(R.drawable.ic_stat_gregal)
            .setContentTitle(nom)
            .setContentText(cos.lineSequence().firstOrNull().orEmpty())
            .setStyle(Notification.BigTextStyle().bigText(cos))
            .setContentIntent(NotificationNavigation.pending(ctx, source, target))
            .setAutoCancel(true)
            .setVisibility(Notification.VISIBILITY_PRIVATE)
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.P) {
            val vent = Person.Builder().setName(nom).build()
            builder.style = Notification.MessagingStyle(vent)
                .addMessage(cos, System.currentTimeMillis(), vent)
        }
        val publica = Notification.Builder(ctx, CHANNEL_ID)
            .setSmallIcon(R.drawable.ic_stat_gregal)
            .setContentTitle("Gregal")
            .build()
        builder.setPublicVersion(publica)
        try {
            ctx.getSystemService(NotificationManager::class.java).notify(NotificationNavigation.identity(source, target).hashCode(), builder.build())
        } catch (_: Exception) {
        }
    }

    internal fun canNotify(ctx: Context): Boolean =
        Build.VERSION.SDK_INT < 33 ||
            ctx.checkSelfPermission(android.Manifest.permission.POST_NOTIFICATIONS) ==
            PackageManager.PERMISSION_GRANTED

    private fun open(ctx: Context, windId: String, rotId: String = ""): PendingIntent = PendingIntent.getActivity(
        ctx, (windId + rotId).hashCode(),
        Intent(ctx, MainActivity::class.java).apply {
            flags = Intent.FLAG_ACTIVITY_SINGLE_TOP or Intent.FLAG_ACTIVITY_CLEAR_TOP
            // Una feina acabada obre el seu fil; una veu, el xat amb el vent.
            if (rotId.isNotEmpty()) putExtra(EXTRA_ROT, rotId)
            else if (windId.isNotEmpty()) putExtra(EXTRA_WIND, windId)
        },
        PendingIntent.FLAG_UPDATE_CURRENT or PendingIntent.FLAG_IMMUTABLE,
    )
}
