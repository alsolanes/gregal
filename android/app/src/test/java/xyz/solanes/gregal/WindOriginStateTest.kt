package xyz.solanes.gregal

import android.app.Application
import android.os.Looper
import androidx.test.core.app.ApplicationProvider
import java.net.ServerSocket
import kotlin.concurrent.thread
import java.util.concurrent.CountDownLatch
import java.util.concurrent.TimeUnit
import kotlinx.coroutines.flow.MutableStateFlow
import org.junit.Assert.*
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.Shadows.shadowOf
import org.robolectric.annotation.LooperMode

@RunWith(RobolectricTestRunner::class)
@LooperMode(LooperMode.Mode.PAUSED)
class WindOriginStateTest {
    private fun vm() = GregalViewModel(ApplicationProvider.getApplicationContext<Application>())

    @Suppress("UNCHECKED_CAST")
    private fun <T> seed(vm: GregalViewModel, field: String, value: T) {
        val f = GregalViewModel::class.java.getDeclaredField(field).apply { isAccessible = true }
        (f.get(vm) as MutableStateFlow<T>).value = value
    }

    @Test fun changingAccountClearsAllCompanionStateEvenOnSameHost() {
        val v = vm()
        v.setConn(Conn("server.example", "8090", "token-a", "alice"))
        val w = Wind("tramuntana", "Tramuntana", "#FF9F00", "música")
        seed(v, "_winds", listOf(w))
        seed(v, "_windChat", mapOf(w.id to listOf(GregalViewModel.PetMsg("pet", "privat"))))
        seed(v, "_rotllanes", listOf(Rotllana("group", "Casa", listOf(w.id))))
        seed(v, "_rotllanaChat", mapOf("group" to listOf(GregalViewModel.PetMsg("user", "privat"))))
        v.setActiveWind(w)
        seed(v, "_windProfile", WindProfile(w, "privat", "memòria personal"))
        v.openVentsChat("wind:${w.id}")
        v.showWindCreator(true, w)
        v.setConn(Conn("server.example", "8090", "token-b", "bob"))
        assertTrue(v.winds.value.isEmpty())
        assertTrue(v.windChat.value.isEmpty())
        assertTrue(v.rotllanes.value.isEmpty())
        assertTrue(v.rotllanaChat.value.isEmpty())
        assertNull(v.activeWind.value)
        assertNull(v.windProfile.value)
        assertNull(v.ventsOpen.value)
        assertFalse(v.windCreator.value.first)
    }

    @Test fun lateCatalogFromPreviousOriginCannotRepopulateNewAccount() {
        val requested = CountDownLatch(1)
        val release = CountDownLatch(1)
        val responded = CountDownLatch(1)
        val server = ServerSocket(0)
        val worker = thread(isDaemon = true) {
            server.accept().use { socket ->
                val reader = socket.getInputStream().bufferedReader()
                while (!reader.readLine().isNullOrEmpty()) { /* HTTP headers */ }
                requested.countDown()
                release.await(3, TimeUnit.SECONDS)
                val body = """{"winds":[{"id":"old","name":"Old account","color":"#FF9F00","role":"private"}]}""".toByteArray()
                val header = "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: ${body.size}\r\nConnection: close\r\n\r\n"
                socket.getOutputStream().apply { write(header.toByteArray()); write(body); flush() }
                responded.countDown()
            }
        }
        try {
            val v = vm()
            v.setConn(Conn("127.0.0.1", server.localPort.toString(), "token-a", "alice"))
            v.loadWinds()
            shadowOf(Looper.getMainLooper()).idle()
            assertTrue("request started", requested.await(3, TimeUnit.SECONDS))
            v.setConn(Conn("127.0.0.1", server.localPort.toString(), "token-b", "bob"))
            release.countDown()
            assertTrue(responded.await(3, TimeUnit.SECONDS))
            repeat(15) { Thread.sleep(20); shadowOf(Looper.getMainLooper()).idle() }
            assertTrue("discard stale response", v.winds.value.isEmpty())
            assertNull(v.activeWind.value)
        } finally { release.countDown(); server.close(); worker.join(1000) }
    }
}
