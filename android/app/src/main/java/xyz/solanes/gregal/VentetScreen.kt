package xyz.solanes.gregal

import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect

/**
 * La secció «Ventet» del menú: el Ventet és una conversa fixada dels Vents,
 * així que hi porta (amb «enrere» a la llista de converses). Una sola casa:
 * abans hi havia la targeta aquí i una altra a la portada.
 */
@Composable
fun VentetScreen(vm: GregalViewModel, @Suppress("UNUSED_PARAMETER") onMenu: () -> Unit) {
    LaunchedEffect(Unit) { vm.showVentet() }
}
