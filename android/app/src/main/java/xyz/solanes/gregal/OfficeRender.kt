package xyz.solanes.gregal

import android.annotation.SuppressLint
import android.util.Base64
import android.webkit.WebView
import android.webkit.WebViewClient
import org.json.JSONObject
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.material3.Card
import androidx.compose.material3.CardDefaults
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.toArgb
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.compose.ui.viewinterop.AndroidView
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext

private const val RENDER_MAX = 8 * 1024 * 1024

/** Pàgina base: carrega els vendors del backend i exposa renderDoc(kind, b64).
 *  docx amb docx-preview; xlsx amb SheetJS sheet_to_html per full; pptx amb
 *  PptxViewJS en canvas amb anterior-seguent. Mateixes versions que la web. */
private fun renderShell(base: String, bg: String, fg: String, accent: String, lang: String): String = """
<!DOCTYPE html><html><head><meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<style>
body{background:$bg;color:$fg;font-family:sans-serif;margin:0;padding:8px;}
table.doc-grid{border-collapse:collapse;font-size:12px;}
table.doc-grid td,table.doc-grid th{border:1px solid $accent;padding:3px 8px;}
table.doc-grid th{background:$accent;color:$bg;}
h3.sheet{margin:14px 0 4px;font-size:14px;}
.pptx-nav{display:flex;gap:12px;align-items:center;margin-bottom:6px;}
.pptx-nav button{font-size:20px;padding:2px 14px;}
canvas{max-width:100%;background:#fff;}
.err{color:#b00;}
.docx-page{background:#fff;color:#000;margin-bottom:8px;padding:12px;}
</style></head><body><div id="box"></div>
<script>
const BASE = "$base";
const TXT = {
  loading:${JSONObject.quote(tr(lang, "app.off.renderLoading"))},
  emptySheet:${JSONObject.quote(tr(lang, "app.off.emptySheet"))},
  noSlides:${JSONObject.quote(tr(lang, "app.off.noSlides"))},
  errorPrefix:${JSONObject.quote(tr(lang, "app.off.renderErrorPrefix"))},
  errorSuffix:${JSONObject.quote(tr(lang, "app.off.renderErrorSuffix"))}
};
document.getElementById('box').textContent=TXT.loading;
function lib(f){return new Promise((res,rej)=>{const s=document.createElement('script');s.src=BASE+'/app/vendor/'+f+'?v=1';s.onload=res;s.onerror=()=>rej(new Error('no '+f));document.head.appendChild(s);});}
function b64bytes(b){const s=atob(b);const u=new Uint8Array(s.length);for(let i=0;i<s.length;i++)u[i]=s.charCodeAt(i);return u;}
function showError(e){const p=document.createElement('p');p.className='err';p.textContent=TXT.errorPrefix+(e&&e.message?e.message:String(e))+TXT.errorSuffix;const box=document.getElementById('box');box.replaceChildren(p);}
async function renderDoc(kind,b64){
  const box=document.getElementById('box');
  try{
    if(kind==='docx'){
      await lib('jszip.min.js');await lib('docx-preview.min.js');
      box.innerHTML='';
      const buf=b64bytes(b64).buffer;
      await window.docx.renderAsync(buf,box,null,{className:'gregal-docx',inWrapper:true,ignoreWidth:true,ignoreHeight:true,breakPages:true,renderHeaders:true,renderFooters:true,renderFootnotes:true});
    }else if(kind==='xlsx'){
      await lib('xlsx.mini.min.js');
      const wb=window.XLSX.read(b64bytes(b64),{type:'array'});
      let html='';
      for(const name of wb.SheetNames){
        const h=window.XLSX.utils.sheet_to_html(wb.Sheets[name],{header:'<h3 class="sheet">'+name+'</h3>'});
        html+=h;
      }
      box.innerHTML=html||TXT.emptySheet;
    }else{
      await lib('jszip.min.js');await lib('chart.umd.min.js');await lib('PptxViewJS.min.js');
      const buf=b64bytes(b64).buffer;
      box.innerHTML='<div class="pptx-nav"><button id="pv">‹</button><span id="ct"></span><button id="nx">›</button></div><canvas id="cv" width="960" height="540"></canvas>';
      const cv=document.getElementById('cv'),ct=document.getElementById('ct');
      const v=new window.PptxViewJS.PPTXViewer({canvas:cv});
      await v.loadFile(buf);
      const total=v.getSlideCount();
      if(!total)throw new Error(TXT.noSlides);
      const show=async i=>{await v.render(cv,{slideIndex:i});ct.textContent=(v.getCurrentSlideIndex()+1)+' / '+total;};
      document.getElementById('pv').onclick=async()=>{await v.previousSlide(cv);await v.render(cv);ct.textContent=(v.getCurrentSlideIndex()+1)+' / '+total;};
      document.getElementById('nx').onclick=async()=>{await v.nextSlide(cv);await v.render(cv);ct.textContent=(v.getCurrentSlideIndex()+1)+' / '+total;};
      window._pptx=v;await show(0);
    }
  }catch(e){showError(e);}
}
</script></body></html>""".trimIndent()

