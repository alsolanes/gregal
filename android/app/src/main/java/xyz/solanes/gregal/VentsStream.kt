package xyz.solanes.gregal

import android.content.Context
import androidx.compose.foundation.lazy.LazyListState
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.rememberUpdatedState
import androidx.compose.ui.platform.LocalContext
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.channels.Channel
import kotlinx.coroutines.delay
import kotlinx.coroutines.launch
import kotlinx.coroutines.coroutineScope
import kotlinx.coroutines.suspendCancellableCoroutine
import kotlinx.coroutines.withContext
import kotlinx.coroutines.withTimeoutOrNull
import okhttp3.Call
import okhttp3.Callback
import okhttp3.Request
import okhttp3.Response
import org.json.JSONObject
import java.io.IOException
import java.util.concurrent.TimeUnit
import kotlin.coroutines.resume

/*
 * Novetats amb l'app oberta (B7): connexió de primer pla al registre durable
 * d'esdeveniments del servidor (/api/v2/events/stream), el mateix bus que fan
 * servir els torns; cap bus nou. Només els tipus «novetat» i «novetat_read»
 * del compte (?scope=user). El cursor es desa per origen: en tornar a obrir
 * l'app es reprèn on es va deixar, sense perdre ni repetir res.
 *
 * Si el stream falla (servidor antic sense ?scope=user, xarxa…) es torna al
 * sondeig de sempre cada 20 s: el comportament antic, declarat.
 */

/** Un event del registre que interessa a les novetats. */
data class NovetatEvent(
    val cursor: Long,
    val kind: String,
    val ids: List<String>,
    val messageId: String = "",
    val winds: List<String> = emptyList(),
)

object NovetatStreamLogic {
    const val KINDS = "novetat,novetat_read"
    const val POLL_MS = 20_000L

    fun path(after: Long): String = "/api/v2/events/stream?scope=user&kinds=$KINDS&limit=200&after=$after"

    /** Llegeix el bloc «data» d'un event SSE; null si no és de novetats o és il·legible. */
    fun parse(data: String): NovetatEvent? = try {
        val o = JSONObject(data)
        val kind = o.optString("kind")
        if (kind != "novetat" && kind != "novetat_read") null else {
            val p = o.optJSONObject("payload") ?: JSONObject()
            val ids = buildList {
                p.optString("id").takeIf { it.isNotBlank() }?.let(::add)
                p.optJSONArray("ids")?.let { a -> for (i in 0 until a.length()) a.optString(i).takeIf { it.isNotBlank() }?.let(::add) }
            }
            val winds = buildList {
                p.optString("wind").takeIf { it.isNotBlank() }?.let(::add)
                p.optJSONArray("winds")?.let { a -> for (i in 0 until a.length()) a.optString(i).takeIf { it.isNotBlank() }?.let(::add) }
            }.distinct()
            NovetatEvent(o.optLong("id"), kind, ids, p.optString("message_id"), winds)
        }
    } catch (_: Exception) { null }

    /** Espera abans de reconnectar: 1, 2, 4… fins a 30 s. */
    fun backoff(attempt: Int): Long = minOf(30_000L, 1_000L shl minOf(attempt, 5))
}

/** Cursor per origen (servidor + compte). */
object NovetatCursor {
    private const val PREFS = "gregal_novetat_cursor"
    private fun prefs(ctx: Context) = ctx.getSharedPreferences(PREFS, Context.MODE_PRIVATE)
    fun get(ctx: Context, origin: String): Long = prefs(ctx).getLong(origin, 0L)
    fun set(ctx: Context, origin: String, cursor: Long) {
        if (cursor > get(ctx, origin)) prefs(ctx).edit().putLong(origin, cursor).apply()
    }
}

/** Llegit/pendent coherent amb el servidor: el que es llegeix aquí es llegeix a tot arreu. */
object NovetatsRead {
    /** Fil obert: totes les novetats del vent queden llegides i el seu avís fora. */
    fun wind(ctx: Context, conn: Conn, windId: String) {
        if (windId.isBlank()) return
        WindNotifications.cancelWind(ctx, conn.notificationOrigin().id, windId)
        if (conn.local) return
        post(conn, JSONObject().put("wind", windId))
    }

    private fun post(conn: Conn, body: JSONObject) {
        Thread {
            try { conn.api().post("/api/winds/novetats/read", body) } catch (_: Throwable) {
                // Servidor antic o sense xarxa: l'estat local ja s'ha actualitzat.
            }
        }.start()
    }
}

private val streamClient by lazy {
    // El servidor envia un batec cada 15 s: si en passen 45 sense res, la connexió és morta.
    GregalApi.http.newBuilder().readTimeout(45, TimeUnit.SECONDS).build()
}

/**
 * Una connexió fins que es talla. Els events arriben per [out] en ordre.
 * Torna normalment quan el servidor tanca, llança si no es pot connectar.
 */
