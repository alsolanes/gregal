package xyz.solanes.gregal

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.Scaffold
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.unit.dp

/**
 * El Ventet amb casa pròpia. Abans vivia a dalt de tot dels Vents i empenyia
 * les converses avall; ara té secció al menú (i qui el vulgui a la portada ho
 * activa a Preferències).
 */
@Composable
fun VentetScreen(vm: GregalViewModel, onMenu: () -> Unit) {
    val idioma by vm.idioma.collectAsState()
    val winds by vm.winds.collectAsState()
    LaunchedEffect(Unit) {
        vm.loadCap()
        if (winds.isEmpty()) vm.loadWinds()
    }
    GregalBg {
        Scaffold(
            containerColor = Color.Transparent,
            topBar = { GregalTopBar(tr(idioma, "app.nav.ventet"), idioma, onMenu = onMenu) },
            snackbarHost = { NoticeHost(vm) },
        ) { pad ->
            // Centrat: la targeta del Ventet tota sola quedava penjada a dalt
            // amb mitja pantalla buida a sota.
            Column(
                Modifier.fillMaxSize().padding(pad).verticalScroll(rememberScrollState())
                    .padding(horizontal = 0.dp, vertical = 12.dp),
                verticalArrangement = Arrangement.Center,
            ) {
                CapHero(vm, idioma, winds)
            }
        }
    }
}
