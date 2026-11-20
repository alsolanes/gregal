package xyz.solanes.gregal

import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.LazyListState
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.lazy.rememberLazyListState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableIntStateOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.test.junit4.createComposeRule
import androidx.compose.ui.test.onNodeWithTag
import androidx.compose.ui.test.performTouchInput
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.unit.dp
import kotlinx.coroutines.runBlocking
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config
import org.robolectric.annotation.GraphicsMode
import org.robolectric.annotation.LooperMode

/**
 * El xat com a les apps de xat amb IA: en enviar, el missatge puja a dalt i la
 * resposta hi creix a sota sense que la llista es mogui. El fil és el de
 * qualsevol xat (missatges amb clau, la targeta de feina, la resposta en
 * directe i l'espai final), amb alçades fixes perquè la posició sigui exacta.
 */
@RunWith(RobolectricTestRunner::class)
@Config(sdk = [34], qualifiers = "w393dp-h851dp")
@GraphicsMode(GraphicsMode.Mode.NATIVE)
@LooperMode(LooperMode.Mode.PAUSED)
class ChatPinTest {
    @get:Rule val ui = createComposeRule()

    private lateinit var list: LazyListState
    private lateinit var follow: ChatFollowState
    /** Clau i alçada (dp) de cada element; les claus «u…» són missatges teus. */
    private var rows by mutableStateOf((0 until 30).map { (if (it % 2 == 0) "u$it" else "a$it") to 80 })
    private var composer by mutableIntStateOf(80)

    private fun host() {
        ui.setContent {
            list = rememberLazyListState()
            follow = rememberChatFollow(list, "fixture", rows.lastOrNull { it.first.startsWith("u") }?.first)
            LazyColumn(Modifier.fillMaxSize().testTag("list"), state = list,
                contentPadding = PaddingValues(top = 56.dp, bottom = composer.dp)) {
                items(rows, key = { it.first }) { Box(Modifier.fillMaxWidth().height(it.second.dp).testTag(it.first)) }
                chatReserve(follow)
            }
        }
        ui.waitForIdle()
        runBlocking { list.scrollToLatest() }
        ui.waitForIdle()
    }

    private fun set(key: String, height: Int) {
        rows = if (rows.any { it.first == key }) rows.map { if (it.first == key) key to height else it } else rows + (key to height)
    }

    private fun send() {
        ui.runOnIdle { follow.send(); set("u-sent", 60) }
        ui.waitForIdle()
    }

    private fun offsetOf(key: String) = list.layoutInfo.visibleItemsInfo.first { it.key == key }.offset
    private fun position() = list.firstVisibleItemIndex to list.firstVisibleItemScrollOffset
    private fun px(dp: Int) = dp * ui.density.density

    @Test fun `en enviar, el missatge puja a dalt sota la capcalera`() {
        host()
        send()
        ui.runOnIdle { assertEquals(0, offsetOf("u-sent")) }
        val top = ui.onNodeWithTag("u-sent").fetchSemanticsNode().boundsInRoot.top
        assertEquals("just sota la capçalera", px(56), top, 1f)
    }

    @Test fun `mentre la resposta creix la llista no es mou encara que passi de la pantalla`() {
        host()
        send()
        val pinned = ui.runOnIdle { position() }
        ui.mainClock.autoAdvance = false
        for (h in 20..1600 step 20) {
            ui.runOnIdle { set("a-reply", h) }
            ui.mainClock.advanceTimeByFrame()
            ui.runOnIdle { assertEquals("alçada $h", pinned, position()) }
        }
        ui.mainClock.autoAdvance = true
        ui.runOnIdle { assertTrue("la resposta passa de la pantalla", list.canScrollForward) }
    }

    @Test fun `una resposta llarga amb fases d'eina no mou la llista`() {
        host()
        send()
        val pinned = ui.runOnIdle { position() }
        ui.mainClock.autoAdvance = false
        fun frame(change: () -> Unit) {
            ui.runOnIdle(change)
            repeat(2) { ui.mainClock.advanceTimeByFrame() }
            ui.runOnIdle { assertEquals(pinned, position()) }
        }
        frame { set("w-work", 48) }
        for (h in 20..200 step 20) frame { set("a-live1", h) }
        // Fase d'eina: el text queda com a missatge (mateixa clau, una mica més baix),
        // la targeta de feina creix i surt el «escrivint».
        frame { set("a-live1", 190); set("w-work", 96); set("t-typing", 40) }
        frame { rows = rows.filter { it.first != "t-typing" }; set("a-live2", 20) }
        for (h in 40..1400 step 40) frame { set("a-live2", h) }
        frame { set("w-work", 64) }
        ui.mainClock.autoAdvance = true
        ui.runOnIdle { assertTrue(list.canScrollForward) }
    }

    @Test fun `quan la resposta passa de la pantalla surt el boto per anar al final`() {
        host()
        send()
        set("a-reply", 200)
        ui.runOnIdle { assertFalse("hi cap: res a baix", follow.behind) }
        val pinned = ui.runOnIdle { position() }
        set("a-reply", 1500)
        ui.runOnIdle {
            assertTrue("surt «↓»", follow.behind)
            assertEquals("però no hi va sol", pinned, position())
        }
    }

    @Test fun `en passar a definitiva el que es veia no salta`() {
        host()
        send()
        set("w-work", 96)
        set("a-reply", 300)
        val before = ui.runOnIdle { offsetOf("a-reply") }
        // La definitiva és més baixa (el directe duia el cursor) i la targeta de
        // feina perd la línia d'activitat.
        ui.runOnIdle { set("a-reply", 270); set("w-work", 94) }
        ui.waitForIdle()
        ui.runOnIdle {
            assertEquals("el missatge enviat continua a dalt", 0f, offsetOf("u-sent").toFloat(), px(2))
            assertEquals("la resposta no es mou", before.toFloat(), offsetOf("a-reply").toFloat(), px(2))
        }
    }

    @Test fun `si arrossegues mentre s'escriu la llista ja no es mou sola`() {
        host()
        send()
        set("a-reply", 300)
        // Un dit que baixa una mica la llista, a poc a poc (sense inèrcia).
        ui.onNodeWithTag("list").performTouchInput {
            down(center)
            repeat(10) { moveBy(Offset(0f, 20f)) }
            up()
        }
        ui.waitForIdle()
        val read = ui.runOnIdle { position() }
        ui.runOnIdle { assertTrue("el missatge enviat ha baixat", offsetOf("u-sent") > 0) }
        ui.runOnIdle { set("a-reply", 100) }
        ui.waitForIdle()
        ui.runOnIdle { set("a-reply", 900) }
        ui.waitForIdle()
        ui.runOnIdle { assertEquals("on l'has deixat", read, position()) }
    }

    @Test fun `el compositor que creix no mou els missatges`() {
        host()
        val open = ui.runOnIdle { position() }
        composer = 240
        ui.runOnIdle { assertEquals("sense enviar", open, position()) }
        composer = 80
        send()
        set("a-reply", 200)
        val pinned = ui.runOnIdle { position() }
        composer = 240
        ui.runOnIdle { assertEquals("amb el missatge a dalt", pinned, position()) }
        composer = 80
        ui.runOnIdle { assertEquals("i quan torna a baixar", pinned, position()) }
    }
}
