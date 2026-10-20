package xyz.solanes.gregal

import androidx.compose.runtime.Composable

/** Mostra una sola destinació de companyia: l'inici o el xat d'una mascota. */
@Composable
internal fun CompanyRoute(
    openPet: Pet?,
    companyContent: @Composable () -> Unit,
    petChatContent: @Composable (Pet) -> Unit,
) {
    if (openPet == null) companyContent() else petChatContent(openPet)
}
