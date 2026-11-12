package xyz.solanes.gregal

import java.io.IOException
import java.net.Inet4Address
import java.net.NetworkInterface
import okhttp3.Interceptor
import okhttp3.Response

/**
 * Com s'arriba al servidor: a casa per la LAN (ràpid, sense sortir de casa),
 * fora pel domini del túnel (xifrat, per internet).
 *
 * El servidor diu a `/api/health` com se'l pot trobar (`lan_ips` i
 * `public_host`) i l'app s'ho desa. La tria es fa mirant si el mòbil és a la
 * mateixa xarxa que el servidor —no provant adreces, que faria perdre segons
 * quan ets fora— i es recorda mentre la connexió i la xarxa no canvien. Si una
 * petició no pot ni connectar, la següent prova l'altra banda.
 */
internal object ServerPick {

    /** Quant de temps val una tria que funciona. */
    private const val STICKY_MS = 10 * 60_000L

    /** Quant de temps dura el canvi forçat per una fallada. */
    private const val FORCED_MS = 2 * 60_000L

    @Volatile private var sticky: String = ""
    @Volatile private var stickyAt: Long = 0L
    @Volatile private var stickyKey: String = ""
    @Volatile private var forced: String = ""
    @Volatile private var forcedAt: Long = 0L
    @Volatile private var forcedKey: String = ""

    /** L'última combinació que s'ha decidit: la fallada necessita saber-la. */
    @Volatile private var snapHost: String = ""
    @Volatile private var snapPort: String = ""
    @Volatile private var snapLan: String = ""
    @Volatile private var snapExt: String = ""
    @Volatile private var snapKey: String = ""

    /**
     * La tria val per a aquesta connexió i aquesta xarxa. Si canvies de servidor
     * o de xarxa (surt de casa, tornes), es torna a decidir tot sol, sense
     * esperar cap fallada.
     */
    private fun key(
        configuredHost: String,
        configuredPort: String,
        lanIp: String,
        exteriorHost: String,
        deviceIps: List<String>,
    ): String = "$configuredHost|$configuredPort|$lanIp|$exteriorHost|" + deviceIps.sorted().joinToString(",")

    /** Dues IPv4 a la mateixa /24 (192.168.1.84 i 192.168.1.30, per exemple). */
    fun sameNet(a: String, b: String): Boolean {
        val pa = a.trim().split('.')
        val pb = b.trim().split('.')
        if (pa.size != 4 || pb.size != 4) return false
        return pa[0] == pb[0] && pa[1] == pb[1] && pa[2] == pb[2]
    }

    /** Port de la LAN: el que fa servir l'app si apunta a casa, si no el de sempre. */
    fun lanPort(configuredHost: String, configuredPort: String): String {
        if (!isPublicDomain(configuredHost) && configuredPort.isNotBlank()) return configuredPort
        return "8097"
    }

    /** L'adreça de la LAN, si se sap. */
    fun lanBase(configuredHost: String, configuredPort: String, lanIp: String): String =
        if (lanIp.isBlank()) "" else baseUrlOf(lanIp, lanPort(configuredHost, configuredPort))

    /** L'adreça de fora, si se sap. */
    fun exteriorBase(exteriorHost: String): String =
        if (exteriorHost.isBlank()) "" else baseUrlOf(exteriorHost, "443")

    /** La IP d'una adreça escrita per algú: «http://192.168.1.84:8097» → «192.168.1.84». */
    fun lanIpOf(host: String): String {
        var h = host.trim()
        val scheme = h.indexOf("://")
        if (scheme >= 0) h = h.substring(scheme + 3)
        h = h.substringBefore('/')
        return if (h.count { it == ':' } == 1) h.substringBefore(':') else h
    }

    /**
     * La decisió neta: a casa la LAN, fora el domini, i si no se sap res el que
     * va escriure l'usuari.
     */
    fun decide(
        configuredHost: String,
        configuredPort: String,
        lanIp: String,
        exteriorHost: String,
        deviceIps: List<String>,
    ): String {
        val lan = lanIp.trim()
        if (lan.isNotEmpty() && deviceIps.any { sameNet(it, lan) }) {
            return baseUrlOf(lan, lanPort(configuredHost, configuredPort))
        }
        val ext = exteriorBase(exteriorHost)
        if (ext.isNotEmpty()) return ext
        return baseUrlOf(configuredHost, configuredPort)
    }

