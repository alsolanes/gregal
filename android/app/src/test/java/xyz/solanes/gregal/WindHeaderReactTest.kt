package xyz.solanes.gregal

import android.app.Application
import android.graphics.Bitmap
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.material3.MaterialTheme
import androidx.compose.runtime.Composable
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.ui.Modifier
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.semantics.SemanticsProperties
import androidx.compose.ui.semantics.getOrNull
import androidx.compose.ui.test.*
import androidx.compose.ui.test.junit4.createAndroidComposeRule
import androidx.test.core.app.ApplicationProvider
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.launch
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config
import org.robolectric.annotation.GraphicsMode
import org.robolectric.annotation.LooperMode
import java.io.File

/**
 * 129: com al web (128), cada vent (Gregal inclòs) té la mateixa capçalera, el vent reacciona amb el
 * cos (assenteix en enviar, s'hi acosta en arribar la resposta, fa que no si falla) i un torn que no
 * ha arribat el diu el vent al fil amb «Torna-ho a provar». Fotogrames clar i fosc a output/ui-review/129/.
 */
@RunWith(RobolectricTestRunner::class)
@Config(sdk = [34], qualifiers = "w393dp-h851dp")
@GraphicsMode(GraphicsMode.Mode.NATIVE)
@LooperMode(LooperMode.Mode.PAUSED)
class WindHeaderReactTest {
    @get:Rule val ui = createAndroidComposeRule<androidx.activity.ComponentActivity>()
    private val gregal = Wind("gregal", "Gregal", "#5AA9E6", "Assistent")
    private val xaloc = Wind("xaloc", "Xaloc", "#EFA68F", "Música i concerts")

    private fun vmWith(wind: Wind, msgs: List<GregalViewModel.PetMsg>) =
        GregalViewModel(ApplicationProvider.getApplicationContext<Application>()).apply {
            // Un servidor que no respon: res de dades reals.
            setConn(Conn("127.0.0.1", "9", "", local = true))
            (windChat as MutableStateFlow<Map<String, List<GregalViewModel.PetMsg>>>).value = mapOf(wind.id to msgs)
        }

    private fun show(pal: GregalPalette = LightPal, motion: Boolean = false, content: @Composable () -> Unit) {
        ui.mainClock.autoAdvance = false
        ui.setContent {
            CompositionLocalProvider(LocalPal provides pal, LocalWindMotion provides motion) {
                MaterialTheme(colorScheme = pal.scheme()) { content() }
            }
        }
        ui.mainClock.advanceTimeBy(800)
    }

    private val chat = listOf(GregalViewModel.PetMsg("user", "Hola"), GregalViewModel.PetMsg("pet", "Bon dia!"))

    private fun menuItems(): List<String> = ui.onAllNodes(hasAnyAncestor(isPopup()) and hasClickAction()).fetchSemanticsNodes()
        .map { n -> n.config.getOrNull(SemanticsProperties.Text)?.joinToString("") { it.text }.orEmpty() }

    private fun header(wind: Wind, lang: String): List<String> {
        show { WindConversation(vmWith(wind, chat), lang, wind, onBack = {}) }
        ui.onNode(hasText(wind.name) and hasAnyAncestor(hasTestTag("chat-header-pill")), useUnmergedTree = true).assertExists()
        ui.onNode(hasText(wind.role) and hasAnyAncestor(hasTestTag("role-pill")) and hasAnyAncestor(hasTestTag("chat-header")),
            useUnmergedTree = true).assertExists()
        // El vent és a la cantonada: el seu forat a la capçalera es plega i el nom no hi deixa un buit.
        ui.onNodeWithTag("chat-header-face", useUnmergedTree = true).getBoundsInRoot().let { assertEquals(0f, (it.right - it.left).value, .5f) }
        ui.mainClock.autoAdvance = true
        ui.onNodeWithTag("chat-more").performClick()
        ui.waitForIdle()
        return menuItems()
    }

    @Test fun gregalTeLaMateixaCapcalera() =
        assertEquals(listOf("Perfil", "Fils", "Desa a la wiki", "L’ordinador de Gregal"), header(gregal, "ca"))

    @Test fun unAltreVentTeLaMateixaCapcalera() =
        assertEquals(listOf("Perfil", "Fils", "Desa a la wiki", "L’ordinador de Gregal"), header(xaloc, "ca"))

    @Test fun laCapcaleraEnAngles() =
        assertEquals(listOf("Profile", "Threads", "Save to wiki", "Gregal’s computer"), header(xaloc, "en"))

