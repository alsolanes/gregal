package xyz.solanes.gregal

import android.app.AlarmManager
import android.app.Notification
import android.app.NotificationChannel
import android.app.NotificationManager
import android.app.PendingIntent
import android.app.Service
import android.content.BroadcastReceiver
import android.content.Context
import android.content.Intent
import android.content.pm.PackageManager
import android.content.pm.ServiceInfo
import android.net.Uri
import android.os.Build
import android.os.IBinder
import android.os.PowerManager
import android.provider.Settings
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.cancel
import kotlinx.coroutines.delay
import kotlinx.coroutines.launch
import java.time.OffsetDateTime

/*
 * Jobs programats amb l'app tancada. El backend només viu mentre l'app és
 * viva, així que a Android el programador és el sistema: una alarma exacta
 * (setExactAndAllowWhileIdle, que també sona en Doze) per a la franja més
 * propera de tots els jobs actius. Quan sona, JobRunService engega el
 * backend si cal, dispara els jobs que toquen per /api/jobs/run, avisa amb
 * el resultat i torna a programar l'alarma. El backend corre amb
 * GREGAL_JOBS_SCHEDULER=external: el seu jobLoop no competeix.
 *
 * La llista de franges es desa a les prefs perquè després d'un reinici es
 * pugui tornar a programar sense engegar el backend.
 */

/** Una franja programada: el job i quan toca (ms d'època). */
data class ScheduledSlot(val id: String, val name: String, val dueAt: Long)

/** Lògica pura de la programació: testejable a la JVM. */
object JobSchedule {
    /** La mateixa gràcia que jobs.Due al backend. */
    const val GRACE_MS = 60_000L
    /** Una alarma al passat (franja perduda) sona d'aquí a poc, no ara mateix. */
    const val MIN_DELAY_MS = 2_000L

    fun parseInstant(s: String): Long? =
        if (s.isBlank()) null
        else try { OffsetDateTime.parse(s.trim()).toInstant().toEpochMilli() } catch (_: Exception) { null }

    private fun scheduled(j: JobItem) = j.enabled && (j.kind == "daily" || j.kind == "interval" || j.kind == "event" || j.kind == "remind")

    /** Franges dels jobs actius, la més propera primer. */
    fun slots(jobs: List<JobItem>): List<ScheduledSlot> = jobs
        .filter(::scheduled)
        .mapNotNull { j -> parseInstant(j.nextDueAt)?.let { ScheduledSlot(j.id, j.name, it) } }
        .sortedBy { it.dueAt }

    /** Jobs que toquen ara segons el backend, el més endarrerit primer. */
    fun due(jobs: List<JobItem>, now: Long): List<JobItem> = jobs
        .filter(::scheduled)
        .mapNotNull { j -> parseInstant(j.nextDueAt)?.let { j to it } }
        .filter { (_, at) -> at <= now + GRACE_MS }
        .sortedBy { it.second }
        .map { it.first }

    /** Quan ha de sonar l'alarma (null = cap job programat). */
    fun nextAlarm(slots: List<ScheduledSlot>, now: Long): Long? =
        slots.minOfOrNull { it.dueAt }?.let { maxOf(it, now + MIN_DELAY_MS) }

    /** L'execució ha acabat quan el backend n'ha apuntat una de nova. */
    fun finished(before: JobItem, after: JobItem?): Boolean =
        after != null && after.lastAt.isNotBlank() && after.lastAt != before.lastAt

    /** Decisió del poll d'un tret: què fa amb l'estat fresc del job. */
    enum class PollDecision { FINISHED, GONE, WAIT }

    /**
     * L'ordre importa: un avís d'un sol tret s'apaga en executar-se
     * (enabled=false AMB lastAt nou) i això ÉS èxit, no abandonament.
     * Només és GONE si s'apaga sense haver-se executat (o s'esborra).
     */
    fun pollDecision(before: JobItem, now: JobItem?): PollDecision =
        when {
            now == null -> PollDecision.GONE
            finished(before, now) -> PollDecision.FINISHED
            !now.enabled -> PollDecision.GONE
            else -> PollDecision.WAIT
        }

    private fun clean(s: String) = s.replace('\t', ' ').replace('\n', ' ').replace('\r', ' ')

