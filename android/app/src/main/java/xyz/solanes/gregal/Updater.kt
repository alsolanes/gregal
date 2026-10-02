package xyz.solanes.gregal

import android.app.Notification
import android.app.NotificationChannel
import android.app.NotificationManager
import android.app.PendingIntent
import android.content.Context
import android.content.Intent
import android.net.ConnectivityManager
import android.net.NetworkCapabilities
import android.net.Uri
import android.os.Build
import android.provider.Settings
import java.io.File
import java.io.FileOutputStream
import java.net.HttpURLConnection
import java.net.URI
import java.net.URL
import java.nio.charset.StandardCharsets
import java.security.MessageDigest

/**
 * Actualitzacions dins l'app, portat del sistema de Marea
 * (`UpdateChecker.kt`) però sense cap dependència nova: Gregal Android és
 * HttpURLConnection + parser fet a mà, res d'OkHttp ni Gson.
 *
 * Què fa, igual que a Marea:
 *  - llegeix el manifest `gregal-android-version.json` (https, amb reintents),
 *  - el desa en caché 6 h per no trucar cada arrencada,
 *  - només accepta descàrregues del MATEIX host que el manifest,
 *  - baixa l'APK amb progrés i **verifica sha256 i mida** abans de donar-lo
 *    per bo (fitxer `.part` i moviment atòmic: mai un APK a mitges),
 *  - instal·la via FileProvider i, si cal, obre els ajustos de «fonts
 *    desconegudes»,
 *  - avisa per notificació i recorda la versió que has decidit ignorar.
 *
 * Les funcions pures (URL de confiança, parser del manifest, caché) es
 * testegen a la JVM: res d'aquí que toqui `android.*` no és verificable.
 */

// Distribution maintainers may set an HTTPS manifest for their own releases.
const val ANDROID_OTA_URL = ""
const val UPDATE_CHANNEL_ID = "gregal_updates"
const val UPDATE_NOTIF_ID = 4100
const val UPDATE_NOTIF_CHECK_ID = 4101
private const val CACHE_FILE = "gregal-update.json"
private const val CACHE_MAX_AGE_MS = 6 * 60 * 60 * 1000L
private const val MAX_MANIFEST_BYTES = 1024 * 1024
private const val MAX_APK_BYTES = 200L * 1024 * 1024

/** El manifest OTA tal com el publica la web (script, mai a mà). */
data class UpdateInfo(
    val versionCode: Int = 0,
    val versionName: String = "",
    val downloadUrl: String = "",
    val changelog: String = "",
    val sha256: String? = null,
    /** Alias per a desplegaments que anomenen el camp `size`. */
    val size: Long? = null,
    val sizeBytes: Long? = null,
) {
    fun hiHaNova(actual: Int): Boolean = versionCode > actual
    fun midaDeclarada(): Long? = sizeBytes ?: size
}

/** Resultat d'una comprovació, ja enfrontat amb la versió instal·lada. */
data class UpdateCheckResult(
    val isAvailable: Boolean = false,
    val latestVersion: String = "",
    val currentVersion: String = "",
    val changelog: String = "",
    val downloadUrl: String = "",
    val latestVersionCode: Int = 0,
    val sha256: String? = null,
    val sizeBytes: Long? = null,
)

/** Valor de `"clau"` al JSON cru: número o cadena (amb desescapament). */
private fun regexCamp(json: String, clau: String): String? {
    val num = Regex("\"$clau\"\\s*:\\s*(-?\\d+)").find(json)
    if (num != null) return num.groupValues[1]
    val txt = Regex("\"$clau\"\\s*:\\s*\"((?:[^\"\\\\]|\\\\.)*)\"").find(json) ?: return null
    return txt.groupValues[1]
        .replace("\\n", "\n").replace("\\t", "\t").replace("\\r", "\r")
        .replace("\\\"", "\"").replace("\\\\", "\\")
}

/**
 * Converteix el manifest en UpdateInfo. Es fa amb regex i no amb `org.json`
 * a posta: als tests de JVM `org.json` és un stub que peta («not mocked»), i
 * aquesta funció ha de ser verificable sense emulador.
 */
fun parseUpdateText(json: String): UpdateInfo = UpdateInfo(
    versionCode = regexCamp(json, "versionCode")?.toIntOrNull() ?: 0,
    versionName = regexCamp(json, "versionName") ?: "",
    downloadUrl = regexCamp(json, "downloadUrl") ?: "",
    changelog = regexCamp(json, "changelog") ?: "",
    sha256 = regexCamp(json, "sha256"),
    size = regexCamp(json, "size")?.toLongOrNull(),
    sizeBytes = regexCamp(json, "sizeBytes")?.toLongOrNull(),
)

/**
 * Només es descarrega del mateix host que el manifest i sempre per https.
 * Sense això, un manifest canviat podria enviar l'APK a qualsevol servidor.
 */
