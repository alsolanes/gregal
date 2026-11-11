package xyz.solanes.gregal

import android.app.PendingIntent
import android.content.Context
import android.content.Intent
import android.net.Uri
import java.net.URI
import java.security.MessageDigest
import java.util.Locale

/** Safe to retain in notification metadata. Never contains an address, account or credential. */
data class NotificationOrigin(val id: String, val local: Boolean)

fun Conn.notificationOrigin(): NotificationOrigin = NotificationOrigins.of(this)

/** Resolves only the two pairings already saved by the app; this is NOT a credential registry. */
object NotificationOrigins {
    private fun hash(value: String): String = MessageDigest.getInstance("SHA-256")
        .digest(value.toByteArray(Charsets.UTF_8)).joinToString("") { "%02x".format(it) }

    fun local(): NotificationOrigin = NotificationOrigin("local-" + hash("gregal-notification-local-v1"), true)

    fun of(conn: Conn): NotificationOrigin {
        if (conn.local) return local() // Embedded port/token can change without changing the store.
        val endpoint = try {
            val uri = URI(baseUrlOf(conn.host, conn.port)).normalize()
            val scheme = uri.scheme.lowercase(Locale.ROOT)
            val port = uri.port.takeUnless { it == -1 || (scheme == "https" && it == 443) || (scheme == "http" && it == 80) }
            URI(scheme, uri.userInfo, uri.host?.lowercase(Locale.ROOT), port ?: -1,
                uri.path.orEmpty().trimEnd('/'), uri.query, null).toASCIIString()
        } catch (_: Exception) { baseUrlOf(conn.host, conn.port) }
        // Length framing prevents delimiter ambiguity. The only output is the full digest.
        val fields = listOf("gregal-notification-remote-v1", endpoint, conn.user, hash(conn.token))
        return NotificationOrigin("remote-" + hash(fields.joinToString("") { "${it.length}:$it" }), false)
    }

    fun remote(prefs: Prefs): NotificationOrigin? =
        if (prefs.host.isBlank() || prefs.token.isBlank()) null
        else of(Conn(prefs.host, prefs.port, prefs.token, prefs.usuari))

    fun current(prefs: Prefs): NotificationOrigin? =
        if (prefs.target == Prefs.TARGET_LOCAL) local() else remote(prefs)

    fun resolve(prefs: Prefs, originId: String): Conn? = when {
        originId == local().id -> Conn("127.0.0.1", prefs.localPort.toString(), prefs.localToken, local = true)
        originId.isNotBlank() && originId == remote(prefs)?.id -> Conn(prefs.host, prefs.port, prefs.token, prefs.usuari)
        else -> null
    }

    internal fun effective(ctx: Context, origin: String): String =
        origin.ifBlank { current(Prefs(ctx))?.id.orEmpty() }

    internal fun label(ctx: Context, origin: String): String = when {
        origin == local().id -> "Local"
        origin.startsWith("remote-") -> if (Prefs(ctx).idioma == "ca") "Servidor" else "Server"
        else -> "Gregal"
    }

    fun unavailableMessage(lang: String): String = if (lang == "ca")
        "No es pot identificar l’origen d’aquest avís o la connexió ja no està desada. Obre les converses i tria Local o Servidor; torna a vincular el servidor si cal."
    else "This notification’s source is unknown or its connection is no longer saved. Open conversations and choose Local or Server; pair the server again if needed."
}

data class NotificationDestination(val originId: String, val target: String, val messageId: String = "")

/**
 * El missatge exacte que ha d'obrir una notificació tocada (B7). La
 * notificació porta `message_id`; en obrir-la, el fil corresponent hi fa
 * scroll quan el missatge ja és carregat i després l'oblida. Caduca sola
 * perquè un avís d'un origen que no s'ha pogut obrir no salti més tard.
 */
data class FocusRequest(val target: String, val messageId: String, val at: Long)

object MessageFocusLogic {
    const val TTL_MS = 2 * 60_000L

    /** Índex del missatge a [ids] si la petició és d'aquest fil i encara és viva; si no, null. */
    fun indexFor(req: FocusRequest?, target: String, ids: List<String>, now: Long): Int? {
        if (req == null || req.target != target || req.messageId.isBlank()) return null
        if (now - req.at > TTL_MS) return null
        return ids.indexOf(req.messageId).takeIf { it >= 0 }
    }