    /** Format de les prefs: una franja per línia, «id\tms\tnom». */
    fun encode(slots: List<ScheduledSlot>): String =
        slots.joinToString("\n") { "${clean(it.id)}\t${it.dueAt}\t${clean(it.name)}" }

    fun decode(s: String): List<ScheduledSlot> = s.lineSequence().mapNotNull { line ->
        val p = line.split('\t', limit = 3)
        val at = p.getOrNull(1)?.toLongOrNull()
        if (p.size < 2 || p[0].isBlank() || at == null) null else ScheduledSlot(p[0], p.getOrElse(2) { "" }, at)
    }.toList()

    /** Text pla per a la notificació: línies sense markdown, sense taules buides. */
    fun excerpt(md: String, max: Int = 600): String {
        val lines = md.lineSequence()
            .map { it.trim() }
            .filter { it.isNotEmpty() && !it.matches(Regex("^[|:\\-\\s]+$")) }
            .map { stripMdForTitle(it.trim('|').replace(" | ", " · "), max) }
            .filter { it.isNotEmpty() }
            .toList()
        val out = StringBuilder()
        for (l in lines) {
            if (out.isNotEmpty()) out.append('\n')
            out.append(l)
            if (out.length >= max) break
        }
        return if (out.length > max) out.take(max - 1).toString() + "…" else out.toString()
    }
}

/** Plantilles per crear jobs des del mòbil amb dos tocs. */
fun jobTemplates(lang: String): List<JobItem> = listOf(
    JobItem(name = tr(lang, "app.jobs.tpl.techName"), prompt = tr(lang, "app.jobs.tpl.techPrompt"),
        kind = "daily", time = "08:00"),
    JobItem(name = tr(lang, "app.jobs.tpl.worldName"), prompt = tr(lang, "app.jobs.tpl.worldPrompt"),
        kind = "daily", time = "07:30"),
    JobItem(name = tr(lang, "app.jobs.tpl.watchName"), prompt = tr(lang, "app.jobs.tpl.watchPrompt"),
        kind = "interval", intervalH = 12.0),
)

/** Què pot frenar els jobs en segon pla en aquest mòbil (per a la targeta de la pantalla). */
data class BackgroundHealth(
    val exactAlarms: Boolean,
    val notifications: Boolean,
    val unrestrictedBattery: Boolean,
)

object JobAlarms {
    const val ACTION_FIRE = "xyz.solanes.gregal.JOB_ALARM"
    const val EXTRA_RUN = "gregal.openRun"
    private const val PREFS = "gregal_jobs"
    private const val KEY_SLOTS = "slots"

    private fun prefs(ctx: Context) = ctx.getSharedPreferences(PREFS, Context.MODE_PRIVATE)

    fun cached(ctx: Context): List<ScheduledSlot> =
        JobSchedule.decode(prefs(ctx).getString(KEY_SLOTS, "") ?: "")

    /** Desa la programació que diu el backend i torna a posar l'alarma. */
    fun sync(ctx: Context, jobs: List<JobItem>) {
        prefs(ctx).edit().putString(KEY_SLOTS, JobSchedule.encode(JobSchedule.slots(jobs))).apply()
        schedule(ctx)
    }

    /** Servidor remot: allà els jobs els programa el mateix servidor. */
    fun clear(ctx: Context) {
        prefs(ctx).edit().remove(KEY_SLOTS).apply()
        alarmManager(ctx).cancel(firePending(ctx))
    }

    /** Programa l'alarma per a la franja més propera de les desades. */
    fun schedule(ctx: Context) {
        val am = alarmManager(ctx)
        val pi = firePending(ctx)
        val at = JobSchedule.nextAlarm(cached(ctx), System.currentTimeMillis())
        if (at == null) {
            am.cancel(pi)
            return
        }
        try {
            if (canExact(ctx)) am.setExactAndAllowWhileIdle(AlarmManager.RTC_WAKEUP, at, pi)
            else am.setAndAllowWhileIdle(AlarmManager.RTC_WAKEUP, at, pi)
        } catch (_: SecurityException) {
            // Permís retirat entre la comprovació i la crida.
            am.setAndAllowWhileIdle(AlarmManager.RTC_WAKEUP, at, pi)
        }
    }

