package xyz.solanes.gregal

import android.content.Context
import android.util.Log
import androidx.compose.material.icons.rounded.Notifications
import androidx.compose.runtime.getValue
import androidx.compose.runtime.setValue
import org.json.JSONObject

/*
 * Push de fons (B7): FCM amb identificador opac.
 *
 * El servidor envia {k:"novetat", id:"n-…"} i res més; aquí es llegeix el
 * contingut amb la sessió desada (GET /api/winds/novetats?id=…) i, si és
 * del compte i encara no s'ha anunciat (NoticeLedger, compartit amb el
 * sondeig), surt la notificació. Amb l'app oberta no sona res: el stream
 * (VentsStream) ja l'ha posat al fil.
 *
 * El transport FCM només existeix si l'APK s'ha compilat amb configuració
 * de Firebase (propietats gregal.fcm.*, mai al repositori): llavors hi ha
 * la classe xyz.solanes.gregal.fcm.FcmTransport. Sense ella (o sense GMS,
 * o si el servidor no té credencials) el mode és el de sempre: WindWatch
 * pregunta cada 15 minuts.
 */

/** Transport de push (FCM): obtenir el token del dispositiu. */
interface PushTransport {
    /** Torna el token o null si no n'hi ha (sense GMS, sense configuració…). Bloquejant. */
    fun token(ctx: Context): String?
}

object PushLogic {
    private val ID = Regex("^n-[0-9a-z]{4,64}$")

    /** L'id opac d'un missatge de dades vàlid, o null. Res més no se'n fa servir. */
    fun novetatId(data: Map<String, String>): String? =
        data["id"]?.takeIf { data["k"] == "novetat" && ID.matches(it) }

    /** Cal tornar a registrar el token? (nou token, nou origen o fa més de [REFRESH_MS]). */
    // `lastEnabled` és el que va contestar el servidor l'última vegada. Si era
    // fals (encara no tenia push), s'ha de tornar a provar a la propera
    // obertura: si no, el telèfon es queda sense avisos fins a 24 h.
    fun needsRegister(token: String, lastToken: String, origin: String, lastOrigin: String, lastAt: Long,
                      lastEnabled: Boolean, now: Long): Boolean =
        token.isNotBlank() && (token != lastToken || origin != lastOrigin || !lastEnabled || now - lastAt > REFRESH_MS)

    const val REFRESH_MS = 24 * 3600_000L
}

object VentsPush {
    private const val PREFS = "gregal_push"
    private const val TRANSPORT = "xyz.solanes.gregal.fcm.FcmTransport"
    private const val TAG = "gregal-push"

    private fun prefs(ctx: Context) = ctx.getSharedPreferences(PREFS, Context.MODE_PRIVATE)

    /** El transport compilat a l'APK, si n'hi ha. */
    val transport: PushTransport? by lazy {
        try { Class.forName(TRANSPORT).getDeclaredConstructor().newInstance() as? PushTransport } catch (_: Throwable) { null }
    }

    /** Push actiu per al servidor desat ara: el servidor té FCM i aquest dispositiu hi és registrat. */
    fun active(ctx: Context): Boolean {
        val origin = NotificationOrigins.remote(Prefs(ctx))?.id ?: return false
        val p = prefs(ctx)
        return transport != null && p.getString("origin", "") == origin && p.getBoolean("enabled", false)
    }

    /** Registra el token al servidor desat si cal. No bloqueja: fil propi. */
    fun ensureRegistered(ctx: Context, fresh: String? = null) {
        // Traça sense secrets: mai el token, només si hi és i què ha fallat.
        // Sense això, un push que no arriba no deixa cap rastre al telèfon.
        val t = transport ?: run { Log.i(TAG, "push: sense transport FCM en aquesta APK"); return }
        val app = ctx.applicationContext
        Thread {
            try {
                val p = Prefs(app)
                val origin = NotificationOrigins.remote(p)?.id
                if (origin == null) { Log.i(TAG, "push: sense servidor desat, no es registra"); return@Thread }
                val token = fresh ?: t.token(app)
                if (token == null) { Log.i(TAG, "push: el telèfon no ha donat cap token (GMS?)"); return@Thread }
                val sp = prefs(app)
                // El servidor ja va dir que podia enviar en un registre
                // anterior: si el telèfon té els avisos bloquejats, s'ha de
                // demanar el permís a cada obertura, no pas esperar 24 h.
                if (sp.getBoolean("enabled", false) && !NotifPermission.granted(app)) {
                    NotifPermission.requestFor(app, NotifAskReason.PUSH)
                }
                if (!PushLogic.needsRegister(token, sp.getString("token", "").orEmpty(), origin,
                        sp.getString("origin", "").orEmpty(), sp.getLong("at", 0L),
                        sp.getBoolean("enabled", false), System.currentTimeMillis())) {
                    Log.i(TAG, "push: token ja registrat en aquest servidor")
                    return@Thread
                }
                val api = GregalApi(baseUrlOf(p.host, p.port), p.token)
                val r = api.post("/api/v2/push", JSONObject().put("token", token).put("platform", "android"))
                val ok = r.optBoolean("enabled", false)
                sp.edit().putString("token", token).putString("origin", origin)
                    .putLong("at", System.currentTimeMillis()).putBoolean("enabled", ok).apply()
                Log.i(TAG, "push: registrat al servidor (enabled=" + ok + ")")
                // El servidor ja pot avisar: és el moment de demanar el permís
                // del telèfon (una sola vegada). Sense permís, Android 13+
                // llença tots els avisos en silenci i no hi ha cap error.
                if (ok) { NotifPermission.requestFor(app, NotifAskReason.PUSH) }
            } catch (e: Throwable) {
                // Sense servidor ara: es tornarà a provar a la propera passada.
                Log.i(TAG, "push: no s'ha pogut registrar: " + e.javaClass.simpleName + " " + (e.message ?: ""))
            }
        }.start()
    }

