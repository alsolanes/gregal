package xyz.solanes.gregal

import android.app.Application
import android.graphics.Bitmap
import androidx.compose.material3.MaterialTheme
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.ui.semantics.SemanticsProperties
import androidx.compose.ui.test.junit4.createAndroidComposeRule
import androidx.compose.ui.test.onAllNodesWithText
import androidx.compose.ui.test.onNodeWithTag
import androidx.compose.ui.test.onNodeWithText
import androidx.test.core.app.ApplicationProvider
import org.junit.After
import org.junit.Assert.assertEquals
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config
import org.robolectric.annotation.GraphicsMode
import org.robolectric.annotation.LooperMode
import android.os.Looper
import org.robolectric.Shadows.shadowOf
import java.io.File
import java.net.InetAddress
import java.net.ServerSocket
import java.net.SocketException
import kotlin.concurrent.thread

/**
 * Webs i Carpeta de treball amb camins de Windows i enllaços llargs: l'enllaç
 * és una línia curta i el camí es llegeix pel final. Dades sintètiques servides
 * per un servidor local. Captures a `build/webs-carpeta-look/`.
 */
@RunWith(RobolectricTestRunner::class)
@Config(sdk = [34], qualifiers = "w360dp-h900dp-xhdpi")
@GraphicsMode(GraphicsMode.Mode.NATIVE)
@LooperMode(LooperMode.Mode.PAUSED)
class WebsCarpetaLookTest {
    @get:Rule val ui = createAndroidComposeRule<androidx.activity.ComponentActivity>()

    private val casa = "C:\\\\Users\\\\anna\\\\AppData\\\\Local\\\\Gregal\\\\projectes"
    private val respostes = mapOf(
        "/api/workspaces" to """{"current":"$casa\\flor-de-lluna","project":"flor-de-lluna","branch":"main","workspaces":[
            {"path":"$casa\\flor-de-lluna","name":"flor-de-lluna"},
            {"path":"C:\\Users\\anna\\Documents\\feina\\clients\\2026\\botiga-de-ceramica","name":"botiga-de-ceramica"},
            {"path":"C:\\Users\\anna\\antic","name":"antic","missing":true}]}""",
        "/api/dirs" to """{"path":"$casa\\flor-de-lluna","name":"flor-de-lluna","git":true,"parent":"$casa",
            "home":"C:\\Users\\anna","dirs":[{"name":"src","path":"$casa\\flor-de-lluna\\src"},{"name":"docs","path":"$casa\\flor-de-lluna\\docs"}]}""",
        "/api/preview" to """{"webs":[]}""",
        "/api/shares" to """{"shares":[{"id":"27d5b9b8558cacf274eb2479be7581e052b86c61","kind":"site","files":3,
            "title":"Receptes de la iaia: coques, cocs i panellets per a tot l'any",
            "url":"http://127.0.0.1:8197/share/27d5b9b8558cacf274eb2479be7581e052b86c61/"}]}""",
        "/api/apps" to """{"apps":[{"id":"joc","name":"joc de la serp","status":"running","command":"node server.js","path":"/apps/joc/"}]}""",
        "/api/server/powers" to """{"tunnels":[{"name":"","url":"https://flor-de-lluna.trycloudflare.com","target":"http://localhost:8197","status":"open"}]}""",
        "/api/webs" to """{"sites":[{"name":"receptes","host":"receptes.example.org","url":"https://receptes.example.org/","visibility":"public","version":3,"state":"live"}]}""",
    )

    // Un servidor HTTP mínim: les proves d'Android no tenen com.sun.net.httpserver.
    private val server = ServerSocket(0, 50, InetAddress.getByName("127.0.0.1"))
    private val worker = thread(isDaemon = true) {
        try {
            while (!server.isClosed) {
                val client = server.accept()
                // Cada connexió en un fil i amb temps límit: una connexió que no
                // envia res (una sonda d'abast) encallava tot el servidor i la
                // prova esperava la dada en va (fallava de tant en tant a la suite).
                thread(isDaemon = true) { client.use { s -> answer(s) } }
            }
        } catch (e: SocketException) { if (!server.isClosed) throw e }
    }

