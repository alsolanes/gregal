package xyz.solanes.gregal

import android.app.Application
import androidx.compose.material3.MaterialTheme
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.ui.test.*
import androidx.compose.ui.test.junit4.createComposeRule
import androidx.test.core.app.ApplicationProvider
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config
import org.robolectric.annotation.GraphicsMode
import org.robolectric.annotation.LooperMode

/** 161: al fil del vent, un encàrrec d'un altre vent és la seva fila, no una bombolla teva, i la ratlla «Nous» hi és. */
@RunWith(RobolectricTestRunner::class)
@Config(sdk = [34], qualifiers = "w393dp-h851dp")
@GraphicsMode(GraphicsMode.Mode.NATIVE)
@LooperMode(LooperMode.Mode.PAUSED)
class HandoffThreadTest {
    @get:Rule val ui = createComposeRule()

    private fun chats(vm: GregalViewModel) = GregalViewModel::class.java.getDeclaredField("_windChat")
        .apply { isAccessible = true }.get(vm) as kotlinx.coroutines.flow.MutableStateFlow<Map<String, List<GregalViewModel.PetMsg>>>

    @Test fun aHandoffIsDrawnAsTheAskersRowWithTheNewDivider() {
        // Un port tancat: sense connexió, el fil real de l'ordinador no substitueix el de la prova.
        val vm = GregalViewModel(ApplicationProvider.getApplicationContext<Application>()).apply { setConn(Conn("127.0.0.1", "9", "t")) }
        val wind = Wind("mestral", "Mestral", "#A8CDE2", "recerca")
        chats(vm).value = mapOf(wind.id to listOf(
            GregalViewModel.PetMsg("pet", "Hola, soc el Mestral", ts = 1L),
            GregalViewModel.PetMsg("user", "Encàrrec de Lluerna: mira el preu", ts = 2L, by = MsgBy("lluerna", "Lluerna")),
        ))
        // L'encàrrec és l'únic missatge nou: la ratlla «Nous» va just davant seu.
        vm.windNew.opened(wind.id, 1)
        vm.windNew.fromServer(wind.id, 2)
        ui.setContent {
            CompositionLocalProvider(LocalPal provides LightPal, LocalWindMotion provides false) {
                MaterialTheme(colorScheme = LightPal.scheme()) { WindConversation(vm, "ca", wind, onBack = {}) }
            }
        }
        ui.onNodeWithTag("handoff-from").assertTextEquals("Encàrrec de Lluerna")
        ui.onNodeWithTag("thread-unread").assertExists()
        ui.onAllNodesWithTag("chat-row").assertCountEquals(1)
        ui.onNode(hasText("mira el preu", substring = true) and hasAnyAncestor(hasTestTag("chat-row"))).assertDoesNotExist()
    }
}
