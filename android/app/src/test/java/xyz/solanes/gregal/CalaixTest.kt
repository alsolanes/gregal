package xyz.solanes.gregal

import android.graphics.Bitmap
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.MaterialTheme
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.test.*
import androidx.compose.ui.test.junit4.createComposeRule
import androidx.compose.ui.unit.dp
import org.junit.Assert.*
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config
import org.robolectric.annotation.GraphicsMode
import org.robolectric.annotation.LooperMode
import java.time.LocalDate
import java.time.LocalDateTime

/** Dades d'exemple compartides: un calaix de cada mena i una previsió de 48 h i 7 dies. */
internal object CalaixSamples {
    val drawer = """{"shelves":[
      {"name":"llocs","title":"Llocs","kind":"places","updated":1,"items":[
        {"name":"Girona","lat":41.98,"lon":2.82,"primary":true},{"name":"Cadaqués","lat":42.29,"lon":3.28}]},
      {"name":"classificacio","title":"Classificació","kind":"table","items":[
        {"id":"fcb","equip":"Barça","pj":12,"punts":31.0},{"id":"gir","equip":"Girona","pj":12,"punts":24.0}]},
      {"name":"llancaments","title":"Llançaments","kind":"releases","items":[
        {"date":"2026-11-21","title":"Nou disc","artist":"Manel","type":"àlbum"},
        {"date":"2026-12-05","title":"Gira","artist":"Txarango","type":"concert"},
        {"date":"2026-10-02","title":"Senzill","artist":"Oques Grasses","type":"senzill"},
        {"title":"Sense data"}]},
      {"name":"receptari","title":"Receptari","kind":"recipes","items":[
        {"title":"Escalivada","servings":4,"time":"1 h","ingredients":["2 albergínies","2 pebrots","1 ceba"],
         "steps":["Escalivar al forn","Pelar i tallar"],"tips":"Amb pa de pagès","source":"Llevantada","reviewed":"2026-11-08"}]},
      {"name":"artistes","title":"Artistes","kind":"list","items":[{"name":"Manel"},{"name":"Rosalía"}]},
      {"name":"rar","title":"Rar","kind":"cosa","items":[]}]}"""

    fun weather(): String {
        val start = LocalDateTime.of(2026, 11, 10, 0, 0)
        val times = (0 until 48).map { start.plusHours(it.toLong()).toString() }
        fun arr(f: (Int) -> Any) = (0 until 48).joinToString(",", "[", "]") { f(it).toString() }
        val temps = arr { 9 + 7 * Math.sin((it - 9) / 24.0 * 2 * Math.PI) }
        val rain = arr { if (it in 16..21) (it - 15) * 14 else 0 }
        val days = (0 until 7).map { LocalDate.of(2026, 11, 10).plusDays(it.toLong()) }
        return """{"place":{"name":"Girona","lat":41.98,"lon":2.82},"forecast":{
          "current":{"time":"2026-11-10T09:15","temperature_2m":12.4,"apparent_temperature":10.9,"weather_code":2,"wind_speed_10m":14.2,"is_day":1},
          "hourly":{"time":${times.joinToString(",", "[", "]") { "\"$it\"" }},"temperature_2m":$temps,"precipitation_probability":$rain,
            "precipitation":${arr { 0 }},"weather_code":${arr { 2 }},"wind_speed_10m":${arr { 10 }},"wind_gusts_10m":${arr { 20 }}},
          "daily":{"time":${days.joinToString(",", "[", "]") { "\"$it\"" }},"weather_code":[2,61,3,0,0,80,95],
            "temperature_2m_max":[16,14,13,17,18,15,12],"temperature_2m_min":[6,8,7,5,6,9,8],
            "precipitation_probability_max":[20,80,30,0,0,60,90],"precipitation_sum":[0,6,1,0,0,3,12],
            "wind_gusts_10m_max":[30,40,30,20,20,35,60],"uv_index_max":[2,1,2,3,3,2,1],
            "sunrise":["2026-11-10T07:40"],"sunset":["2026-11-10T17:30"]}}}"""
    }
}