    private fun answer(s: java.net.Socket) {
        // Un client que penja a mitja resposta no ha d'aturar el servidor:
        // la pantalla torna a demanar i la prova espera la dada.
        try {
            s.soTimeout = 5_000
            val first = s.getInputStream().bufferedReader().readLine().orEmpty()
            val path = first.split(' ').getOrElse(1) { "/" }.substringBefore('?')
            val body = (respostes[path] ?: "{}").toByteArray()
            s.getOutputStream().apply {
                write("HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: ${body.size}\r\nConnection: close\r\n\r\n".toByteArray())
                write(body)
                flush()
            }
        } catch (e: java.io.IOException) {
            System.err.println("WebsCarpetaLookTest: resposta tallada: ${e.message}")
        }
    }

    @After fun stop() {
        server.close()
        worker.join(1000)
    }

    private fun capture(name: String, pal: GregalPalette, screen: @androidx.compose.runtime.Composable (GregalViewModel) -> Unit, ready: (GregalViewModel) -> Boolean) {
        // Altres proves poden haver deixat una LAN o un domini de fora: aquí s'ha d'anar al servidor local.
        ServerNet.lanIp = ""
        ServerNet.exteriorHost = ""
        ServerPick.forget()
        val vm = GregalViewModel(ApplicationProvider.getApplicationContext<Application>())
        vm.setConn(Conn("127.0.0.1", server.localPort.toString(), ""))
        ui.setContent {
            CompositionLocalProvider(LocalPal provides pal, LocalWindMotion provides false) {
                MaterialTheme(colorScheme = pal.scheme()) { screen(vm) }
            }
        }
        // El ViewModel publica el resultat al fil principal, que amb el looper
        // aturat no sempre es buida sol: sense això la prova fallava de tant en tant.
        ui.waitUntil(30_000) { shadowOf(Looper.getMainLooper()).idle(); ready(vm) }
        ui.waitForIdle()
        val view = ui.activity.window.decorView
        val bmp = Bitmap.createBitmap(view.width, view.height, Bitmap.Config.ARGB_8888)
        ui.runOnUiThread { view.draw(android.graphics.Canvas(bmp)) }
        File("build/webs-carpeta-look/$name.png").also { it.parentFile?.mkdirs() }.outputStream().use { bmp.compress(Bitmap.CompressFormat.PNG, 100, it) }
    }

    private fun webs(name: String, pal: GregalPalette) {
        capture(name, pal, { WebsScreen(it) {} }) { it.webs.value.items.isNotEmpty() }
        ui.onNodeWithText("127.0.0.1:8197/share/…2b86c61/").assertExists()
        ui.onNodeWithText("http://127.0.0.1:8197/share/27d5b9b8558cacf274eb2479be7581e052b86c61/").assertDoesNotExist()
        // El túnel es diu com el seu enllaç: només surt una vegada.
        ui.onNodeWithText("flor-de-lluna.trycloudflare.com").assertExists()
    }

    private fun carpeta(name: String, pal: GregalPalette) {
        capture(name, pal, { CarpetaScreen(it) {} }) { it.workspaces.value != null && it.carpeta.value != null && !it.carpetaBusy.value }
        // La icona de la carpeta, no les inicials del projecte.
        ui.onNodeWithTag("carpeta-icona", useUnmergedTree = true).assertExists()
        ui.onNodeWithText("FD").assertDoesNotExist()
        // Capçalera, dues files de projecte i navegador: cada camí llarg guarda
        // el final i es talla per un separador, no a mig nom.
        val camins = ui.onAllNodesWithText("…", substring = true, useUnmergedTree = true).fetchSemanticsNodes()
            .map { n -> n.config[SemanticsProperties.Text].joinToString("") { it.text } }
        assertEquals(4, camins.size)
        assertEquals(3, camins.count { it.startsWith("…\\") && it.endsWith("\\flor-de-lluna") })
        assertEquals(1, camins.count { it.startsWith("…\\") && it.endsWith("\\botiga-de-ceramica") })
    }

    @Test fun websDark() = webs("webs-dark", DarkPal)
    @Test fun websLight() = webs("webs-light", LightPal)
    @Test fun carpetaDark() = carpeta("carpeta-dark", DarkPal)
    @Test fun carpetaLight() = carpeta("carpeta-light", LightPal)
}
