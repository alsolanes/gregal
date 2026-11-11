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

    /**
     * Les que fan sonar el mòbil: les tranquil·les (hores tranquil·les o mode
     * tranquil) esperen a la safata «Avui». El cursor avança igualment amb totes.
     */
    fun toRing(novetats: List<WindNovetat>): List<WindNovetat> = novetats.filterNot { it.quiet }
}

/**
 * Deduplicació dels avisos (B7): la mateixa novetat pot arribar pel push i
 * pel sondeig, o tornar a sortir després d'un reinici. Es recorda per id
 * opac (i per message_id, que és el mateix missatge) uns dies.
 */
object NoticeDedupe {
    const val KEEP_MS = 7 * 24 * 3600_000L

    /** Claus que identifiquen la novetat: l'id opac i el missatge; sense cap (servidor antic), ts + vent. */
    fun keys(n: WindNovetat): List<String> = buildList {
        if (n.id.isNotBlank()) add(n.id)
        if (n.messageId.isNotBlank()) add("m:" + n.messageId)
        if (isEmpty()) add("t:${n.ts}:${n.wind}")
    }

    /** D'aquest compte? Servidor sense comptes o novetat antiga sense owner: sí. */
    fun ownedBy(n: WindNovetat, me: String): Boolean =
        n.owner.isBlank() || me.isBlank() || n.owner.equals(me, ignoreCase = true)

    /** Les que encara es poden anunciar: no llegides, no anunciades i del compte. */
    fun announceable(list: List<WindNovetat>, announced: Map<String, Long>, me: String): List<WindNovetat> =
        list.filter { n -> !n.read && ownedBy(n, me) && keys(n).none { it in announced } }

    /** Afegeix les anunciades i oblida les velles. */
    fun record(announced: Map<String, Long>, items: List<WindNovetat>, now: Long): Map<String, Long> =
        (announced + items.flatMap(::keys).associateWith { now }).filterValues { now - it < KEEP_MS }

    /** Vents amb l'última novetat ja llegida (en un altre lloc): el seu avís sobra. */
    fun readWinds(list: List<WindNovetat>): Set<String> =
        list.groupBy { it.wind }.mapNotNull { (w, l) -> w.takeIf { l.maxByOrNull { it.ts }?.read == true } }.toSet()
}

/** On es recorda què s'ha anunciat, per origen (servidor i compte). */
object NoticeLedger {
    private const val PREFS = "gregal_notice_ledger"

    private fun prefs(ctx: Context) = ctx.getSharedPreferences(PREFS, Context.MODE_PRIVATE)

    @Synchronized
    fun load(ctx: Context, origin: String): Map<String, Long> = try {
        val o = org.json.JSONObject(prefs(ctx).getString(origin, "{}") ?: "{}")
        o.keys().asSequence().associateWith { o.optLong(it) }
    } catch (_: Exception) { emptyMap() }

    @Synchronized
    fun save(ctx: Context, origin: String, map: Map<String, Long>) {
        val o = org.json.JSONObject()
        map.forEach { (k, v) -> o.put(k, v) }
        prefs(ctx).edit().putString(origin, o.toString()).apply()
    }

    /** Apunta ids ja vistos dins l'app (stream, push amb l'app oberta): no han de sonar després. */
    @Synchronized
    fun mark(ctx: Context, origin: String, ids: List<String>, now: Long = System.currentTimeMillis()) {
        if (ids.isEmpty()) return
        save(ctx, origin, (load(ctx, origin) + ids.filter { it.isNotBlank() }.associateWith { now })
            .filterValues { now - it < NoticeDedupe.KEEP_MS })
    }

