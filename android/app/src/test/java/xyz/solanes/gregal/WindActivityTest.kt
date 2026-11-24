package xyz.solanes.gregal

import android.app.Application
import android.os.Looper
import androidx.compose.material3.MaterialTheme
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.ui.test.assertIsDisplayed
import androidx.compose.ui.test.junit4.createComposeRule
import androidx.compose.ui.test.onNodeWithTag
import androidx.compose.ui.test.onNodeWithText
import androidx.compose.ui.test.performClick
import androidx.test.core.app.ApplicationProvider
import java.net.ServerSocket
import java.net.URLEncoder
import java.time.Duration
import java.time.LocalDate
import java.time.ZoneId
import java.util.concurrent.CopyOnWriteArrayList
import java.util.concurrent.TimeUnit
import kotlin.concurrent.thread
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.Shadows.shadowOf
import org.robolectric.annotation.Config
import org.robolectric.annotation.LooperMode

/** L'activitat dels vents (GET /api/winds/activity): agrupada per dies, una fila i les pàgines més velles. */
@RunWith(RobolectricTestRunner::class)
@Config(sdk = [34])
@LooperMode(LooperMode.Mode.PAUSED)
class WindActivityTest {
    @get:Rule val ui = createComposeRule()
    private val zone = ZoneId.of("Europe/Madrid")

    private fun item(at: String, title: String = at, kind: String = "task", status: String = "done") =
        WindActivityItem(at, "xaloc", kind, title, status, "t-$title")

    @Test fun readsTheItemsOfTheApi() {
        val got = parseWindActivity("""{"items":[{"at":"2026-11-24T18:05:00+01:00","wind":"xaloc","kind":"news",
            "title":"Concerts aquest cap de setmana","status":"unread","ref":"n-1"}]}""")
        assertEquals(listOf(WindActivityItem("2026-11-24T18:05:00+01:00", "xaloc", "news", "Concerts aquest cap de setmana", "unread", "n-1")), got)
    }

    @Test fun groupsByTheLocalDayKeepingTheNewestFirst() {
        val items = listOf(
            item("2026-11-24T18:05:00+01:00", "a"),
            item("2026-11-24T00:30:00+01:00", "b"),
            // 23:30 UTC del 23 ja és el 24 a Barcelona.
            item("2026-11-23T23:30:00Z", "c"),
            item("2026-11-23T10:00:00+01:00", "d"),
            item("2026-11-20T10:00:00+01:00", "e"),
        )
        val days = windActivityDays(items, zone)
        assertEquals(listOf(LocalDate.of(2026, 11, 24), LocalDate.of(2026, 11, 23), LocalDate.of(2026, 11, 20)), days.map { it.first })
        assertEquals(listOf(listOf("a", "b", "c"), listOf("d"), listOf("e")), days.map { d -> d.second.map { it.title } })
        assertEquals("18:05", windActivityTime("2026-11-24T18:05:00+01:00", zone))
        assertEquals("00:30", windActivityTime("2026-11-23T23:30:00Z", zone))
    }

    @Test fun oneItemShowsTheFaceTimeTitleAndStatusAndOpensOnTap() {
        val it = item("2026-11-24T18:05:00+01:00", "Resum de les factures", status = "failed")
        var opened: WindActivityItem? = null
        ui.setContent {
            CompositionLocalProvider(LocalPal provides LightPal, LocalWindMotion provides false) {
                MaterialTheme(colorScheme = LightPal.scheme()) {
                    WindActivityRow("ca", it, Wind("xaloc", "Xaloc", "#EFA68F", "Factures")) { opened = it }
                }
            }
        }
        ui.onNodeWithText("Resum de les factures").assertIsDisplayed()
        ui.onNodeWithText("Ha fallat").assertIsDisplayed()
        ui.onNodeWithText(windActivityTime(it.at) + " · Xaloc · Feina").assertIsDisplayed()
        ui.onNodeWithTag("activity-face", useUnmergedTree = true).assertIsDisplayed()
        ui.onNodeWithText("Resum de les factures").performClick()
        assertEquals(it, opened)
    }

