package xyz.solanes.gregal

import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.rememberLazyListState
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.rounded.History
import androidx.compose.material.icons.rounded.Refresh
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import org.json.JSONArray
import org.json.JSONObject
import java.time.LocalDate
import java.time.OffsetDateTime
import java.time.ZoneId

// L'activitat dels vents: el que han fet en segon pla (feines, novetats,
// rutines, cors i recordatoris), de la més nova a la més vella, per dies.
// Llegeix GET /api/winds/activity i demana les més velles amb `before`.

/** Quantes en demana cada vegada: una pàgina curta vol dir que no n'hi ha més. */
const val WIND_ACTIVITY_PAGE = 50

/** Una cosa que un vent ha fet. `at` és RFC 3339, tal com ve del servidor. */
data class WindActivityItem(val at: String, val wind: String, val kind: String, val title: String, val status: String, val ref: String)

/** La llista oberta: de quin vent ("" = tots), el que ja hi ha i si en queden de més velles. */
data class WindActivityFeed(
    val wind: String = "",
    val items: List<WindActivityItem> = emptyList(),
    val loading: Boolean = false,
    val loaded: Boolean = false,
    val more: Boolean = false,
    val error: String? = null,
)

fun parseWindActivity(raw: String): List<WindActivityItem> {
    val arr = JSONObject(raw).optJSONArray("items") ?: JSONArray()
    return (0 until arr.length()).mapNotNull { i ->
        val x = arr.optJSONObject(i) ?: return@mapNotNull null
        WindActivityItem(x.optString("at"), x.optString("wind"), x.optString("kind"), x.optString("title"), x.optString("status"), x.optString("ref"))
    }
}

fun GregalApi.getWindActivity(wind: String, before: String, limit: Int): List<WindActivityItem> {
    val q = listOfNotNull(
        wind.takeIf { it.isNotBlank() }?.let { "wind=" + java.net.URLEncoder.encode(it, "UTF-8") },
        // L'«at» porta un «+» de zona: sense codificar arribaria com un espai.
        before.takeIf { it.isNotBlank() }?.let { "before=" + java.net.URLEncoder.encode(it, "UTF-8") },
        "limit=$limit",
    ).joinToString("&")
    return parseWindActivity(get("/api/winds/activity?$q").toString())
}

private fun activityInstant(at: String, zone: ZoneId) =
    runCatching { OffsetDateTime.parse(at).atZoneSameInstant(zone) }.getOrNull()

/** Els elements per dia local, en l'ordre que venen (els més nous primer). Pura. */
fun windActivityDays(items: List<WindActivityItem>, zone: ZoneId = ZoneId.systemDefault()): List<Pair<LocalDate?, List<WindActivityItem>>> =
    items.groupBy { activityInstant(it.at, zone)?.toLocalDate() }.toList()

/** «18:05» en l'hora del telèfon. Pura. */
fun windActivityTime(at: String, zone: ZoneId = ZoneId.systemDefault()): String =
    activityInstant(at, zone)?.let { "%02d:%02d".format(it.hour, it.minute) }.orEmpty()

/** Un element: la cara del vent, l'hora, què ha fet i com ha anat. */
@Composable
fun WindActivityRow(lang: String, item: WindActivityItem, wind: Wind?, onOpen: (WindActivityItem) -> Unit) {
    val accent = Color(wind?.argb ?: 0xFFFF9F00)
    Surface(color = Ink.copy(alpha = 0.04f), shape = MaterialTheme.shapes.medium,
        modifier = Modifier.fillMaxWidth().clickable { onOpen(item) }) {
        Row(Modifier.padding(12.dp), verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(10.dp)) {
            Box(Modifier.testTag("activity-face")) { WindFace(size = 36, accent = accent, style = wind?.faceStyle ?: WindStyle()) }
            Column(Modifier.weight(1f), verticalArrangement = Arrangement.spacedBy(2.dp)) {
                Text(item.title.ifBlank { tr(lang, "winds.activity.kind.${item.kind}") }, color = Ink, fontWeight = FontWeight.SemiBold,
                    fontSize = 14.sp, maxLines = 2, overflow = TextOverflow.Ellipsis)
                Text(listOf(windActivityTime(item.at), wind?.name ?: item.wind, tr(lang, "winds.activity.kind.${item.kind}"))
                    .filter { it.isNotBlank() }.joinToString(" · "), color = Muted, fontSize = 12.sp, maxLines = 1, overflow = TextOverflow.Ellipsis)
            }
            if (item.status.isNotBlank()) ActivityStatusChip(lang, item.status)
        }
    }
}