    /** Una comprovació d'aquí a poc (canvi d'hora o de fus horari). */
    fun checkSoon(ctx: Context) {
        val at = System.currentTimeMillis() + JobSchedule.MIN_DELAY_MS * 5
        val am = alarmManager(ctx)
        try {
            if (canExact(ctx)) am.setExactAndAllowWhileIdle(AlarmManager.RTC_WAKEUP, at, firePending(ctx))
            else am.setAndAllowWhileIdle(AlarmManager.RTC_WAKEUP, at, firePending(ctx))
        } catch (_: SecurityException) {
        }
    }

    fun canExact(ctx: Context): Boolean =
        Build.VERSION.SDK_INT < Build.VERSION_CODES.S || alarmManager(ctx).canScheduleExactAlarms()

    fun health(ctx: Context): BackgroundHealth = BackgroundHealth(
        exactAlarms = canExact(ctx),
        notifications = Build.VERSION.SDK_INT < 33 ||
            ctx.checkSelfPermission(android.Manifest.permission.POST_NOTIFICATIONS) == PackageManager.PERMISSION_GRANTED,
        unrestrictedBattery = ctx.getSystemService(PowerManager::class.java).isIgnoringBatteryOptimizations(ctx.packageName),
    )

    /** Pantalla del sistema per permetre alarmes exactes (Android 12+). */
    fun exactAlarmSettings(ctx: Context): Intent =
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.S)
            Intent(Settings.ACTION_REQUEST_SCHEDULE_EXACT_ALARM, Uri.parse("package:${ctx.packageName}"))
        else appSettings(ctx)

    fun notificationSettings(ctx: Context): Intent =
        Intent(Settings.ACTION_APP_NOTIFICATION_SETTINGS).putExtra(Settings.EXTRA_APP_PACKAGE, ctx.packageName)

    /** Diàleg del sistema «deixa que funcioni sense restriccions». */
    @android.annotation.SuppressLint("BatteryLife")
    fun batterySettings(ctx: Context): Intent =
        Intent(Settings.ACTION_REQUEST_IGNORE_BATTERY_OPTIMIZATIONS, Uri.parse("package:${ctx.packageName}"))

    fun appSettings(ctx: Context): Intent =
        Intent(Settings.ACTION_APPLICATION_DETAILS_SETTINGS, Uri.parse("package:${ctx.packageName}"))

    private fun alarmManager(ctx: Context) = ctx.getSystemService(AlarmManager::class.java)

    private fun firePending(ctx: Context): PendingIntent = PendingIntent.getBroadcast(
        ctx, 0,
        Intent(ctx, JobAlarmReceiver::class.java).setAction(ACTION_FIRE),
        PendingIntent.FLAG_UPDATE_CURRENT or PendingIntent.FLAG_IMMUTABLE,
    )
}

/** L'alarma ha sonat: el servei fa la feina en primer pla. */
class JobAlarmReceiver : BroadcastReceiver() {
    override fun onReceive(context: Context, intent: Intent) {
        if (intent.action != JobAlarms.ACTION_FIRE) return
        JobRunService.start(context)
    }
}

/**
 * Reinici, actualització de l'app o canvi d'hora: les alarmes no
 * sobreviuen a un reinici, i un canvi de fus mou les franges diàries.
 */
class JobBootReceiver : BroadcastReceiver() {
    override fun onReceive(context: Context, intent: Intent) {
        when (intent.action) {
            Intent.ACTION_BOOT_COMPLETED, Intent.ACTION_MY_PACKAGE_REPLACED,
            AlarmManager.ACTION_SCHEDULE_EXACT_ALARM_PERMISSION_STATE_CHANGED -> JobAlarms.schedule(context)
            Intent.ACTION_TIME_CHANGED, Intent.ACTION_TIMEZONE_CHANGED ->
                if (JobAlarms.cached(context).isNotEmpty()) JobAlarms.checkSoon(context)
        }
    }
}

/**
 * Executa els jobs que toquen. Primer pla (dataSync) i wake lock només
 * mentre corren: amb la pantalla apagada el procés i el backend no es
 * congelen a mig butlletí. En acabar, si el backend l'havíem engegat
 * nosaltres i l'app no és a la pantalla, s'atura.
 */
class JobRunService : Service() {
    private val scope = CoroutineScope(SupervisorJob() + Dispatchers.IO)
    @Volatile private var running = false

