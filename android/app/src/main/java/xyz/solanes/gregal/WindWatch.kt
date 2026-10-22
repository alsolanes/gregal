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

    private fun prefs(ctx: Context) = ctx.getSharedPreferences(PREFS, Context.MODE_PRIVATE)
    fun lastTs(ctx: Context): Long = prefs(ctx).getLong(KEY_TS, 0L)
    fun saveTs(ctx: Context, ts: Long) { prefs(ctx).edit().putLong(KEY_TS, ts).apply() }

    /** Només en mode servidor (host configurat): en local ja avisa JobAlarms. */
    fun wanted(ctx: Context): Boolean {
        val p = Prefs(ctx)
        return p.host.isNotBlank() && p.target != Prefs.TARGET_LOCAL
    }

    /** Programa la propera passada (exacta si es pot, si no inexacta). */
    fun schedule(ctx: Context) {
        if (!wanted(ctx)) return
        val am = ctx.getSystemService(AlarmManager::class.java)
        val at = System.currentTimeMillis() + INTERVAL_MS
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
            if (p.host.isBlank()) return
            val wake = getSystemService(PowerManager::class.java)
                .newWakeLock(PowerManager.PARTIAL_WAKE_LOCK, "gregal:windwatch")
            wake.acquire(60_000L)
            try {
                val api = GregalApi(baseUrlOf(p.host, p.port), p.token)
                val desDe = WindWatch.lastTs(this)
                val noves = try { api.windNovetats(desDe) } catch (_: Exception) { return }
                val (perVent, nouTs) = WindWatchLogic.noves(noves, desDe)
                WindWatch.saveTs(this, nouTs)
                if (perVent.isEmpty() || AppVisibility.foreground) return
                if (!WindNotifications.canNotify(this)) return
                for ((_, n) in perVent) WindNotifications.veu(this, n)
            } finally {
                if (wake.isHeld) wake.release()
            }
        } finally {
            try { WindWatch.schedule(applicationContext) } catch (_: Exception) {}
        }
    }

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

    /** Una veu nova d'un vent: nom + resum; tocar-la obre el seu xat. */
    fun veu(ctx: Context, n: WindNovetat) {
        if (Build.VERSION.SDK_INT >= 33 &&
            ctx.checkSelfPermission(android.Manifest.permission.POST_NOTIFICATIONS) != PackageManager.PERMISSION_GRANTED) return
        ensureChannel(ctx)
        val nom = n.name.ifBlank { text(ctx, "app.vents.notif.wind") }
        val cos = JobSchedule.excerpt(n.text, 600)
        val builder = Notification.Builder(ctx, CHANNEL_ID)
            .setSmallIcon(R.drawable.ic_stat_gregal)
            .setContentTitle(nom)
            .setContentText(cos.lineSequence().firstOrNull().orEmpty())
            .setStyle(Notification.BigTextStyle().bigText(cos))
            .setContentIntent(open(ctx, n.wind))
            .setAutoCancel(true)
            .setVisibility(Notification.VISIBILITY_PRIVATE)
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.P) {
            val vent = Person.Builder().setName(nom).build()
            builder.style = Notification.MessagingStyle(vent)
                .addMessage(cos, System.currentTimeMillis(), vent)
        }
        val publica = Notification.Builder(ctx, CHANNEL_ID)
            .setSmallIcon(R.drawable.ic_stat_gregal)
            .setContentTitle(nom)
            .build()
        builder.setPublicVersion(publica)
        try {
            ctx.getSystemService(NotificationManager::class.java).notify(n.wind.hashCode(), builder.build())
        } catch (_: Exception) {
        }
    }

    internal fun canNotify(ctx: Context): Boolean =
        Build.VERSION.SDK_INT < 33 ||
            ctx.checkSelfPermission(android.Manifest.permission.POST_NOTIFICATIONS) ==
            PackageManager.PERMISSION_GRANTED

    private fun open(ctx: Context, windId: String): PendingIntent = PendingIntent.getActivity(
        ctx, windId.hashCode(),
        Intent(ctx, MainActivity::class.java).apply {
            flags = Intent.FLAG_ACTIVITY_SINGLE_TOP or Intent.FLAG_ACTIVITY_CLEAR_TOP
            if (windId.isNotEmpty()) putExtra(EXTRA_WIND, windId)
        },
        PendingIntent.FLAG_UPDATE_CURRENT or PendingIntent.FLAG_IMMUTABLE,
    )
}
