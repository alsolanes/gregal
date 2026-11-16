package xyz.solanes.gregal

import org.junit.After
import org.junit.Assert.*
import org.junit.Before
import org.junit.Test
import java.net.InetAddress
import java.net.ServerSocket
import kotlin.concurrent.thread

/* El carregador d'imatges del xat (CompanyScreen.kt): segueix redireccions,
 * diu qui és i torna null (no l'adreça) quan la foto no hi és. */
class ChatImageFetchTest {
    private lateinit var server: ServerSocket
    @Volatile private var agent = ""
    private val png = byteArrayOf(0x89.toByte(), 0x50, 0x4E, 0x47, 1, 2, 3)

    @Before
    fun up() {
        server = ServerSocket(0, 50, InetAddress.getLoopbackAddress())
        thread(isDaemon = true) {
            while (!server.isClosed) {
                val sock = try { server.accept() } catch (_: Exception) { break }
                sock.use { s ->
                    val reader = s.getInputStream().bufferedReader()
                    val path = reader.readLine().orEmpty().split(" ").getOrElse(1) { "" }
                    while (true) {
                        val line = reader.readLine() ?: break
                        if (line.isEmpty()) break
                        if (line.startsWith("User-Agent:", ignoreCase = true)) agent = line.substringAfter(':').trim()
                    }
                    val out = s.getOutputStream()
                    when (path) {
                        "/img.png" -> { out.write("HTTP/1.1 200 OK\r\nContent-Length: ${png.size}\r\nContent-Type: image/png\r\nConnection: close\r\n\r\n".toByteArray()); out.write(png) }
                        "/moved" -> out.write("HTTP/1.1 301 Moved\r\nLocation: /img.png\r\nContent-Length: 0\r\nConnection: close\r\n\r\n".toByteArray())
                        else -> out.write("HTTP/1.1 404 Not Found\r\nContent-Length: 0\r\nConnection: close\r\n\r\n".toByteArray())
                    }
                    out.flush()
                }
            }
        }
    }

    @After
    fun down() = server.close()

    private fun url(path: String) = "http://127.0.0.1:${server.localPort}$path"

    @Test
    fun followsRedirectsAndSaysWhoItIs() {
        assertArrayEquals(png, fetchChatImageBytes(url("/moved")))
        assertTrue(agent, agent.startsWith("Gregal"))
    }

    @Test
    fun aMissingPhotoIsNothing() {
        assertNull(fetchChatImageBytes(url("/missing.jpg")))
    }

    @Test
    fun tooBigIsNothing() {
        assertNull(fetchChatImageBytes(url("/img.png"), maxBytes = 3))
    }
}