    @Test fun tocarLaCaraObreElPerfil() {
        show { WindConversation(vmWith(xaloc, chat), "ca", xaloc, onBack = {}) }
        ui.mainClock.autoAdvance = true
        ui.onNodeWithTag("wind-presence").performClick()
        ui.waitForIdle()
        ui.onNode(isDialog()).assertExists()
    }

    @Test fun ambElFilBuitTocarLaCaraDeLaCapcaleraObreElPerfil() {
        show { WindConversation(vmWith(xaloc, emptyList()), "ca", xaloc, onBack = {}) }
        ui.mainClock.autoAdvance = true
        ui.onNodeWithTag("chat-header-pill").performClick()
        ui.waitForIdle()
        ui.onNode(isDialog()).assertExists()
    }

    @Test fun cadaEsdevenimentTeLaSevaReaccio() {
        assertEquals(WindReaction.NOD, windReaction(WindEvent.SENT))
        assertEquals(WindReaction.NOD, windReaction(WindEvent.APPROVED))
        assertEquals(WindReaction.LEAN, windReaction(WindEvent.REPLY))
        assertEquals(WindReaction.SHAKE, windReaction(WindEvent.FAILED))
        assertEquals(WindReaction.SHAKE, windReaction(WindEvent.DECLINED))
    }

    @Test fun elFinalDelTornDiuQueHaPassat() {
        val user = GregalViewModel.PetMsg("user", "Hola")
        assertEquals(WindEvent.REPLY, windTurnEvent(listOf(user, GregalViewModel.PetMsg("pet", "Fet"))))
        // «Ho recordaré» arriba després de la resposta: no canvia res.
        assertEquals(WindEvent.REPLY, windTurnEvent(listOf(user, GregalViewModel.PetMsg("pet", "Fet"), GregalViewModel.PetMsg("pet", "x", kind = "memory"))))
        assertEquals(WindEvent.FAILED, windTurnEvent(listOf(user, GregalViewModel.PetMsg("system", "Sense connexió", kind = "error"))))
        // Aturat per tu sense resposta: no és cap error ni cap resposta.
        assertNull(windTurnEvent(listOf(user)))
        assertNull(windTurnEvent(emptyList()))
    }

    /** El vent de la cantonada sol, reaccionant a cada [kinds] per torns: on és en repòs i on és a mig gest (200 ms). */
    private fun reactAt(motion: Boolean, vararg kinds: WindReaction): List<Pair<androidx.compose.ui.unit.DpRect, androidx.compose.ui.unit.DpRect>> {
        val perch = WindPerch().apply { root = Offset.Zero; corner = Offset(40f, 600f); header = Offset(120f, 20f) }
        lateinit var scope: CoroutineScope
        show(motion = motion) {
            scope = rememberCoroutineScope()
            Box(Modifier.fillMaxSize()) {
                WindPresence(perch, up = false, mood = WindMood.IDLE, accent = Color(xaloc.argb), style = xaloc.faceStyle,
                    label = "Xaloc", hidden = false) {}
            }
        }
        return kinds.map { kind ->
            val rest = ui.onNodeWithTag("wind-presence").getBoundsInRoot()
            ui.runOnIdle { scope.launch { perch.react(kind) } }
            ui.mainClock.advanceTimeBy(200)
            val mid = ui.onNodeWithTag("wind-presence").getBoundsInRoot()
            ui.mainClock.advanceTimeBy(1000)
            rest to mid
        }
    }

    @Test fun ambMovimentElVentSAcostaAlFil() {
        val (rest, mid) = reactAt(true, WindReaction.LEAN).single()
        // Cap al fil: a la dreta, una mica més gran.
        assertTrue("lean moves right: $rest -> $mid", mid.left > rest.left)
        assertTrue("lean grows: $rest -> $mid", (mid.right - mid.left) > (rest.right - rest.left))
    }

    @Test fun ambMovimentFaQueNoIAssenteix() {
        for ((rest, mid) in reactAt(true, WindReaction.SHAKE, WindReaction.NOD)) assertTrue("$rest -> $mid", rest != mid)
    }

    @Test fun senseMovimentElVentNoEsMou() {
        for ((rest, mid) in reactAt(false, *WindReaction.entries.toTypedArray())) assertEquals(rest, mid)
    }

    private val failed = listOf(
        GregalViewModel.PetMsg("user", "Busca'm un concert per divendres"),
        GregalViewModel.PetMsg("system", "No s'ha pogut connectar amb el servidor.", kind = "error"),
    )

