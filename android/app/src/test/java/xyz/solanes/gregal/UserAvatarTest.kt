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
    private fun server(profile: String = """{"prefs":{},"keys":[]}""", photo: String = ""): FakeGregal {
        val stored = JSONObject(profile).getJSONObject("prefs")
        lateinit var fake: FakeGregal
        fake = FakeGregal { path ->
            when {
                path.startsWith("/api/me") -> """{"user":"alice","admin":true,"local":false,"photo":"$photo"}"""
                path.startsWith("/api/profile/photo") ->
                    if (fake.requests.last().startsWith("DELETE ")) """{"photo":""}""" else """{"photo":"/api/profile/photo?v=2"}"""
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

    private fun png(): ByteArray {
        val out = java.io.ByteArrayOutputStream()
        android.graphics.Bitmap.createBitmap(8, 8, android.graphics.Bitmap.Config.ARGB_8888).compress(android.graphics.Bitmap.CompressFormat.PNG, 100, out)
        return out.toByteArray()
    }

    @Test fun `a chosen photo is posted raw to the profile and the drawer shows it`() {
        server().use { server ->
            server.bytes = { path -> if (path.startsWith("/api/profile/photo") && server.requests.last().startsWith("GET ")) png() else null }
            val vm = connected(server, false)
            drawer(vm)
            val photo = png()
            vm.uploadUserPhoto(photo)
            idleUntil { vm.me.value.photo.isNotEmpty() }
            val i = server.requests.indexOfFirst { it.startsWith("POST /api/profile/photo") }
            assertArrayEquals(photo, server.bodies[i])
            assertEquals("/api/profile/photo?v=2", vm.me.value.photo)
            idleUntil { ui.onAllNodesWithTag("user-avatar-photo", useUnmergedTree = true).fetchSemanticsNodes().isNotEmpty() }
            ui.onNodeWithTag("user-avatar-photo", useUnmergedTree = true).assertExists()
        }
    }

    @Test fun `Treu la foto sends DELETE and the initials come back`() {
        server(photo = "/api/profile/photo?v=1").use { server ->
            server.bytes = { path -> if (path.startsWith("/api/profile/photo") && server.requests.last().startsWith("GET ")) png() else null }
            val vm = connected(server, false)
            idleUntil { vm.userPhoto.value != null }
            prefsScreen(vm)
            val lang = vm.idioma.value
            ui.onNodeWithText(tr(lang, "app.prefs.userPhotoUpload")).assertExists()
            ui.onNodeWithText(tr(lang, "app.prefs.userPhotoRemove")).performScrollTo().performClick()
            idleUntil { server.requests.any { it.startsWith("DELETE /api/profile/photo") } && vm.me.value.photo.isEmpty() }
            assertTrue(server.requests.any { it.startsWith("DELETE /api/profile/photo") })
            assertNull(vm.userPhoto.value)
        }
    }

    /** A 2×2 BMP: an image the phone decodes and the server does not take, like HEIC. */
    private fun bmp(): ByteArray {
        val b = java.nio.ByteBuffer.allocate(54 + 16).order(java.nio.ByteOrder.LITTLE_ENDIAN)
        b.put('B'.code.toByte()).put('M'.code.toByte()).putInt(70).putInt(0).putInt(54)
        b.putInt(40).putInt(2).putInt(2).putShort(1).putShort(24).putInt(0).putInt(16).putInt(2835).putInt(2835).putInt(0).putInt(0)
        repeat(2) { b.put(ByteArray(6) { 0x7f }).put(ByteArray(2)) }
        return b.array()
    }

    @Test fun `a photo in a format the server does not take is sent as a JPEG and a JPEG goes as it is`() {
        val (jpeg, type) = requireNotNull(photoForUpload(bmp()))
        assertEquals("image/jpeg", type)
        assertEquals(listOf(0xFF, 0xD8, 0xFF), jpeg.take(3).map { it.toInt() and 0xFF })
        assertEquals("image/jpeg", photoForUpload(jpeg)?.second)
        assertSame(jpeg, photoForUpload(jpeg)?.first)
        assertEquals("image/png", photoForUpload(png())?.second)
        assertNull(photoForUpload("not an image".toByteArray()))
    }

    @Test fun `a JPEG over 5 MB is sent as a smaller JPEG at most 1024 px wide`() {
        val out = java.io.ByteArrayOutputStream()
        android.graphics.Bitmap.createBitmap(3000, 1500, android.graphics.Bitmap.Config.ARGB_8888).compress(android.graphics.Bitmap.CompressFormat.JPEG, 90, out)
        // Bytes after the end of the image, as a big camera file has metadata: decoders ignore them.
        val big = out.toByteArray() + ByteArray(5 * 1024 * 1024)
        val (jpeg, type) = requireNotNull(photoForUpload(big))
        assertEquals("image/jpeg", type)
        assertNotSame(big, jpeg)
        assertTrue(jpeg.size < 5 * 1024 * 1024)
        val bounds = android.graphics.BitmapFactory.Options().apply { inJustDecodeBounds = true }
        android.graphics.BitmapFactory.decodeByteArray(jpeg, 0, jpeg.size, bounds)
        assertEquals(750, bounds.outWidth)
        assertEquals(375, bounds.outHeight)
    }

    @Test fun `the drawer shows the photo from api me round instead of the initials`() {
        server(photo = "/api/profile/photo?v=1").use { server ->
            server.bytes = { path -> if (path.startsWith("/api/profile/photo") && server.requests.last().startsWith("GET ")) png() else null }
            val vm = connected(server, true)
            drawer(vm)
            idleUntil { ui.onAllNodesWithTag("user-avatar-photo", useUnmergedTree = true).fetchSemanticsNodes().isNotEmpty() }
            ui.onNodeWithTag("user-avatar-photo", useUnmergedTree = true).assertExists()
            ui.onNodeWithTag("user-avatar-initials", useUnmergedTree = true).assertDoesNotExist()
        }
    }

    @Test fun `a photo that does not load leaves the initials`() {
        server(photo = "/api/profile/photo?v=1").use { server ->
            server.bytes = { path -> if (path.startsWith("/api/profile/photo") && server.requests.last().startsWith("GET ")) "not an image".toByteArray() else null }
            val vm = connected(server, true)
            drawer(vm)
            idleUntil { server.requests.any { it.startsWith("GET /api/profile/photo") } }
            repeat(20) { shadowOf(Looper.getMainLooper()).idleFor(Duration.ofMillis(20)); Thread.sleep(5) }
            assertEquals("/api/profile/photo?v=1", vm.me.value.photo)
            ui.onNodeWithTag("user-avatar-photo", useUnmergedTree = true).assertDoesNotExist()
            ui.onNodeWithTag("user-avatar-initials", useUnmergedTree = true).assertExists()
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
