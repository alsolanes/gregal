package xyz.solanes.gregal

import android.annotation.SuppressLint
import android.content.Intent
import android.net.Uri
import android.webkit.WebResourceError
import android.webkit.WebResourceRequest
import android.webkit.WebView
import android.webkit.WebViewClient
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.rounded.ArrowBack
import androidx.compose.material.icons.rounded.ContentCopy
import androidx.compose.material.icons.rounded.OpenInNew
import androidx.compose.material.icons.rounded.Refresh
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.LinearProgressIndicator
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.runtime.DisposableEffect
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.LocalClipboardManager
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.text.AnnotatedString
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.compose.ui.viewinterop.AndroidView
import androidx.core.content.ContextCompat

/**
 * Previsualitzar la feina: la web que el vent acaba de fer, dins de l'app.
 *
 * Abans calia que el vent engegués un `python3 -m http.server` al servidor:
 * moria en acabar l'ordre i el port canviava a cada intent, així que la URL
 * que donava ja era morta. Aquí el servidor és el Gregal, la carpeta de
 * treball es publica en lectura i aquesta pantalla la carrega amb el WebView
 * del telèfon: res a teclejar.
 */
@SuppressLint("SetJavaScriptEnabled")
@Composable
fun PreviewScreen(vm: GregalViewModel, onMenu: () -> Unit) {
    val idioma by vm.idioma.collectAsState()
    val url by vm.webUrl.collectAsState()
    val titol by vm.webTitle.collectAsState()
    val error by vm.webError.collectAsState()
    val ctx = LocalContext.current
    val clipboard = LocalClipboardManager.current
    var carregant by remember { mutableStateOf(true) }
    var fallada by remember { mutableStateOf("") }
    var refresc by remember { mutableStateOf(0) }
    // El WebView viu fora de Compose: es guarda per poder recarregar i aturar-lo.
    var web by remember { mutableStateOf<WebView?>(null) }
    fun t(k: String, vararg args: Any?) = tr(idioma, k, *args)

    DisposableEffect(Unit) {
        vm.loadPreview()
        onDispose { web?.stopLoading(); web?.destroy(); web = null }
    }

    GregalBg {
        Scaffold(
            containerColor = Color.Transparent,
            topBar = {
                GregalTopBar(titol.ifBlank { t("app.preview.title") }, idioma, onMenu = onMenu, onBack = { vm.go(Screen.Carpeta) }, actions = {
                    if (url.isNotBlank()) {
                        IconButton(onClick = { refresc++ }) {
                            Icon(Icons.Rounded.Refresh, t("app.preview.reload"), tint = Muted)
                        }
                        IconButton(onClick = {
                            clipboard.setText(AnnotatedString(url))
                            vm.notify(t("app.vents.linkCopied"))
                        }) {
                            Icon(Icons.Rounded.ContentCopy, t("app.vents.copyLink"), tint = Muted)
                        }
                        IconButton(onClick = {
                            runCatching {
                                ContextCompat.startActivity(ctx, Intent(Intent.ACTION_VIEW, Uri.parse(url)), null)
                            }
                        }) {
                            Icon(Icons.Rounded.OpenInNew, t("app.preview.external"), tint = Muted)
                        }
                    }
                })
            },
            snackbarHost = { NoticeHost(vm) },
        ) { pad ->
            Column(Modifier.fillMaxSize().padding(pad)) {
                if (carregant) {
                    LinearProgressIndicator(Modifier.fillMaxWidth().size(2.dp), color = Sea, trackColor = Line)
                }
                if (url.isBlank()) {
                    Box(Modifier.fillMaxSize(), contentAlignment = Alignment.Center) {
                        Column(horizontalAlignment = Alignment.CenterHorizontally, verticalArrangement = Arrangement.spacedBy(10.dp)) {
                            Text(t("app.preview.pick"), color = Muted, fontSize = 14.sp)
                            if (error.isNotBlank()) Text(error, color = Muted, fontSize = 12.sp)
                            TextButton(onClick = { vm.go(Screen.Carpeta) }) {
                                Text(t("app.carpeta.title"), color = Sea, fontWeight = FontWeight.SemiBold)
                            }
                        }
                    }
                    return@Column
                }
                if (fallada.isNotBlank() || error.isNotBlank()) {
                    Box(Modifier.fillMaxWidth().padding(16.dp)) {
                        GCard(Modifier.fillMaxWidth()) {
                            Text(
                                (fallada.ifBlank { error }) + "\n\n" + url,
                                color = Ink, fontSize = 13.sp,
                            )
                            Text(t("app.preview.hint"), color = Muted, fontSize = 12.sp)
                        }
                    }
                }
                AndroidView(
                    modifier = Modifier.fillMaxSize(),
                    factory = { c ->
                        WebView(c).apply {
                            settings.javaScriptEnabled = true
                            // Els jocs desen la puntuació a localStorage.
                            settings.domStorageEnabled = true
                            settings.allowFileAccess = false
                            settings.setSupportZoom(false)
                            webViewClient = object : WebViewClient() {
                                override fun onPageFinished(view: WebView?, u: String?) {
                                    carregant = false
                                }
                                override fun onReceivedError(view: WebView?, req: WebResourceRequest?, e: WebResourceError?) {
                                    // Només la pàgina principal: un asset que falta no tomba la vista.
                                    if (req?.isForMainFrame == true) {
                                        carregant = false
                                        fallada = e?.description?.toString().orEmpty().ifBlank { "error" }
                                    }
                                }
                            }
                            web = this
                        }
                    },
                    update = { view ->
                        // Es recarrega quan canvia la URL o quan es prem refrescar.
                        val key = url + "#" + refresc
                        if (view.tag != key) {
                            view.tag = key
                            carregant = true
                            fallada = ""
                            view.loadUrl(url)
                        }
                    },
                )
            }
        }
    }
}
