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

/** Your name and initials at the foot of the drawer, and «Com et dius» in Preferences, shared with the web through /api/profile. */
@RunWith(RobolectricTestRunner::class)
@Config(sdk = [34], qualifiers = "w393dp-h851dp")
@GraphicsMode(GraphicsMode.Mode.NATIVE)
@LooperMode(LooperMode.Mode.PAUSED)
class UserAvatarTest {
    @get:Rule val ui = createComposeRule()

    private fun idleUntil(done: () -> Boolean) {
        repeat(300) { if (!done()) { shadowOf(Looper.getMainLooper()).idleFor(Duration.ofMillis(20)); Thread.sleep(5) } }
    }

    /** A server whose POST /api/profile answers the saved keys merged into `profile`, as profile.go does. */
    private fun server(profile: String = """{"prefs":{},"keys":[]}"""): FakeGregal {
        val stored = JSONObject(profile).getJSONObject("prefs")
        lateinit var fake: FakeGregal
        fake = FakeGregal { path ->
            when {
                path.startsWith("/api/me") -> """{"user":"alice","admin":true,"local":false}"""
                path.startsWith("/api/profile") -> {
                    val last = fake.requests.last()
                    if (last.startsWith("POST ")) JSONObject(last.substringAfter("/api/profile ")).getJSONObject("prefs").let { sent ->
                        sent.keys().forEach { stored.put(it, sent.get(it)) }
                    }
                    JSONObject().put("prefs", stored).toString()
                }
                else -> "{}"
            }
        }
        return fake
    }

    private fun prefsScreen(vm: GregalViewModel) {
        vm.openPreferences(PrefsPage.General)
        ui.setContent {
            CompositionLocalProvider(LocalPal provides LightPal, LocalWindMotion provides false) {
                MaterialTheme(colorScheme = LightPal.scheme()) { PrefsScreen(vm) {} }
            }
        }
    }

    private fun connected(server: FakeGregal, simple: Boolean): GregalViewModel {
        val app = ApplicationProvider.getApplicationContext<Application>()
        val vm = GregalViewModel(app)
        vm.attachPrefs(Prefs(app))
        vm.setSimpleVents(simple)
        vm.setConn(Conn("127.0.0.1", server.port, "token", "alice"))
        vm.connect()
        idleUntil { server.requests.any { it.startsWith("GET /api/profile") } && vm.connected.value }
        return vm
    }

    private fun drawer(vm: GregalViewModel) {
        ui.setContent {
            CompositionLocalProvider(LocalPal provides LightPal, LocalWindMotion provides false) {
                MaterialTheme(colorScheme = LightPal.scheme()) {
                    GregalDrawerContent(vm, "ca", Screen.Company, false, "127.0.0.1", "", "", { vm.go(it) }, {})
                }
            }
        }
    }

    private fun opensPreferencesAtTheName(simple: Boolean) {
        server().use { server ->
            val vm = connected(server, simple)
            drawer(vm)
            idleUntil { ui.onAllNodes(hasTestTag("user-avatar") and hasText("alice")).fetchSemanticsNodes().isNotEmpty() }
            ui.onNodeWithTag("user-avatar").assert(hasText("alice")).performClick()
            assertEquals(Screen.Prefs, vm.screen.value)
            assertEquals(PrefsPage.General, vm.prefsPage.value)
        }
    }

    @Test fun `the Codi drawer shows the account name when none is saved and the row opens Preferences at the name`() =
        opensPreferencesAtTheName(simple = false)

    @Test fun `the Vents drawer shows the account name when none is saved and the row opens Preferences at the name`() =
        opensPreferencesAtTheName(simple = true)

    @Test fun `a name saved in the profile wins over the account name`() {
        server("""{"prefs":{"gregal_user_name":{"v":"Maria del Mar Puig","t":1},"gregal_user_color":{"v":"#EFA68F","t":1}}}""").use { server ->
            val vm = connected(server, true)
            idleUntil { vm.userName.value.isNotEmpty() }
            assertEquals("#EFA68F", vm.userColor.value)
            drawer(vm)
            ui.onNodeWithTag("user-avatar").assert(hasText("Maria del Mar Puig"))
        }
    }