    /** La petició ja no serveix (caducada) i s'ha d'esborrar. */
    fun expired(req: FocusRequest?, now: Long): Boolean = req != null && now - req.at > TTL_MS
}

object MessageFocus {
    val request = kotlinx.coroutines.flow.MutableStateFlow<FocusRequest?>(null)

    fun open(target: String, messageId: String, now: Long = System.currentTimeMillis()) {
        request.value = if (messageId.isBlank()) null else FocusRequest(target, messageId, now)
    }

    /** El fil ja hi ha fet scroll: no ho tornis a fer. */
    fun done(req: FocusRequest) { request.compareAndSet(req, null) }
}

/** Same identity rules for replies, proactive voices and scheduled jobs. */
object NotificationNavigation {
    const val EXTRA_ORIGIN = "xyz.solanes.gregal.NOTIFICATION_ORIGIN"
    const val EXTRA_TARGET = "xyz.solanes.gregal.NOTIFICATION_TARGET"
    /** Missatge del fil al qual porta l'avís (message_id del contracte). */
    const val EXTRA_MESSAGE = "xyz.solanes.gregal.NOTIFICATION_MESSAGE"

    fun identity(origin: String, target: String): String = "${origin.length}:$origin:$target"

    fun uri(origin: String, target: String): Uri = Uri.Builder().scheme("gregal").authority("notification")
        .appendPath(origin.ifBlank { "unknown" }).appendPath(target).build()

    fun pending(ctx: Context, origin: String, target: String, messageId: String = ""): PendingIntent = PendingIntent.getActivity(
        ctx, identity(origin, target).hashCode(), Intent(ctx, MainActivity::class.java).apply {
            flags = Intent.FLAG_ACTIVITY_SINGLE_TOP or Intent.FLAG_ACTIVITY_CLEAR_TOP
            data = uri(origin, target)
            putExtra(EXTRA_ORIGIN, origin)
            putExtra(EXTRA_TARGET, target)
            if (messageId.isNotBlank()) putExtra(EXTRA_MESSAGE, messageId)
            // Keep old clients/tests compatible; new dispatch always consumes the source-bound target first.
            when {
                target.startsWith("wind:") -> putExtra(WindNotifications.EXTRA_WIND, target.removePrefix("wind:"))
                target.startsWith("rot:") -> putExtra(WindNotifications.EXTRA_ROT, target.removePrefix("rot:"))
                target.startsWith("run:") -> putExtra(JobAlarms.EXTRA_RUN, target.removePrefix("run:"))
                target.startsWith("screen:") -> putExtra(GregalNotifications.EXTRA_SCREEN, target.removePrefix("screen:"))
            }
        }, PendingIntent.FLAG_UPDATE_CURRENT or PendingIntent.FLAG_IMMUTABLE,
    )

    fun consume(intent: Intent?): NotificationDestination? {
        if (intent == null) return null
        val target = intent.getStringExtra(EXTRA_TARGET)?.takeIf { it.isNotBlank() }
            ?: intent.getStringExtra(WindNotifications.EXTRA_ROT)?.takeIf { it.isNotBlank() }?.let { "rot:$it" }
            ?: intent.getStringExtra(WindNotifications.EXTRA_WIND)?.takeIf { it.isNotBlank() }?.let { "wind:$it" }
            ?: intent.getStringExtra(JobAlarms.EXTRA_RUN)?.takeIf { it.isNotBlank() }?.let { "run:$it" }
            ?: intent.getStringExtra(GregalNotifications.EXTRA_SCREEN)?.takeIf { it.isNotBlank() }?.let { "screen:$it" }
            ?: return null
        val origin = intent.getStringExtra(EXTRA_ORIGIN).orEmpty()
        val message = intent.getStringExtra(EXTRA_MESSAGE).orEmpty()
        listOf(EXTRA_TARGET, EXTRA_ORIGIN, EXTRA_MESSAGE, WindNotifications.EXTRA_WIND, WindNotifications.EXTRA_ROT,
            JobAlarms.EXTRA_RUN, GregalNotifications.EXTRA_SCREEN).forEach(intent::removeExtra)
        // El fil que s'obrirà hi farà scroll (FocusMessageEffect).
        MessageFocus.open(target, message)
        return NotificationDestination(origin, target, message)
    }
}