    override fun onStartCommand(intent: Intent?, flags: Int, startId: Int): Int {
        try {
            JobNotifications.ensureChannel(this)
            val n = JobNotifications.running(this)
            if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.Q) {
                startForeground(JobNotifications.RUNNING_ID, n, ServiceInfo.FOREGROUND_SERVICE_TYPE_DATA_SYNC)
            } else {
                startForeground(JobNotifications.RUNNING_ID, n)
            }
        } catch (_: Exception) {
            stopSelf()
            return START_NOT_STICKY
        }
        if (!running) {
            running = true
            scope.launch {
                try { runDue() } finally { finish() }
            }
        }
        return START_NOT_STICKY
    }

    private suspend fun runDue() {
        val prefs = Prefs(applicationContext)
        if (prefs.target != Prefs.TARGET_LOCAL || !LocalServer.available(this)) {
            JobAlarms.clear(this)
            return
        }
        val wake = getSystemService(PowerManager::class.java)
            .newWakeLock(PowerManager.PARTIAL_WAKE_LOCK, "gregal:jobs")
        wake.acquire(JOB_WAIT_MS + 5 * 60_000L)
        val startedHere = !LocalServer.alive()
        try {
            val srv = try {
                LocalServer.ensure(applicationContext, prefs.idioma)
            } catch (e: Exception) {
                JobNotifications.failed(this, "", e.message.orEmpty())
                JobAlarms.schedule(this)
                return
            }
            val api = GregalApi("http://127.0.0.1:${srv.port}", srv.token)
            var jobs = api.getJobs()
            // Un per un, com el jobLoop: el model del mòbil no fa dos butlletins alhora.
            for (job in JobSchedule.due(jobs, System.currentTimeMillis())) {
                val result = runOne(api, job)
                if (result != null) JobNotifications.result(this, job, result)
            }
            jobs = api.getJobs()
            JobAlarms.sync(this, jobs)
        } catch (_: Exception) {
            // Sense backend no sabem la nova franja: tornem a provar la desada.
            JobAlarms.schedule(this)
        } finally {
            if (startedHere && !AppVisibility.foreground) {
                try { LocalServer.stop() } catch (_: Exception) {}
            }
            if (wake.isHeld) wake.release()
        }
    }

    /** Dispara el job i espera que el backend n'apunti l'execució. */
    private suspend fun runOne(api: GregalApi, job: JobItem): RunItem? {
        val deadline = System.currentTimeMillis() + JOB_WAIT_MS
        var accepted = false
        while (System.currentTimeMillis() < deadline) {
            if (!accepted) accepted = try { api.runScheduledJob(job.id) } catch (_: Exception) { false }
            delay(if (accepted) 3_000 else 15_000)
            val now = try { api.getJobs().firstOrNull { it.id == job.id } } catch (_: Exception) { continue }
            when (JobSchedule.pollDecision(job, now)) {
                JobSchedule.PollDecision.FINISHED ->
                    return try { api.getRuns(job.id, 1).firstOrNull() } catch (_: Exception) { null }
                JobSchedule.PollDecision.GONE -> return null
                JobSchedule.PollDecision.WAIT -> { /* segueix esperant el tret */ }
            }
        }
        return null
    }

    private fun finish() {
        running = false
        try { stopForeground(STOP_FOREGROUND_REMOVE) } catch (_: Exception) {}
        stopSelf()
    }

    override fun onDestroy() {
        scope.cancel()
        super.onDestroy()
    }

    override fun onBind(intent: Intent?): IBinder? = null

    companion object {
        /** El backend talla un job als 15 min; deixem marge per a la cua. */
        const val JOB_WAIT_MS = 25 * 60_000L

        fun start(ctx: Context) {
            val i = Intent(ctx, JobRunService::class.java)
            try {
                if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) ctx.startForegroundService(i) else ctx.startService(i)
            } catch (_: Exception) {
                // API 31+ sense alarma exacta: el sistema no deixa engegar el
                // servei en segon pla. Ho diem; en obrir l'app es torna a provar.
                JobNotifications.blocked(ctx)
            }
        }
    }
}

