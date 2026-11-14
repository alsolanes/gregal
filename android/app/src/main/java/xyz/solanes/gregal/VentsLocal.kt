package xyz.solanes.gregal

import android.Manifest
import android.annotation.SuppressLint
import android.content.Context
import android.content.pm.PackageManager
import android.location.Location
import android.location.LocationManager
import android.os.Build
import androidx.activity.compose.rememberLauncherForActivityResult
import androidx.activity.result.contract.ActivityResultContracts
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.*
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.rounded.MyLocation
import androidx.compose.material.icons.rounded.Place
import androidx.compose.material3.Checkbox
import androidx.compose.material3.CheckboxDefaults
import androidx.compose.material3.Icon
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.OutlinedTextFieldDefaults
import androidx.compose.material3.Switch
import androidx.compose.material3.SwitchDefaults
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.core.content.ContextCompat
import org.json.JSONArray
import org.json.JSONObject

// «A prop teu»: novetats locals del lloc on ets (concerts, festes majors,
// esdeveniments, obertures, notícies de la zona), a la safata «Avui» amb la
// font citada. Mateixa API i mateixes regles que vents-local.js de la web
// (/api/winds/local): consentiment explícit, només el nom de la ciutat o el
// barri (la ubicació del mòbil és aproximada, s'arrodoneix a ~1 km i no es
// desa mai), temes, distància, freqüència, hora i veu.

data class LocalPlace(
    val label: String = "",
    val region: String = "",
    val country: String = "",
    /** manual | device | browser */
    val source: String = "manual",
)

data class LocalTopic(val id: String, val label: String)

data class LocalInfo(
    val enabled: Boolean = false,
    val consent: Boolean = false,
    val place: LocalPlace = LocalPlace(),
    val topics: List<String> = listOf("concerts", "festes", "events", "openings", "news"),
    val interests: String = "",
    val avoid: String = "",
    val radiusKm: Int = 10,
    /** daily | 2days | weekly */
    val frequency: String = "daily",
    val time: String = "09:00",
    val wind: String = "",
    val voice: Boolean = false,
    val catalog: List<LocalTopic> = emptyList(),
    val nextDueAt: String = "",
)

/** Lectura i escriptura pures de /api/winds/local (es proven sense xarxa). */
object LocalJson {
    val RADII = listOf(2, 5, 10, 25, 50)
    val FREQUENCIES = listOf("daily", "2days", "weekly")

    fun place(o: JSONObject?): LocalPlace = if (o == null) LocalPlace() else LocalPlace(
        label = o.optString("label"), region = o.optString("region"), country = o.optString("country"),
        source = o.optString("source").ifBlank { "manual" },
    )

    /** GET o POST /api/winds/local → la vista. */
    fun view(raw: String): LocalInfo {
        val o = JSONObject(raw)
        val s = o.optJSONObject("settings") ?: JSONObject()
        val topics = s.optJSONArray("topics") ?: JSONArray()
        val cat = o.optJSONArray("topics") ?: JSONArray()
        return LocalInfo(
            enabled = s.optBoolean("enabled"),
            consent = s.optLong("consent") > 0,
            place = place(s.optJSONObject("place")),
            topics = (0 until topics.length()).map { topics.optString(it) }.filter { it.isNotBlank() },
            interests = s.optString("interests"), avoid = s.optString("avoid"),
            radiusKm = s.optInt("radius_km", 10).takeIf { it > 0 } ?: 10,
            frequency = s.optString("frequency").takeIf { it in FREQUENCIES } ?: "daily",
            time = s.optString("time").ifBlank { "09:00" },
            wind = s.optString("wind"), voice = s.optBoolean("voice"),
            catalog = (0 until cat.length()).mapNotNull { i ->
                cat.optJSONObject(i)?.let { LocalTopic(it.optString("id"), it.optString("label")) }?.takeIf { it.id.isNotBlank() }
            },
            nextDueAt = o.optJSONObject("job")?.optString("next_due_at").orEmpty(),
        )
    }

    /** POST /api/winds/local/locate → el lloc (mai coordenades). */
    fun located(raw: String): LocalPlace = place(JSONObject(raw).optJSONObject("place"))

    /** El cos per desar. Mai porta coordenades. */
    fun body(i: LocalInfo): JSONObject {
        val o = JSONObject()
            .put("enabled", i.enabled).put("consent", i.consent)
            .put("topics", JSONArray(i.topics))
            .put("interests", i.interests).put("avoid", i.avoid)
            .put("radius_km", i.radiusKm).put("frequency", i.frequency).put("time", i.time)
            .put("voice", i.voice)
        if (i.wind.isNotBlank()) o.put("wind", i.wind)
        if (i.place.label.isNotBlank()) o.put(
            "place",
            JSONObject().put("label", i.place.label).put("region", i.place.region)
                .put("country", i.place.country).put("source", i.place.source),
        )
        return o
    }

