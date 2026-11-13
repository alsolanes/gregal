package xyz.solanes.gregal

import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.text.KeyboardActions
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.role
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.input.ImeAction
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.compose.ui.window.Dialog
import androidx.compose.ui.window.DialogProperties
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext
import org.json.JSONArray
import org.json.JSONObject

/**
 * Regles en llenguatge natural (com Grok i dots): què pot fer Gregal sol, què
 * t'ha de preguntar i què no ha de fer mai. El servidor les posa al context
 * de cada torn i un revisor les comprova abans de cada acció
 * (internal/web/winds_rules.go). La web fa el mateix a rulesView.
 */
data class WindRules(val alone: List<String> = emptyList(), val ask: List<String> = emptyList(), val never: List<String> = emptyList()) {
    fun of(kind: String) = when (kind) { "alone" -> alone; "ask" -> ask; else -> never }
    fun with(kind: String, list: List<String>) = when (kind) { "alone" -> copy(alone = list); "ask" -> copy(ask = list); else -> copy(never = list) }
    fun toJson(): JSONObject = JSONObject().put("alone", JSONArray(alone)).put("ask", JSONArray(ask)).put("never", JSONArray(never))

    companion object {
        fun parse(o: JSONObject): WindRules {
            fun list(k: String) = o.optJSONArray(k)?.let { a -> (0 until a.length()).map { a.optString(it).trim() }.filter { it.isNotEmpty() } }.orEmpty()
            return WindRules(list("alone"), list("ask"), list("never"))
        }
    }
}

fun GregalApi.getWindRules(): WindRules = WindRules.parse(get("/api/winds/regles"))
fun GregalApi.saveWindRules(r: WindRules): WindRules = WindRules.parse(post("/api/winds/regles", r.toJson()))

internal val RULE_KINDS = listOf("alone", "ask", "never")

internal fun rulesCopy(lang: String, key: String): String {
    val en = lang == "en"
    return when (key) {
        "title" -> if (en) "Rules" else "Regles"
        "help" -> if (en) "Tell it in your own words what it can do alone, what it must ask first and what it must never do. A reviewer checks them before anything that changes something."
            else "Digues-li amb les teves paraules què pot fer sol, què t'ha de preguntar i què no ha de fer mai. Un revisor les comprova abans de fer res que canviï alguna cosa."
        "alone" -> if (en) "It can do this alone" else "Pot fer-ho sol"
        "alone.sub" -> if (en) "Without asking you." else "Sense preguntar-te."
        "ask" -> if (en) "Ask me first" else "Pregunta'm abans"
        "ask.sub" -> if (en) "It shows you what it wants to do and waits for your yes." else "T'ensenya què vol fer i espera el teu sí."
        "never" -> if (en) "Never" else "Mai"
        "never.sub" -> if (en) "It will not do it, even if asked." else "No ho fa, encara que l'hi demanin."
        "add" -> if (en) "Add a rule…" else "Afegeix una regla…"
        "remove" -> if (en) "Remove the rule" else "Treu la regla"
        "examples" -> if (en) "For example:" else "Per exemple:"
        "close" -> if (en) "Close" else "Tanca"
        "error" -> if (en) "Could not load the rules." else "No s'han pogut carregar les regles."
        else -> key
    }
}

internal fun ruleExamples(lang: String, kind: String): List<String> {
    val en = lang == "en"
    return when (kind) {
        "alone" -> if (en) listOf("Create and edit files in the projects folder", "Set reminders for me", "Add events to my calendar")
            else listOf("Crear i editar fitxers a la carpeta de projectes", "Programar-me recordatoris", "Afegir esdeveniments al meu calendari")
        "ask" -> if (en) listOf("Send any email or message", "Buy or pay for anything", "Post anything online")
            else listOf("Enviar qualsevol correu o missatge", "Comprar o pagar res", "Publicar res a internet")
        else -> if (en) listOf("Delete anything in the Documents folder", "Share my personal data", "Accept terms or cookies for me")
            else listOf("Esborrar res de la carpeta Documents", "Compartir les meves dades personals", "Acceptar condicions o galetes per mi")
    }
}

private fun ruleColor(kind: String) = when (kind) {
    "alone" -> Color(0xFF4EC98A)
    "ask" -> Color(0xFFE9B24A)
    else -> Color(0xFFEF6B6B)
}

