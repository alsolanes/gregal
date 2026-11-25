package xyz.solanes.gregal

import android.graphics.Bitmap
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.material3.MaterialTheme
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.test.*
import androidx.compose.ui.test.junit4.createComposeRule
import java.time.OffsetDateTime
import java.time.ZoneId
import java.time.temporal.ChronoUnit
import org.junit.Assert.*
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config
import org.robolectric.annotation.GraphicsMode
import org.robolectric.annotation.LooperMode

/** La ullada de dalt d'«Avui»: el temps, el calendari i el que segueixes, amb dades d'exemple. */
@RunWith(RobolectricTestRunner::class)
@Config(sdk = [34], qualifiers = "w393dp-h1500dp")
@GraphicsMode(GraphicsMode.Mode.NATIVE)
@LooperMode(LooperMode.Mode.PAUSED)
class AvuiGlanceUiTest {
    @get:Rule val ui = createComposeRule()

    private val winds = listOf(Wind("gregal", "Gregal", "#4F5BE8", "notícies"), Wind("xaloc", "Xaloc", "#EFA68F", "agenda"))

    // Relatiu al rellotge: el compte enrere és el de veritat.
    private val inTwoHours = OffsetDateTime.now().plusHours(2).plusSeconds(30).truncatedTo(ChronoUnit.SECONDS).toString()
    private val today = java.time.LocalDate.now(ZoneId.systemDefault())

    private fun glance(weather: Boolean = true, events: Boolean = true, follows: Boolean = true) = AvuiGlance(
        weather = if (weather) AvuiWeather("Girona", 13.6, 8.2, 17.0, 3.4, listOf("16", "17", "18"), 61, "07:52", "17:21") else null,
        events = if (!events) emptyList() else listOf(
            AvuiEvent("${today}T23:30:00${OffsetDateTime.now().offset}", "", "Trucada amb en Pau", "Feina"),
            AvuiEvent(today.plusDays(1).atStartOfDay(ZoneId.systemDefault()).toOffsetDateTime().toString(),
                today.plusDays(2).atStartOfDay(ZoneId.systemDefault()).toOffsetDateTime().toString(), "Aniversari de la Marta", "Família"),
        ),
        follows = if (!follows) emptyList() else listOf(
            AvuiFollow("f1", "team", "Barça", title = "Barça – Madrid", at = inTwoHours, detail = "Lliga, jornada 14", url = "https://example.com/partit"),
            AvuiFollow("f2", "other", "Fórmula 1", wind = "xaloc", title = "GP d'Abu Dhabi", detail = "L'última del curs"),
            AvuiFollow("f3", "team", "Girona FC"),
        ),
    )

    private fun view(g: AvuiGlance, place: String = "Girona") = AvuiView(
        "2026-11-24", emptyList(), emptyList(), emptyList(), emptyList(), 0,
        AvuiSettings(place = place, follows = g.follows.map { AvuiFollow(it.id, it.kind, it.name, it.wind) }), g,
    )

    private val follows = mutableListOf<Pair<String, String>>()
    private val unfollows = mutableListOf<String>()
    private val places = mutableListOf<String>()

    private fun show(v: AvuiView, pal: GregalPalette = DarkPal, motion: Boolean = false) {
        ui.setContent {
            CompositionLocalProvider(LocalWindMotion provides motion, LocalPal provides pal) {
                MaterialTheme(colorScheme = pal.scheme()) {
                    Box(Modifier.fillMaxSize().background(pal.surface).testTag("shot")) {
                        AvuiTray("ca", v, winds, false, { _, _ -> }, { _, _ -> }, { _, _ -> }, {},
                            onFollow = { k, n -> follows += k to n }, onUnfollow = { unfollows += it }, onPlace = { places += it })
                    }
                }
            }
        }
    }

