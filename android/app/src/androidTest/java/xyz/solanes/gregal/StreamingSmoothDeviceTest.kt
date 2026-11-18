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
 * el fil de Vents (seguiment + «llegeix des del principi») amb un streaming
 * a trossos com el del servidor.
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
        val history = (0 until 12).flatMap { i ->
            listOf(GregalViewModel.PetMsg("user", "Pregunta $i sobre el projecte"),
                GregalViewModel.PetMsg("pet", "Resposta $i: una línia de text perquè el fil ocupi la pantalla."))
        } + GregalViewModel.PetMsg("user", "Com va la memòria cau?")
        var msgs by mutableStateOf(history)
        var busy by mutableStateOf(true)
        ui.mainClock.autoAdvance = false
        ui.setContent {
            list = rememberLazyListState()
            val follow = rememberChatFollow(list, "fixture")
            LaunchedEffect(Unit) { follow.requestFollow() }
            ReadReplyFromStart(list, follow, busy, currentReplyKey(msgs))
            LazyColumn(Modifier.fillMaxSize(), state = list,
                contentPadding = PaddingValues(12.dp)) {
                items(msgs.size, key = { it }) { idx ->
                    val m = msgs[idx]
                    Box(Modifier.padding(top = 8.dp).then(if (m.streaming) Modifier.testTag("live") else Modifier)) {
                        WindBubble(m, Color(0xFF3A7BD5))
                    }
                }
            }
        }
        repeat(10) { ui.mainClock.advanceTimeByFrame() }
        // Trossos de ~14 caràcters cada 3 fotogrames (~50 ms), com arriben del servidor.
        val chunks = reply.chunked(14)
        var sent = 0
        var frame = 0
        val tops = mutableListOf<Float>()
        val heights = mutableListOf<Float>()
        while (sent < chunks.size || frame < chunks.size * 3 + 120) {
            if (frame % 3 == 0 && sent < chunks.size) {
                sent++
                val text = chunks.take(sent).joinToString("")
                msgs = history + GregalViewModel.PetMsg("pet", text, streaming = true)
            }
            ui.mainClock.advanceTimeByFrame()
            frame++
            val nodes = ui.onAllNodesWithTag("live").fetchSemanticsNodes()
            if (nodes.isNotEmpty()) {
                val b = nodes[0].boundsInRoot
                tops += b.top; heights += b.height
            }
        }
        // Tremolor: la resposta es mou (amunt i avall) un cop ja es veu, o s'encongeix.
        var moves = 0; var reversals = 0; var shrinks = 0; var lastDir = 0
        for (i in 1 until tops.size) {
            val d = tops[i] - tops[i - 1]
            if (kotlin.math.abs(d) > 0.5f) {
                moves++
                val dir = if (d > 0) 1 else -1
                if (lastDir != 0 && dir != lastDir) reversals++
                lastDir = dir
            }
            if (heights[i] < heights[i - 1] - 0.5f) shrinks++
        }
        // Esglaons: la resposta es mou, s'atura un fotograma i es torna a moure
        // (el seguiment que arribava tard), i el salt més gran d'un sol fotograma
        // un cop la resposta ja s'escriu (el primer, quan apareix, no compta).
        var stalls = 0; var maxStep = 0f
        for (i in 2 until tops.size) {
            val a = kotlin.math.abs(tops[i - 1] - tops[i - 2]); val b = kotlin.math.abs(tops[i] - tops[i - 1])
            if (a > 0.5f && b <= 0.5f && i + 1 < tops.size && kotlin.math.abs(tops[i + 1] - tops[i]) > 0.5f) stalls++
            if (i > 6) maxStep = maxOf(maxStep, b)
        }
        Log.i("StreamSmooth", "stalls=$stalls maxStep=$maxStep")
        Log.i("StreamSmooth", "frames=${tops.size} moves=$moves reversals=$reversals shrinks=$shrinks " +
            "top=${tops.firstOrNull()}..${tops.lastOrNull()} h=${heights.firstOrNull()}..${heights.lastOrNull()}")
        Log.i("StreamSmooth", "tops=" + tops.joinToString(",") { it.toInt().toString() })
        Log.i("StreamSmooth", "heights=" + heights.joinToString(",") { it.toInt().toString() })
        assertEquals("la resposta no s'encongeix mentre s'escriu", 0, shrinks)
        assertEquals("la resposta no va amunt i avall", 0, reversals)
        assertTrue("el seguiment no va a esglaons ($stalls aturades)", stalls <= 3)
        // Quan el començament arriba a la meitat de dalt, la resposta es queda quieta i creix cap avall.
        val half = ui.onRoot().fetchSemanticsNode().boundsInRoot.height / 2
        val high = tops.indexOfFirst { it <= half + 40f }
        assertTrue("la resposta arriba a dalt", high >= 0)
        val settled = tops.drop(high + 20)
        assertTrue("quieta un cop a dalt: ${settled.distinct()}", settled.distinct().size == 1)
        assertTrue("cap salt brusc d'un fotograma ($maxStep px)", maxStep < 40f)
        // En acabar, el final de la resposta es veu (el seguiment no s'ha perdut).
        // En acabar el torn, el fil va al final (com sempre).
        msgs = history + GregalViewModel.PetMsg("pet", reply)
        busy = false
        repeat(90) { ui.mainClock.advanceTimeByFrame() }
        assertFalse("el final es veu en acabar", list.canScrollForward)
    }
}
