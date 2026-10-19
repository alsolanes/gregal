package xyz.solanes.gregal

import android.app.Application
import androidx.test.core.app.ApplicationProvider
import kotlinx.coroutines.flow.MutableStateFlow
import org.junit.Assert.*
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config

@RunWith(RobolectricTestRunner::class)
@Config(sdk = [34])
class ProviderConnectionTest {
    @Test fun staleProviderResponseCannotRepopulateAnotherServer() {
        val started = java.util.concurrent.CountDownLatch(1)
        val release = java.util.concurrent.CountDownLatch(1)
        val completed = java.util.concurrent.CountDownLatch(1)
        val server = java.net.ServerSocket(0, 1, java.net.InetAddress.getByName("127.0.0.1"))
        val worker = Thread {
            server.accept().use { socket ->
                val reader = socket.getInputStream().bufferedReader()
                while (!reader.readLine().isNullOrBlank()) { }
                started.countDown()
                release.await(3, java.util.concurrent.TimeUnit.SECONDS)
                val body = """{"providers":[{"name":"old","has_api_key":true}]}""".toByteArray()
                socket.getOutputStream().apply {
                    write("HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: ${body.size}\r\nConnection: close\r\n\r\n".toByteArray())
                    write(body)
                    flush()
                }
                completed.countDown()
            }
        }.apply { isDaemon = true; start() }
        try {
            val vm = GregalViewModel(ApplicationProvider.getApplicationContext<Application>())
            vm.setConn(Conn("127.0.0.1", server.localPort.toString(), ""))
            vm.loadProviders()
            org.robolectric.Shadows.shadowOf(android.os.Looper.getMainLooper()).idle()
            assertTrue(started.await(3, java.util.concurrent.TimeUnit.SECONDS))
            vm.setConn(Conn("127.0.0.1", "1", ""))
            release.countDown()
            assertTrue(completed.await(3, java.util.concurrent.TimeUnit.SECONDS))
            repeat(20) {
                org.robolectric.Shadows.shadowOf(android.os.Looper.getMainLooper()).idle()
                Thread.sleep(5)
            }
            assertTrue(vm.providers.value.isEmpty())
            assertFalse(vm.needsProvider.value)
        } finally { release.countDown(); server.close(); worker.join(1000) }
    }

    @Suppress("UNCHECKED_CAST")
    @Test fun changingConnectionClearsProviderChoicesCatalogAndErrors() {
        val vm = GregalViewModel(ApplicationProvider.getApplicationContext<Application>())
        fun <T> seed(name: String, value: T) {
            val field = vm.javaClass.getDeclaredField(name).apply { isAccessible = true }
            (field.get(vm) as MutableStateFlow<T>).value = value
        }
        vm.setConn(Conn("old.invalid", "8097", ""))
        seed("_providers", listOf(ProviderInfo(name = "old", key = "sí")))
        seed("_catalog", listOf(CatalogEntry("old", "Old", false, "", "model")))
        seed("_models", mapOf("old" to listOf("model")))
        seed("_provMsg", "old status")
        vm.setConn(Conn("new.invalid", "8097", ""))
        assertTrue(vm.providers.value.isEmpty())
        assertTrue(vm.models.value.isEmpty())
        assertTrue(vm.catalog.value.isEmpty())
        assertEquals("", vm.provMsg.value)
    }
}