fun isTrustedDownloadUrl(manifestUrl: String, downloadUrl: String): Boolean = try {
    val m = URI(manifestUrl)
    val d = URI(downloadUrl)
    m.scheme.equals("https", true) &&
        d.scheme.equals("https", true) &&
        !m.host.isNullOrBlank() &&
        m.host.equals(d.host, ignoreCase = true)
} catch (_: Exception) {
    false
}

/** El manifest és plausible? (mida i hash declarats dins d'uns límits) */
fun manifestPlausible(info: UpdateInfo): Boolean {
    val mida = info.midaDeclarada()
    if (mida != null && (mida <= 0 || mida > MAX_APK_BYTES)) return false
    val sha = info.sha256
    if (sha != null && !sha.matches(Regex("[0-9a-fA-F]{64}"))) return false
    return true
}

/** `latest <= skipped` amaga l'avís; `skipped == 0` no amaga res. */
fun avisAmagat(latest: Int, skipped: Int): Boolean = skipped > 0 && latest <= skipped


/** Llegeix una URL absoluta sense token (manifest i APK són públics). */
fun fetchTextAbsolut(url: String, timeoutMs: Int = 10_000): String {
    val c = URL(url).openConnection() as HttpURLConnection
    c.connectTimeout = timeoutMs
    c.readTimeout = timeoutMs
    c.setRequestProperty("User-Agent", "Gregal-Android")
    try {
        c.inputStream.bufferedReader(StandardCharsets.UTF_8).use { r ->
            val sb = StringBuilder()
            val buf = CharArray(8192)
            while (sb.length < MAX_MANIFEST_BYTES) {
                val n = r.read(buf)
                if (n < 0) break
                sb.append(buf, 0, n)
            }
            return sb.toString()
        }
    } finally {
        c.disconnect()
    }
}

/** Caché del manifest: `{json}\n<epoch_ms>` — sense JSON a mitges. */
fun llegeixCache(cacheDir: File, nowMs: Long): UpdateInfo? = try {
    val f = File(cacheDir, CACHE_FILE)
    if (!f.exists()) null
    else if (nowMs - f.lastModified() > CACHE_MAX_AGE_MS) null
    else {
        val cos = f.readText()
        val tall = cos.lastIndexOf('\n')
        if (tall <= 0) null else parseUpdateText(cos.substring(0, tall))
    }
} catch (_: Exception) { null }

fun desaCache(cacheDir: File, json: String, nowMs: Long) {
    try {
        val f = File(cacheDir, CACHE_FILE)
        f.writeText(json + "\n" + nowMs)
    } catch (_: Exception) { }
}

fun netejaCache(cacheDir: File) {
    try { File(cacheDir, CACHE_FILE).delete() } catch (_: Exception) { }
}

/** Baixa l'APK amb progrés a `desti`, verificant mida i sha256. */
fun baixaApk(
    context: Context,
    url: String,
    manifestUrl: String,
    expectedSha256: String?,
    expectedSize: Long?,
    onProgress: (Int) -> Unit,
): File? {
    if (!isTrustedDownloadUrl(manifestUrl, url)) return null
    if (expectedSize != null && (expectedSize <= 0 || expectedSize > MAX_APK_BYTES)) return null
    if (expectedSha256 != null && !expectedSha256.matches(Regex("[0-9a-fA-F]{64}"))) return null
    val desti = File(context.cacheDir, "gregal-update.apk")
    val part = File(context.cacheDir, "gregal-update.apk.part")
    try {
        val c = URL(url).openConnection() as HttpURLConnection
        c.connectTimeout = 15_000
        c.readTimeout = 60_000
        c.setRequestProperty("User-Agent", "Gregal-Android")
        val total = if (c.contentLengthLong > 0) c.contentLengthLong else (expectedSize ?: -1L)
        if (c.contentLengthLong > MAX_APK_BYTES) return null
        if (expectedSize != null && c.contentLengthLong > 0 && c.contentLengthLong != expectedSize) return null
        val digest = MessageDigest.getInstance("SHA-256")
        var llegit = 0L
        c.inputStream.use { input ->
            FileOutputStream(part).use { out ->
                val buf = ByteArray(8192)
                while (true) {
                    val n = input.read(buf)
                    if (n < 0) break
                    if (llegit + n > MAX_APK_BYTES) return null
                    out.write(buf, 0, n)
                    digest.update(buf, 0, n)
                    llegit += n
                    if (total > 0) onProgress(((llegit * 100) / total).toInt())
                }
                out.flush()
                out.fd.sync()
            }
        }
        val hash = digest.digest().joinToString("") { "%02x".format(it) }
        if (expectedSize != null && part.length() != expectedSize) { part.delete(); return null }
        if (expectedSha256 != null && !hash.equals(expectedSha256, true)) { part.delete(); return null }
        if (part.length() >= MAX_APK_BYTES || part.length() <= 0) { part.delete(); return null }
        desti.delete()
        if (!part.renameTo(desti)) { part.delete(); return null }
        desti.setReadable(true, false)
        onProgress(100)
        return desti
    } catch (_: Exception) {
        part.delete()
        return null
    }
}

