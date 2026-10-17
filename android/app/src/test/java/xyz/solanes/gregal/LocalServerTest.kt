package xyz.solanes.gregal

import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNotEquals
import org.junit.Assert.assertTrue
import org.junit.Test

class LocalServerTest {
    private val paths = LocalPaths(
        binary = "/data/app/x/lib/arm64/libgregal.so",
        home = "/data/user/0/xyz.solanes.gregal/files/home",
        dataDir = "/data/user/0/xyz.solanes.gregal/files/home/.local/share/gregal",
        authFile = "/data/user/0/xyz.solanes.gregal/no_backup/auth.yaml",
        workspace = "/data/user/0/xyz.solanes.gregal/files/workspace",
        tmp = "/data/user/0/xyz.solanes.gregal/cache/tmp",
        logFile = "/data/user/0/xyz.solanes.gregal/files/logs/backend.log",
    )

    @Test
    fun argsListenOnLoopbackWithTokenAndLifeline() {
        val a = LocalSpec.args(paths.binary, 18097, "tok", paths)
        assertEquals(paths.binary, a.first())
        assertEquals("127.0.0.1", a[a.indexOf("--addr") + 1])
        assertEquals("18097", a[a.indexOf("--port") + 1])
        assertEquals("tok", a[a.indexOf("--token") + 1])
        assertEquals(paths.workspace, a[a.indexOf("--dir") + 1])
        assertTrue("--serve" in a)
        // Sense línia de vida el backend quedaria orfe en morir l'app.
        assertTrue("--lifeline" in a)
    }

    @Test
    fun envKeepsEverythingInAppStorage() {
        val e = LocalSpec.env(paths, "Europe/Madrid", "ca")
        assertEquals(paths.home, e["HOME"])
        assertEquals(paths.dataDir, e["GREGAL_DATA_DIR"])
        assertEquals(paths.authFile, e["GREGAL_AUTH"])
        assertEquals("1", e["GREGAL_PERSIST_KEYS"])
        assertEquals("android", e["GREGAL_PLATFORM"])
        assertEquals(paths.tmp, e["TMPDIR"])
        assertEquals("Europe/Madrid", e["TZ"])
        e.values.forEach { assertFalse("outside app storage: $it", it.startsWith("/sdcard")) }
    }

    @Test
    fun restartBackoffGrowsAndCaps() {
        assertEquals(1000L, LocalSpec.restartDelayMs(1))
        assertEquals(2000L, LocalSpec.restartDelayMs(2))
        assertEquals(4000L, LocalSpec.restartDelayMs(3))
        assertEquals(30_000L, LocalSpec.restartDelayMs(20))
        assertEquals(1000L, LocalSpec.restartDelayMs(0))
    }

    @Test
    fun givesUpOnlyAfterRepeatedCrashesInTheWindow() {
        val now = 1_000_000L
        assertFalse(LocalSpec.shouldGiveUp(listOf(now - 10, now - 20), now))
        assertTrue(LocalSpec.shouldGiveUp(List(5) { now - it * 1000L }, now))
        // Caigudes velles no compten.
        assertFalse(LocalSpec.shouldGiveUp(List(5) { now - 300_000L - it }, now))
    }

    @Test
    fun tokensAreRandomAndLong() {
        val a = LocalSpec.newToken()
        val b = LocalSpec.newToken()
        assertEquals(48, a.length)
        assertNotEquals(a, b)
    }

    @Test
    fun localConnPointsAtLoopback() {
        val c = Conn("127.0.0.1", "18097", "t", local = true)
        assertEquals("http://127.0.0.1:18097", baseUrlOf(c.host, c.port))
        assertTrue(c.local)
        assertFalse(Conn("casa.lan", "8097", "t").local)
    }
}
