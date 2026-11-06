package xyz.solanes.gregal

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.rounded.Menu
import androidx.compose.material.icons.rounded.Refresh
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.IconButton
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.material3.TopAppBar
import androidx.compose.material3.TopAppBarDefaults
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp

/** Una cosa que es fa sola. Totes les que hi ha al servidor i al mòbil acaben
 *  aquí: recordatoris, tasques programades, plans dels {vents} i fluxos. */
internal data class Automatitzacio(
    val titol: String,
    val detall: String,
    val quan: String,
    val activa: Boolean,
    val tipus: String,
)

/** Un bloc de la llista. `items` buit vol dir que aquell tipus no en té cap. */
internal data class GrupAutomatitzacio(
    val clau: String,
    val titol: String,
    val items: List<Automatitzacio>,
)

/**
 * Plega tot el que es fa sol en quatre blocs, en aquest ordre: recordatoris,
 * tasques programades, plans dels {vents} i fluxos. Els manuals no hi entren:
 * si l'has de prémer tu, no és una automatització.
 *
 * És una funció pura a posta: els tests en comproven l'ordre i el recompte
 * sense haver de muntar la pantalla.
 */
internal fun agrupaAutomatitzacions(
    lang: String,
    jobs: List<JobItem>,
    rutines: List<GregalApi.WindRutina>,
    fluxos: List<FlowSummary>,
    recordatoris: List<ProgramatReminder>,
): List<GrupAutomatitzacio> {
    fun detallDe(text: String): String = text.replace(Regex("\\s+"), " ").trim().take(150)

    val recordatorisItems = jobs.filter { it.kind == "remind" }.map { j ->
        Automatitzacio(
            titol = j.name.ifBlank { tr(lang, "auto.reminders") },
            detall = detallDe(j.prompt),
            quan = j.nextDue.ifBlank { j.time },
            activa = j.enabled && j.notify,
            tipus = "recordatori",
        )
    } + recordatoris.map { r ->
        Automatitzacio(
            titol = r.windName.ifBlank { tr(lang, "auto.reminders") },
            detall = detallDe(r.text),
            quan = r.at,
            activa = true,
            tipus = "recordatori",
        )
    }

    val programades = jobs.filter { it.kind == "daily" || it.kind == "interval" || it.kind == "event" }.map { j ->
        val quan = when (j.kind) {
            "daily" -> j.time
            "interval" -> if (lang == "en") "Every ${j.intervalH}h" else "Cada ${j.intervalH} h"
            else -> j.nextDue
        }
        Automatitzacio(
            titol = j.name.ifBlank { j.id },
            detall = detallDe(j.prompt),
            quan = quan,
            activa = j.enabled,
            tipus = "programada",
        )
    }

    val plans = rutines.map { r ->
        Automatitzacio(
            titol = r.name.ifBlank { tr(lang, "auto.plans") },
            detall = detallDe(r.prompt),
            quan = r.nextDueAt,
            activa = true,
            tipus = "pla",
        )
    }

    val fluxosItems = fluxos.map { f ->
        Automatitzacio(
            titol = f.name.ifBlank { f.slug },
            detall = detallDe(f.desc),
            quan = if (lang == "en") "${f.steps} steps" else "${f.steps} passos",
            activa = true,
            tipus = "flux",
        )
    }

    return listOf(
        GrupAutomatitzacio("recordatoris", tr(lang, "auto.reminders"), recordatorisItems),
        GrupAutomatitzacio("programades", tr(lang, "auto.scheduled"), programades),
        GrupAutomatitzacio("plans", tr(lang, "auto.plans"), plans),
        GrupAutomatitzacio("fluxos", tr(lang, "auto.flows"), fluxosItems),
    )
}

/** Activitat: el que els {vents} faran sols, amb accions a mà (fes-ho ara,
 *  pausa, treu), i a sota la resta de feines programades del servidor i els
 *  fluxos, només si n'hi ha. */
