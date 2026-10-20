package xyz.solanes.gregal

import android.app.Application
import android.os.Looper
import androidx.test.core.app.ApplicationProvider
import java.net.ServerSocket
import java.util.concurrent.CountDownLatch
import java.util.concurrent.TimeUnit
import kotlin.concurrent.thread
import org.json.JSONObject
import org.junit.Assert.*
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.Shadows.shadowOf
import org.robolectric.annotation.LooperMode

/** Explicit HTTP test fixtures; no real model is called by these contract tests. */
@RunWith(RobolectricTestRunner::class)
@LooperMode(LooperMode.Mode.PAUSED)
class WindProfileApiTest {
    @Test fun profileSeparatesActualPersonalityTopicAndPersonalMemory() {
        val p = WindProfile.parse("""{"wind":{"id":"stable","name":"Marinada","color":"#EFC69A","role":"calma","style":"bsn","persona":"Parlo amb calma."},"knowledge":"- El tema que conec","personal_memory":"- Una preferència consentida"}""")
        assertEquals("stable", p.wind.id)
        assertEquals("Parlo amb calma.", p.wind.persona)
        assertEquals("- El tema que conec", p.knowledge)
        assertEquals("- Una preferència consentida", p.personalMemory)
    }

    @Test fun saveWindSendsStableIdWhenRenaming() {
        val server = ServerSocket(0)
        var requestBody = ""
        val worker = thread(isDaemon = true) {
            server.accept().use { s ->
                val reader = s.getInputStream().bufferedReader()
                var length = 0
                while (true) {
                    val line = reader.readLine() ?: break
                    if (line.isEmpty()) break
                    if (line.startsWith("Content-Length:", ignoreCase = true)) length = line.substringAfter(':').trim().toInt()
                }
                val chars = CharArray(length)
                var offset = 0
                while (offset < length) { val n = reader.read(chars, offset, length - offset); if (n < 0) break; offset += n }
                requestBody = String(chars)
                s.getOutputStream().write("HTTP/1.1 200 OK\r\nContent-Length: 2\r\nConnection: close\r\n\r\n{}".toByteArray())
            }
        }
        try {
            Conn("127.0.0.1", server.localPort.toString(), "test", "alice").api().saveWind("Un altre nom", "#EFC69A", "calma", "bsn", "Calm", "stable-id")
            worker.join(2000)
            assertEquals("stable-id", JSONObject(requestBody).getString("id"))
            assertEquals("Un altre nom", JSONObject(requestBody).getString("name"))
        } finally { server.close(); worker.join(1000) }
    }

    @Test fun failedSuggestionReleasesPendingCreatorState() {
        val server = ServerSocket(0)
        val done = CountDownLatch(1)
        val worker = thread(isDaemon = true) {
            server.accept().use { s ->
                val reader = s.getInputStream().bufferedReader()
                while (!reader.readLine().isNullOrEmpty()) { }
                s.getOutputStream().write("HTTP/1.1 503 Unavailable\r\nContent-Length: 0\r\nConnection: close\r\n\r\n".toByteArray())
                done.countDown()
            }
        }
        try {
            val vm = GregalViewModel(ApplicationProvider.getApplicationContext<Application>())
            vm.setConn(Conn("127.0.0.1", server.localPort.toString(), "test", "alice"))
            var failed = false
            vm.suggestWind("Marinada", "calma", onError = { failed = true }) { _, _ -> fail("unexpected success") }
            shadowOf(Looper.getMainLooper()).idle()
            assertTrue(done.await(3, TimeUnit.SECONDS))
            repeat(15) { Thread.sleep(20); shadowOf(Looper.getMainLooper()).idle() }
            assertTrue("pending UI receives failure callback", failed)
        } finally { server.close(); worker.join(1000) }
    }
}
