package xyz.solanes.gregal

import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.rounded.Close
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.Text
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.semantics.heading
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.compose.ui.window.Dialog
import androidx.compose.ui.window.DialogProperties

/**
 * «Què pot fer Gregal» (B14): una guia curta i bonica del que l'assistent fa i
 * de com demanar-ho, amb exemples que es proven d'un toc (s'envien al fil de
 * Gregal). S'obre la primera vegada i des del menú. Mateix contingut que la web
 * (`internal/web/app/vents-guide.js`).
 */
internal data class GuideTopic(val icon: String, val title: String, val how: String, val examples: List<String>)

internal fun guideIntro(lang: String): Pair<String, String> = if (lang == "en")
    "What Gregal can do" to "Talk to it as you would to an assistant who has its own computer. It does the work itself, comes back here with the result and only asks before acting outside."
else
    "Què pot fer Gregal" to "Parla-li com a un assistent que té el seu propi ordinador. Fa la feina ell mateix, torna aquí amb el resultat i només et pregunta abans d'actuar fora."

internal fun guideTopics(lang: String): List<GuideTopic> {
    val en = lang == "en"
    fun t(ca: String, e: String) = if (en) e else ca
    return listOf(
        GuideTopic("🔎", t("Encarrega-li feina", "Hand it a task"),
            t("Cerca, compara, resumeix i prepara. No cal dir-li com: ho resol i et porta el resultat al fil.",
              "It searches, compares, summarises and prepares. No need to tell it how: it works it out and brings the result to the thread."),
            listOf(t("Busca'm tres restaurants tranquils per sopar divendres a prop de casa", "Find me three quiet restaurants for dinner on Friday near home"),
                t("Compara els tres mòbils més ben valorats per menys de 400 €", "Compare the three best-rated phones under 400"))),
        GuideTopic("⏰", t("Recorda-te'n per mi", "Remember it for me"),
            t("Recordatoris d'una vegada o que es repeteixen. Arriben a l'hora encara que no obris l'app, i els pots ajornar o canviar.",
              "One-off or repeating reminders. They arrive on time even if you don't open the app, and you can snooze or change them."),
            listOf(t("Recorda'm demà a les 9 de trucar al metge", "Remind me tomorrow at 9 to call the doctor"),
                t("Cada dilluns a les 8, recorda'm de revisar les factures", "Every Monday at 8, remind me to check the bills"))),
        GuideTopic("👀", t("Vigila coses per tu", "Keep an eye on things"),
            t("Una web, un preu, el teu calendari o el correu. Si no canvia res, calla; quan canvia, prepara la feina i t'avisa. Sota cada avís, «Per què?» diu d'on ve.",
              "A web page, a price, your calendar or your mail. If nothing changes it stays quiet; when it does, it prepares the work and tells you. Under each note, «Why?» says where it came from."),
            listOf(t("Vigila el preu del Zen 14 i avisa'm si baixa de 900 €", "Watch the price of the Zen 14 and tell me if it drops under 900"),
                t("Cada matí mira la meva agenda i digues-me què he de preparar", "Every morning look at my calendar and tell me what to prepare"))),
        GuideTopic("🔗", t("Connecta els teus comptes", "Connect your accounts"),
            t("Correu, calendari i apps des de Preferències → Connexions. Les contrasenyes es guarden xifrades i Gregal no les veu mai.",
              "Mail, calendar and apps from Preferences → Connections. Passwords are stored encrypted and Gregal never sees them."),
            listOf(t("Què tinc avui al calendari?", "What's on my calendar today?"),
                t("Tinc algun correu important d'aquesta setmana sense respondre?", "Do I have any important unanswered email from this week?"))),
        GuideTopic("🖥️", t("El seu ordinador", "Its own computer"),
            t("Navega i fa tràmits en un ordinador propi. Toca 🖥 a dalt per veure on clica i què escriu, i pren el control quan calgui iniciar sessió.",
              "It browses and runs errands on a computer of its own. Tap 🖥 at the top to see where it clicks and what it types, and take control when you need to sign in."),
            listOf(t("Entra a la web de l'ajuntament i mira quins tràmits puc fer en línia", "Go to the city council website and see which errands I can do online"))),
        GuideTopic("🧑‍🤝‍🧑", t("Especialistes quan calen", "Specialists when they help"),
            t("Si un dels teus vents en sap més (compres, dades, codi…), Gregal li ho pregunta en segon pla i et diu qui l'ha ajudat. També pots parlar-hi directament des del menú.",
              "If one of your winds knows more (shopping, data, code…), Gregal asks it in the background and tells you who helped. You can also talk to them directly from the menu."),
            listOf(t("Demana als especialistes quin portàtil em convé per programar", "Ask the specialists which laptop suits me for coding"))),
        GuideTopic("🌐", t("Webs que es publiquen", "Websites that go live"),
            t("Et fa una web o una app petita, te l'ensenya en privat i la publica amb el teu nom, a Internet o només a casa.",
              "It builds a website or a small app, shows it to you privately and publishes it under your name, on the internet or only at home."),
            listOf(t("Fes-me una web senzilla amb les receptes de la família", "Make me a simple website with our family recipes"))),
        GuideTopic("✋", t("Tu tens l'última paraula", "You have the last word"),
            t("Abans d'enviar, comprar, publicar o esborrar, et pregunta. Pots dir-li «fes-ho així la propera vegada» i ho recordarà.",
              "Before sending, buying, publishing or deleting, it asks you. Say «do it like this next time» and it remembers."),
            emptyList()),
    )
}