    /** FCM ha rotat el token. */
    fun onNewToken(ctx: Context, token: String) = ensureRegistered(ctx, token)

    /**
     * Ha arribat un push. Bloquejant (es crida des del fil del servei de
     * FCM). Només fa xarxa amb la sessió desada i només avisa si toca.
     */
    fun onMessage(ctx: Context, data: Map<String, String>) {
        val id = PushLogic.novetatId(data) ?: return
        val app = ctx.applicationContext
        val p = Prefs(app)
        val origin = NotificationOrigins.remote(p)?.id ?: return
        // App oberta: el stream ja la posa al fil; cap notificació, ni ara ni
        // després (el sondeig la trobarà apuntada).
        if (AppVisibility.foreground) { NoticeLedger.mark(app, origin, listOf(id)); return }
        val n = try {
            AvuiJson.novetat(GregalApi(baseUrlOf(p.host, p.port), p.token).get("/api/winds/novetats?id=$id").toString())
        } catch (_: Throwable) { null } ?: return // d'un altre compte (404), esborrada o sense xarxa
        // El torn acabat pot haver avisat ja des del fil (app en segon pla amb
        // la connexió viva): el push del servidor no ha de duplicar-lo.
        if (n.source == "torn" && GregalNotifications.turnJustNotified(origin, System.currentTimeMillis())) {
            NoticeLedger.mark(app, origin, listOf(id))
            return
        }
        if (n.quiet) return
        val fresh = NoticeLedger.claim(app, origin, listOf(n), p.usuari)
        if (fresh.isEmpty() || !WindNotifications.canNotify(app)) return
        WindNotifications.veu(app, fresh.first(), origin)
    }
}

/** Estat del push per a Preferències: què arriba i com, sense configuració amagada. */
enum class PushState { ACTIVE, NOT_CONFIGURED, APP_ONLY, SERVER_ONLY, UNKNOWN, BLOCKED }

object PushStatusLogic {
    /** [serverMode]: "fcm" | "polling" | "" (servidor antic o sense resposta). */
    // `notifGranted`: el permís del telèfon. És el cas que enganya —tot
    // muntat i cap avís—, així que mana sobre tota la resta.
    fun state(appHasFcm: Boolean, serverMode: String, notifGranted: Boolean = true): PushState = when {
        !notifGranted -> PushState.BLOCKED
        serverMode.isBlank() -> if (appHasFcm) PushState.UNKNOWN else PushState.NOT_CONFIGURED
        serverMode == "fcm" && appHasFcm -> PushState.ACTIVE
        serverMode == "fcm" -> PushState.SERVER_ONLY
        appHasFcm -> PushState.APP_ONLY
        else -> PushState.NOT_CONFIGURED
    }

    fun text(lang: String, s: PushState): String {
        val en = lang == "en"
        return when (s) {
            PushState.ACTIVE -> if (en) "Push active (FCM): notifications arrive with the app closed" else "Push actiu (FCM): els avisos arriben amb l'app tancada"
            PushState.BLOCKED -> if (en) "Notifications are blocked on the phone: allow them in Android settings" else "El telèfon té les notificacions bloquejades: permet-les als ajustos d'Android"
            PushState.NOT_CONFIGURED -> if (en) "Push not configured: foreground connection and periodic checks" else "Push no configurat: connexió de primer pla i comprovació periòdica"
            PushState.SERVER_ONLY -> if (en) "Push not configured in this app build: periodic checks" else "Push no configurat en aquesta app: comprovació periòdica"
            PushState.APP_ONLY -> if (en) "Push not configured on the server (fcm-service-account.json): periodic checks" else "Push no configurat al servidor (fcm-service-account.json): comprovació periòdica"
            PushState.UNKNOWN -> if (en) "Push status unknown: periodic checks" else "Estat del push desconegut: comprovació periòdica"
        }
    }

    fun title(lang: String): String = if (lang == "en") "Background notifications" else "Avisos en segon pla"
}

/** Fila de Preferències amb l'estat del push del servidor actual. */
@androidx.compose.runtime.Composable
internal fun PushStatusRow(conn: Conn, lang: String) {
    val ctx = androidx.compose.ui.platform.LocalContext.current
    var mode by androidx.compose.runtime.remember(conn) { androidx.compose.runtime.mutableStateOf("") }
    androidx.compose.runtime.LaunchedEffect(conn) {
        mode = if (conn.local) "polling" else kotlinx.coroutines.withContext(kotlinx.coroutines.Dispatchers.IO) {
            try { conn.api().get("/api/v2/push").optString("mode") } catch (_: Throwable) { "" }
        }
    }
    val granted = NotifPermission.granted(ctx)
    val state = PushStatusLogic.state(VentsPush.transport != null, mode, granted)
    SettingsRow(
        androidx.compose.material.icons.Icons.Rounded.Notifications, PushStatusLogic.title(lang),
        PushStatusLogic.text(lang, state),
        // Bloquejades: la fila porta als ajustos del telèfon, que és l'únic
        // lloc on es pot arreglar un permís denegat.
        onClick = if (state == PushState.BLOCKED) {
            {   try {
                    ctx.startActivity(
                        android.content.Intent(android.provider.Settings.ACTION_APP_NOTIFICATION_SETTINGS)
                            .putExtra(android.provider.Settings.EXTRA_APP_PACKAGE, ctx.packageName)
                            .addFlags(android.content.Intent.FLAG_ACTIVITY_NEW_TASK),
                    )
                } catch (_: Exception) {
                } }
        } else null,
    )
}
