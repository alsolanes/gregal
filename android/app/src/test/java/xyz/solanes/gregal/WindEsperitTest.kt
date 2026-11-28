package xyz.solanes.gregal

import android.app.Application
import android.graphics.Bitmap
import android.os.Looper
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Row
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.test.junit4.createComposeRule
import androidx.compose.ui.test.onNodeWithTag
import androidx.test.core.app.ApplicationProvider
import java.net.ServerSocket
import java.net.SocketException
import java.util.concurrent.CopyOnWriteArrayList
import kotlin.concurrent.thread
import org.junit.Assert.*
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.Shadows.shadowOf
import org.robolectric.annotation.Config
import org.robolectric.annotation.GraphicsMode
import org.robolectric.annotation.LooperMode

/** A local server that answers every request with [reply] and keeps "METHOD path body". */
internal class FakeGregal(private val status: Int = 200, private val reply: (String) -> String) : AutoCloseable {
    private val socket = ServerSocket(0)
    val port get() = socket.localPort.toString()
    val requests = CopyOnWriteArrayList<String>()
    /** The raw request bodies, in the order of [requests], for binary uploads. */
    val bodies = CopyOnWriteArrayList<ByteArray>()
    /** A binary answer for a path (a photo); null = the text of [reply]. */
    var bytes: (String) -> ByteArray? = { null }

    init {
        thread(isDaemon = true) {
            while (true) {
                // close() at the end of the test ends the loop.
                val client = try { socket.accept() } catch (_: SocketException) { break }
                client.use { c ->
                    // Bytes, not a reader: a photo body is not text and its length is in bytes.
                    val input = c.getInputStream().buffered()
                    fun line() = generateSequence { input.read().takeIf { it >= 0 } }.takeWhile { it != '\n'.code }
                        .map { it.toByte() }.toList().toByteArray().toString(Charsets.UTF_8).trimEnd('\r')
                    val (method, path) = line().split(" ")
                    var length = 0
                    while (true) {
                        val header = line()
                        if (header.isEmpty()) break
                        if (header.startsWith("Content-Length:", ignoreCase = true)) length = header.substringAfter(':').trim().toInt()
                    }
                    val body = input.readNBytes(length)
                    bodies += body
                    requests += "$method $path ${String(body, Charsets.UTF_8)}".trim()
                    val out = bytes(path) ?: reply(path).toByteArray()
                    c.getOutputStream().apply {
                        write("HTTP/1.1 $status X\r\nContent-Type: application/json\r\nContent-Length: ${out.size}\r\nConnection: close\r\n\r\n".toByteArray())
                        write(out); flush()
                    }
                }
            }
        }
    }

    override fun close() = socket.close()
}

/** Esperit de vent: the same direction as the web, the pack chosen by the theme, and still without motion. */
@RunWith(RobolectricTestRunner::class)
@Config(sdk = [34], qualifiers = "w393dp-h851dp")
@GraphicsMode(GraphicsMode.Mode.NATIVE)
@LooperMode(LooperMode.Mode.PAUSED)
class WindEsperitTest {
    @get:Rule val ui = createComposeRule()

    private fun image(tag: String): Bitmap {
        val bounds = ui.onNodeWithTag(tag).fetchSemanticsNode().boundsInRoot
        lateinit var bitmap: Bitmap
        ui.runOnIdle {
            val activity = androidx.test.runner.lifecycle.ActivityLifecycleMonitorRegistry.getInstance()
                .getActivitiesInStage(androidx.test.runner.lifecycle.Stage.RESUMED).single()
            fun composeView(view: android.view.View): android.view.View? {
                if (view.javaClass.simpleName == "AndroidComposeView") return view
                if (view is android.view.ViewGroup) for (i in 0 until view.childCount) {
                    composeView(view.getChildAt(i))?.let { return it }
                }
                return null
            }
            val view = requireNotNull(composeView(activity.window.decorView))
            val root = Bitmap.createBitmap(view.width, view.height, Bitmap.Config.ARGB_8888)
            view.draw(android.graphics.Canvas(root))
            bitmap = Bitmap.createBitmap(root, bounds.left.toInt(), bounds.top.toInt(), bounds.width.toInt(), bounds.height.toInt())
        }
        return bitmap
    }

    private fun identical(a: Bitmap, b: Bitmap): Boolean {
        if (a.width != b.width || a.height != b.height) return false
        for (y in 0 until a.height) for (x in 0 until a.width) {
            if (a.getPixel(x, y) != b.getPixel(x, y)) return false
        }
        return true
    }

