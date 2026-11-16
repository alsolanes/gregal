package xyz.solanes.gregal

import androidx.compose.animation.core.RepeatMode
import androidx.compose.animation.core.animateFloat
import androidx.compose.animation.core.infiniteRepeatable
import androidx.compose.animation.core.rememberInfiniteTransition
import androidx.compose.animation.core.tween
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.alpha
import androidx.compose.ui.draw.clip
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import org.json.JSONObject

/*
 * Tempesta (winds_tempesta.go): «A fons» fa que diversos vents treballin la
 * mateixa pregunta alhora, cadascun des d'un angle, i en surt una sola
 * resposta. Aquí, l'estat de cada agent mentre treballen.
 */

data class TempestaAgent(val index: Int, val angle: String, val ca: String, val en: String, val model: String, val state: String) {
    fun label(lang: String) = (if (lang == "en") en else ca).ifBlank { angle }

    companion object {
        /** L'esdeveniment «tempesta»: {agents:[{index, angle, ca, en, model, state}]}. */
        fun listOf(data: String): List<TempestaAgent> = try {
            val arr = JSONObject(data).optJSONArray("agents")
            if (arr == null) emptyList() else (0 until arr.length()).mapNotNull { i ->
                arr.optJSONObject(i)?.let {
                    TempestaAgent(it.optInt("index", i), it.optString("angle"), it.optString("ca").ifBlank { it.optString("label") },
                        it.optString("en").ifBlank { it.optString("label") }, it.optString("model").substringAfterLast('/'), it.optString("state", "working"))
                }
            }
        } catch (_: Exception) { emptyList() }
    }
}

/** Títol del tauler: quants hi treballen, o que ja ho ajunten. */
internal fun tempestaHeadline(lang: String, agents: List<TempestaAgent>): String {
    val n = agents.size
    val working = agents.count { it.state == "working" }
    return when {
        working > 0 -> if (lang == "en") "$n winds are on it at once" else "$n vents hi treballen alhora"
        else -> if (lang == "en") "Merging what they found" else "Ajuntant el que han trobat"
    }
}

@Composable
internal fun TempestaPanel(lang: String, agents: List<TempestaAgent>) {
    if (agents.isEmpty()) return
    val shape = RoundedCornerShape(12.dp)
    val pulse by rememberInfiniteTransition(label = "tempesta").animateFloat(1f, .35f, infiniteRepeatable(tween(600), RepeatMode.Reverse), label = "dot")
    val motion = LocalWindMotion.current
    Column(Modifier.padding(start = 48.dp, end = 16.dp, top = 2.dp, bottom = 8.dp).widthIn(max = 420.dp).clip(shape).background(Superficie)
        .border(1.dp, Line, shape).padding(horizontal = 12.dp, vertical = 10.dp).testTag("tempesta-panel"),
        verticalArrangement = Arrangement.spacedBy(6.dp)) {
        Text(tempestaHeadline(lang, agents), color = Muted, fontSize = 12.5.sp, fontWeight = FontWeight.SemiBold)
        agents.forEach { a ->
            Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                val dot = when (a.state) { "done" -> Green; "failed" -> Red; else -> Sea }
                Box(Modifier.size(8.dp).alpha(if (a.state == "working" && motion) pulse else 1f).clip(CircleShape).background(dot))
                Text(a.label(lang), color = Ink, fontSize = 13.sp, fontWeight = FontWeight.SemiBold)
                Text(a.model, color = Faint, fontSize = 11.5.sp, maxLines = 1, overflow = TextOverflow.Ellipsis, modifier = Modifier.weight(1f))
                Text(when (a.state) {
                    "done" -> "✓"
                    "failed" -> if (lang == "en") "failed" else "no ha pogut"
                    else -> if (lang == "en") "working" else "treballant"
                }, color = if (a.state == "done") Green else Muted, fontSize = 12.sp)
            }
        }
    }
}
