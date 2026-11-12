package xyz.solanes.gregal

import androidx.compose.foundation.combinedClickable
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.background
import androidx.compose.foundation.horizontalScroll
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.layout.widthIn
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.rounded.ContentCopy
import androidx.compose.material3.Icon
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.remember
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.hapticfeedback.HapticFeedbackType
import androidx.compose.ui.platform.LocalClipboardManager
import androidx.compose.ui.platform.LocalHapticFeedback
import androidx.compose.ui.platform.LocalUriHandler
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.AnnotatedString
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp

/**
 * Enllaços d'un missatge, a punt per copiar.
 *
 * Al mòbil, tocar un enllaç dins del text l'obre (com sempre): per copiar-lo
 * calia mantenir premuda la bombolla sencera i després retallar l'adreça a mà.
 * Aquí cada URL del missatge surt com una pastilla sota el text: un toc la
 * copia (amb vibració i avís) i una premuda llarga l'obre, igual que el text.
 */
internal data class MessageLink(val label: String, val url: String)

/** URLs crues dins del text (fora de blocs de codi ni d'imatges markdown). */
private val rawUrlRe = Regex("""https?://[^\s<>"'`]+""")

/** Enllaços markdown [text](url): la imatge ![alt](url) no és un enllaç. */
private val mdLinkRe = Regex("""(?<!!)\[([^\]]*)]\((https?://[^)\s]+)\)""")

/** Imatges markdown: es pinten com a imatge, no com a enllaç copiable. */
private val mdImageRe = Regex("""!\[[^\]]*]\(https?://[^)\s]+\)""")

/** Cues que la prosa enganxa a l'adreça i que no en formen part. */
private const val URL_TAIL = ".,;:!?»”'\"*_)]}"

/** Treu el que la prosa afegeix al final d'una adreça («…/x).», «…/x,»). */
internal fun trimUrlTail(raw: String): String = raw.trim().trimEnd { it in URL_TAIL }

/** El nom curt d'una adreça per a la pastilla: el domini sense www. */
internal fun linkHost(url: String): String {
    var h = url.trim()
    val scheme = h.indexOf("://")
    if (scheme >= 0) h = h.substring(scheme + 3)
    h = h.substringBefore('/').substringBefore('?').substringBefore('#')
    h = h.substringAfter('@') // usuari:clau@host
    if (h.startsWith("www.", ignoreCase = true)) h = h.substring(4)
    return h.ifBlank { url }
}

/**
 * Tots els enllaços d'un missatge, en ordre d'aparició i sense repetir.
 * Les imatges markdown queden fora: es pinten com a imatge, no com a enllaç.
 * Pura: testejable.
 */
internal fun messageLinks(src: String, max: Int = 6): List<MessageLink> {
    if (src.isBlank()) return emptyList()
    // Els blocs de codi es poden menjar mitja resposta: fora del recompte.
    // Les imatges també: es pinten com a imatge i la seva URL no s'ha de
    // poder copiar com si fos una font.
    val text = src.replace(Regex("(?s)```.*?```"), " ").replace(mdImageRe, " ")
    data class Hit(val at: Int, val label: String, val url: String)
    val hits = mutableListOf<Hit>()
    val covered = mutableListOf<IntRange>()
    for (m in mdLinkRe.findAll(text)) {
        hits += Hit(m.range.first, m.groupValues[1], trimUrlTail(m.groupValues[2]))
        covered += m.range
    }
    for (m in rawUrlRe.findAll(text)) {
        if (covered.any { m.range.first in it }) continue
        hits += Hit(m.range.first, "", trimUrlTail(m.value))
    }
    // Ordre del missatge: el lector els llegeix d'aquí a baix.
    val found = mutableListOf<MessageLink>()
    val seen = mutableSetOf<String>()
    for (h in hits.sortedBy { it.at }) {
        val clean = h.url
        if (!clean.startsWith("http://") && !clean.startsWith("https://")) continue
        if (!seen.add(clean)) continue
        val shown = h.label.trim().takeIf { it.isNotEmpty() && !it.startsWith("http") } ?: linkHost(clean)
        found += MessageLink(shown, clean)
        if (found.size == max) break
    }
    return found
}

/**
 * Fila de pastilles amb els enllaços del missatge: un toc copia, una premuda
 * llarga obre al navegador. [onCopied] deixa avisar (p. ex. «Enllaç copiat»).
 */
@OptIn(androidx.compose.foundation.ExperimentalFoundationApi::class)
@Composable
internal fun MessageLinks(
    lang: String,
    text: String,
    link: Color,
    modifier: Modifier = Modifier,
    onCopied: (() -> Unit)? = null,
) {
    val links = remember(text) { messageLinks(text) }
    if (links.isEmpty()) return
    val clipboard = LocalClipboardManager.current
    val uri = LocalUriHandler.current
    val haptic = LocalHapticFeedback.current
    Row(
        modifier.fillMaxWidth().horizontalScroll(rememberScrollState()).padding(top = 6.dp),
        horizontalArrangement = Arrangement.spacedBy(6.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        // Les fonts, lleugeres (direcció visual §4): un punt i el nom; tocar obre, mantenir premut copia.
        Text(if (lang == "en") "Sources" else "Fonts", color = Muted, fontSize = 12.sp)
        links.forEach { l ->
            Row(
                Modifier.clip(RoundedCornerShape(8.dp))
                    .combinedClickable(
                        onClick = { uri.openUri(l.url) },
                        onLongClick = {
                            haptic.performHapticFeedback(HapticFeedbackType.LongPress)
                            clipboard.setText(AnnotatedString(l.url))
                            onCopied?.invoke()
                        },
                    )
                    .semantics { contentDescription = l.label }
                    .heightIn(min = 32.dp).padding(horizontal = 4.dp),
                verticalAlignment = Alignment.CenterVertically,
            ) {
                Box(Modifier.size(8.dp).clip(RoundedCornerShape(50)).background(link.copy(alpha = 0.55f)))
                Spacer(Modifier.width(5.dp))
                Text(
                    l.label, color = link, fontSize = 12.5.sp, fontWeight = FontWeight.Medium,
                    maxLines = 1, overflow = TextOverflow.Ellipsis,
                    modifier = Modifier.widthIn(max = 170.dp),
                )
            }
        }
    }
}
