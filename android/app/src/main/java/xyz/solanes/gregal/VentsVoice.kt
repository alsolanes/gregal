package xyz.solanes.gregal

import android.content.Context
import android.content.Intent
import android.speech.RecognizerIntent
import android.speech.tts.TextToSpeech
import android.speech.tts.UtteranceProgressListener
import androidx.activity.compose.rememberLauncherForActivityResult
import androidx.activity.result.contract.ActivityResultContracts
import androidx.compose.foundation.layout.size
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.rounded.VolumeUp
import androidx.compose.material.icons.rounded.Stop
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.runtime.Composable
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.remember
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.unit.dp
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import java.util.Locale

/**
 * La veu dels Vents, com als grans assistents: dictar al camp (el reconeixedor
 * del sistema, sense permís de micròfon) i escoltar una resposta (el motor de
 * veu del telèfon). Tot local i opcional: si el telèfon no en té, els botons no
 * surten.
 */

internal fun voiceCopy(lang: String, key: String): String {
    val en = lang == "en"
    return when (key) {
        "dictate" -> if (en) "Dictate" else "Dicta"
        "listen" -> if (en) "Read aloud" else "Llegeix en veu alta"
        "stop" -> if (en) "Stop reading" else "Atura la lectura"
        "prompt" -> if (en) "Speak now" else "Parla ara"
        else -> key
    }
}

/** El que es llegeix en veu alta: sense marques, codi, taules, imatges ni enllaços crus. */
internal fun speakableText(md: String): String {
    var t = md.replace(Regex("```[\\s\\S]*?```"), " ")
    t = t.lines().filterNot { it.trimStart().startsWith("|") }.joinToString("\n")
    t = t.replace(Regex("!\\[[^\\]]*]\\([^)]*\\)"), " ")
    t = t.replace(Regex("\\[([^\\]]+)]\\([^)]*\\)"), "$1")
    t = t.replace(Regex("https?://\\S+"), " ")
    t = stripMdInline(t)
    return t.replace(Regex("[ \\t]+"), " ").replace(Regex("\\n{2,}"), "\n").trim()
}

internal fun speechLocale(lang: String): Locale = if (lang == "en") Locale.US else Locale("ca", "ES")

internal object VentsSpeaker {
    private var tts: TextToSpeech? = null
    private var ready = false
    private var pending: (() -> Unit)? = null
    private val _speaking = MutableStateFlow<String?>(null)
    /** La clau del missatge que s'està llegint (null si cap). */
    val speaking: StateFlow<String?> = _speaking

    private fun engine(ctx: Context): TextToSpeech? {
        tts?.let { return it }
        val t = TextToSpeech(ctx.applicationContext) { status ->
            ready = status == TextToSpeech.SUCCESS
            if (ready) pending?.invoke() else _speaking.value = null
            pending = null
        }
        t.setOnUtteranceProgressListener(object : UtteranceProgressListener() {
            override fun onStart(id: String?) {}
            override fun onDone(id: String?) { if (_speaking.value == id) _speaking.value = null }
            @Deprecated("Deprecated in Java") override fun onError(id: String?) { if (_speaking.value == id) _speaking.value = null }
        })
        tts = t
        return t
    }

    fun speak(ctx: Context, key: String, text: String, lang: String) {
        val clean = speakableText(text)
        if (clean.isBlank()) return
        val t = engine(ctx) ?: return
        _speaking.value = key
        val go = {
            val loc = speechLocale(lang)
            // Sense veu catalana instal·lada, la del telèfon (millor que res).
            if (t.isLanguageAvailable(loc) >= TextToSpeech.LANG_AVAILABLE) t.language = loc
            // El motor té un límit per frase: es trosseja per paràgrafs.
            val parts = clean.split("\n").map { it.trim() }.filter { it.isNotEmpty() }
                .flatMap { it.chunked(TextToSpeech.getMaxSpeechInputLength() - 1) }
            parts.forEachIndexed { i, p ->
                t.speak(p, if (i == 0) TextToSpeech.QUEUE_FLUSH else TextToSpeech.QUEUE_ADD, null,
                    if (i == parts.lastIndex) key else "$key#$i")
            }
        }
        if (ready) go() else pending = go
    }

    fun stop() {
        tts?.stop()
        _speaking.value = null
    }
}

/** L'altaveu sota una resposta: un toc la llegeix, un altre l'atura. */
@Composable
internal fun ReadAloudButton(key: String, text: String, lang: String, tint: Color) {
    val ctx = LocalContext.current
    val speaking by VentsSpeaker.speaking.collectAsState()
    val on = speaking == key
    IconButton(onClick = { if (on) VentsSpeaker.stop() else VentsSpeaker.speak(ctx, key, text, lang) },
        modifier = Modifier.size(28.dp)) {
        Icon(if (on) Icons.Rounded.Stop else Icons.AutoMirrored.Rounded.VolumeUp,
            voiceCopy(lang, if (on) "stop" else "listen"), tint = tint, modifier = Modifier.size(16.dp))
    }
}

/**
 * Dictar: torna l'acció que obre el reconeixedor del sistema (o null si el
 * telèfon no en té). El text reconegut arriba a [onText].
 */
@Composable
internal fun rememberDictation(lang: String, onText: (String) -> Unit): (() -> Unit)? {
    val ctx = LocalContext.current
    val available = remember {
        try {
            ctx.packageManager.queryIntentActivities(Intent(RecognizerIntent.ACTION_RECOGNIZE_SPEECH), 0).isNotEmpty()
        } catch (_: Exception) { false }
    }
    val launcher = rememberLauncherForActivityResult(ActivityResultContracts.StartActivityForResult()) { res ->
        res.data?.getStringArrayListExtra(RecognizerIntent.EXTRA_RESULTS)?.firstOrNull()
            ?.trim()?.takeIf { it.isNotEmpty() }?.let(onText)
    }
    if (!available) return null
    return {
        val tag = speechLocale(lang).toLanguageTag()
        val intent = Intent(RecognizerIntent.ACTION_RECOGNIZE_SPEECH)
            .putExtra(RecognizerIntent.EXTRA_LANGUAGE_MODEL, RecognizerIntent.LANGUAGE_MODEL_FREE_FORM)
            .putExtra(RecognizerIntent.EXTRA_LANGUAGE, tag)
            .putExtra(RecognizerIntent.EXTRA_LANGUAGE_PREFERENCE, tag)
            .putExtra(RecognizerIntent.EXTRA_PROMPT, voiceCopy(lang, "prompt"))
        try { launcher.launch(intent) } catch (_: Exception) { }
    }
}

/** Afegeix el que s'ha dictat al que ja hi havia al camp. */
internal fun appendDictated(current: String, dictated: String): String =
    (current.trimEnd() + " " + dictated.trim()).trim()
