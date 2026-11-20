package xyz.solanes.gregal

import android.util.Log
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.LazyListState
import androidx.compose.foundation.lazy.rememberLazyListState
import androidx.compose.runtime.*
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.test.*
import androidx.compose.ui.test.junit4.createComposeRule
import androidx.compose.ui.unit.dp
import androidx.test.ext.junit.runners.AndroidJUnit4
import org.junit.Assert.*
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith

/**
 * La bombolla que s'escriu en directe no ha de tremolar: fotograma a
 * fotograma, la resposta no es mou amunt i avall ni s'encongeix. Reprodueix
 * el fil de Vents (la pregunta a dalt i la resposta a sota, amb una fase
 * d'eina pel mig) amb un streaming a trossos com el del servidor.
 */
@RunWith(AndroidJUnit4::class)
class StreamingSmoothDeviceTest {
    @get:Rule val ui = createComposeRule()

    private val reply = """
        Bon dia! T'explico com funciona la **memòria cau del proveïdor** i per què ara els Vents van més ràpid.

        ## Què ha canviat

        - El prefix del prompt es manté igual d'un torn a l'altre, així el proveïdor el pot reaprofitar.
        - Les notes apreses es *congelen* durant la conversa i no canvien a cada missatge.
        - L'historial porta l'hora amb què es va enviar cada missatge.

        Amb deepseek, la primera crida de cada torn ha passat del 18% al 52% de memòria cau, que vol dir menys espera i menys cost per a converses llargues.

        Si vols, et puc ensenyar `com mesurar-ho` amb una conversa de prova i comparar-ne els temps abans i després del canvi.

        ## Com ho pots comprovar

        1. Obre una conversa nova amb el vent i fes-li tres o quatre preguntes seguides sobre el mateix tema.
        2. Mira el registre del servidor: cada crida diu quants tokens ha llegit de la memòria cau.
        3. Compara-ho amb una conversa d'abans del canvi, on les notes canviaven a cada torn.

        Si la proporció no puja, segurament el proveïdor no reaprofita el prefix perquè alguna cosa del principi del prompt canvia a cada missatge: l'hora, una nota nova o l'ordre de les eines.
    """.trimIndent()

    @Test fun liveReplyDoesNotShake() {
        lateinit var list: LazyListState
        lateinit var follow: ChatFollowState
        val history = (0 until 12).flatMap { i ->
            listOf(GregalViewModel.PetMsg("user", "Pregunta $i sobre el projecte"),
                GregalViewModel.PetMsg("pet", "Resposta $i: una línia de text perquè el fil ocupi la pantalla."))
        }
        var msgs by mutableStateOf(history)
        ui.mainClock.autoAdvance = false
        ui.setContent {
            list = rememberLazyListState()
            follow = rememberChatFollow(list, "fixture", msgs.indexOfLast { it.from == "user" })
            LazyColumn(Modifier.fillMaxSize(), state = list,
                contentPadding = PaddingValues(12.dp)) {
                items(msgs.size, key = { it }) { idx ->
                    val m = msgs[idx]
                    Box(Modifier.padding(top = 8.dp).then(if (idx == history.size + 1) Modifier.testTag("live") else Modifier)) {
                        WindBubble(m, Color(0xFF3A7BD5))
                    }
                }
                chatReserve(follow)
            }
        }
        repeat(10) { ui.mainClock.advanceTimeByFrame() }
        // Envia: la pregunta puja a dalt.
        follow.send()
        msgs = history + GregalViewModel.PetMsg("user", "Com va la memòria cau?")
        repeat(60) { ui.mainClock.advanceTimeByFrame() }
        val asked = history.size
        assertEquals("la pregunta queda a dalt", asked, list.firstVisibleItemIndex)
        val pinned = list.firstVisibleItemIndex to list.firstVisibleItemScrollOffset
        // Trossos de ~14 caràcters cada 3 fotogrames (~50 ms), com arriben del servidor;
        // a mig camí, una fase d'eina tanca el text com a missatge i en comença un altre.
        val chunks = reply.chunked(14)
        val toolAt = chunks.size / 3
        var sent = 0
        var frame = 0
        val tops = mutableListOf<Float>()
        val heights = mutableListOf<Float>()
        while (sent < chunks.size || frame < chunks.size * 3 + 120) {
            if (frame % 3 == 0 && sent < chunks.size) {
                sent++
                val first = chunks.take(minOf(sent, toolAt)).joinToString("")
                val second = chunks.drop(toolAt).take(maxOf(0, sent - toolAt)).joinToString("")
                msgs = history + GregalViewModel.PetMsg("user", "Com va la memòria cau?") +
                    GregalViewModel.PetMsg("pet", first, streaming = sent <= toolAt) +
                    (if (sent > toolAt) listOf(GregalViewModel.PetMsg("pet", second, streaming = true)) else emptyList())
            }
            ui.mainClock.advanceTimeByFrame()
            frame++
            assertEquals("la llista no es mou mentre s'escriu (fotograma $frame)", pinned,
                list.firstVisibleItemIndex to list.firstVisibleItemScrollOffset)
            val nodes = ui.onAllNodesWithTag("live").fetchSemanticsNodes()
            if (nodes.isNotEmpty()) {
                val b = nodes[0].boundsInRoot
                tops += b.top; heights += b.height
            }
        }
        val moves = (1 until tops.size).count { kotlin.math.abs(tops[it] - tops[it - 1]) > 0.5f }
        val shrinks = (1 until heights.size).count { heights[it] < heights[it - 1] - 0.5f }
        Log.i("StreamSmooth", "frames=${tops.size} moves=$moves shrinks=$shrinks")
        assertEquals("la resposta no es mou", 0, moves)
        assertEquals("la resposta no s'encongeix mentre s'escriu", 0, shrinks)
        assertTrue("la resposta passa de la pantalla", list.canScrollForward)
        // En acabar el torn, la llista no es mou.
        msgs = msgs.map { it.copy(streaming = false) }
        repeat(90) { ui.mainClock.advanceTimeByFrame() }
        assertEquals(pinned, list.firstVisibleItemIndex to list.firstVisibleItemScrollOffset)
    }
}