    @Test fun reachingTheEndAsksForOlderItemsWithBefore() {
        val first = (0 until WIND_ACTIVITY_PAGE).map { i -> """{"at":"2026-11-24T18:%02d:00+01:00","wind":"xaloc","kind":"task","title":"f$i","status":"done","ref":"t$i"}""".format(59 - i) }
        ActivityServer { line ->
            if (!line.startsWith("GET /api/winds/activity")) "{}"
            else if (line.contains("before=")) """{"items":[{"at":"2026-11-23T09:00:00+01:00","wind":"xaloc","kind":"routine","title":"vell","status":"ok","ref":"job-1"}]}"""
            else """{"items":[${first.joinToString(",")}]}"""
        }.use { s ->
            val vm = GregalViewModel(ApplicationProvider.getApplicationContext<Application>())
                .apply { setConn(Conn("127.0.0.1", s.port.toString(), "activitat")) }
            vm.openWindActivity("xaloc")
            assertEquals(Screen.Activitat, vm.screen.value)
            waitFor { vm.windActivity.value.items.size == WIND_ACTIVITY_PAGE && !vm.windActivity.value.loading }
            assertTrue(vm.windActivity.value.more)
            vm.loadWindActivity(older = true)
            waitFor { vm.windActivity.value.items.size == WIND_ACTIVITY_PAGE + 1 && !vm.windActivity.value.loading }
            val lastAt = vm.windActivity.value.items[WIND_ACTIVITY_PAGE - 1].at
            assertEquals("2026-11-24T18:10:00+01:00", lastAt)
            val older = s.lines.single { it.contains("before=") }
            assertTrue(older, older.contains("wind=xaloc"))
            assertTrue(older, older.contains("before=" + URLEncoder.encode(lastAt, "UTF-8")))
            assertEquals("vell", vm.windActivity.value.items.last().title)
            // La pàgina curta vol dir que no n'hi ha més: no es torna a demanar.
            assertFalse(vm.windActivity.value.more)
            vm.loadWindActivity(older = true)
            repeat(20) { idle(); Thread.sleep(5) }
            assertEquals(2, s.lines.count { it.startsWith("GET /api/winds/activity") })
        }
    }

    @Test fun aFailedLoadIsAnErrorNotAnEmptyList() {
        val vm = GregalViewModel(ApplicationProvider.getApplicationContext<Application>())
            .apply { setConn(Conn("127.0.0.1", "9", "t")) }
        vm.openWindActivity("")
        waitFor { vm.windActivity.value.error != null }
        assertTrue(vm.windActivity.value.error != null)
        assertTrue(vm.windActivity.value.items.isEmpty())
    }

    @Test fun theNewStringsExistInBothLanguages() {
        for (k in listOf("winds.activity.title", "winds.activity.emptyAll", "winds.activity.emptyWind",
            "winds.activity.status.running", "winds.activity.status.unread", "winds.activity.kind.heartbeat")) {
            assertTrue(k, STR_CA.containsKey(k) && STR_EN.containsKey(k))
        }
    }

    private fun idle() = shadowOf(Looper.getMainLooper()).idleFor(Duration.ofMillis(20))

    private fun waitFor(seconds: Long = 8, ok: () -> Boolean) {
        val deadline = System.nanoTime() + TimeUnit.SECONDS.toNanos(seconds)
        while (!ok() && System.nanoTime() < deadline) { idle(); Thread.sleep(10) }
    }

    /** Un servidor de veritat (un socket) que respon el JSON de [answer] per a cada línia de petició. */
    private class ActivityServer(private val answer: (String) -> String) : AutoCloseable {
        private val socket = ServerSocket(0)
        val port: Int get() = socket.localPort
        val lines = CopyOnWriteArrayList<String>()
        private val acceptor = thread(isDaemon = true) {
            while (!socket.isClosed) {
                val client = try { socket.accept() } catch (_: Exception) { break }
                thread(isDaemon = true) {
                    client.use { c ->
                        val r = c.getInputStream().bufferedReader()
                        val line = r.readLine() ?: return@use
                        while (true) { val h = r.readLine() ?: break; if (h.isEmpty()) break }
                        lines += line
                        val bytes = answer(line).toByteArray()
                        val out = c.getOutputStream()
                        out.write("HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: ${bytes.size}\r\nConnection: close\r\n\r\n".toByteArray())
                        out.write(bytes); out.flush()
                    }
                }
            }
        }

        override fun close() { socket.close(); acceptor.join(1000) }
    }
}
