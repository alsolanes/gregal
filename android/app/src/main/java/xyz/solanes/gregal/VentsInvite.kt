package xyz.solanes.gregal

import androidx.compose.foundation.layout.padding
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp

/**
 * Un vent convidat al fil amb «Pregunta-li» (internal/web/winds_invite.go): la
 * seva resposta va dins el fil d'un altre vent amb kind «guest». Sense aquesta
 * línia semblaria que la diu el vent del fil. La web fa el mateix a paintHistory.
 */
internal fun guestJoinsText(lang: String, name: String): String =
    if (lang == "en") "→ $name joins in" else "→ $name s'hi afegeix"

@Composable
internal fun GuestJoinsLine(lang: String, name: String) {
    Text(guestJoinsText(lang, name), color = Muted, fontSize = GregalSize.caption,
        modifier = Modifier.padding(start = 46.dp, bottom = 2.dp).testTag("guest-joins"))
}