    /** El que surt del mòbil: dues xifres decimals (~1 km). */
    fun round(v: Double): Double = Math.round(v * 100) / 100.0

    fun locateBody(lat: Double, lon: Double): JSONObject =
        JSONObject().put("lat", round(lat)).put("lon", round(lon)).put("source", "device")
}

object LocalLogic {
    /** null si es pot activar; si no, la clau del que falta (mateixes regles que el servidor). */
    fun enableProblem(i: LocalInfo): String? = when {
        !i.consent -> "app.local.needConsent"
        i.place.label.isBlank() -> "app.local.needPlace"
        i.topics.isEmpty() -> "app.local.needTopic"
        else -> null
    }

    fun frequencyLabel(lang: String, freq: String, time: String): String = when (freq) {
        "2days" -> tr(lang, "app.local.freq2days", time)
        "weekly" -> tr(lang, "app.local.freqWeekly", time)
        else -> tr(lang, "app.local.freqDaily", time)
    }

    fun summary(lang: String, i: LocalInfo): String {
        if (!i.enabled) return tr(lang, "app.local.off")
        val names = i.topics.map { id -> i.catalog.firstOrNull { it.id == id }?.label ?: id }
        return listOf(frequencyLabel(lang, i.frequency, i.time), i.place.label, tr(lang, "app.local.radius", i.radiusKm), names.joinToString(", "))
            .filter { it.isNotBlank() }.joinToString(" · ")
    }

    fun toggleTopic(i: LocalInfo, id: String): LocalInfo =
        i.copy(topics = if (id in i.topics) i.topics - id else i.topics + id)

    /** El missatge d'un error del servidor sense «HTTP 400:». */
    fun serverMessage(e: Throwable): String =
        (e.message ?: "").replace(Regex("""^HTTP \d+:\s*"""), "").trim()
}

/** La ubicació aproximada del mòbil (xarxa, no GPS). Null si no n'hi ha. */
@SuppressLint("MissingPermission")
internal fun coarseLocation(ctx: Context, onResult: (Location?) -> Unit) {
    if (ContextCompat.checkSelfPermission(ctx, Manifest.permission.ACCESS_COARSE_LOCATION) != PackageManager.PERMISSION_GRANTED) {
        onResult(null); return
    }
    val lm = ctx.getSystemService(Context.LOCATION_SERVICE) as? LocationManager ?: run { onResult(null); return }
    val providers = listOf(LocationManager.NETWORK_PROVIDER, LocationManager.PASSIVE_PROVIDER)
        .filter { runCatching { lm.isProviderEnabled(it) }.getOrDefault(false) || it == LocationManager.PASSIVE_PROVIDER }
    val last = providers.mapNotNull { runCatching { lm.getLastKnownLocation(it) }.getOrNull() }.maxByOrNull { it.time }
    if (last != null && System.currentTimeMillis() - last.time < 3_600_000L) { onResult(last); return }
    if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.R && LocationManager.NETWORK_PROVIDER in providers) {
        runCatching {
            lm.getCurrentLocation(LocationManager.NETWORK_PROVIDER, null, ContextCompat.getMainExecutor(ctx)) { onResult(it ?: last) }
        }.onFailure { onResult(last) }
    } else onResult(last)
}

/**
 * La targeta «A prop teu» de la safata «Avui». [info] null = carregant.
 * [found] és el lloc que ha trobat la ubicació del mòbil (encara sense desar).
 */