/** Instal·la l'APK baixat; si falta el permís, obre els ajustos. */
fun installaApk(context: Context, apk: File): Boolean {
    if (!apk.exists()) return false
    if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O &&
        !context.packageManager.canRequestPackageInstalls()
    ) {
        obreAjustosInstalacio(context)
        return false
    }
    return try {
        val uri = androidx.core.content.FileProvider.getUriForFile(
            context, "${context.packageName}.fileprovider", apk,
        )
        context.startActivity(
            Intent(Intent.ACTION_VIEW).apply {
                setDataAndType(uri, "application/vnd.android.package-archive")
                addFlags(Intent.FLAG_GRANT_READ_URI_PERMISSION)
                addFlags(Intent.FLAG_ACTIVITY_NEW_TASK)
            }
        )
        true
    } catch (_: Exception) {
        obreAjustosInstalacio(context)
        false
    }
}

fun obreAjustosInstalacio(context: Context) {
    try {
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
            context.startActivity(
                Intent(Settings.ACTION_MANAGE_UNKNOWN_APP_SOURCES).apply {
                    data = Uri.parse("package:${context.packageName}")
                    addFlags(Intent.FLAG_ACTIVITY_NEW_TASK)
                }
            )
        }
    } catch (_: Exception) {
        try {
            context.startActivity(
                Intent(Settings.ACTION_SECURITY_SETTINGS).addFlags(Intent.FLAG_ACTIVITY_NEW_TASK)
            )
        } catch (_: Exception) { }
    }
}

fun potInstalar(context: Context): Boolean =
    if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) context.packageManager.canRequestPackageInstalls()
    else true

fun esWifi(context: Context): Boolean {
    val cm = context.getSystemService(Context.CONNECTIVITY_SERVICE) as? ConnectivityManager ?: return false
    val xarxa = cm.activeNetwork ?: return false
    val caps = cm.getNetworkCapabilities(xarxa) ?: return false
    return caps.hasTransport(NetworkCapabilities.TRANSPORT_WIFI)
}

/** Canal de notificacions de les actualitzacions (una vegada, a l'arrencada). */
fun creaCanalActualitzacions(context: Context) {
    if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
        val lang = Prefs(context).idioma
        val ch = NotificationChannel(
            UPDATE_CHANNEL_ID, tr(lang, "app.update.channel"), NotificationManager.IMPORTANCE_DEFAULT,
        ).apply { description = tr(lang, "app.update.channelDescription") }
        context.getSystemService(NotificationManager::class.java)?.createNotificationChannel(ch)
    }
}

fun avisaActualitzacio(context: Context, latest: String, changelog: String) {
    try {
        val lang = Prefs(context).idioma
        val launch = context.packageManager.getLaunchIntentForPackage(context.packageName) ?: return
        val pi = PendingIntent.getActivity(
            context, 0, launch,
            PendingIntent.FLAG_IMMUTABLE or PendingIntent.FLAG_UPDATE_CURRENT,
        )
        val n = Notification.Builder(context, UPDATE_CHANNEL_ID)
            .setSmallIcon(R.drawable.ic_stat_gregal)
            .setContentTitle(tr(lang, "app.update.availableNotification", latest))
            .setContentText(changelog.ifBlank { tr(lang, "app.update.tapToUpdate") })
            .setContentIntent(pi)
            .setAutoCancel(true)
            .build()
        context.getSystemService(NotificationManager::class.java)?.notify(UPDATE_NOTIF_CHECK_ID, n)
    } catch (_: Exception) { }
}

fun cancelAvisActualitzacio(context: Context) {
    try {
        context.getSystemService(NotificationManager::class.java)?.cancel(UPDATE_NOTIF_CHECK_ID)
    } catch (_: Exception) { }
}

fun avisaBaixada(context: Context, apk: File, version: String) {
    try {
        val lang = Prefs(context).idioma
        val uri = androidx.core.content.FileProvider.getUriForFile(
            context, "${context.packageName}.fileprovider", apk,
        )
        val pi = PendingIntent.getActivity(
            context, 0,
            Intent(Intent.ACTION_VIEW).apply {
                setDataAndType(uri, "application/vnd.android.package-archive")
                addFlags(Intent.FLAG_GRANT_READ_URI_PERMISSION)
                addFlags(Intent.FLAG_ACTIVITY_NEW_TASK)
            },
            PendingIntent.FLAG_IMMUTABLE or PendingIntent.FLAG_UPDATE_CURRENT,
        )
        val n = Notification.Builder(context, UPDATE_CHANNEL_ID)
            .setSmallIcon(R.drawable.ic_stat_gregal)
            .setContentTitle(tr(lang, "app.update.downloadedNotification", version))
            .setContentText(tr(lang, "app.update.tapToInstall"))
            .setContentIntent(pi)
            .setAutoCancel(true)
            .build()
        context.getSystemService(NotificationManager::class.java)?.notify(UPDATE_NOTIF_ID, n)
    } catch (_: Exception) { }
}