    @Test fun laTargetaDelTemps() {
        show(view(glance(events = false, follows = false)))
        ui.onNodeWithTag("glance-weather").assertExists()
        ui.onNodeWithText("14°").assertExists()
        ui.onNodeWithText("Pluja").assertExists()
        ui.onNodeWithText("↑ 17°  ↓ 8°").assertExists()
        ui.onNodeWithText("16 h").assertExists(); ui.onNodeWithText("18 h").assertExists()
        ui.onNodeWithText("07:52").assertExists(); ui.onNodeWithText("17:21").assertExists()
        ui.onNodeWithText("3 mm").assertExists()
    }

    @Test fun elCalendariDAvuiIDeDema() {
        show(view(glance(weather = false, follows = false)))
        ui.onNodeWithTag("glance-events").assertExists()
        ui.onNodeWithText("CALENDARI").assertExists()
        ui.onNodeWithText("23:30").assertExists()
        ui.onNodeWithText("Trucada amb en Pau").assertExists()
        ui.onNodeWithText("Demà").assertExists()
        ui.onNodeWithText("Tot el dia").assertExists()
        ui.onNodeWithText("Aniversari de la Marta").assertExists()
    }

    @Test fun unaTargetaPerCadaCosaQueSegueixes() {
        show(view(glance(weather = false, events = false)))
        ui.onNodeWithTag("glance-follow-f1").assertExists()
        ui.onNodeWithText("Barça – Madrid").assertExists()
        ui.onNodeWithText("d’aquí a 2 h").assertExists()
        ui.onNodeWithText("Lliga, jornada 14").assertExists()
        ui.onNodeWithText("GP d'Abu Dhabi").assertExists()
        // Encara sense resposta: el nom i que ho està buscant.
        ui.onNodeWithTag("glance-follow-f3").assertExists()
        ui.onNodeWithText("Ho estic buscant…").assertExists()
        ui.onNodeWithContentDescription("Deixa de seguir Barça").performClick()
        assertEquals(listOf("f1"), unfollows)
    }

    @Test fun segueixUnaCosaNovaDesDeLaPantalla() {
        show(view(glance()))
        ui.onNodeWithTag("glance-add").performScrollTo().performClick()
        ui.onNodeWithTag("glance-follow-name").performTextInput("Girona FC")
        ui.onNodeWithText("Un equip").performClick()
        ui.onNodeWithTag("glance-follow-ok").performClick()
        assertEquals(listOf("team" to "Girona FC"), follows)
    }

    @Test fun posaElLlocDesDeLaPantalla() {
        show(view(glance(weather = false, events = false, follows = false), place = ""))
        ui.onNodeWithText("On ets? Digues-me la ciutat i et posaré el temps aquí.").assertExists()
        ui.onNodeWithTag("glance-follow-empty").assertExists()
        ui.onNodeWithTag("glance-place").performClick()
        ui.onNodeWithTag("glance-place-name").performTextInput("Olot")
        ui.onNodeWithTag("glance-place-ok").performClick()
        assertEquals(listOf("Olot"), places)
    }

    @Test fun laTeclaDelTecladoTambeDesa() {
        show(view(glance(weather = false, events = false, follows = false), place = ""))
        ui.onNodeWithTag("glance-add").performScrollTo().performClick()
        ui.onNodeWithTag("glance-follow-name").performTextInput("Barça")
        ui.onNodeWithTag("glance-follow-name").performImeAction()
        assertEquals(listOf("team" to "Barça"), follows)
        ui.onNodeWithTag("glance-place").performClick()
        ui.onNodeWithTag("glance-place-name").performTextInput("Girona")
        ui.onNodeWithTag("glance-place-name").performImeAction()
        assertEquals(listOf("Girona"), places)
    }

    @Test fun senseResultatsOAmbErrorHoDiu() {
        val g = AvuiGlance(follows = listOf(
            AvuiFollow("f1", "team", "Barça", none = true),
            AvuiFollow("f2", "other", "Eclipsi", none = true),
            AvuiFollow("f3", "team", "Girona FC", error = "No ho he pogut buscar: sense xarxa"),
        ), weatherError = "no he trobat el temps de «Xyz»")
        show(view(g, place = "Xyz"))
        ui.onNodeWithText("No he trobat partits per a Barça.").assertExists()
        ui.onNodeWithText("No he trobat res sobre Eclipsi.").assertExists()
        ui.onNodeWithText("⚠ No ho he pogut buscar: sense xarxa").assertExists()
        ui.onNodeWithText("⚠ no he trobat el temps de «Xyz»").assertExists()
    }

