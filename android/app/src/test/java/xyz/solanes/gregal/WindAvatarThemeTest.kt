package xyz.solanes.gregal

import android.app.Application
import android.os.Looper
import java.time.Duration
import org.robolectric.Shadows.shadowOf
import androidx.compose.material3.MaterialTheme
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.ui.test.*
import androidx.compose.ui.test.junit4.createComposeRule
import androidx.test.core.app.ApplicationProvider
import org.json.JSONObject
import org.junit.Assert.*
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config
import org.robolectric.annotation.GraphicsMode
import org.robolectric.annotation.LooperMode

/** «Avatars» in Preferences saves the account's theme; the wind editor offers a pack only in Free mode. */
@RunWith(RobolectricTestRunner::class)
@Config(sdk = [34], qualifiers = "w393dp-h851dp")
@GraphicsMode(GraphicsMode.Mode.NATIVE)
@LooperMode(LooperMode.Mode.PAUSED)
class WindAvatarThemeTest {
    @get:Rule val ui = createComposeRule()

    @Test fun `Avatars in Preferences offers Cares, Esperit de vent and Free and saves the theme`() {
        FakeGregal { """{"theme":"esperit"}""" }.use { server ->
            val vm = GregalViewModel(ApplicationProvider.getApplicationContext<Application>())
            vm.setConn(Conn("127.0.0.1", server.port, "token", "alice"))
            vm.openPreferences(PrefsPage.Appearance)
            ui.setContent {
                CompositionLocalProvider(LocalPal provides LightPal, LocalWindMotion provides false) {
                    MaterialTheme(colorScheme = LightPal.scheme()) { PrefsScreen(vm) {} }
                }
            }
            val lang = vm.idioma.value
            ui.onNodeWithText(tr(lang, "winds.avatars")).performScrollTo().assertIsDisplayed()
            ui.onNodeWithText(tr(lang, "winds.avatarCares")).assertIsSelected()
            ui.onNodeWithText(tr(lang, "winds.avatarFree")).assertIsNotSelected()
            ui.onNodeWithText(tr(lang, "winds.avatarEsperit")).performScrollTo().performClick()
            repeat(150) { if (vm.avatarTheme.value != "esperit") { shadowOf(Looper.getMainLooper()).idleFor(Duration.ofMillis(20)); Thread.sleep(5) } }
            assertEquals("esperit", vm.avatarTheme.value)
            ui.onNodeWithText(tr(lang, "winds.avatarEsperit")).assertIsSelected()
            val post = server.requests.single { it.startsWith("POST /api/winds/avatar-theme ") }
            assertEquals("esperit", JSONObject(post.substringAfter("avatar-theme ")).getString("theme"))
        }
    }

    private fun editor(theme: String, edit: Wind?, onSave: (WindDraft) -> Unit) {
        ui.setContent {
            CompositionLocalProvider(LocalPal provides LightPal, LocalWindMotion provides false, LocalAvatarTheme provides theme) {
                MaterialTheme(colorScheme = LightPal.scheme()) {
                    WindEditor("ca", edit, {}, { _, _, _, _ -> }, { draft, _ -> onSave(draft) })
                }
            }
        }
    }

    @Test fun `in Free mode the editor offers the pack and saves it with the wind`() {
        var saved: WindDraft? = null
        editor("free", Wind("bit", "Bit", "#29ADFF", "lectura")) { saved = it }
        ui.onNodeWithText(tr("ca", "winds.avatarCares")).performScrollTo().assertIsSelected()
        ui.onNodeWithText(tr("ca", "winds.avatarEsperit")).performScrollTo().performClick()
        ui.onNodeWithText(tr("ca", "app.vents.saveChanges")).performClick()
        ui.runOnIdle { assertEquals("esperit", saved?.avatar) }
        assertEquals("esperit", JSONObject(WindRequests.save("Bit", "#29ADFF", "lectura", avatar = "esperit")).getString("avatar"))
    }

    @Test fun `with a theme the editor names it, offers no pack and keeps the wind's own`() {
        var saved: WindDraft? = null
        editor("esperit", Wind("bit", "Bit", "#29ADFF", "lectura", avatar = "cares")) { saved = it }
        ui.onNodeWithText(tr("ca", "winds.avatarThemeInUse", tr("ca", "winds.avatarEsperit"))).performScrollTo().assertIsDisplayed()
        ui.onNodeWithText(tr("ca", "winds.avatarEsperit")).assertDoesNotExist()
        ui.onNodeWithText(tr("ca", "winds.avatarCares")).assertDoesNotExist()
        ui.onNodeWithText(tr("ca", "app.vents.saveChanges")).performClick()
        ui.runOnIdle { assertEquals("", saved?.avatar) }
        assertFalse("An empty avatar keeps the stored one", JSONObject(WindRequests.save("Bit", "#29ADFF", "lectura")).has("avatar"))
    }
}
