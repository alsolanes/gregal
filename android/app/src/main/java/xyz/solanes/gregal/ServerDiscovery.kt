package xyz.solanes.gregal

import android.content.Context
import android.net.nsd.NsdManager
import android.net.nsd.NsdServiceInfo
import android.net.wifi.WifiManager
import android.os.Handler
import android.os.Looper
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.runtime.DisposableEffect
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateListOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import java.net.Inet4Address

private data class DiscoveredGregal(val name: String, val address: String, val port: Int)

/** LAN discovery suggests endpoints; selecting one never sends credentials. */
@Composable
internal fun ServerDiscovery(lang: String, onSelect: (String, String) -> Unit) {
    val ctx = LocalContext.current
    val manager = remember(ctx) { ctx.getSystemService(Context.NSD_SERVICE) as NsdManager }
    val wifi = remember(ctx) { ctx.applicationContext.getSystemService(Context.WIFI_SERVICE) as? WifiManager }
    val main = remember { Handler(Looper.getMainLooper()) }
    val found = remember { mutableStateListOf<DiscoveredGregal>() }
    var searching by remember { mutableStateOf(false) }
    var scanned by remember { mutableStateOf(false) }
    var listener by remember { mutableStateOf<NsdManager.DiscoveryListener?>(null) }
    var multicast by remember { mutableStateOf<WifiManager.MulticastLock?>(null) }
    val queue = remember { ArrayDeque<NsdServiceInfo>() }
    var resolving by remember { mutableStateOf(false) }
    var generation by remember { mutableStateOf(0) }
    fun stop() {
        generation++
        listener?.let { try { manager.stopServiceDiscovery(it) } catch (_: Exception) {} }
        listener = null; searching = false; queue.clear(); resolving = false
        multicast?.let { try { if (it.isHeld) it.release() } catch (_: Exception) {} }; multicast = null
    }
    fun start() {
        stop(); found.clear(); scanned = true; searching = true
        val active = generation
        try { multicast = wifi?.createMulticastLock("gregal-discovery")?.apply { setReferenceCounted(false); acquire() } } catch (_: Exception) {}
        lateinit var resolveNext: () -> Unit
        resolveNext = next@{
            if (active != generation || resolving || queue.isEmpty()) return@next
            resolving = true
            val service = queue.removeFirst()
            try {
                @Suppress("DEPRECATION")
                manager.resolveService(service, object : NsdManager.ResolveListener {
                    override fun onResolveFailed(info: NsdServiceInfo, code: Int) { main.post { if (active == generation) { resolving = false; resolveNext() } } }
                    override fun onServiceResolved(info: NsdServiceInfo) { main.post {
                        if (active == generation) {
                            @Suppress("DEPRECATION") val host = info.host
                            if (host is Inet4Address && info.port in 1..65535) {
                                val item = DiscoveredGregal(info.serviceName, host.hostAddress ?: "", info.port)
                                if (item.address.isNotEmpty() && found.none { it.address == item.address && it.port == item.port }) found.add(item)
                            }
                            resolving = false; resolveNext()
                        }
                    } }
                })
            } catch (_: Exception) { resolving = false; resolveNext() }
        }
        val search = object : NsdManager.DiscoveryListener {
            override fun onDiscoveryStarted(type: String) {}
            override fun onDiscoveryStopped(type: String) {}
            override fun onStartDiscoveryFailed(type: String, code: Int) { main.post { if (active == generation) stop() } }
            override fun onStopDiscoveryFailed(type: String, code: Int) {}
            override fun onServiceFound(info: NsdServiceInfo) { main.post { if (active == generation) { queue.addLast(info); resolveNext() } } }
            override fun onServiceLost(info: NsdServiceInfo) { main.post { if (active == generation) found.removeAll { it.name == info.serviceName } } }
        }
        listener = search
        try { manager.discoverServices("_gregal._tcp.", NsdManager.PROTOCOL_DNS_SD, search); main.postDelayed({ if (active == generation) stop() }, 10000) }
        catch (_: Exception) { stop() }
    }
    LaunchedEffect(Unit) { start() }
    DisposableEffect(Unit) { onDispose { stop() } }
    Column(Modifier.fillMaxWidth()) {
        TextButton(onClick = { start() }, enabled = !searching) { Text(if (lang == "ca") { if (searching) "Cercant servidors…" else "Cerca servidors a la xarxa" } else { if (searching) "Searching for servers…" else "Find servers on the network" }) }
        found.forEach { server -> OutlinedButton(onClick = { stop(); onSelect(server.address, server.port.toString()) }, modifier = Modifier.fillMaxWidth()) { Text("${server.name} · ${server.address}:${server.port}") } }
        if (scanned && !searching && found.isEmpty()) Text(if (lang == "ca") "No se n’ha trobat cap. Pots escriure l’adreça manualment." else "No server found. You can enter the address manually.")
    }
}