@OptIn(ExperimentalLayoutApi::class)
@Composable
internal fun LocalInfoCard(
    lang: String,
    info: LocalInfo?,
    winds: List<Wind>,
    message: String?,
    found: LocalPlace?,
    onSave: (LocalInfo) -> Unit,
    onLocate: (Double, Double) -> Unit,
    onLocateFailed: (String) -> Unit,
    onRun: () -> Unit,
    onForget: () -> Unit,
    modifier: Modifier = Modifier,
) {
    val p = LocalPal.current
    val ctx = LocalContext.current
    val base = info ?: LocalInfo()
    var draft by remember(info) { mutableStateOf(base) }
    var open by remember { mutableStateOf(false) }
    var problem by remember { mutableStateOf<String?>(null) }
    var typed by remember(info) { mutableStateOf("") }
    LaunchedEffect(found) { if (found != null && found.label.isNotBlank()) draft = draft.copy(place = found) }
    val locate = {
        coarseLocation(ctx) { loc ->
            if (loc == null) onLocateFailed(tr(lang, "app.local.noLocation")) else onLocate(loc.latitude, loc.longitude)
        }
    }
    val permission = rememberLauncherForActivityResult(ActivityResultContracts.RequestPermission()) { granted ->
        if (granted) locate() else onLocateFailed(tr(lang, "app.local.permissionDenied"))
    }
    Column(
        modifier.fillMaxWidth().clip(GregalShape.lg).background(SuperficieAlta.copy(alpha = .35f))
            .border(1.dp, Line, GregalShape.lg).padding(14.dp).testTag("local-card"),
        verticalArrangement = Arrangement.spacedBy(10.dp),
    ) {
        Row(
            Modifier.fillMaxWidth().clickable { open = !open }.testTag("local-head"),
            verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(10.dp),
        ) {
            Icon(Icons.Rounded.Place, null, tint = if (base.enabled) p.sea else Muted, modifier = Modifier.size(18.dp))
            Column(Modifier.weight(1f)) {
                Text(tr(lang, "app.local.title"), color = Ink, style = GregalType.bodySmall.copy(fontWeight = FontWeight.SemiBold))
                Text(if (info == null) tr(lang, "app.avui.loading") else LocalLogic.summary(lang, base), color = Muted,
                    style = GregalType.caption, modifier = Modifier.testTag("local-summary"))
            }
            Text(tr(lang, if (open) "app.local.close" else if (base.enabled) "app.local.edit" else "app.local.setup"),
                color = p.sea, fontSize = 13.sp, fontWeight = FontWeight.SemiBold)
        }
        if (!open || info == null) {
            if (message != null) Text(message, color = Muted, style = GregalType.caption, modifier = Modifier.testTag("local-message"))
            return@Column
        }
        Text(tr(lang, "app.local.intro"), color = Muted, style = GregalType.caption)
        // On ets
        Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(8.dp)) {
            Text(draft.place.label.ifBlank { tr(lang, "app.local.noPlace") }, color = Ink,
                style = GregalType.bodySmall.copy(fontWeight = FontWeight.Medium), modifier = Modifier.weight(1f).testTag("local-place"))
            if (draft.place.label.isNotBlank()) Text(tr(lang, "app.local.src." + draft.place.source), color = Muted, style = GregalType.caption)
        }
        TextButton(onClick = {
            if (ContextCompat.checkSelfPermission(ctx, Manifest.permission.ACCESS_COARSE_LOCATION) == PackageManager.PERMISSION_GRANTED) locate()
            else permission.launch(Manifest.permission.ACCESS_COARSE_LOCATION)
        }, modifier = Modifier.testTag("local-locate")) {
            Icon(Icons.Rounded.MyLocation, null, tint = p.sea, modifier = Modifier.size(16.dp))
            Spacer(Modifier.width(6.dp))
            Text(tr(lang, "app.local.useLocation"), color = p.sea)
        }
        Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(8.dp)) {
            LocalField(typed, { typed = it }, tr(lang, "app.local.typePlace"), Modifier.weight(1f).testTag("local-type"))
            if (typed.isNotBlank()) TextButton(onClick = { draft = draft.copy(place = LocalPlace(label = typed.trim())); typed = "" }) {
                Text(tr(lang, "app.local.usePlace"), color = p.sea, fontWeight = FontWeight.SemiBold)
            }
        }
        // Temes
        Text(tr(lang, "app.local.topics"), color = Ink, style = GregalType.caption.copy(fontWeight = FontWeight.SemiBold))
        FlowRow(horizontalArrangement = Arrangement.spacedBy(6.dp), verticalArrangement = Arrangement.spacedBy(6.dp)) {
            base.catalog.forEach { t ->
                val on = t.id in draft.topics
                Text(t.label, color = if (on) p.onSea else Ink, fontSize = 13.sp,
                    modifier = Modifier.clip(GregalShape.pill).background(if (on) p.sea else Superficie.copy(alpha = .7f))
                        .border(1.dp, if (on) p.sea else Line, GregalShape.pill)
                        .clickable { draft = LocalLogic.toggleTopic(draft, t.id) }
                        .padding(horizontal = 12.dp, vertical = 6.dp).testTag("local-topic-" + t.id))
            }
        }
        LocalField(draft.interests, { draft = draft.copy(interests = it) }, tr(lang, "app.local.interests"), Modifier.fillMaxWidth())
        LocalField(draft.avoid, { draft = draft.copy(avoid = it) }, tr(lang, "app.local.avoid"), Modifier.fillMaxWidth())
        // Distància i freqüència
        Text(tr(lang, "app.local.distance"), color = Ink, style = GregalType.caption.copy(fontWeight = FontWeight.SemiBold))
        FlowRow(horizontalArrangement = Arrangement.spacedBy(6.dp)) {
            LocalJson.RADII.forEach { km ->
                LocalPill(tr(lang, "app.local.km", km), draft.radiusKm == km, "local-radius-$km") { draft = draft.copy(radiusKm = km) }
            }
        }
        Text(tr(lang, "app.local.when"), color = Ink, style = GregalType.caption.copy(fontWeight = FontWeight.SemiBold))
        FlowRow(horizontalArrangement = Arrangement.spacedBy(6.dp), verticalArrangement = Arrangement.spacedBy(6.dp)) {
            LocalJson.FREQUENCIES.forEach { f ->
                LocalPill(tr(lang, "app.local.f.$f"), draft.frequency == f, "local-freq-$f") { draft = draft.copy(frequency = f) }
            }
        }
        LocalField(draft.time, { draft = draft.copy(time = it.take(5)) }, tr(lang, "app.local.time"), Modifier.width(120.dp).testTag("local-time"))
        if (winds.size > 1) FlowRow(horizontalArrangement = Arrangement.spacedBy(6.dp), verticalArrangement = Arrangement.spacedBy(6.dp)) {
            val chosen = draft.wind.ifBlank { winds.first().id }
            winds.forEach { w -> LocalPill(w.name, w.id == chosen, "local-wind-" + w.id) { draft = draft.copy(wind = w.id) } }
        }
        Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(10.dp)) {
            Column(Modifier.weight(1f)) {
                Text(tr(lang, "app.local.voice"), color = Ink, style = GregalType.bodySmall.copy(fontWeight = FontWeight.Medium))
                Text(tr(lang, "app.local.voiceHelp"), color = Muted, style = GregalType.caption)
            }
            Switch(draft.voice, { draft = draft.copy(voice = it) },
                colors = SwitchDefaults.colors(checkedTrackColor = p.sea, checkedThumbColor = p.onSea), modifier = Modifier.testTag("local-voice"))
        }
        Row(verticalAlignment = Alignment.Top, horizontalArrangement = Arrangement.spacedBy(4.dp)) {
            Checkbox(draft.consent, { draft = draft.copy(consent = it) },
                colors = CheckboxDefaults.colors(checkedColor = p.sea), modifier = Modifier.testTag("local-consent"))
            Text(tr(lang, "app.local.consent"), color = Ink, style = GregalType.caption, modifier = Modifier.padding(top = 12.dp))
        }
        val shown = problem ?: message
        if (shown != null) Text(shown, color = Muted, style = GregalType.caption, modifier = Modifier.testTag("local-message"))
        FlowRow(horizontalArrangement = Arrangement.spacedBy(4.dp)) {
            TextButton(onClick = {
                val next = draft.copy(enabled = true)
                val why = LocalLogic.enableProblem(next)
                problem = why?.let { tr(lang, it) }
                if (why == null) onSave(next)
            }, modifier = Modifier.testTag("local-save")) {
                Text(tr(lang, if (base.enabled) "app.local.save" else "app.local.turnOn"), color = p.sea, fontWeight = FontWeight.SemiBold)
            }
            if (base.enabled) {
                TextButton(onClick = onRun, modifier = Modifier.testTag("local-run")) { Text(tr(lang, "app.local.run"), color = p.sea) }
                TextButton(onClick = { problem = null; onSave(draft.copy(enabled = false)) }, modifier = Modifier.testTag("local-off")) { Text(tr(lang, "app.local.turnOff"), color = Muted) }
            }
            if (base.place.label.isNotBlank() || base.consent)
                TextButton(onClick = onForget, modifier = Modifier.testTag("local-forget")) { Text(tr(lang, "app.local.forget"), color = Muted) }
        }
    }
}

