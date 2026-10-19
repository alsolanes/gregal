package xyz.solanes.gregal

import androidx.compose.foundation.layout.*
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp

/** A provider/model pair always retains its endpoint identity. No catalogue fallback. */
data class ModelChoice(val provider: String, val model: String) {
    val ref: String get() = "$provider/$model"
}

fun configuredModelChoices(providers: List<ProviderInfo>, models: Map<String, List<String>>,
                           query: String = ""): Map<String, List<ModelChoice>> {
    val q = query.trim()
    return providers.filter { it.hasKey() }.distinctBy { it.name }.sortedBy { it.name }
        .associate { p -> p.name to models[p.name].orEmpty().filter { it.isNotBlank() }.distinct()
            .filter { q.isEmpty() || p.name.contains(q, true) || it.contains(q, true) }
            .map { ModelChoice(p.name, it) } }.filterValues { it.isNotEmpty() }
}

@Composable
fun ProviderSettingsCta(lang: String, onSettings: () -> Unit) {
    Column(Modifier.padding(12.dp), verticalArrangement = Arrangement.spacedBy(8.dp)) {
        Text(tr(lang, "app.picker.setupHelp"), color = Muted, fontSize = 13.sp)
        TextButton(onClick = onSettings, modifier = Modifier.testTag("provider-settings-cta")) {
            Text(tr(lang, "prefs.title"))
        }
    }
}

/** Shared real picker content, independent of networking/ViewModel for host UI QA. */
@Composable
fun ModelPickerContent(lang: String, providers: List<ProviderInfo>, models: Map<String, List<String>>,
                       state: ServerState, onSelect: (String) -> Unit, onSettings: () -> Unit) {
    var query by rememberSaveable { mutableStateOf("") }
    Column(Modifier.width(320.dp)) {
        if (providers.none { it.hasKey() }) {
            ProviderSettingsCta(lang, onSettings)
        } else {
            OutlinedTextField(query, { query = it }, singleLine = true,
                placeholder = { Text(tr(lang, "app.picker.search")) },
                modifier = Modifier.fillMaxWidth().padding(horizontal = 12.dp).testTag("model-search"))
            val groups = configuredModelChoices(providers, models, query)
            LazyColumn(Modifier.fillMaxWidth().height(280.dp)) {
                if (groups.isEmpty()) item {
                    Text(tr(lang, "app.picker.noModels"), Modifier.padding(12.dp), color = Muted)
                }
                groups.forEach { (provider, choices) ->
                    item(key = "provider:$provider") {
                        Text(provider, Modifier.padding(horizontal = 12.dp, vertical = 8.dp),
                            color = Muted, fontWeight = FontWeight.SemiBold, fontSize = 12.sp)
                    }
                    items(choices, key = { it.ref }) { choice ->
                        DropdownMenuItem(text = { Text(choice.model, fontSize = 14.sp) },
                            trailingIcon = { if (state.provider == choice.provider && state.model == choice.model) Text("✓", color = Sea) },
                            onClick = { onSelect(choice.ref) })
                    }
                }
            }
            TextButton(onClick = onSettings, modifier = Modifier.padding(horizontal = 4.dp)) {
                Text(tr(lang, "prefs.title"))
            }
        }
    }
}