    @Test fun `Com et dius and the avatar colour are saved to the profile in one POST`() {
        server().use { server ->
            val vm = connected(server, false)
            prefsScreen(vm)
            val lang = vm.idioma.value
            ui.onNodeWithText(tr(lang, "app.prefs.userName")).performTextInput("Maria del Mar Puig")
            ui.onNodeWithContentDescription("Xaloc").performClick()
            ui.onNodeWithText(tr(lang, "app.prefs.userSave")).performClick()
            idleUntil { vm.userName.value.isNotEmpty() }
            val prefs = JSONObject(server.requests.single { it.startsWith("POST /api/profile") }.substringAfter("/api/profile ")).getJSONObject("prefs")
            assertEquals("Maria del Mar Puig", prefs.getJSONObject("gregal_user_name").getString("v"))
            assertEquals("#EFA68F", prefs.getJSONObject("gregal_user_color").getString("v"))
            assertTrue(prefs.getJSONObject("gregal_user_name").getLong("t") > 1_700_000_000_000)
            assertEquals("Maria del Mar Puig", vm.userName.value)
        }
    }

    @Test fun `changing only the name leaves the colour out of the POST so a newer one from the web is kept`() {
        server("""{"prefs":{"gregal_user_color":{"v":"#EFA68F","t":1}}}""").use { server ->
            val vm = connected(server, false)
            idleUntil { vm.userColor.value == "#EFA68F" }
            prefsScreen(vm)
            val lang = vm.idioma.value
            ui.onNodeWithText(tr(lang, "app.prefs.userName")).performTextInput("Mar")
            ui.onNodeWithText(tr(lang, "app.prefs.userSave")).performClick()
            idleUntil { vm.userName.value.isNotEmpty() }
            val prefs = JSONObject(server.requests.single { it.startsWith("POST /api/profile") }.substringAfter("/api/profile ")).getJSONObject("prefs")
            assertEquals("Mar", prefs.getJSONObject("gregal_user_name").getString("v"))
            assertFalse(prefs.has("gregal_user_color"))
            assertEquals("#EFA68F", vm.userColor.value)
            ui.onNodeWithText(tr(lang, "app.prefs.userSaved")).assertIsDisplayed()
        }
    }

    @Test fun `a save the server refuses does not say Saved`() {
        FakeGregal(500) { "boom" }.use { server ->
            val vm = GregalViewModel(ApplicationProvider.getApplicationContext<Application>())
            vm.setConn(Conn("127.0.0.1", server.port, "token", "alice"))
            prefsScreen(vm)
            val lang = vm.idioma.value
            ui.onNodeWithText(tr(lang, "app.prefs.userName")).performTextInput("Mar")
            ui.onNodeWithText(tr(lang, "app.prefs.userSave")).performClick()
            idleUntil { server.requests.any { it.startsWith("POST /api/profile") } }
            repeat(20) { shadowOf(Looper.getMainLooper()).idleFor(Duration.ofMillis(20)); Thread.sleep(5) }
            ui.onNodeWithText(tr(lang, "app.prefs.userSaved")).assertDoesNotExist()
            assertEquals("", vm.userName.value)
        }
    }

    @Test fun `the name falls back like the web and the initials take the first and last words`() {
        assertEquals("Mar", userDisplayName(" Mar ", "alice", false, "ca"))
        assertEquals("alice", userDisplayName("", "alice", false, "ca"))
        assertEquals("Tu", userDisplayName("", "alice", true, "ca"))
        assertEquals("You", userDisplayName("", "", false, "en"))
        assertEquals("MP", userInitials("Maria del Mar Puig"))
        assertEquals("À", userInitials("àlex"))
        assertEquals("", userInitials("  "))
        assertEquals("MP", userInitials("Maria Puig"))
    }
}