@Composable
private fun LocalPill(label: String, on: Boolean, tag: String, onClick: () -> Unit) {
    val p = LocalPal.current
    Text(label, color = if (on) p.onSea else Ink, fontSize = 13.sp,
        modifier = Modifier.clip(GregalShape.pill).background(if (on) p.sea else Superficie.copy(alpha = .7f))
            .border(1.dp, if (on) p.sea else Line, GregalShape.pill).clickable(onClick = onClick)
            .padding(horizontal = 12.dp, vertical = 6.dp).testTag(tag))
}

@Composable
private fun LocalField(value: String, onChange: (String) -> Unit, hint: String, modifier: Modifier) {
    val p = LocalPal.current
    OutlinedTextField(
        value, onChange, singleLine = true,
        placeholder = { Text(hint, fontSize = 13.sp, color = Muted) },
        textStyle = GregalType.bodySmall.copy(color = Ink, fontSize = 14.sp),
        shape = GregalShape.sm,
        colors = OutlinedTextFieldDefaults.colors(
            focusedContainerColor = Superficie.copy(alpha = .6f), unfocusedContainerColor = Superficie.copy(alpha = .6f),
            focusedBorderColor = p.sea, unfocusedBorderColor = Line, cursorColor = p.sea,
        ),
        modifier = modifier,
    )
}