@OptIn(ExperimentalMaterial3Api::class)
@Composable
internal fun AutomatitzacionsScreen(vm: GregalViewModel, onMenu: () -> Unit) {
    val lang by vm.idioma.collectAsState()
    val jobs by vm.jobs.collectAsState()
    val fluxos by vm.flows.collectAsState()
    val activitat by vm.automations.collectAsState()
    val delsVents = activitat[""]?.items.orEmpty().map { it.id }.toSet()

    fun carrega() {
        vm.loadAutomations("")
        vm.loadJobs()
        vm.loadFlows()
    }
    LaunchedEffect(Unit) { carrega() }

    // Els recordatoris i plans dels vents ja surten a dalt: aquí només la
    // feina programada que no és de cap vent i els fluxos.
    val altres = agrupaAutomatitzacions(lang, jobs.filter { it.kind != "remind" && !it.id.startsWith("wind-") && it.id !in delsVents }, emptyList(), fluxos, emptyList())
        .filter { it.clau != "recordatoris" && it.clau != "plans" && it.items.isNotEmpty() }

    Scaffold(containerColor = Superficie, snackbarHost = { NoticeHost(vm) }, topBar = {
        TopAppBar(
            title = { Text(tr(lang, "act.title"), fontSize = 18.sp, fontWeight = FontWeight.SemiBold) },
            navigationIcon = { IconButton(onClick = onMenu) { Icon(Icons.Rounded.Menu, tr(lang, "app.nav.menu")) } },
            actions = { IconButton(onClick = { carrega() }) { Icon(Icons.Rounded.Refresh, tr(lang, "auto.refresh")) } },
            colors = TopAppBarDefaults.topAppBarColors(containerColor = Superficie),
        )
    }) { pad ->
        LazyColumn(Modifier.fillMaxSize().padding(pad), contentPadding = androidx.compose.foundation.layout.PaddingValues(16.dp),
            verticalArrangement = Arrangement.spacedBy(10.dp)) {
            item(key = "ajuda") {
                Text(tr(lang, "act.help"), color = Muted, fontSize = 13.sp, lineHeight = 18.sp)
            }
            item(key = "vents") { WindAutomationList(vm, lang) }
            for (grup in altres) {
                item(key = "cap-" + grup.clau) {
                    Row(Modifier.fillMaxWidth().padding(top = 10.dp), horizontalArrangement = Arrangement.SpaceBetween) {
                        Text(grup.titol, color = Ink, fontWeight = FontWeight.SemiBold, fontSize = 15.sp)
                        Text(grup.items.size.toString(), color = Muted, fontSize = 13.sp)
                    }
                }
                itemsIndexedCompat(grup) { index, a -> AutomatitzacioRow(lang, a, grup.clau + index) }
            }
        }
    }
}

/** `items` de la llista amb clau estable: el grup no es repinta sol. */
private inline fun androidx.compose.foundation.lazy.LazyListScope.itemsIndexedCompat(
    grup: GrupAutomatitzacio,
    crossinline fila: @Composable (Int, Automatitzacio) -> Unit,
) {
    grup.items.forEachIndexed { index, a ->
        item(key = grup.clau + "-" + index) { fila(index, a) }
    }
}

@Composable
private fun AutomatitzacioRow(lang: String, a: Automatitzacio, key: String) {
    Surface(color = Ink.copy(alpha = 0.03f), shape = MaterialTheme.shapes.medium, modifier = Modifier.fillMaxWidth()) {
        Column(Modifier.padding(14.dp), verticalArrangement = Arrangement.spacedBy(4.dp)) {
            Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.SpaceBetween) {
                Text(a.titol, color = Ink, fontWeight = FontWeight.SemiBold, fontSize = 14.sp,
                    modifier = Modifier.padding(end = 8.dp))
                Text(tr(lang, if (a.activa) "auto.on" else "auto.off"),
                    color = if (a.activa) Sea else Muted, fontSize = 12.sp)
            }
            if (a.detall.isNotBlank()) Text(a.detall, color = Tinta, fontSize = 13.sp, lineHeight = 18.sp)
            if (a.quan.isNotBlank()) Text(tr(lang, "auto.next", a.quan), color = Muted, fontSize = 12.sp)
        }
    }
}