/** Vista fidel del document amb les mateixes llibreries vendoritzades que
 *  la web (servides pel backend a app-vendor: sempre en sync). Els bytes
 *  els baixa l'app (Bearer) i els injecta per pont JS: el WebView no toca
 *  l'API. Per sobre de 8 MB, cau al text (el pont JS s'ennuega). */
@SuppressLint("SetJavaScriptEnabled")
@Composable
fun OfficeRender(vm: GregalViewModel, doc: OfficeDoc, modifier: Modifier = Modifier) {
    val conn by vm.conn.collectAsState()
    val lang by vm.idioma.collectAsState()
    var error by remember(doc.id, lang) { mutableStateOf("") }
    var wv by remember(doc.id) { mutableStateOf<WebView?>(null) }

    Card(modifier.fillMaxWidth(), colors = CardDefaults.cardColors(containerColor = CodeDark)) {
        if (error.isNotBlank()) {
            Text(error, color = Ink, fontSize = 13.sp, modifier = Modifier.padding(12.dp))
        }
        AndroidView(
            factory = { ctx ->
                WebView(ctx).apply {
                    settings.javaScriptEnabled = true
                    settings.domStorageEnabled = true
                    settings.builtInZoomControls = true
                    settings.displayZoomControls = false
                    webViewClient = WebViewClient()
                    wv = this
                }
            },
            modifier = Modifier.fillMaxWidth().heightIn(min = 200.dp, max = 480.dp),
        )
    }

    LaunchedEffect(doc.id, wv, lang) {
        val w = wv ?: return@LaunchedEffect
        try {
            val base = baseUrlOf(conn.host, conn.port)
            val kind = when {
                doc.name.endsWith(".docx", true) || doc.kind.contains("word", true) -> "docx"
                doc.name.endsWith(".xlsx", true) || doc.kind.contains("sheet", true) -> "xlsx"
                else -> "pptx"
            }
            withContext(Dispatchers.Main) {
                w.loadDataWithBaseURL(base, renderShell(base, "#ffffff", "#111111", "#0E8C74", lang),
                    "text/html", "utf-8", null)
            }
            val bytes = withContext(Dispatchers.IO) { vm.officeDownloadBytes() }
            if (bytes == null) { error = tr(lang, "app.off.downloadError"); return@LaunchedEffect }
            if (bytes.size > RENDER_MAX) {
                error = tr(lang, "app.off.renderTooLarge", bytes.size / 1024 / 1024)
                return@LaunchedEffect
            }
            val b64 = withContext(Dispatchers.Default) { Base64.encodeToString(bytes, Base64.NO_WRAP) }
            // Espera que el shell hagi carregat abans d'injectar.
            withContext(Dispatchers.Main) {
                w.evaluateJavascript(
                    "(function(){let n=0;const t=setInterval(()=>{if(window.renderDoc||++n>100){clearInterval(t);} },50);})()", null)
                // Petit marge perquè el DOM estigui llest; renderDoc és global.
                w.postDelayed({
                    w.evaluateJavascript("renderDoc('$kind','$b64')", null)
                }, 600)
            }
        } catch (e: Exception) {
            error = tr(lang, "app.off.renderErrorPrefix") + e.message.orEmpty() +
                tr(lang, "app.off.renderErrorSuffix")
        }
    }
}