    /** Apunta [items] com a anunciades i torna les que eren noves (atòmic dins el procés). */
    @Synchronized
    fun claim(ctx: Context, origin: String, items: List<WindNovetat>, me: String, now: Long = System.currentTimeMillis()): List<WindNovetat> {
        val before = load(ctx, origin)
        val fresh = NoticeDedupe.announceable(items, before, me)
        save(ctx, origin, NoticeDedupe.record(before, items.filter { NoticeDedupe.ownedBy(it, me) }, now))
        return fresh
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

    /**
     * Un vent acaba de programar recordatoris (l'usuari li ho ha demanat):
     * és el moment de demanar el permís d'avisos, una sola vegada
     * (NotifPermission). Al servidor remot, a més, la passada s'avança.
     */
    fun reminded(ctx: Context, times: List<Long>, remote: Boolean) {
        if (times.isEmpty()) return
        NotifPermission.requestFor(ctx, NotifAskReason.REMINDER)
        if (remote) expect(ctx, times)
    }

    /** Amb push actiu el sondeig només reconcilia: cada hora n'hi ha prou. */
    const val PUSH_INTERVAL_MS = 60 * 60_000L

    /** Quan toca la passada: el primer avís pendent si arriba abans que l'interval. */
    fun nextCheckDelay(delayMs: Long, due: List<Long>, now: Long): Long {
        val first = due.firstOrNull { it + DUE_GRACE_MS > now } ?: return delayMs
        return minOf(delayMs, maxOf(5_000L, first + DUE_GRACE_MS - now))
    }

    /** Programa la propera passada (exacta si es pot, si no inexacta). */
    fun schedule(ctx: Context, delayMs: Long = INTERVAL_MS) {
        if (!wanted(ctx)) return
        // Registra (o refresca) el token de push si n'hi ha; si no, sondeig.
        VentsPush.ensureRegistered(ctx)
        val am = ctx.getSystemService(AlarmManager::class.java)
        val now = System.currentTimeMillis()
        val every = if (delayMs == INTERVAL_MS && VentsPush.active(ctx)) PUSH_INTERVAL_MS else delayMs
        val at = now + nextCheckDelay(every, dueTimes(ctx, now), now)
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
                val source = NotificationOrigins.remote(p)?.id.orEmpty()
                // Llegides en un altre lloc (fil obert, web, «Avui»): l'avís d'aquí sobra.
                for (w in NoticeDedupe.readWinds(noves)) WindNotifications.cancelWind(this, source, w)
                val candidates = WindWatchLogic.toRing(OfflineLogic.notYetShown(noves, shown)).filter { it.ts > desDe }
                WindWatch.saveTs(this, nouTs)
                // Primera passada en aquest servidor/compte: només marca on som.
                // Avisar de tot el que ja hi havia seria una ràfega de notificacions velles.
                if (desDe == 0L) { NoticeLedger.claim(this, source, candidates, p.usuari); return }
                aviat = try { api.windTasks().any { it.running } } catch (_: Exception) { false }
                // Un sol registre per a push i sondeig: el que ja ha sonat no torna a sonar.
                // Amb l'app oberta tampoc no sona: la novetat ja és al fil (stream).
                val fresh = NoticeLedger.claim(this, source, candidates, p.usuari)
                val perVent = WindWatchLogic.noves(fresh, desDe).first
                if (perVent.isEmpty() || AppVisibility.foreground) return
                if (!WindNotifications.canNotify(this)) return
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
            // Obre el missatge exacte del fil (message_id), no només el xat.
            .setContentIntent(NotificationNavigation.pending(ctx, source, target, n.messageId))
            .setAutoCancel(true)
            .setVisibility(Notification.VISIBILITY_PRIVATE)
        // Glif, color i insígnia segons el que diu el vent (resum del matí, recordatori, temps…).
        val kind = NotifLook.apply(ctx, builder, NotifLook.of(n.source, "", n.text, hour = java.time.LocalTime.now().hour))
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.P) {
            val vent = Person.Builder().setName(nom)
                .apply { NotifLook.badge(ctx, kind)?.let { setIcon(android.graphics.drawable.Icon.createWithBitmap(it)) } }
                .build()
            builder.style = Notification.MessagingStyle(vent)
                .addMessage(cos, System.currentTimeMillis(), vent)
        }
        val publica = Notification.Builder(ctx, CHANNEL_ID)
            .setSmallIcon(R.drawable.ic_stat_gregal)
            .setContentTitle("Gregal")
            .build()
        builder.setPublicVersion(publica)
        val notifId = NotificationNavigation.identity(source, target).hashCode()
        // Una targeta d'«Avui»: es pot decidir des de l'avís mateix (Fet / Més tard).
        if (n.card.isNotBlank()) {
            builder.addAction(Notification.Action.Builder(null, text(ctx, "app.avui.done"),
                AvuiActionReceiver.pending(ctx, source, n.card, "done", notifId)).build())
            builder.addAction(Notification.Action.Builder(null, text(ctx, "app.avui.later"),
                AvuiActionReceiver.pending(ctx, source, n.card, "later", notifId)).build())
        }
        try {
            ctx.getSystemService(NotificationManager::class.java).notify(notifId, builder.build())
        } catch (_: Exception) {
        }
    }