/** Avisos dels jobs: un canal propi perquè es puguin silenciar a part del xat. */
object JobNotifications {
    const val CHANNEL_ID = "gregal_jobs"
    const val RUNNING_ID = 441
    private const val BLOCKED_ID = 442

    private fun text(ctx: Context, key: String, vararg a: Any): String = tr(Prefs(ctx).idioma, key, *a)

    fun ensureChannel(ctx: Context) {
        if (Build.VERSION.SDK_INT < Build.VERSION_CODES.O) return
        val ch = NotificationChannel(CHANNEL_ID, text(ctx, "app.jobs.notif.channel"), NotificationManager.IMPORTANCE_DEFAULT)
            .apply { description = text(ctx, "app.jobs.notif.channelDescription") }
        ctx.getSystemService(NotificationManager::class.java).createNotificationChannel(ch)
    }

    internal fun running(ctx: Context): Notification = Notification.Builder(ctx, CHANNEL_ID)
        .setSmallIcon(R.drawable.ic_stat_gregal)
        .setContentTitle(text(ctx, "app.jobs.notif.running"))
        .setUsesChronometer(true)
        .setOngoing(true)
        .setContentIntent(open(ctx, ""))
        .setVisibility(Notification.VISIBILITY_PRIVATE)
        .build()

    /** El butlletí: títol = nom del job, cos = el resum; tocar-lo obre l'execució. */
    fun result(ctx: Context, job: JobItem, run: RunItem) {
        val ok = run.status == "ok"
        val title = if (ok) stripMdForTitle(job.name) else text(ctx, "app.jobs.notif.failed", stripMdForTitle(job.name))
        val body = if (ok) JobSchedule.excerpt(run.output) else run.error.take(300)
        post(ctx, job.id.hashCode(), title, body, run.id)
    }

    fun failed(ctx: Context, name: String, why: String) =
        post(ctx, BLOCKED_ID, text(ctx, "app.jobs.notif.failed", name.ifEmpty { "Gregal" }), why.take(300), "")

    /** Prova de so: notificació instantània per comprovar permís i canal. */
    fun test(ctx: Context) =
        post(ctx, 0x47524547, text(ctx, "app.vents.testNotifTitle"), text(ctx, "app.vents.testNotifBody"), "")

    fun blocked(ctx: Context) =
        post(ctx, BLOCKED_ID, text(ctx, "app.jobs.notif.blockedTitle"), text(ctx, "app.jobs.notif.blockedBody"), "")

    private fun post(ctx: Context, id: Int, title: String, body: String, runId: String) {
        if (Build.VERSION.SDK_INT >= 33 &&
            ctx.checkSelfPermission(android.Manifest.permission.POST_NOTIFICATIONS) != PackageManager.PERMISSION_GRANTED) return
        ensureChannel(ctx)
        // A la pantalla bloquejada, només que hi ha un resultat: el contingut
        // del butlletí es veu en desbloquejar.
        val public = Notification.Builder(ctx, CHANNEL_ID)
            .setSmallIcon(R.drawable.ic_stat_gregal)
            .setContentTitle(text(ctx, "app.jobs.notif.publicTitle"))
            .build()
        val n = Notification.Builder(ctx, CHANNEL_ID)
            .setSmallIcon(R.drawable.ic_stat_gregal)
            .setContentTitle(title)
            .setContentText(body.lineSequence().firstOrNull().orEmpty())
            .setStyle(Notification.BigTextStyle().bigText(body))
            .setContentIntent(open(ctx, runId))
            .setAutoCancel(true)
            .setVisibility(Notification.VISIBILITY_PRIVATE)
            .setPublicVersion(public)
            .build()
        try {
            ctx.getSystemService(NotificationManager::class.java).notify(id, n)
        } catch (_: Exception) {
        }
    }

    private fun open(ctx: Context, runId: String): PendingIntent = PendingIntent.getActivity(
        ctx, runId.hashCode(),
        Intent(ctx, MainActivity::class.java).apply {
            flags = Intent.FLAG_ACTIVITY_SINGLE_TOP or Intent.FLAG_ACTIVITY_CLEAR_TOP
            if (runId.isNotEmpty()) putExtra(JobAlarms.EXTRA_RUN, runId)
        },
        PendingIntent.FLAG_UPDATE_CURRENT or PendingIntent.FLAG_IMMUTABLE,
    )
}
