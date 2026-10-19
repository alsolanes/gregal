package xyz.solanes.gregal

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ExperimentalLayoutApi
import androidx.compose.foundation.layout.FlowRow
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.material3.Button
import androidx.compose.material3.FilterChip
import androidx.compose.material3.FilterChipDefaults
import androidx.compose.material3.LinearProgressIndicator
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.material3.DropdownMenu
import androidx.compose.material3.DropdownMenuItem
import androidx.compose.foundation.layout.Box
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.text.input.PasswordVisualTransformation
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp

/*
 * Connectar un proveïdor de models en un sol pas: triar-lo, enganxar la clau
 * i començar. És el que necessita una instal·lació nova en mode «aquest
 * dispositiu» (abans el primer torn acabava en un 401 cru) i el mateix que
 * fa el preset del web: la clau es desa i tots els rols passen a aquest
 * proveïdor amb el seu model per defecte.
 */

/** Una entrada del catàleg de proveïdors del servidor (`catalog` de /api/providers). */
data class CatalogEntry(
    val name: String,
    val label: String,
    val local: Boolean,
    val note: String,
    val defaultModel: String,
)

/** Llegeix el catàleg del JSON de /api/providers (pur: MiniJson, testejable). */
fun parseCatalog(json: String): List<CatalogEntry> {
    val root = MiniJson.obj(json) ?: return emptyList()
    val list = root["catalog"] as? List<*> ?: return emptyList()
    return list.mapNotNull { item ->
        @Suppress("UNCHECKED_CAST")
        val o = item as? Map<String, Any?> ?: return@mapNotNull null
        val name = o["name"] as? String ?: return@mapNotNull null
        CatalogEntry(
            name = name,
            label = (o["label"] as? String).orEmpty().ifBlank { name },
            local = o["local"] == true,
            note = (o["note"] as? String).orEmpty(),
            defaultModel = (o["default_model"] as? String).orEmpty(),
        )
    }
}

/**
 * Proveïdors que té sentit oferir per començar: els del núvol. Els locals
 * (Ollama, LM Studio, llama.cpp) escolten a localhost de l'ordinador, que
 * des del mòbil no hi és.
 */
fun setupChoices(catalog: List<CatalogEntry>): List<CatalogEntry> = catalog.filter { !it.local }

/** On aconseguir la clau, tret de la nota del catàleg (que és en català). */
fun keyDomain(note: String): String =
    Regex("""[a-z0-9.-]+\.[a-z]{2,}(/[A-Za-z0-9/_-]*)?""").find(note)?.value.orEmpty()

/** El proveïdor escolta a l'ordinador mateix (localhost): no fa servir clau. */
fun isLocalUrl(url: String): Boolean {
    val u = url.lowercase()
    return "://localhost" in u || "://127." in u || "://[::1]" in u || "://0.0.0.0" in u
}

/** Credential entry is used only by Settings, never the chat model picker. */
@Composable
fun ProviderKeyField(lang: String, key: String, onChange: (String) -> Unit) {
    OutlinedTextField(key, onChange,
        label = { Text(tr(lang, "app.menu.keyPlaceholder")) }, singleLine = true,
        visualTransformation = PasswordVisualTransformation(),
        keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.Password),
        shape = RoundedCornerShape(14.dp), modifier = Modifier.fillMaxWidth())
}

@OptIn(ExperimentalLayoutApi::class)
@Composable
fun ProviderSetup(
    vm: GregalViewModel,
    modifier: Modifier = Modifier,
    current: String = "",
    change: Boolean = false,
    onDone: () -> Unit = {},
) {
    val lang by vm.idioma.collectAsState()
    val catalog by vm.catalog.collectAsState()
    val providers by vm.providers.collectAsState()
    val conn by vm.conn.collectAsState()
    val busy by vm.providerSetupBusy.collectAsState()
    val msg by vm.provMsg.collectAsState()
    var chosen by remember(conn) { mutableStateOf("") }
    var key by remember(conn) { mutableStateOf("") }
    var expanded by remember(conn) { mutableStateOf(false) }
    LaunchedEffect(conn) { vm.loadCatalog(); vm.loadProviders() }
    val choices = (providers.map { p ->
        catalog.firstOrNull { it.name == p.name } ?: CatalogEntry(p.name, p.name, false, "", "")
    } + setupChoices(catalog)).distinctBy { it.name }
    // Si ja n'hi ha un (Preferències), aquest surt marcat.
    LaunchedEffect(choices, current) {
        if (chosen.isEmpty() && choices.isNotEmpty()) chosen = choices.firstOrNull { it.name == current }?.name ?: choices.first().name
    }
    GlassCard(modifier.fillMaxWidth(), hi = true) {
        Column(Modifier.padding(16.dp), verticalArrangement = Arrangement.spacedBy(10.dp)) {
            Text(tr(lang, if (change) "app.setup.changeTitle" else "app.setup.title"), color = Ink, fontSize = 17.sp, fontWeight = FontWeight.SemiBold)
            Text(tr(lang, if (change) "app.prefs.modelHelp" else "app.setup.help"), color = Muted, fontSize = 13.sp)
            Box {
                TextButton(onClick = { expanded = true }) {
                    Text(choices.firstOrNull { it.name == chosen }?.label ?: tr(lang, "app.setup.addProvider"))
                }
                DropdownMenu(expanded, onDismissRequest = { expanded = false }) {
                    choices.forEach { c ->
                        DropdownMenuItem(text = { Text(c.label) },
                            onClick = { chosen = c.name; key = ""; expanded = false })
                    }
                }
            }
            val sel = choices.firstOrNull { it.name == chosen }
            val where = sel?.let { keyDomain(it.note) }.orEmpty()
            if (where.isNotEmpty()) Text(tr(lang, "app.setup.where", where), color = Faint, fontSize = 12.sp)
            ProviderKeyField(lang, key) { key = it.trim() }
            Button(
                onClick = { vm.connectProvider(chosen, key) { key = ""; onDone() } },
                enabled = !busy && chosen.isNotEmpty() && key.isNotBlank(),
                shape = RoundedCornerShape(14.dp),
                modifier = Modifier.fillMaxWidth().height(48.dp),
            ) { Text(tr(lang, if (change) "btn.save" else "app.setup.save")) }
            TextButton(onClick = { vm.testProvider(chosen) }, enabled = !busy && chosen.isNotEmpty()) {
                Text(tr(lang, "app.menu.test"))
            }
            if (busy) LinearProgressIndicator(Modifier.fillMaxWidth(), color = Sea, trackColor = Line)
            if (msg.isNotBlank() && !busy) Text(msg, color = Muted, fontSize = 12.sp)
            Spacer(Modifier.height(2.dp))
            // Mateixa distinció que feia l'antic diàleg de clau del menú de
            // model: en remot la clau viu a la memòria del servidor fins que
            // es reiniciï; prometre que "es desa al dispositiu" seria fals.
            Text(tr(lang, if (conn.local) "app.setup.stored" else "app.menu.keyMemory"), color = Faint, fontSize = 11.sp)
        }
    }
}