@RunWith(RobolectricTestRunner::class)
class CalaixJsonTest {
    @Test fun llegeixElsPrestatgesIMenes() {
        val sh = CalaixJson.shelves(CalaixSamples.drawer)
        assertEquals(listOf("places", "table", "releases", "recipes", "list", "table"), sh.map { it.kind })
        val table = sh[1]
        assertEquals(listOf("equip", "pj", "punts"), CalaixJson.columns(table.items))
        assertEquals("31", CalaixJson.cell(table.items[0]["punts"]))
        assertEquals("fcb", CalaixJson.rowKey(table.items[0]))
        val recipe = sh[3].items[0]
        assertEquals(listOf("2 albergínies", "2 pebrots", "1 ceba"), CalaixJson.strings(recipe, "ingredients"))
        assertEquals("4", CalaixJson.str(recipe, "servings"))
        assertEquals("escalivada", CalaixJson.rowKey(recipe))
        assertEquals("Girona", CalaixJson.str(CalaixJson.primaryPlace(sh[0].items)!!, "name"))
    }

    @Test fun elTempsDeLaPrevisio() {
        val f = WeatherJson.parse(CalaixSamples.weather())
        assertEquals("Girona", f.place?.name)
        assertEquals(12.4, f.now!!.temp, 0.01); assertEquals(2, f.now!!.code); assertTrue(f.now!!.isDay)
        assertEquals(48, f.hours.size); assertEquals(7, f.days.size)
        assertEquals(80, f.days[1].rainProb)
        val next = WeatherJson.next24(f)
        assertEquals(24, next.size)
        assertEquals("2026-11-10T09:00", next.first().time)
        assertNull(WeatherJson.parse("""{"place":null}""").now)
        assertEquals(listOf("Girona"), WeatherJson.places("""{"places":[{"name":"Girona","where":"Catalunya, Espanya","lat":41.9,"lon":2.8},{"name":""}]}""").map { it.name })
    }
}

class CalaixLogicTest {
    private fun row(date: String, title: String) = mapOf<String, Any?>("date" to date, "title" to title)

    @Test fun llancamentsPerMesosElsQueVenenPrimer() {
        val g = CalaixJson.releaseGroups(listOf(row("2026-12-05", "b"), row("2026-10-02", "old"), row("2026-11-21", "a"), row("", "x")),
            LocalDate.of(2026, 11, 10))
        assertEquals(listOf("2026-11", "2026-12", "", "2026-10"), g.map { it.key })
        assertEquals(listOf(false, false, false, true), g.map { it.past })
        assertEquals("novembre 2026", CalaixFmt.month("ca", "2026-11"))
        assertEquals("8 de novembre", CalaixFmt.day("ca", "2026-11-08"))
        assertEquals("2 d’octubre", CalaixFmt.day("ca", "2026-10-02"))
    }

    @Test fun elCelDelsCodisWmo() {
        assertEquals(Sky.CLEAR, WeatherJson.sky(0)); assertEquals(Sky.PARTLY, WeatherJson.sky(2))
        assertEquals(Sky.CLOUDY, WeatherJson.sky(3)); assertEquals(Sky.FOG, WeatherJson.sky(45))
        assertEquals(Sky.DRIZZLE, WeatherJson.sky(53)); assertEquals(Sky.RAIN, WeatherJson.sky(81))
        assertEquals(Sky.SNOW, WeatherJson.sky(75)); assertEquals(Sky.STORM, WeatherJson.sky(95))
        assertEquals("12°", WeatherJson.deg(12.4)); assertEquals("-3°", WeatherJson.deg(-2.6))
    }

