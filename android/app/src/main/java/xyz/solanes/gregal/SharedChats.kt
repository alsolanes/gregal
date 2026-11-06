package xyz.solanes.gregal

import android.app.Notification
import android.app.NotificationManager
import android.content.Context
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.withContext
import org.json.JSONArray
import org.json.JSONObject

/*
 * Rotllanes compartides: un xat de grup entre persones del mateix servidor
 * (comptes) i els vents de qui el crea. Cada convidat rep una invitació
 * (Accepta / Rebutja) i només després d'acceptar veu el xat i hi escriu.
 * Servidor: /api/winds/shared (winds_shared.go).
 */

/** Una persona del xat i l'estat de la seva invitació (invited, accepted, declined, left). */
data class SharedPersonInfo(val name: String, val status: String)

/** Un xat compartit tal com el veu qui hi participa (o qui hi és convidat). */
data class SharedChatInfo(
    val id: String,
    val name: String,
    val owner: String,
    val mine: Boolean,
    val winds: List<Wind>,
    val people: List<SharedPersonInfo>,
    val updated: Long = 0L,
    val lastAuthor: String = "",
    val lastText: String = "",
) {
    /** Les persones que hi participen ara: el propietari i els que han acceptat. */
    val humans: List<String> get() = listOf(owner) + people.filter { it.status == "accepted" }.map { it.name }
}

/** Un missatge del fil: d'una persona (author = compte) o d'un vent (speakerId = id del vent). */
data class SharedMsg(val from: String, val text: String, val ts: Long, val speakerId: String,
                     val speakerName: String, val author: String = "", val mine: Boolean = false) {
    /** Per a WindBubble: el meu a la dreta; la resta (persones i vents) a l'esquerra amb el nom. */
    fun asPetMsg(): GregalViewModel.PetMsg =
        GregalViewModel.PetMsg(if (mine) "user" else "pet", text, speakerId, speakerName, ts = ts * 1000)
}

/** Llegeix les respostes del servidor (lògica pura, sense xarxa). */
object SharedChatsJson {
    fun chat(o: JSONObject): SharedChatInfo {
        val wa = o.optJSONArray("winds") ?: JSONArray()
        val winds = (0 until wa.length()).mapNotNull { i ->
            val w = wa.optJSONObject(i) ?: return@mapNotNull null
            Wind(w.optString("id"), w.optString("name"), w.optString("color"), w.optString("role"), w.optString("style", "pso").ifBlank { "pso" })
        }
        val pa = o.optJSONArray("people") ?: JSONArray()
        val people = (0 until pa.length()).mapNotNull { i ->
            val p = pa.optJSONObject(i) ?: return@mapNotNull null
            SharedPersonInfo(p.optString("name"), p.optString("status"))
        }
        val last = o.optJSONObject("last")
        return SharedChatInfo(o.optString("id"), o.optString("name"), o.optString("owner"), o.optBoolean("mine"),
            winds, people, o.optLong("updated") * 1000, last?.optString("author_name").orEmpty(), last?.optString("text").orEmpty())
    }

    fun list(o: JSONObject): Triple<String, List<SharedChatInfo>, List<SharedChatInfo>> {
        fun arr(key: String) = (o.optJSONArray(key) ?: JSONArray()).let { a -> (0 until a.length()).mapNotNull { a.optJSONObject(it)?.let(::chat) } }
        return Triple(o.optString("me"), arr("chats"), arr("invites"))
    }

    fun history(o: JSONObject): List<SharedMsg> {
        val a = o.optJSONArray("history") ?: JSONArray()
        return (0 until a.length()).mapNotNull { i ->
            val m = a.optJSONObject(i) ?: return@mapNotNull null
            SharedMsg(m.optString("from"), m.optString("text"), m.optLong("ts"), m.optString("speaker_id"),
                m.optString("speaker_name"), m.optString("author"), m.optBoolean("mine"))
        }
    }

    fun people(o: JSONObject): List<String> {
        val a = o.optJSONArray("people") ?: JSONArray()
        return (0 until a.length()).map { a.optString(it) }.filter { it.isNotBlank() }
    }

    fun save(name: String, winds: List<String>, people: List<String>, id: String = ""): JSONObject {
        val o = JSONObject().put("name", name.trim()).put("winds", JSONArray(winds)).put("people", JSONArray(people))
        if (id.isNotBlank()) o.put("id", id)
        return o
    }
}

private fun enc(s: String) = java.net.URLEncoder.encode(s, "UTF-8")

fun GregalApi.sharedChats() = SharedChatsJson.list(get("/api/winds/shared"))
fun GregalApi.sharedPeople() = SharedChatsJson.people(get("/api/winds/shared/people"))
fun GregalApi.saveSharedChat(name: String, winds: List<String>, people: List<String>, id: String = ""): SharedChatInfo =
    SharedChatsJson.chat(post("/api/winds/shared", SharedChatsJson.save(name, winds, people, id)).optJSONObject("chat") ?: JSONObject())
fun GregalApi.answerSharedInvite(id: String, accept: Boolean) { post("/api/winds/shared/answer", JSONObject().put("id", id).put("accept", accept)) }
fun GregalApi.leaveSharedChat(id: String) { post("/api/winds/shared/leave", JSONObject().put("id", id)) }
fun GregalApi.sharedHistory(id: String): Pair<List<SharedMsg>, Boolean> {
    val o = get("/api/winds/shared/chat?id=" + enc(id))
    return SharedChatsJson.history(o) to o.optBoolean("busy")
}
fun GregalApi.sendShared(id: String, message: String): List<SharedMsg> =
    SharedChatsJson.history(post("/api/winds/shared/chat", JSONObject().put("id", id).put("message", message)))

