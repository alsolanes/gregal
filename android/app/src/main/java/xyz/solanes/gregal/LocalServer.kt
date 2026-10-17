package xyz.solanes.gregal

import android.content.Context
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.delay
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.sync.Mutex
import kotlinx.coroutines.sync.withLock
import kotlinx.coroutines.withContext
import java.io.File
import java.io.OutputStream
import java.net.InetAddress
import java.net.ServerSocket
import java.security.SecureRandom
import java.util.TimeZone

/*
 * Mode «aquest dispositiu»: l'app executa el mateix servidor Go que
 * l'escriptori (libgregal.so, compilat per a Android) com a procés fill i
 * hi parla per 127.0.0.1, igual que l'Electron amb gregal.exe. Tot el que
 * fa el backend (sessions, cua de torns, events durables, reintents,
 * respostes parcials, eines, MCP) és el mateix codi a totes les
 * plataformes.
 *
 * Robustesa:
 *  - stdin obert com a línia de vida (--lifeline): si l'app mor, el
 *    backend surt net i no queda cap procés orfe gastant bateria;
 *  - si el backend cau, es torna a engegar amb espera creixent, i un torn
 *    que quedés a mitges el tanca el mateix backend en arrencar;
 *  - el port i el token es reutilitzen entre arrencades, i el token mai
 *    surt del dispositiu (només escolta a loopback).
 */

/** Rutes del servidor local, totes dins l'emmagatzematge privat de l'app. */
data class LocalPaths(
    val binary: String,
    val home: String,
    val dataDir: String,
    val authFile: String,
    val workspace: String,
    val tmp: String,
    val logFile: String,
) {
    companion object {
        fun of(ctx: Context): LocalPaths {
            val files = ctx.filesDir.absolutePath
            return LocalPaths(
                binary = File(ctx.applicationInfo.nativeLibraryDir, LocalSpec.BINARY).absolutePath,
                home = "$files/home",
                dataDir = "$files/home/.local/share/gregal",
                // Les claus fora de la còpia de seguretat: no surten del mòbil.
                authFile = File(ctx.noBackupFilesDir, "auth.yaml").absolutePath,
                workspace = "$files/workspace",
                tmp = File(ctx.cacheDir, "tmp").absolutePath,
                logFile = "$files/logs/backend.log",
            )
        }
    }
}

/** Arguments, entorn i política de reintents: pur, testejable a la JVM. */
object LocalSpec {
    const val BINARY = "libgregal.so"
    const val DEFAULT_PORT = 18097

    fun args(binary: String, port: Int, token: String, paths: LocalPaths): List<String> = listOf(
        binary, "--serve",
        "--addr", "127.0.0.1",
        "--port", port.toString(),
        "--token", token,
        "--dir", paths.workspace,
        "--log-file", paths.logFile,
        "--lifeline",
    )

    /**
     * Entorn del backend. HOME mana sobre tot (config a ~/.config/gregal,
     * dades a ~/.local/share/gregal): sense, Go a Android tria /sdcard.
     * TMPDIR també: el /data/local/tmp per defecte no és nostre.
     */
    fun env(paths: LocalPaths, timeZone: String, lang: String): Map<String, String> = mapOf(
        "HOME" to paths.home,
        "GREGAL_DATA_DIR" to paths.dataDir,
        "GREGAL_AUTH" to paths.authFile,
        "GREGAL_PERSIST_KEYS" to "1",
        "GREGAL_PLATFORM" to "android",
        "TMPDIR" to paths.tmp,
        "TZ" to timeZone,
        "LANG" to if (lang == "ca") "ca_ES.UTF-8" else "en_US.UTF-8",
    )

    /** Espera abans del reintent n (1, 2, 4… fins a 30 s). */
    fun restartDelayMs(attempt: Int): Long =
        (1000L shl (attempt - 1).coerceIn(0, 5)).coerceAtMost(30_000L)

    /** Massa caigudes seguides en poc temps: millor aturar-se i ensenyar el log. */
    fun shouldGiveUp(crashTimes: List<Long>, now: Long, windowMs: Long = 120_000L, max: Int = 5): Boolean =
        crashTimes.count { now - it <= windowMs } >= max

    fun newToken(): String {
        val b = ByteArray(24)
        SecureRandom().nextBytes(b)
        return b.joinToString("") { "%02x".format(it) }
    }
}

/** Estat visible del servidor local. */
sealed class LocalState {
    object Stopped : LocalState()
    object Starting : LocalState()
    data class Running(val port: Int, val token: String) : LocalState()
    data class Failed(val message: String, val log: String) : LocalState()
}

class LocalServerException(msg: String) : Exception(msg)

object LocalServer {
    private val mutex = Mutex()
    private val _state = MutableStateFlow<LocalState>(LocalState.Stopped)
    val state: StateFlow<LocalState> = _state

    @Volatile private var process: Process? = null
    @Volatile private var lifeline: OutputStream? = null
    @Volatile private var stopping = false
    private val logLines = ArrayDeque<String>()
    private val crashes = mutableListOf<Long>()
    private lateinit var appCtx: Context

    /** El backend és dins l'APK (no ho és amb -Pgregal.skipBackend). */
    fun available(ctx: Context): Boolean = File(LocalPaths.of(ctx).binary).exists()

    /** Últimes línies del log del backend (per a l'error que veu la persona). */
    fun logTail(n: Int = 30): String = synchronized(logLines) { logLines.takeLast(n).joinToString("\n") }

    fun alive(): Boolean = process?.isAlive == true

