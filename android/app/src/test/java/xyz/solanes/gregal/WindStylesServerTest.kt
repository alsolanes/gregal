package xyz.solanes.gregal

import android.app.Application
import android.graphics.Bitmap
import android.os.Looper
import androidx.test.core.app.ApplicationProvider
import java.time.Duration
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.Shadows.shadowOf
import org.robolectric.annotation.Config
import org.robolectric.annotation.GraphicsMode
import org.robolectric.annotation.LooperMode

/** Each wind looks on the phone as the server says it looks on the desktop (task 141). */
@RunWith(RobolectricTestRunner::class)
@Config(sdk = [34])
@GraphicsMode(GraphicsMode.Mode.NATIVE)
@LooperMode(LooperMode.Mode.PAUSED)
class WindStylesServerTest {
    // What a server built from this branch answers to GET /api/winds before and after
    // the desktop picks Free and gives Tramuntana its own pack, eyes, mouth and shape.
    private val before = """{"avatarTheme":"cares","winds":[{"avatar":"","color":"#3B82F6","followups":"on","id":"tramuntana","kit":"research","name":"Tramuntana","role":"Recerca","specialty":"","squad":"","style":"pso"},
        {"avatar":"","color":"#EF4444","followups":"on","id":"mestral","kit":"","name":"Mestral","role":"Escriptura","specialty":"","squad":"","style":"pso"}]}"""
    private val after = """{"avatarTheme":"free","winds":[{"avatar":"esperit","color":"#3B82F6","followups":"on","id":"tramuntana","kit":"research","name":"Tramuntana","role":"Recerca","specialty":"","squad":"","style":"bns"},
        {"avatar":"minairo","color":"#EF4444","followups":"on","id":"mestral","kit":"","name":"Mestral","role":"Escriptura","specialty":"","squad":"","style":"moom"}]}"""

    private fun idleUntil(done: () -> Boolean) {
        repeat(200) { if (!done()) { shadowOf(Looper.getMainLooper()).idleFor(Duration.ofMillis(50)); Thread.sleep(5) } }
    }

    @Test fun `a look changed on the desktop shows when the app comes back to the foreground`() {
        var winds = before
        FakeGregal { if (it == "/api/winds") winds else "{}" }.use { server ->
            val app = ApplicationProvider.getApplicationContext<Application>()
            val vm = GregalViewModel(app)
            vm.attachPrefs(Prefs(app))
            vm.setConn(Conn("127.0.0.1", server.port, "token", "alice"))
            vm.connect()
            idleUntil { vm.connected.value && vm.winds.value.size == 2 }
            assertEquals("cares", vm.avatarTheme.value)

            // The person changes the theme and the winds on the desktop, then opens the phone again.
            winds = after
            vm.onForegrounded()
            idleUntil { vm.avatarTheme.value == "free" }
            assertEquals("free", vm.avatarTheme.value)
            val (tramuntana, mestral) = vm.winds.value
            assertEquals("esperit", windPack(vm.avatarTheme.value, tramuntana.faceStyle))
            assertEquals(WindEyes.WIDE, tramuntana.faceStyle.eyes)
            assertEquals(WindShape.SPIRAL, tramuntana.faceStyle.shape)
            assertEquals("minairo", windPack(vm.avatarTheme.value, mestral.faceStyle))
            assertEquals(WindMouth.OPEN, mestral.faceStyle.mouth)
            assertEquals("m", mestral.faceStyle.barretina)
        }
    }

    private fun identical(a: Bitmap, b: Bitmap): Boolean {
        if (a.width != b.width || a.height != b.height) return false
        for (y in 0 until a.height) for (x in 0 until a.width) if (a.getPixel(x, y) != b.getPixel(x, y)) return false
        return true
    }

    @Test fun `a notification draws the wind with its pack, as the list does`() {
        fun face(avatar: String, theme: String) = windFaceBitmap(Wind("bit", "Bit", "#29ADFF", "r", avatar = avatar), theme)
        val cares = face("cares", "free")
        for (pack in listOf("esperit", "cap", "rosa", "mapa", "minairo")) {
            val own = face(pack, "free")
            assertFalse("a $pack wind in Free is not drawn as Cares", identical(own, cares))
            assertTrue("the $pack theme draws every wind as $pack", identical(own, face("cares", pack)))
        }
        assertTrue("the Cares theme draws an Esperit wind as Cares", identical(cares, face("esperit", "cares")))
    }
}