/**
 * Estat de les rotllanes compartides a l'app: la llista, les invitacions
 * pendents i els fils oberts. Un sol lloc perquè l'inici, el xat i el
 * vigilant en segon pla en vegin el mateix.
 */
object SharedChatsStore {
    private val _me = MutableStateFlow("")
    val me: StateFlow<String> = _me
    private val _chats = MutableStateFlow<List<SharedChatInfo>>(emptyList())
    val chats: StateFlow<List<SharedChatInfo>> = _chats
    private val _invites = MutableStateFlow<List<SharedChatInfo>>(emptyList())
    val invites: StateFlow<List<SharedChatInfo>> = _invites
    private val _history = MutableStateFlow<Map<String, List<SharedMsg>>>(emptyMap())
    val history: StateFlow<Map<String, List<SharedMsg>>> = _history
    /** Xats on els vents pensen (al servidor) o on hi ha un enviament meu en curs. */
    private val _busy = MutableStateFlow<Set<String>>(emptySet())
    val busy: StateFlow<Set<String>> = _busy
    private val sending = mutableSetOf<String>()
    private var base = ""

    /** Canvi de servidor o de compte: res del d'abans. */
    private fun forConn(api: GregalApi) {
        val key = api.base + "|" + api.token
        if (key == base) return
        base = key
        _me.value = ""; _chats.value = emptyList(); _invites.value = emptyList(); _history.value = emptyMap(); _busy.value = emptySet()
        synchronized(sending) { sending.clear() }
    }

    /** Torna a llegir la llista; un servidor sense comptes (404) vol dir «no n'hi ha». */
    suspend fun refresh(api: GregalApi) = withContext(Dispatchers.IO) {
        forConn(api)
        try {
            val (me, chats, invites) = api.sharedChats()
            _me.value = me; _chats.value = chats.sortedByDescending { it.updated }; _invites.value = invites
        } catch (e: Exception) {
            if (e.message.orEmpty().startsWith("HTTP 404")) { _chats.value = emptyList(); _invites.value = emptyList() }
        }
    }

    suspend fun load(api: GregalApi, id: String) = withContext(Dispatchers.IO) {
        forConn(api)
        val (msgs, serverBusy) = api.sharedHistory(id)
        _history.value = _history.value + (id to msgs)
        val mineSending = synchronized(sending) { id in sending }
        _busy.value = if (serverBusy || mineSending) _busy.value + id else _busy.value - id
    }

    /** Envia: el missatge surt de seguida i el fil es refà amb la resposta dels vents. */
    suspend fun send(api: GregalApi, id: String, text: String) = withContext(Dispatchers.IO) {
        forConn(api)
        synchronized(sending) { sending += id }
        _busy.value = _busy.value + id
        _history.value = _history.value + (id to (_history.value[id].orEmpty() +
            SharedMsg("user", text, System.currentTimeMillis() / 1000, "user:" + _me.value, _me.value, _me.value, true)))
        try {
            _history.value = _history.value + (id to api.sendShared(id, text))
        } finally {
            synchronized(sending) { sending -= id }
            _busy.value = _busy.value - id
        }
    }

    suspend fun answer(api: GregalApi, id: String, accept: Boolean) = withContext(Dispatchers.IO) {
        api.answerSharedInvite(id, accept)
        _invites.value = _invites.value.filterNot { it.id == id }
        refresh(api)
    }

    suspend fun leave(api: GregalApi, id: String) = withContext(Dispatchers.IO) {
        api.leaveSharedChat(id)
        _chats.value = _chats.value.filterNot { it.id == id }
        _history.value = _history.value - id
    }

    suspend fun save(api: GregalApi, name: String, winds: List<String>, people: List<String>, id: String = ""): SharedChatInfo =
        withContext(Dispatchers.IO) {
            val chat = api.saveSharedChat(name, winds, people, id)
            refresh(api)
            chat
        }
}

/** El vigilant en segon pla: una invitació nova avisa amb notificació (una sola vegada per xat). */
object SharedInviteWatch {
    private const val PREFS = "gregal_shared_invites"
    private const val KEY = "seen"

    /** Les invitacions que encara no s'han avisat; desa-les totes com a vistes. */
    fun fresh(invites: List<SharedChatInfo>, seen: Set<String>): List<SharedChatInfo> = invites.filter { it.id !in seen }

    fun check(ctx: Context, api: GregalApi, notify: Boolean) {
        val invites = try { api.sharedChats().third } catch (_: Exception) { return }
        val prefs = ctx.getSharedPreferences(PREFS, Context.MODE_PRIVATE)
        val seen = prefs.getStringSet(KEY, emptySet()).orEmpty()
        val nous = fresh(invites, seen)
        prefs.edit().putStringSet(KEY, (seen + invites.map { it.id }).toList().takeLast(200).toSet()).apply()
        if (!notify || nous.isEmpty() || !WindNotifications.canNotify(ctx)) return
        WindNotifications.ensureChannel(ctx)
        val lang = Prefs(ctx).idioma
        val source = NotificationOrigins.remote(Prefs(ctx))?.id.orEmpty()
        for (c in nous) {
            val n = Notification.Builder(ctx, WindNotifications.CHANNEL_ID)
                .setSmallIcon(R.drawable.ic_stat_gregal)
                .setContentTitle(tr(lang, "app.shared.inviteTitle"))
                .setContentText(tr(lang, "app.shared.inviteText", c.owner, c.name))
                .setContentIntent(NotificationNavigation.pending(ctx, NotificationOrigins.effective(ctx, source), "screen:vents"))
                .setAutoCancel(true)
                .setVisibility(Notification.VISIBILITY_PRIVATE)
                .build()
            try { ctx.getSystemService(NotificationManager::class.java).notify(("shared:" + c.id).hashCode(), n) } catch (_: Exception) {}
        }
    }
}