    @Test fun unErrorEnDesarEsVeu() {
        ui.setContent {
            CompositionLocalProvider(LocalWindMotion provides false, LocalPal provides DarkPal) {
                MaterialTheme(colorScheme = DarkPal.scheme()) {
                    AvuiTray("ca", view(glance()), winds, false, { _, _ -> }, { _, _ -> }, { _, _ -> }, {},
                        glanceMessage = "digues què vols seguir")
                }
            }
        }
        ui.onNodeWithText("⚠ digues què vols seguir").assertExists()
    }

    @Test fun senseMovimentTotQuiet() {
        show(view(glance()))
        // Sense moviment no hi ha cap animació infinita: la prova arriba a repòs.
        ui.waitForIdle()
        ui.onNodeWithTag("glance-weather").assertExists()
    }

    private fun capture(tag: String): Bitmap {
        val bounds = ui.onNodeWithTag(tag).fetchSemanticsNode().boundsInRoot
        lateinit var bitmap: Bitmap
        ui.runOnIdle {
            val activity = androidx.test.runner.lifecycle.ActivityLifecycleMonitorRegistry.getInstance()
                .getActivitiesInStage(androidx.test.runner.lifecycle.Stage.RESUMED).single()
            fun composeView(view: android.view.View): android.view.View? {
                if (view.javaClass.simpleName == "AndroidComposeView") return view
                if (view is android.view.ViewGroup) for (i in 0 until view.childCount) composeView(view.getChildAt(i))?.let { return it }
                return null
            }
            val view = requireNotNull(composeView(activity.window.decorView))
            val root = Bitmap.createBitmap(view.width, view.height, Bitmap.Config.ARGB_8888)
            view.draw(android.graphics.Canvas(root))
            bitmap = Bitmap.createBitmap(root, bounds.left.toInt(), bounds.top.toInt(), bounds.width.toInt(), bounds.height.toInt())
        }
        return bitmap
    }

    private fun save(shot: Bitmap, name: String, dir: String = "107") {
        java.io.File("../../output/ui-review/$dir/$name.png").also { it.parentFile!!.mkdirs() }.outputStream().use {
            shot.compress(Bitmap.CompressFormat.PNG, 100, it)
        }
    }

    /** Uns quants fotogrames amb moviment (entrada amb molla, la pluja que cau) a output/ui-review/107/. */
    @Test fun fotogramesAmbMoviment() {
        ui.mainClock.autoAdvance = false
        show(view(glance()), DarkPal, motion = true)
        var t = 0L
        listOf(16L, 140L, 400L, 1500L).forEach { at ->
            ui.mainClock.advanceTimeBy(at - t); t = at
            save(capture("shot"), "glance-dark-${at}ms")
        }
    }

    @Test fun fotogramaClar() {
        show(view(glance()), LightPal)
        save(capture("shot"), "glance-light")
    }

    /** Tasca 140: un seguiment amb partit, un sense, un que ha fallat i el temps, a output/ui-review/140/. */
    private val glance140 get() = glance(events = false).let { it.copy(follows = listOf(it.follows[0],
        AvuiFollow("f4", "team", "Girona FC", none = true), AvuiFollow("f5", "other", "Eclipsi", error = "No ho he pogut buscar: sense xarxa"))) }

    @Test fun fotogramaSeguimentsITempsClar() {
        show(view(glance140), LightPal)
        save(capture("shot"), "android-avui-light", "140")
    }

    @Test fun fotogramaSeguimentsITempsFosc() {
        show(view(glance140), DarkPal)
        save(capture("shot"), "android-avui-dark", "140")
    }

    @Test fun fotogramaBuit() {
        show(view(glance(weather = false, events = false, follows = false), place = ""), LightPal)
        save(capture("shot"), "glance-empty-light")
    }
}
