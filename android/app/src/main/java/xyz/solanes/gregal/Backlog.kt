package xyz.solanes.gregal

import org.json.JSONArray
import org.json.JSONObject

// El backlog del projecte (/api/backlog, internal/backlog al servidor). Les
// regles les aplica el servidor; aquí només es llegeix i es mostra.

data class BacklogCriterion(val text: String, val done: Boolean, val evidence: String)
data class BacklogLog(val at: String, val who: String, val text: String)
data class BacklogReviewCriterion(val text: String, val status: String, val evidence: String)
data class BacklogReviewCheck(val command: String, val passed: Boolean)

/** La revisió independent de la tasca (running, met, not_met o error). */
data class BacklogReview(
    val status: String,
    val atMillis: Long,
    val summary: String,
    val criteria: List<BacklogReviewCriterion>,
    val checks: List<BacklogReviewCheck>,
) {
    /** Com al servidor: als 30 minuts una revisió «running» es dona per perduda. */
    fun running(now: Long = System.currentTimeMillis()): Boolean =
        status == "running" && (atMillis == 0L || now - atMillis < 30 * 60 * 1000L)
}

data class BacklogItem(
    val id: Int,
    val type: String,
    val title: String,
    val status: String,
    val priority: String,
    val size: String,
    val parent: Int,
    val dependsOn: List<Int>,
    val why: String,
    val criteria: List<BacklogCriterion>,
    val outOfScope: String,
    val reason: String,
    val log: List<BacklogLog>,
    val updated: String,
    val review: BacklogReview? = null,
) {
    val open: Boolean get() = status != "done" && status != "dropped"
    val doneCriteria: Int get() = criteria.count { it.done }
}

data class BacklogState(val root: String, val project: String, val storage: String, val items: List<BacklogItem>)

object BacklogJson {
    /** Ordre dels grups oberts: el que es mou primer, el que espera al final. */
    val OPEN_ORDER = listOf("in_progress", "in_review", "ready", "blocked", "proposed")
    val TYPES = listOf("feature", "bug", "task", "spike", "epic")

    fun parse(raw: String): BacklogState {
        val o = JSONObject(raw)
        val arr = o.optJSONArray("items") ?: JSONArray()
        return BacklogState(
            root = o.optString("root"), project = o.optString("project"), storage = o.optString("storage", "app"),
            items = (0 until arr.length()).map { item(arr.getJSONObject(it)) },
        )
    }

    fun item(o: JSONObject): BacklogItem {
        val cr = o.optJSONArray("criteria") ?: JSONArray()
        val lg = o.optJSONArray("log") ?: JSONArray()
        val deps = o.optJSONArray("depends_on") ?: JSONArray()
        return BacklogItem(
            id = o.optInt("id"), type = o.optString("type"), title = o.optString("title"),
            status = o.optString("status"), priority = o.optString("priority", "next"), size = o.optString("size"),
            parent = o.optInt("parent"), dependsOn = (0 until deps.length()).map { deps.optInt(it) },
            why = o.optString("why"), outOfScope = o.optString("out_of_scope"), reason = o.optString("reason"),
            criteria = (0 until cr.length()).map { i ->
                val c = cr.getJSONObject(i)
                BacklogCriterion(c.optString("text"), c.optBoolean("done"), c.optString("evidence"))
            },
            log = (0 until lg.length()).map { i ->
                val l = lg.getJSONObject(i)
                BacklogLog(l.optString("at"), l.optString("who"), l.optString("text"))
            },
            updated = o.optString("updated"),
            review = o.optJSONObject("review")?.let { review(it) },
        )
    }

    fun review(o: JSONObject): BacklogReview {
        val cr = o.optJSONArray("criteria") ?: JSONArray()
        val ch = o.optJSONArray("checks") ?: JSONArray()
        val at = try { java.time.Instant.parse(o.optString("at")).toEpochMilli() } catch (_: Exception) { 0L }
        return BacklogReview(
            status = o.optString("status"), atMillis = at, summary = o.optString("summary"),
            criteria = (0 until cr.length()).map { i ->
                val c = cr.getJSONObject(i)
                BacklogReviewCriterion(c.optString("text"), c.optString("status"), c.optString("evidence"))
            },
            checks = (0 until ch.length()).map { i -> ch.getJSONObject(i).let { BacklogReviewCheck(it.optString("command"), it.optBoolean("passed")) } },
        )
    }

    /** El servidor torna {"error": "..."}; l'ApiException hi posa «HTTP 400: » davant. */
    fun errorText(message: String?): String {
        val m = message.orEmpty()
        val brace = m.indexOf('{')
        if (brace >= 0) {
            try {
                val e = JSONObject(m.substring(brace)).optString("error")
                if (e.isNotBlank()) return e
            } catch (_: Exception) { }
        }
        return m
    }

    fun create(type: String, title: String, why: String, criteria: String, size: String, priority: String): JSONObject =
        JSONObject().put("action", "create").put("type", type).put("title", title.trim()).put("why", why.trim())
            .put("size", size).put("priority", priority)
            .put("criteria", JSONArray(criteria.lines().map { it.trim() }.filter { it.isNotEmpty() }))

    fun move(id: Int, status: String, reason: String = ""): JSONObject =
        JSONObject().put("action", "move").put("id", id).put("status", status).put("reason", reason)

    fun check(id: Int, n: Int, done: Boolean, evidence: String = ""): JSONObject =
        JSONObject().put("action", "check").put("id", id).put("n", n).put("done", done).put("evidence", evidence)

    fun review(id: Int): JSONObject = JSONObject().put("action", "review").put("id", id)

    fun log(id: Int, text: String): JSONObject = JSONObject().put("action", "log").put("id", id).put("text", text)
}
