package xyz.solanes.gregal

import android.app.Activity
import android.content.Intent
import android.os.Bundle

/**
 * Tornada del consentiment OAuth (B5): la pàgina del servidor enllaça a
 * `<id de paquet>://connexions?id=con-…`. Aquesta activitat no pinta res:
 * porta al davant la tasca de l'app que ja hi era (sense crear-ne una altra
 * instància) i es tanca. El full de connexions torna a llegir l'estat del
 * servidor en reprendre's, així que no cal passar-li res.
 */
class ConnexioReturnActivity : Activity() {
    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        packageManager.getLaunchIntentForPackage(packageName)?.let {
            it.addFlags(Intent.FLAG_ACTIVITY_NEW_TASK or Intent.FLAG_ACTIVITY_RESET_TASK_IF_NEEDED)
            startActivity(it)
        }
        finish()
    }
}