    /** Treu l'avís d'un vent (el seu fil ja s'ha llegit, aquí o en un altre dispositiu). */
    fun cancelWind(ctx: Context, origin: String, windId: String) {
        if (windId.isBlank()) return
        val source = NotificationOrigins.effective(ctx, origin)
        try {
            ctx.getSystemService(NotificationManager::class.java)
                .cancel(NotificationNavigation.identity(source, "wind:$windId").hashCode())
        } catch (_: Exception) {}
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

/**
 * Els botons Fet / Més tard d'un avís amb targeta: diuen la decisió al
 * mateix servidor (i compte) que el va fer sonar, sense obrir l'app, i
 * treuen l'avís. Si falla, la targeta continua oberta a la safata.
 */
class AvuiActionReceiver : BroadcastReceiver() {
    override fun onReceive(context: Context, intent: Intent) {
        if (intent.action != ACTION) return
        val card = intent.getStringExtra(EXTRA_CARD).orEmpty()
        val action = intent.getStringExtra(EXTRA_ACTION).orEmpty()
        val origin = intent.getStringExtra(EXTRA_ORIGIN).orEmpty()
        val notifId = intent.getIntExtra(EXTRA_NOTIF, 0)
        val app = context.applicationContext
        if (card.isBlank() || action !in setOf("done", "later")) return
        val pending = goAsync()
        Thread {
            try {
                val conn = NotificationOrigins.resolve(Prefs(app), origin)
                if (conn != null) conn.api().avuiAction(card, action)
            } catch (_: Throwable) {
                // Sense servidor: la targeta queda oberta a «Avui»; res no peta.
            } finally {
                try { app.getSystemService(NotificationManager::class.java).cancel(notifId) } catch (_: Exception) {}
                try { pending.finish() } catch (_: Exception) {}
            }
        }.start()
    }

    companion object {
        const val ACTION = "xyz.solanes.gregal.AVUI_ACTION"
        const val EXTRA_CARD = "xyz.solanes.gregal.AVUI_CARD"
        const val EXTRA_ACTION = "xyz.solanes.gregal.AVUI_DO"
        const val EXTRA_ORIGIN = "xyz.solanes.gregal.AVUI_ORIGIN"
        const val EXTRA_NOTIF = "xyz.solanes.gregal.AVUI_NOTIF"

        fun pending(ctx: Context, origin: String, card: String, action: String, notifId: Int): PendingIntent =
            PendingIntent.getBroadcast(
                ctx, ("avui:$card:$action").hashCode(),
                Intent(ctx, AvuiActionReceiver::class.java).setAction(ACTION)
                    .putExtra(EXTRA_CARD, card).putExtra(EXTRA_ACTION, action)
                    .putExtra(EXTRA_ORIGIN, origin).putExtra(EXTRA_NOTIF, notifId),
                PendingIntent.FLAG_UPDATE_CURRENT or PendingIntent.FLAG_IMMUTABLE,
            )
    }
}