    @Test fun nousALaCollaIElVentDelTemps() {
        val daily = GregalApi.Squad("daily", "Cada dia", "Everyday", listOf(
            GregalApi.SquadMember("shopping", "Garbí", "Compres", "Shopping", "#29BCA3", "pso"),
            GregalApi.SquadMember("cooking", "Llevantada", "Cuina i receptes", "Cooking", "#EFA68F", "pso"),
            GregalApi.SquadMember("weather", "Oreig", "El temps", "Weather", "#4F5BE8", "pso")))
        val mine = listOf(Wind("w-1", "Garbí", "#29BCA3", "compres", specialty = "shopping", squad = "daily"))
        val (s, missing) = missingSquadMembers(listOf(daily), mine)!!
        assertEquals("daily", s.id)
        assertEquals(listOf("Llevantada", "Oreig"), missing.map { it.name })
        assertNull(missingSquadMembers(listOf(daily), emptyList()))
        assertNull(weatherWind(mine))
        assertEquals("o", weatherWind(mine + Wind("o", "Oreig", "#4F5BE8", "temps", specialty = "weather"))?.id)
    }
}

/** El temps i el calaix pintats, en fosc i en clar (build/calaix/calaix-dark.png i calaix-light.png). */
@RunWith(RobolectricTestRunner::class)
@Config(sdk = [34], qualifiers = "w393dp-h2400dp")
@GraphicsMode(GraphicsMode.Mode.NATIVE)
@LooperMode(LooperMode.Mode.PAUSED)
class CalaixUiTest {
    @get:Rule val ui = createComposeRule()

    private fun capture(tag: String): Bitmap {
        val bounds = ui.onNodeWithTag(tag).fetchSemanticsNode().boundsInRoot
        lateinit var bitmap: Bitmap
        ui.runOnIdle {
            val activity = androidx.test.runner.lifecycle.ActivityLifecycleMonitorRegistry.getInstance()
                .getActivitiesInStage(androidx.test.runner.lifecycle.Stage.RESUMED).single()
            val view = activity.window.decorView
            val root = Bitmap.createBitmap(view.width, view.height, Bitmap.Config.ARGB_8888)
            view.draw(android.graphics.Canvas(root))
            bitmap = Bitmap.createBitmap(root, bounds.left.toInt(), bounds.top.toInt(), bounds.width.toInt(),
                minOf(bounds.height.toInt(), root.height - bounds.top.toInt()))
        }
        return bitmap
    }

    @Test fun pintaElTempsIElCalaix() {
        var palette by mutableStateOf(DarkPal)
        val f = WeatherJson.parse(CalaixSamples.weather())
        val shelves = CalaixJson.shelves(CalaixSamples.drawer)
        val removed = mutableListOf<String>()
        val accent = Color(0xFF4F5BE8)
        ui.setContent {
            CompositionLocalProvider(LocalWindMotion provides false, LocalPal provides palette) {
                MaterialTheme(colorScheme = palette.scheme()) {
                    Column(Modifier.fillMaxWidth().background(palette.surface).padding(16.dp).testTag("shot"),
                        verticalArrangement = Arrangement.spacedBy(16.dp)) {
                        WeatherCard("ca", f, accent)
                        WeatherCard("ca", f, accent, compact = true)
                        CalaixSection("ca", "Oreig", shelves, accent, onAction = { _, a, k, _ -> removed += "$a:$k" })
                    }
                }
            }
        }
        assertTrue(ui.onAllNodesWithTag("weather-temp").fetchSemanticsNodes().size == 2)
        assertEquals(2, ui.onAllNodesWithText("Mig ennuvolat").fetchSemanticsNodes().size)
        ui.onNodeWithText("novembre 2026").assertExists()
        ui.onNodeWithText("Escalivada").performClick()
        ui.onNodeWithText("Escalivar al forn").assertExists()
        ui.onNodeWithText("Fes-lo principal").performClick()
        assertEquals(listOf("primary:cadaqués"), removed)
        listOf("dark" to DarkPal, "light" to LightPal).forEach { (name, colors) ->
            ui.runOnIdle { palette = colors }
            val shot = capture("shot")
            java.io.File("build/calaix/calaix-$name.png").also { it.parentFile!!.mkdirs() }.outputStream().use {
                shot.compress(Bitmap.CompressFormat.PNG, 100, it)
            }
            assertTrue(shot.width > 300)
        }
    }
}
