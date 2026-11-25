package xyz.solanes.gregal

import android.app.Application
import androidx.compose.material3.MaterialTheme
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.ui.test.SemanticsMatcher
import androidx.compose.ui.test.filter
import androidx.compose.ui.test.hasAnyDescendant
import androidx.compose.ui.test.hasTestTag
import androidx.compose.ui.test.hasText
import androidx.compose.ui.test.junit4.createComposeRule
import androidx.compose.ui.test.onAllNodesWithTag
import androidx.compose.ui.test.onNodeWithText
import androidx.test.core.app.ApplicationProvider
import kotlinx.coroutines.flow.MutableStateFlow
import org.junit.Assert.assertEquals
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config
import org.robolectric.annotation.GraphicsMode
import org.robolectric.annotation.LooperMode

/**
 * La resposta d'un vent creix al seu lloc: els punts d'espera, el text en
 * directe i la resposta acabada són la mateixa fila del fil (mateixa clau,
 * mateix node), i el que hi ha a sobre no es mou.
 */
@RunWith(RobolectricTestRunner::class)
@Config(sdk = [34], qualifiers = "w393dp-h851dp")
@GraphicsMode(GraphicsMode.Mode.NATIVE)
@LooperMode(LooperMode.Mode.PAUSED)
class WindReplyInPlaceTest {
    @get:Rule val ui = createComposeRule()
    private val wind = Wind("mestral", "Mestral", "#A8CDE2", "recerca")

    private fun <T> flow(vm: GregalViewModel, name: String) = GregalViewModel::class.java.getDeclaredField(name)
        .apply { isAccessible = true }.get(vm) as MutableStateFlow<T>

    private fun rowWith(inner: SemanticsMatcher) = ui.onAllNodesWithTag("chat-row")
        .filter(hasAnyDescendant(inner)).fetchSemanticsNodes().single()

    @Test fun `l'espera, el directe i la resposta son la mateixa fila i el de dalt no es mou`() {
        // Port tancat: l'historial no es demana a cap servidor de veritat.
        val vm = GregalViewModel(ApplicationProvider.getApplicationContext<Application>()).apply { setConn(Conn("127.0.0.1", "9", "t")) }
        val chats = flow<Map<String, List<GregalViewModel.PetMsg>>>(vm, "_windChat")
        val busy = flow<Set<String>>(vm, "_windBusy")
        val history = (0 until 8).flatMap { listOf(GregalViewModel.PetMsg("user", "Pregunta $it"), GregalViewModel.PetMsg("pet", "Resposta $it")) } +
            GregalViewModel.PetMsg("user", "I ara què?")
        chats.value = mapOf(wind.id to history)
        busy.value = setOf(wind.id)
        ui.mainClock.autoAdvance = false
        ui.setContent {
            CompositionLocalProvider(LocalPal provides LightPal, LocalWindMotion provides false) {
                MaterialTheme(colorScheme = LightPal.scheme()) { WindConversation(vm, "ca", wind, onBack = {}) }
            }
        }
        ui.mainClock.advanceTimeBy(600)
        fun sentTop() = ui.onNodeWithText("I ara què?").fetchSemanticsNode().boundsInRoot.top
        val top = sentTop()
        val pending = rowWith(hasTestTag("wind-typing")).id

        // El directe ensenya paraules senceres: cada tros acaba en espai.
        for (text in listOf("Hola, ", "Hola, sóc ", "Hola, sóc aquí. ")) {
            chats.value = mapOf(wind.id to history + GregalViewModel.PetMsg("pet", text, streaming = true))
            ui.mainClock.advanceTimeBy(600)
            assertEquals("en directe ($text): la mateixa fila que l'espera", pending, rowWith(hasText(text.trim(), substring = true)).id)
            assertEquals("el missatge enviat no es mou", top, sentTop(), 0.5f)
        }

        chats.value = mapOf(wind.id to history + GregalViewModel.PetMsg("pet", "Hola, sóc aquí. Ja està.", ts = 1_764_000_000L))
        busy.value = emptySet()
        ui.mainClock.advanceTimeBy(600)
        assertEquals("la resposta acabada és la mateixa fila", pending, rowWith(hasText("Ja està.", substring = true)).id)
        assertEquals("el missatge enviat no es mou", top, sentTop(), 0.5f)
    }
}