@Composable
private fun ActivityStatusChip(lang: String, status: String) {
    val color = when (status) {
        "done", "ok", "read" -> Algua
        "failed", "error", "interrupted" -> Red
        "running", "unread" -> Sea
        else -> Muted
    }
    // Un estat nou del servidor surt tal com ve.
    val label = tr(lang, "winds.activity.status.$status").takeIf { it != "winds.activity.status.$status" } ?: status
    Surface(color = color.copy(alpha = 0.18f), shape = RoundedCornerShape(50)) {
        Text(label, color = color, fontSize = 11.sp, fontWeight = FontWeight.SemiBold, maxLines = 1,
            modifier = Modifier.padding(horizontal = 8.dp, vertical = 2.dp))
    }
}

/** La pantalla: d'un vent o de tots, per dies; arribar al final en carrega més. */
@Composable
internal fun WindActivityScreen(vm: GregalViewModel, onMenu: () -> Unit) {
    val lang by vm.idioma.collectAsState()
    val feed by vm.windActivity.collectAsState()
    val winds by vm.winds.collectAsState()
    val byId = winds.associateBy { it.id }
    val title = tr(lang, "winds.activity.title") + (byId[feed.wind]?.name?.let { " · $it" } ?: "")
    val list = rememberLazyListState()
    val atEnd by remember { derivedStateOf {
        val info = list.layoutInfo
        info.totalItemsCount > 0 && (info.visibleItemsInfo.lastOrNull()?.index ?: 0) >= info.totalItemsCount - 1
    } }
    LaunchedEffect(atEnd, feed.items.size, feed.more) {
        if (atEnd && feed.more && feed.error == null) vm.loadWindActivity(older = true)
    }
    // Les tasques i novetats són al xat del vent; Android no té pantalla de runs d'una rutina.
    val open: (WindActivityItem) -> Unit = { vm.openWindActivityItem(it) }
    Scaffold(containerColor = Color.Transparent, snackbarHost = { NoticeHost(vm) }, topBar = {
        GregalTopBar(title, lang, onMenu = onMenu,
            actions = { IconButton(onClick = { vm.loadWindActivity() }) { Icon(Icons.Rounded.Refresh, tr(lang, "auto.refresh")) } })
    }) { pad ->
        when {
            !feed.loaded -> LinearProgressIndicator(Modifier.fillMaxWidth().padding(pad))
            feed.items.isEmpty() && feed.error != null -> ErrorBoard(tr(lang, "act.loadError", feed.error), "", lang,
                onRetry = { vm.loadWindActivity() }, modifier = Modifier.padding(pad))
            feed.items.isEmpty() -> EmptyState(Icons.Rounded.History, tr(lang, "winds.activity.title"),
                tr(lang, if (feed.wind.isBlank()) "winds.activity.emptyAll" else "winds.activity.emptyWind"), Modifier.padding(pad))
            else -> LazyColumn(Modifier.fillMaxSize().padding(pad), state = list, contentPadding = PaddingValues(16.dp),
                verticalArrangement = Arrangement.spacedBy(8.dp)) {
                for ((day, items) in windActivityDays(feed.items)) {
                    item(key = "day-$day") {
                        val label = day?.let { savedDay(lang, it.atStartOfDay(ZoneId.systemDefault()).toEpochSecond()) }.orEmpty()
                        Text(label.uppercase(), color = Muted, style = GregalType.overline, modifier = Modifier.padding(top = 8.dp))
                    }
                    for (it in items) item(key = it.kind + it.ref + it.at) { WindActivityRow(lang, it, byId[it.wind], open) }
                }
                item(key = "end") {
                    when {
                        feed.loading -> LinearProgressIndicator(Modifier.fillMaxWidth())
                        feed.error != null -> TextButton(onClick = { vm.loadWindActivity(older = true) }) {
                            Text(tr(lang, "act.loadError", feed.error) + " · " + tr(lang, "app.chat.retry"))
                        }
                    }
                }
            }
        }
    }
}