@Composable
internal fun RulesDialog(conn: Conn, lang: String, onDismiss: () -> Unit) {
    var rules by remember { mutableStateOf<WindRules?>(null) }
    var error by remember { mutableStateOf(false) }
    val scope = rememberCoroutineScope()
    LaunchedEffect(conn) {
        rules = try { withContext(Dispatchers.IO) { conn.api().getWindRules() } } catch (e: Exception) {
            if (e is kotlinx.coroutines.CancellationException) throw e
            error = true; null
        }
    }
    val save: (WindRules) -> Unit = { next ->
        rules = next
        scope.launch {
            runCatching { withContext(Dispatchers.IO) { conn.api().saveWindRules(next) } }.onSuccess { rules = it }
        }
    }
    Dialog(onDismissRequest = onDismiss, properties = DialogProperties(usePlatformDefaultWidth = false)) {
        Surface(shape = MaterialTheme.shapes.large, color = MaterialTheme.colorScheme.background) {
            Column(Modifier.fillMaxWidth().fillMaxHeight(0.9f).padding(top = 16.dp).testTag("rules-dialog")) {
                Row(Modifier.fillMaxWidth().padding(horizontal = 20.dp), verticalAlignment = Alignment.CenterVertically) {
                    Text(rulesCopy(lang, "title"), color = Ink, fontWeight = FontWeight.Bold, fontSize = 18.sp, modifier = Modifier.weight(1f))
                    IconButton(onClick = onDismiss, modifier = Modifier.semantics { contentDescription = rulesCopy(lang, "close") }) { Text("✕", color = Muted, fontSize = 16.sp) }
                }
                Column(Modifier.verticalScroll(rememberScrollState()).padding(horizontal = 16.dp, vertical = 8.dp), verticalArrangement = Arrangement.spacedBy(12.dp)) {
                    Text(rulesCopy(lang, "help"), color = Muted, fontSize = 13.sp, lineHeight = 18.sp, modifier = Modifier.padding(horizontal = 4.dp))
                    val r = rules
                    when {
                        r != null -> RulesContent(lang, r, save)
                        error -> Text(rulesCopy(lang, "error"), color = Muted, fontSize = 14.sp)
                        else -> CircularProgressIndicator(Modifier.size(24.dp))
                    }
                }
            }
        }
    }
}

/** Les tres llistes, editables. Pura respecte de la xarxa (RulesTest). */
@Composable
internal fun RulesContent(lang: String, rules: WindRules, onChange: (WindRules) -> Unit) {
    for (kind in RULE_KINDS) {
        val list = rules.of(kind)
        var draft by remember(kind) { mutableStateOf("") }
        Column(
            Modifier.fillMaxWidth().clip(RoundedCornerShape(14.dp)).background(SuperficieAlta).border(1.dp, Line, RoundedCornerShape(14.dp))
                .padding(14.dp).testTag("rules-$kind"),
            verticalArrangement = Arrangement.spacedBy(8.dp),
        ) {
            Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                Box(Modifier.size(9.dp).clip(CircleShape).background(ruleColor(kind)))
                Text(rulesCopy(lang, kind), color = Ink, fontWeight = FontWeight.SemiBold, fontSize = 15.sp)
            }
            Text(rulesCopy(lang, "$kind.sub"), color = Muted, fontSize = 12.sp)
            list.forEachIndexed { i, rule ->
                Row(
                    Modifier.fillMaxWidth().clip(RoundedCornerShape(10.dp)).background(Ink.copy(alpha = 0.05f)).padding(start = 12.dp),
                    verticalAlignment = Alignment.CenterVertically,
                ) {
                    Text(rule, color = Ink, fontSize = 14.sp, modifier = Modifier.weight(1f).padding(vertical = 8.dp))
                    Box(
                        Modifier.size(48.dp).semantics { role = Role.Button; contentDescription = rulesCopy(lang, "remove") + ": " + rule }
                            .clickable { onChange(rules.with(kind, list.filterIndexed { j, _ -> j != i })) }.testTag("rule-x-$kind-$i"),
                        contentAlignment = Alignment.Center,
                    ) { Text("✕", color = Muted, fontSize = 14.sp) }
                }
            }
            OutlinedTextField(
                value = draft, onValueChange = { draft = it.take(300) },
                placeholder = { Text(rulesCopy(lang, "add"), fontSize = 14.sp) },
                singleLine = true, modifier = Modifier.fillMaxWidth().testTag("rule-add-$kind"),
                keyboardOptions = KeyboardOptions(imeAction = ImeAction.Done),
                keyboardActions = KeyboardActions(onDone = {
                    val t = draft.trim()
                    if (t.isNotEmpty()) { onChange(rules.with(kind, list + t)); draft = "" }
                }),
            )
            val left = ruleExamples(lang, kind).filter { ex -> list.none { it.equals(ex, ignoreCase = true) } }
            if (left.isNotEmpty() && list.size < 3) {
                Text(rulesCopy(lang, "examples"), color = Muted, fontSize = 12.sp)
                for (ex in left) {
                    Text("+ $ex", color = Ink, fontSize = 13.sp,
                        modifier = Modifier.clip(RoundedCornerShape(999.dp)).border(1.dp, Line, RoundedCornerShape(999.dp))
                            .clickable { onChange(rules.with(kind, list + ex)) }.padding(horizontal = 12.dp, vertical = 10.dp))
                }
            }
        }
    }
}