    /** L'adreça que toca ara, recordant la que funcionava. */
    fun base(
        configuredHost: String,
        configuredPort: String,
        lanIp: String,
        exteriorHost: String,
        deviceIps: List<String>,
        now: Long = System.currentTimeMillis(),
    ): String {
        val k = key(configuredHost, configuredPort, lanIp, exteriorHost, deviceIps)
        snapHost = configuredHost
        snapPort = configuredPort
        snapLan = lanIp
        snapExt = exteriorHost
        snapKey = k
        if (forced.isNotEmpty() && forcedKey == k && now - forcedAt < FORCED_MS) return forced
        if (sticky.isNotEmpty() && stickyKey == k && now - stickyAt < STICKY_MS) return sticky
        val b = decide(configuredHost, configuredPort, lanIp, exteriorHost, deviceIps)
        sticky = b
        stickyKey = k
        stickyAt = now
        return b
    }

    /**
     * Una petició a [failed] no ha pogut ni connectar. Si hi ha una altra banda
     * coneguda, la propera anirà per allà. Retorna l'adreça nova, o null si no
     * hi ha res a canviar.
     */
    fun failedToConnect(failed: String, now: Long = System.currentTimeMillis()): String? {
        if (snapKey.isEmpty()) return null
        val lan = lanBase(snapHost, snapPort, snapLan)
        val ext = exteriorBase(snapExt)
        val other = when {
            lan.isNotEmpty() && ext.isNotEmpty() && ext != lan && failed.startsWith(ext) -> lan
            lan.isNotEmpty() && ext.isNotEmpty() && ext != lan && failed.startsWith(lan) -> ext
            else -> ""
        }
        if (other.isEmpty()) return null
        forced = other
        forcedKey = snapKey
        forcedAt = now
        sticky = ""
        stickyAt = 0L
        return other
    }

    /** Oblida la tria (canvi de servidor, o en tornar a obrir l'app). */
    fun forget() {
        sticky = ""
        stickyKey = ""
        stickyAt = 0L
        forced = ""
        forcedKey = ""
        forcedAt = 0L
        snapKey = ""
    }

    /** Les IPv4 d'aquest mòbil a la xarxa on és ara (WiFi o dades). */
    fun deviceIps(): List<String> = try {
        NetworkInterface.getNetworkInterfaces().toList()
            .filter { it.isUp && !it.isLoopback }
            .flatMap { it.inetAddresses.toList() }
            .filterIsInstance<Inet4Address>()
            .filter { it.isSiteLocalAddress }
            .mapNotNull { it.hostAddress }
    } catch (_: Exception) {
        emptyList()
    }

    @Volatile private var ipsAt = 0L
    @Volatile private var ips: List<String> = emptyList()

    /** deviceIps() amb memòria curta: enumerar interfícies no és gratis. */
    fun deviceIpsCached(now: Long = System.currentTimeMillis()): List<String> {
        if (now - ipsAt < 10_000L) return ips
        ips = deviceIps()
        ipsAt = now
        return ips
    }
}

/**
 * El que el servidor ha dit de si mateix (`/api/health` → net). Viu aquí perquè
 * Conn.api() el pugui fer servir sense passar per Prefs a cada crida.
 */
internal object ServerNet {
    @Volatile var lanIp: String = ""
    @Volatile var exteriorHost: String = ""

    fun hydrate(lan: String, exterior: String) {
        if (lan.isNotBlank()) lanIp = lan.trim()
        if (exterior.isNotBlank()) exteriorHost = exterior.trim()
    }
}

/**
 * Si una petició no pot ni connectar, apunta que aquella banda no serveix i la
 * propera prova l'altra. No reintenta aquí: reintentar un POST podria duplicar-lo.
 */
internal class ServerFailoverInterceptor : Interceptor {
    override fun intercept(chain: Interceptor.Chain): Response {
        val req = chain.request()
        return try {
            chain.proceed(req)
        } catch (e: IOException) {
            val defecte = req.url.port
            val intentada = "${req.url.scheme}://${req.url.host}" + if (defecte != 80 && defecte != 443) ":$defecte" else ""
            ServerPick.failedToConnect(intentada)
            throw e
        }
    }
}