private suspend fun streamOnce(conn: Conn, after: Long, out: Channel<NovetatEvent>) =
    suspendCancellableCoroutine<Unit> { cont ->
        val b = Request.Builder().url(baseUrlOf(conn.host, conn.port) + NovetatStreamLogic.path(after))
            .header("Accept", "text/event-stream")
        if (conn.token.isNotBlank()) b.header("Authorization", "Bearer ${conn.token}")
        val call = streamClient.newCall(b.build())
        cont.invokeOnCancellation { call.cancel() }
        call.enqueue(object : Callback {
            override fun onFailure(call: Call, e: IOException) {
                if (cont.isActive) cont.resumeWith(Result.failure(e))
            }

            override fun onResponse(call: Call, response: Response) {
                response.use { r ->
                    if (!r.isSuccessful) {
                        if (cont.isActive) cont.resumeWith(Result.failure(IOException("HTTP ${r.code}")))
                        return
                    }
                    try {
                        val src = r.body.source()
                        val data = StringBuilder()
                        while (true) {
                            val line = src.readUtf8Line() ?: break
                            when {
                                line.isEmpty() -> {
                                    if (data.isNotEmpty()) NovetatStreamLogic.parse(data.toString())?.let { out.trySend(it) }
                                    data.setLength(0)
                                }
                                line.startsWith("data:") -> data.append(line.removePrefix("data:").trimStart())
                            }
                        }
                    } catch (_: IOException) {
                        // Tall o cancel·lació: es reconnecta (o s'ha acabat la pantalla).
                    }
                    if (cont.isActive) cont.resume(Unit)
                }
            }
        })
    }

/**
 * Manté les novetats al dia amb la pantalla visible. [onChange] es crida en
 * connectar i quan arriba alguna cosa (agrupat), perquè el ViewModel torni
 * a llegir insígnies i el fil obert. Substitueix el sondeig de 20 s.
 */
@Composable
internal fun NovetatsLive(conn: Conn, onChange: () -> Unit) {
    val ctx = LocalContext.current.applicationContext
    val change by rememberUpdatedState(onChange)
    LifecyclePoll(conn) {
        if (conn.local) {
            // Al mòbil no hi ha vents que parlin sols pel servidor: el de sempre.
            while (true) { change(); delay(NovetatStreamLogic.POLL_MS) }
        }
        val origin = conn.notificationOrigin().id
        var attempt = 0
        coroutineScope {
            val events = Channel<NovetatEvent>(Channel.UNLIMITED)
            launch {
                // Agrupa ràfegues (un replay en porta moltes): una sola recàrrega.
                for (first in events) {
                    val batch = mutableListOf(first)
                    withTimeoutOrNull(300) { for (e in events) batch += e }
                    apply(ctx, conn, origin, batch)
                    change()
                }
            }
            while (true) {
                change() // posa al dia el que hagi passat mentre no hi érem
                val ok = try {
                    streamOnce(conn, NovetatCursor.get(ctx, origin), events); true
                } catch (e: kotlinx.coroutines.CancellationException) { throw e } catch (_: Exception) { false }
                if (ok) attempt = 0
                if (!ok && attempt >= 3) {
                    // El servidor no té stream de novetats: sondeig declarat.
                    repeat(15) { delay(NovetatStreamLogic.POLL_MS); change() }
                    attempt = 0
                } else delay(NovetatStreamLogic.backoff(attempt++))
            }
        }
    }
}

/** Aplica un lot: cursor, avisos que ja sobren i què ja s'ha vist dins l'app. */
private suspend fun apply(ctx: Context, conn: Conn, origin: String, batch: List<NovetatEvent>) = withContext(Dispatchers.IO) {
    // Arribades amb l'app oberta: ja són al fil; no han de sonar més tard.
    NoticeLedger.mark(ctx, origin, batch.filter { it.kind == "novetat" }.flatMap { it.ids })
    // Llegides en un altre dispositiu: fora l'avís d'aquí.
    batch.filter { it.kind == "novetat_read" }.flatMap { it.winds }.distinct()
        .forEach { WindNotifications.cancelWind(ctx, origin, it) }
    batch.maxOfOrNull { it.cursor }?.let { NovetatCursor.set(ctx, origin, it) }
}

/**
 * El fil d'un vent és visible: les seves novetats queden llegides (aquí, al
 * servidor i als altres dispositius) i l'avís del sistema desapareix.
 * [newsKey] canvia quan n'arriba una de nova amb el fil obert.
 */
@Composable
internal fun ThreadReadEffect(conn: Conn, windId: String, newsKey: Any?) {
    val ctx = LocalContext.current.applicationContext
    LifecyclePoll(conn, windId, newsKey) { NovetatsRead.wind(ctx, conn, windId) }
}

/**
 * Obrir des d'una notificació: quan el missatge [MessageFocus] d'aquest fil
 * ja és a la llista, hi fa scroll (una vegada). [ids] són els id dels
 * missatges en l'ordre de la llista; [offset] els elements que hi ha abans.
 */
@Composable
internal fun FocusMessageEffect(target: String, listState: LazyListState, ids: List<String>, offset: Int = 0) {
    val req by MessageFocus.request.collectAsState()
    LaunchedEffect(req, ids.size, target) {
        val r = req ?: return@LaunchedEffect
        val now = System.currentTimeMillis()
        if (MessageFocusLogic.expired(r, now)) { MessageFocus.done(r); return@LaunchedEffect }
        val idx = MessageFocusLogic.indexFor(r, target, ids, now) ?: return@LaunchedEffect
        // Deixa passar el «segueix el final» de l'obertura del fil i després ves al missatge.
        delay(150)
        listState.scrollToItem(idx + offset)
        MessageFocus.done(r)
    }
}