    @Test fun `the eight Catalan winds point where they blow from, by name`() {
        val rose = listOf("Tramuntana" to 0, "Gregal" to 45, "Llevant" to 90, "Xaloc" to 135,
            "Migjorn" to 180, "Garbí" to 225, "Ponent" to 270, "Mestral" to 315)
        for ((name, deg) in rose) assertEquals(name, deg, windDirection("any-id", name))
        assertEquals("Case, accents and spaces aside", 225, windDirection("x", "  GARBI "))
    }

    @Test fun `any other wind takes one of the eight from its id, the same values as the web`() {
        // Pinned in internal/web/app/wind-face.test.cjs.
        assertEquals(225, windDirection("bit", "Bit"))
        assertEquals(315, windDirection("teca", "Teca"))
        assertEquals(135, windDirection("llevant-2", "Llevant 2"))
        assertEquals(90, windDirection("espurna", "Espurna"))
        assertEquals(windDirection("espurna", "Espurna"), windDirection("espurna", "Un altre nom"))
        assertEquals("A wind not saved yet points where its id will", windDirection("llevant-2", ""), windDirection("", "Llevant 2"))
    }

    @Test fun `the theme from GET api winds picks the pack of every face, or each wind's own in Free`() {
        val response = """{"avatarTheme":"%s","winds":[{"id":"bit","name":"Bit","color":"#29ADFF","role":"r"},
            {"id":"teca","name":"Teca","color":"#29ADFF","role":"r","avatar":"esperit"}]}"""
        fun load(theme: String): GregalViewModel {
            val vm = GregalViewModel(ApplicationProvider.getApplicationContext<Application>())
            FakeGregal { if (it == "/api/winds") response.format(theme) else "{}" }.use { server ->
                vm.setConn(Conn("127.0.0.1", server.port, "token", "alice"))
                vm.loadWinds()
                repeat(100) { if (vm.winds.value.isEmpty()) { Thread.sleep(20); shadowOf(Looper.getMainLooper()).idle() } }
            }
            assertEquals(2, vm.winds.value.size)
            return vm
        }
        val themed = load("esperit")
        assertEquals("esperit", themed.avatarTheme.value)
        assertEquals(listOf("", "esperit"), themed.winds.value.map { it.avatar })
        assertEquals(listOf("esperit", "esperit"), themed.winds.value.map { windPack(themed.avatarTheme.value, it.faceStyle) })
        val free = load("free")
        assertEquals(listOf("folk", "esperit"), free.winds.value.map { windPack(free.avatarTheme.value, it.faceStyle) })
        assertEquals("An unknown theme is Llegendes", "folk", load("rainbow").avatarTheme.value)

        // WindFace draws what windPack says: Bit in Free (no pack of its own) is Llegendes, not Cares; Esperit is neither.
        val bit = free.winds.value.first()
        ui.setContent {
            CompositionLocalProvider(LocalWindMotion provides false) {
                Row(Modifier.background(Color.Black)) {
                    for (theme in listOf("free", "cares", "esperit")) {
                        CompositionLocalProvider(LocalAvatarTheme provides theme) {
                            Box(Modifier.testTag(theme)) { WindFace(size = 96, accent = Color(bit.argb), style = bit.faceStyle) }
                        }
                    }
                }
            }
        }
        assertFalse(identical(image("free"), image("cares")))
        assertFalse(identical(image("esperit"), image("cares")))
    }

    // Amb el moviment apagat, ni la feina el mou (com el web i el mapa).
    @Test fun `with motion off an Esperit is still, idle or working`() {
        ui.mainClock.autoAdvance = false
        val style = Wind("bit", "Bit", "#29ADFF", "r", avatar = "esperit").faceStyle
        ui.setContent {
            CompositionLocalProvider(LocalWindMotion provides false, LocalAvatarTheme provides "free") {
                Row(Modifier.background(Color.Black)) {
                    Box(Modifier.testTag("idle")) { WindFace(size = 96, accent = Color(0xFF29ADFF), style = style) }
                    Box(Modifier.testTag("working")) { WindFace(size = 96, mood = WindMood.WORKING, accent = Color(0xFF29ADFF), style = style) }
                }
            }
        }
        ui.mainClock.advanceTimeBy(100)
        val idle = image("idle")
        val working = image("working")
        ui.mainClock.advanceTimeBy(300)
        assertTrue("Without motion an idle face does not move", identical(idle, image("idle")))
        // The left half holds Bit's tails (south-west) and not the mood badge (top right).
        fun tails(b: Bitmap) = Bitmap.createBitmap(b, 0, 0, b.width / 2, b.height)
        assertTrue("Without motion work does not move it either", identical(tails(working), tails(image("working"))))
    }
}