@Composable
internal fun GregalGuide(lang: String, accent: Color, onTry: (String) -> Unit, onClose: () -> Unit) {
    val pal = LocalPal.current
    Dialog(onDismissRequest = onClose, properties = DialogProperties(usePlatformDefaultWidth = false)) {
        Box(Modifier.fillMaxSize().background(Superficie).openingBloom(accent, pal.lila, working = false).testTag("gregal-guide")) {
            Column(Modifier.fillMaxSize().verticalScroll(rememberScrollState()).padding(horizontal = 20.dp, vertical = 16.dp),
                verticalArrangement = Arrangement.spacedBy(12.dp)) {
                Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.End) {
                    IconButton(onClick = onClose, modifier = Modifier.testTag("guide-close")) {
                        Icon(Icons.Rounded.Close, tr(lang, "app.vents.back"), tint = Ink)
                    }
                }
                val (title, intro) = guideIntro(lang)
                Text(title, color = Ink, fontSize = 26.sp, lineHeight = 32.sp, fontWeight = FontWeight.Bold,
                    modifier = Modifier.semantics { heading() })
                Text(intro, color = Muted, fontSize = 16.sp, lineHeight = 23.sp)
                Spacer(Modifier.height(4.dp))
                for (topic in guideTopics(lang)) GuideCard(topic, accent) { onTry(it); onClose() }
                Spacer(Modifier.height(24.dp))
            }
        }
    }
}

@Composable
private fun GuideCard(topic: GuideTopic, accent: Color, onTry: (String) -> Unit) {
    val shape = RoundedCornerShape(15.dp)
    Column(Modifier.fillMaxWidth().clip(shape).background(SuperficieAlta.copy(alpha = .6f)).border(1.dp, Line, shape)
        .padding(16.dp), verticalArrangement = Arrangement.spacedBy(8.dp)) {
        Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(10.dp)) {
            Box(Modifier.size(36.dp).clip(RoundedCornerShape(12.dp)).background(accent.copy(alpha = .16f)), contentAlignment = Alignment.Center) {
                Text(topic.icon, fontSize = 18.sp)
            }
            Text(topic.title, color = Ink, fontSize = 16.sp, fontWeight = FontWeight.SemiBold)
        }
        Text(topic.how, color = Muted, fontSize = 14.sp, lineHeight = 20.sp)
        for (ex in topic.examples) {
            Text("“$ex”", color = Ink, fontSize = 14.sp, lineHeight = 20.sp,
                modifier = Modifier.fillMaxWidth().clip(RoundedCornerShape(14.dp)).border(1.dp, accent.copy(alpha = .45f), RoundedCornerShape(14.dp))
                    .clickable(role = Role.Button) { onTry(ex) }.padding(horizontal = 14.dp, vertical = 10.dp).testTag("guide-example"))
        }
    }
}