    /**
     * Torna un servidor sa (port, token), engegant-lo si cal. Es pot cridar
     * tantes vegades com calgui: si ja corre i respon, no fa res.
     */
    suspend fun ensure(ctx: Context, lang: String = "en"): LocalState.Running = mutex.withLock {
        appCtx = ctx.applicationContext
        val cur = _state.value
        if (cur is LocalState.Running && alive() && healthy(cur.port, cur.token)) return cur
        // L'app ha reviscut però el backend d'abans encara respon (s'està
        // tancant o la línia de vida no ha arribat): fes-lo servir. Si cau,
        // la propera crida l'engega de nou.
        if (process == null) {
            val prefs = Prefs(appCtx)
            val port = prefs.localPort
            val token = prefs.localToken
            if (port > 0 && token.isNotBlank() && withContext(Dispatchers.IO) { healthy(port, token) }) {
                return LocalState.Running(port, token).also { _state.value = it }
            }
        }
        startLocked(lang)
    }

    /** Atura el backend: primer net (tancant la línia de vida), després a la força. */
    suspend fun stop() = mutex.withLock { stopLocked() }

    private suspend fun stopLocked() = withContext(Dispatchers.IO) {
        stopping = true
        val p = process
        try { lifeline?.close() } catch (_: Exception) {}
        if (p != null) {
            val deadline = System.currentTimeMillis() + 3000
            while (p.isAlive && System.currentTimeMillis() < deadline) delay(100)
            if (p.isAlive) p.destroyForcibly()
        }
        process = null
        lifeline = null
        _state.value = LocalState.Stopped
    }

    private suspend fun startLocked(lang: String): LocalState.Running = withContext(Dispatchers.IO) {
        if (process != null) stopLocked()
        stopping = false
        val ctx = appCtx
        val paths = LocalPaths.of(ctx)
        val bin = File(paths.binary)
        if (!bin.exists()) throw LocalServerException("this build has no on-device backend")
        listOf(paths.home, paths.dataDir, paths.workspace, paths.tmp, File(paths.logFile).parent!!)
            .forEach { File(it).mkdirs() }
        val prefs = Prefs(ctx)
        val token = prefs.localToken.ifBlank { LocalSpec.newToken().also { prefs.localToken = it } }
        val port = pickPort(prefs.localPort)
        prefs.localPort = port
        _state.value = LocalState.Starting
        val pb = ProcessBuilder(LocalSpec.args(bin.absolutePath, port, token, paths))
            .directory(File(paths.workspace))
            .redirectErrorStream(true)
        pb.environment().putAll(LocalSpec.env(paths, TimeZone.getDefault().id, lang))
        val p = try { pb.start() } catch (e: Exception) {
            val f = LocalState.Failed(e.message ?: "cannot start backend", logTail())
            _state.value = f
            throw LocalServerException(f.message)
        }
        process = p
        lifeline = p.outputStream
        pump(p)
        watch(p, lang)
        // Espera que respongui; si el procés mor abans, error amb el log.
        val deadline = System.currentTimeMillis() + 30_000
        while (System.currentTimeMillis() < deadline) {
            if (!p.isAlive) break
            if (healthy(port, token)) {
                val ok = LocalState.Running(port, token)
                _state.value = ok
                return@withContext ok
            }
            delay(200)
        }
        val msg = if (p.isAlive) "backend did not answer in 30 s" else "backend exited (code ${p.exitValue()})"
        if (p.isAlive) p.destroyForcibly()
        val f = LocalState.Failed(msg, logTail())
        _state.value = f
        throw LocalServerException(msg)
    }

    /** El port desat si és lliure; si no, un d'efímer. */
    private fun pickPort(saved: Int): Int {
        for (candidate in listOf(saved, LocalSpec.DEFAULT_PORT)) {
            if (candidate <= 0) continue
            try {
                ServerSocket(candidate, 1, InetAddress.getByName("127.0.0.1")).use { return candidate }
            } catch (_: Exception) {
            }
        }
        ServerSocket(0, 1, InetAddress.getByName("127.0.0.1")).use { return it.localPort }
    }

    private fun healthy(port: Int, token: String): Boolean = try {
        GregalApi("http://127.0.0.1:$port", token).health().status == "ok"
    } catch (_: Exception) { false }

    /** Llegeix stdout+stderr al buffer circular (si no, el pipe s'omple i el backend es bloqueja). */
    private fun pump(p: Process) {
        Thread({
            try {
                p.inputStream.bufferedReader().forEachLine { line ->
                    synchronized(logLines) {
                        logLines.addLast(line)
                        while (logLines.size > 200) logLines.removeFirst()
                    }
                }
            } catch (_: Exception) {
            }
        }, "gregal-backend-log").apply { isDaemon = true }.start()
    }

    /** Si el backend cau sense que l'haguem aturat, el torna a engegar. */
    private fun watch(p: Process, lang: String) {
        Thread({
            val code = try { p.waitFor() } catch (_: InterruptedException) { return@Thread }
            if (stopping || process !== p) return@Thread
            val now = System.currentTimeMillis()
            val giveUp = synchronized(crashes) {
                crashes += now
                crashes.removeAll { now - it > 600_000L }
                LocalSpec.shouldGiveUp(crashes, now)
            }
            if (giveUp) {
                _state.value = LocalState.Failed("backend keeps crashing (exit $code)", logTail())
                return@Thread
            }
            _state.value = LocalState.Starting
            val attempt = synchronized(crashes) { crashes.size }
            kotlinx.coroutines.runBlocking {
                delay(LocalSpec.restartDelayMs(attempt))
                try { ensure(appCtx, lang) } catch (_: Exception) { }
            }
        }, "gregal-backend-watch").apply { isDaemon = true }.start()
    }

    /** Una nova oportunitat després d'un «ha caigut massa cops» (botó Reintenta). */
    fun resetCrashes() = synchronized(crashes) { crashes.clear() }
}