    @Test fun unTornQueFallaElDiuElVentAlFil() {
        show { WindConversation(vmWith(xaloc, failed), "ca", xaloc, onBack = {}) }
        val bubble = hasAnyAncestor(hasTestTag("wind-failed"))
        ui.onNode(hasText("No he pogut arribar-hi") and bubble, useUnmergedTree = true).assertExists()
        ui.onNode(hasText("No s'ha pogut connectar amb el servidor.") and bubble, useUnmergedTree = true).assertExists()
        ui.onNode(hasText("Torna-ho a provar") and hasClickAction()).assertExists()
        // El diu el vent: el seu nom al principi, com a les respostes, i dins del fil.
        ui.onNode(hasTestTag("wind-byline") and hasAnyAncestor(hasTestTag("chat-row")), useUnmergedTree = true).assertExists()
        ui.onNode(hasTestTag("wind-failed") and hasAnyAncestor(hasTestTag("chat-row")), useUnmergedTree = true).assertExists()
    }

    @Test fun unTornQueFallaEnAngles() {
        show { WindConversation(vmWith(xaloc, failed), "en", xaloc, onBack = {}) }
        ui.onNode(hasText("I couldn’t get to it"), useUnmergedTree = true).assertExists()
        ui.onNode(hasText("Try again") and hasClickAction()).assertExists()
    }

    /** La finestra i, a sobre, els menús oberts (cada un és una finestra pròpia). */
    private fun save(name: String) {
        val view = ui.activity.window.decorView
        val bmp = Bitmap.createBitmap(view.width, view.height, Bitmap.Config.ARGB_8888)
        val global = Class.forName("android.view.WindowManagerGlobal")
        val roots = global.getDeclaredField("mViews").apply { isAccessible = true }
            .get(global.getMethod("getInstance").invoke(null)) as List<*>
        ui.runOnUiThread {
            val canvas = android.graphics.Canvas(bmp)
            view.draw(canvas)
            for (popup in roots.filterIsInstance<android.view.View>().filter { it !== view }) {
                val at = popup.layoutParams as? android.view.WindowManager.LayoutParams ?: continue
                canvas.save(); canvas.translate(at.x.toFloat(), at.y.toFloat()); popup.draw(canvas); canvas.restore()
            }
        }
        File("../../output/ui-review/129/$name.png").also { it.parentFile!!.mkdirs() }.outputStream().use {
            bmp.compress(Bitmap.CompressFormat.PNG, 100, it)
        }
    }

    private fun frames(name: String, pal: GregalPalette) {
        show(pal) { WindConversation(vmWith(gregal, failed), "ca", gregal, onBack = {}) }
        save("failed-$name")
        ui.mainClock.autoAdvance = true
        ui.onNodeWithTag("chat-more").performClick()
        ui.waitForIdle()
        save("menu-$name")
    }

    @Test fun fotogramesClar() = frames("light", LightPal)
    @Test fun fotogramesFosc() = frames("dark", DarkPal)

    /** Arriba la resposta d'un torn de debò (ocupat → lliure): fotogrames a mig gest, amb l'esclat del seu color. */
    private fun leanFrames(name: String, pal: GregalPalette) {
        val vm = vmWith(gregal, listOf(GregalViewModel.PetMsg("user", "Quin temps farà demà?")))
        val busy = vm.windBusy as MutableStateFlow<Set<String>>
        busy.value = setOf(gregal.id)
        show(pal, motion = true) { WindConversation(vm, "ca", gregal, onBack = {}) }
        val rest = ui.onNodeWithTag("wind-presence").getBoundsInRoot()
        ui.runOnIdle {
            (vm.windChat as MutableStateFlow<Map<String, List<GregalViewModel.PetMsg>>>).value = mapOf(gregal.id to listOf(
                GregalViewModel.PetMsg("user", "Quin temps farà demà?"), GregalViewModel.PetMsg("pet", "Sol al matí i núvols a la tarda.")))
            busy.value = emptySet()
        }
        ui.mainClock.advanceTimeBy(250)
        val mid = ui.onNodeWithTag("wind-presence").getBoundsInRoot()
        save("lean-$name")
        ui.mainClock.advanceTimeBy(200)
        save("lean-$name-2")
        assertTrue("the reply makes the wind lean towards the thread: $rest -> $mid", mid.left > rest.left)
    }

    @Test fun laRespostaFaQueSAcostiClar() = leanFrames("light", LightPal)
    @Test fun laRespostaFaQueSAcostiFosc() = leanFrames("dark", DarkPal)
}
