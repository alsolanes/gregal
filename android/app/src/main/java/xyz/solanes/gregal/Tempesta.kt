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
import androidx.compose.material3.DropdownMenu
import androidx.compose.material3.DropdownMenuItem
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
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

data class TempestaAgent(val index: Int, val angle: String, val ca: String, val en: String, val model: String, val state: String,
                         val descCa: String = "", val descEn: String = "") {
    fun label(lang: String) = (if (lang == "en") en else ca).ifBlank { angle }
    /** Què fa l'agent; buit en una rotllana o amb un servidor que no l'envia. */
    fun desc(lang: String) = if (lang == "en") descEn else descCa

    companion object {
        /** L'esdeveniment «tempesta»: {agents:[{index, angle, ca, en, desc_ca?, desc_en?, model, state}]}. */
        fun listOf(data: String): List<TempestaAgent> = try {
            val arr = JSONObject(data).optJSONArray("agents")
            if (arr == null) emptyList() else (0 until arr.length()).mapNotNull { i ->
                arr.optJSONObject(i)?.let {
                    TempestaAgent(it.optInt("index", i), it.optString("angle"), it.optString("ca").ifBlank { it.optString("label") },
                        it.optString("en").ifBlank { it.optString("label") }, it.optString("model").substringAfterLast('/'), it.optString("state", "working"),
                        it.optString("desc_ca"), it.optString("desc_en"))
                }
            }
        } catch (_: Exception) { emptyList() }
    }
}

/** Els models d'«A fons» (/api/winds/tempesta/models): "provider/model" per agent i síntesi, "" = automàtic. */
object TempestaModels {
    val SLOTS = listOf("direct", "sources", "skeptic", "synth")

    fun parse(body: String): Map<String, String> {
        val m = JSONObject(body).optJSONObject("models") ?: JSONObject()
        return SLOTS.associateWith { m.optString(it) }
    }

    fun payload(slot: String, model: String): String = JSONObject().put("slot", slot).put("model", model).toString()
}

/** Preferències › Models › «A fons»: un selector per a cada agent i la síntesi, amb «Automàtic». */
@Composable
internal fun TempestaModelsSection(lang: String, chosen: Map<String, String>, models: Map<String, List<String>>, onPick: (slot: String, model: String) -> Unit) {
    val all = models.toSortedMap().flatMap { (provider, ids) -> ids.map { "$provider/$it" } }
    Column(Modifier.fillMaxWidth().padding(horizontal = 12.dp), verticalArrangement = Arrangement.spacedBy(4.dp)) {
        Text(tr(lang, "app.tempesta.help"), color = Muted, fontSize = GregalSize.caption, lineHeight = 17.sp)
        TempestaModels.SLOTS.forEach { slot ->
            val current = chosen[slot].orEmpty()
            var open by remember { mutableStateOf(false) }
            Row(Modifier.fillMaxWidth().padding(vertical = 8.dp), verticalAlignment = Alignment.CenterVertically,
                horizontalArrangement = Arrangement.spacedBy(12.dp)) {
                Column(Modifier.weight(1f), verticalArrangement = Arrangement.spacedBy(2.dp)) {
                    Text(tr(lang, "app.tempesta.$slot"), color = Ink, fontSize = GregalSize.body)
                    Text(tr(lang, "app.tempesta.${slot}Help"), color = Muted, fontSize = GregalSize.caption, lineHeight = 17.sp)
                }
                Box {
                    OutlinedButton(onClick = { open = true }, modifier = Modifier.widthIn(max = 170.dp).testTag("tempesta-model-$slot")) {
                        Text(current.ifBlank { tr(lang, "app.tempesta.auto") }, fontSize = GregalSize.small, maxLines = 1, overflow = TextOverflow.Ellipsis)
                    }
                    DropdownMenu(open, { open = false }) {
                        DropdownMenuItem(text = { Text(tr(lang, "app.tempesta.auto")) }, onClick = { open = false; onPick(slot, "") },
                            modifier = Modifier.testTag("tempesta-option-auto"))
                        // Un model triat que el proveïdor ja no llista segueix sortint: el servidor fa servir l'automàtic.
                        (if (current.isNotBlank() && current !in all) listOf(current) + all else all).forEach { m ->
                            DropdownMenuItem(text = { Text(m, fontSize = GregalSize.body) }, onClick = { open = false; onPick(slot, m) })
                        }
                    }
                }
            }
        }
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
        Text(tempestaHeadline(lang, agents), color = Muted, fontSize = GregalSize.caption, fontWeight = FontWeight.SemiBold)
        agents.forEach { a ->
            Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                val dot = when (a.state) { "done" -> Green; "failed" -> Red; else -> Sea }
                Box(Modifier.size(8.dp).alpha(if (a.state == "working" && motion) pulse else 1f).clip(CircleShape).background(dot))
                Text(a.label(lang), color = Ink, fontSize = GregalSize.small, fontWeight = FontWeight.SemiBold)
                Text(a.model, color = Faint, fontSize = GregalSize.label, maxLines = 1, overflow = TextOverflow.Ellipsis, modifier = Modifier.weight(1f))
                Text(when (a.state) {
                    "done" -> "✓"
                    "failed" -> if (lang == "en") "failed" else "no ha pogut"
                    else -> if (lang == "en") "working" else "treballant"
                }, color = if (a.state == "done") Green else Muted, fontSize = GregalSize.caption)
            }
            a.desc(lang).takeIf { it.isNotBlank() }?.let {
                Text(it, color = Muted, fontSize = GregalSize.caption, lineHeight = 16.sp,
                    modifier = Modifier.padding(start = 16.dp).offset(y = (-4).dp).testTag("tempesta-desc"))
            }
        }
    }
}
