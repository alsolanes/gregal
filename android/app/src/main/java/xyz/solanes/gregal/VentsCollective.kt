package xyz.solanes.gregal

import androidx.compose.foundation.clickable
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.compose.ui.window.Dialog

/*
 * El primer cop que obres l'app: com vols dir als teus ajudants. No als
 * individuals (Marinada, Bolet…), sinó al col·lectiu: vents, fullets,
 * minairons, el que vulguis. Els textos de l'app i els noms que es proposen
 * per a cada ajudant nou s'hi adapten. «Ara no» no torna a preguntar.
 */

/** Un col·lectiu proposat: com es diu un i com es diuen tots. */
data class CollectiveOption(val singular: String, val plural: String, val hint: String)

object CollectiveOptions {
    fun of(lang: String): List<CollectiveOption> = if (lang == "en") listOf(
        CollectiveOption("Wind", "Winds", "Zephyr, Gust, Squall…"),
        CollectiveOption("Sprite", "Sprites", "Acorn, Moss, Bramble…"),
        CollectiveOption("Helper", "Helpers", "Tinker, Pinch, Spool…"),
        CollectiveOption("Spirit", "Spirits", "Mist, Ember, Echo…"),
    ) else listOf(
        CollectiveOption("Vent", "Vents", "Garbinada, Terral, Embat…"),
        CollectiveOption("Fullet", "Fullets", "Bolet, Rovelló, Molsa…"),
        CollectiveOption("Minairó", "Minairons", "Pessic, Picarol, Didal…"),
        CollectiveOption("Esperit", "Esperits", "Boira, Guspira, Eco…"),
    )

    /**
     * Un a partir de molts, perquè només calgui escriure'n un: «pirates» →
     * «pirata», «minairons» → «minairó», «follets» → «follet». Es pot corregir.
     */
    fun singularOf(plural: String, lang: String = "ca"): String {
        val p = plural.trim()
        val low = p.lowercase()
        if (lang == "en") return when {
            low.length > 4 && low.endsWith("ies") -> p.dropLast(3) + "y"
            low.length > 4 && (low.endsWith("ches") || low.endsWith("shes") || low.endsWith("xes") || low.endsWith("sses")) -> p.dropLast(2)
            low.length > 3 && low.endsWith("ves") -> p.dropLast(3) + "f"
            low.length > 2 && low.endsWith("s") && !low.endsWith("ss") -> p.dropLast(1)
            else -> p
        }
        return when {
            low.length > 4 && low.endsWith("ons") -> p.dropLast(3) + "ó"
            low.length > 4 && low.endsWith("ies") && !low.endsWith("eies") -> p.dropLast(3) + "y"
            low.length > 3 && low.endsWith("es") -> p.dropLast(2) + "a"
            low.length > 2 && low.endsWith("s") -> p.dropLast(1)
            else -> p
        }
    }
}

@Composable
internal fun CollectivePickerDialog(vm: GregalViewModel, lang: String) {
    val options = remember(lang) { CollectiveOptions.of(lang) }
    var picked by remember { mutableStateOf<CollectiveOption?>(options.first()) }
    var custom by remember { mutableStateOf(false) }
    var plural by remember { mutableStateOf("") }
    var singular by remember { mutableStateOf("") }
    var singularEdited by remember { mutableStateOf(false) }
    Dialog(onDismissRequest = { vm.skipCollective() }) {
        Surface(shape = RoundedCornerShape(24.dp), color = Superficie, modifier = Modifier.testTag("collective-picker")) {
            // Amb el teclat obert, el camp «un» i els botons han de quedar a l'abast.
            Column(Modifier.imePadding().verticalScroll(rememberScrollState()).padding(22.dp), verticalArrangement = Arrangement.spacedBy(12.dp)) {
                Text(tr(lang, "collective.title"), color = Ink, fontSize = 20.sp, fontWeight = FontWeight.Bold)
                Text(tr(lang, "collective.help"), color = Muted, fontSize = 13.5.sp, lineHeight = 19.sp)
                options.forEach { o ->
                    CollectiveRow(o.plural, o.hint, selected = !custom && picked == o) { custom = false; picked = o }
                }
                CollectiveRow(tr(lang, "collective.other"), tr(lang, "collective.otherHint"), selected = custom) { custom = true }
                if (custom) {
                    OutlinedTextField(plural, {
                        plural = it.take(20)
                        if (!singularEdited) singular = CollectiveOptions.singularOf(plural, lang)
                    }, singleLine = true, label = { Text(tr(lang, "prefs.ventsNamePlur")) },
                        modifier = Modifier.fillMaxWidth().testTag("collective-plural"))
                    OutlinedTextField(singular, { singular = it.take(20); singularEdited = true }, singleLine = true,
                        label = { Text(tr(lang, "prefs.ventsNameSing")) }, modifier = Modifier.fillMaxWidth())
                }
                Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.SpaceBetween, verticalAlignment = Alignment.CenterVertically) {
                    TextButton(onClick = { vm.skipCollective() }) { Text(tr(lang, "collective.laterButton"), color = Muted) }
                    val ok = !custom || plural.isNotBlank()
                    Button(enabled = ok, onClick = {
                        if (custom) vm.desaNomsDelVent(singular.ifBlank { CollectiveOptions.singularOf(plural, lang) }.lowercase(), plural.trim().lowercase())
                        else picked?.let { o ->
                            // «Vents» és el de sempre: es desa buit perquè l'idioma hi posi el seu.
                            if (o == options.first()) vm.desaNomsDelVent("", "") else vm.desaNomsDelVent(o.singular.lowercase(), o.plural.lowercase())
                        }
                    }) { Text(tr(lang, "collective.ok")) }
                }
            }
        }
    }
}

@Composable
private fun CollectiveRow(title: String, hint: String, selected: Boolean, onClick: () -> Unit) {
    Row(Modifier.fillMaxWidth().clickable(onClick = onClick), verticalAlignment = Alignment.CenterVertically,
        horizontalArrangement = Arrangement.spacedBy(6.dp)) {
        RadioButton(selected = selected, onClick = onClick)
        Column(Modifier.weight(1f)) {
            Text(title, color = Ink, fontSize = 15.sp, fontWeight = FontWeight.SemiBold)
            Text(hint, color = Muted, fontSize = 12.5.sp)
        }
    }
}
