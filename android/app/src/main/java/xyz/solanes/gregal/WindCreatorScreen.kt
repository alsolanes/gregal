package xyz.solanes.gregal

import androidx.activity.compose.BackHandler
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.horizontalScroll
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.rounded.ArrowBack
import androidx.compose.material.icons.rounded.AutoAwesome
import androidx.compose.material.icons.rounded.ExpandMore
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp

internal data class WindDraft(val name: String, val color: String, val role: String, val style: String, val persona: String, val id: String)

@Composable
internal fun WindCreatorScreen(vm: GregalViewModel, lang: String, edit: Wind?) {
    WindEditor(lang, edit,
        onBack = { vm.showWindCreator(false) },
        onSuggest = { name, role, error, done -> vm.suggestWind(name, role, error, done) },
        onSave = { draft, done -> vm.createWind(draft.name, draft.color, draft.role, draft.style, draft.persona, draft.id, done) })
}

/** A route, not a dialog: the gallery is the first decision, details stay editable. */
@OptIn(ExperimentalMaterial3Api::class)
@Composable
internal fun WindEditor(
    lang: String,
    edit: Wind?,
    onBack: () -> Unit,
    onSuggest: (String, String, () -> Unit, (String, String) -> Unit) -> Unit,
    onSave: (WindDraft, (Boolean) -> Unit) -> Unit,
) {
    var name by remember(edit?.id) { mutableStateOf(edit?.name.orEmpty()) }
    var color by remember(edit?.id) { mutableStateOf(edit?.color ?: WindSuggestions.all.first().color) }
    var role by remember(edit?.id) { mutableStateOf(edit?.role.orEmpty()) }
    var style by remember(edit?.id) { mutableStateOf(windStyleOf(edit?.style).encode()) }
    var persona by remember(edit?.id) { mutableStateOf(edit?.persona.orEmpty()) }
    var gallery by remember(edit?.id) { mutableStateOf(edit == null) }
    var customize by remember { mutableStateOf(false) }
    var suggesting by remember { mutableStateOf(false) }
    var saving by remember { mutableStateOf(false) }
    var error by remember { mutableStateOf<String?>(null) }
    val navigateBack: () -> Unit = {
        if (!saving) {
            if (gallery && edit != null) gallery = false else if (!gallery && edit == null) gallery = true else onBack()
        }
    }
    BackHandler { navigateBack() }
    Scaffold(containerColor = Superficie,
        topBar = { TopAppBar(
            title = { Text(tr(lang, if (edit == null) "app.company.windNewTitle" else "app.company.windEditTitle")) },
            navigationIcon = { IconButton(onClick = navigateBack, enabled = !saving) { Icon(Icons.Rounded.ArrowBack, tr(lang, "app.vents.back")) } },
            colors = TopAppBarDefaults.topAppBarColors(containerColor = Superficie)) },
        bottomBar = {
            Surface(color = Superficie, tonalElevation = 2.dp) {
                Column(Modifier.navigationBarsPadding().imePadding().padding(horizontal = 20.dp, vertical = 12.dp)) {
                    error?.let { Text(it, color = MaterialTheme.colorScheme.error, modifier = Modifier.padding(bottom = 8.dp)) }
                    Button(onClick = {
                        saving = true; error = null
                        onSave(WindDraft(name.trim(), color, role.trim(), style, persona, edit?.id.orEmpty())) { success ->
                            saving = false
                            if (success) onBack() else error = tr(lang, "app.vents.saveError")
                        }
                    }, enabled = !gallery && name.isNotBlank() && !saving && !suggesting, modifier = Modifier.fillMaxWidth().heightIn(min = 50.dp),
                        colors = ButtonDefaults.buttonColors(containerColor = Sea, contentColor = OnSea)) {
                        if (saving) { CircularProgressIndicator(Modifier.size(18.dp), strokeWidth = 2.dp, color = OnSea); Spacer(Modifier.width(8.dp)) }
                        Text(tr(lang, if (saving) "app.vents.saving" else if (edit != null) "app.vents.saveChanges" else "app.company.windSave"))
                    }
                }
            }
        }) { padding ->
        Column(Modifier.fillMaxSize().padding(padding).verticalScroll(rememberScrollState()).padding(20.dp),
            verticalArrangement = Arrangement.spacedBy(16.dp)) {
            if (gallery) {
            Text(tr(lang, "app.vents.galleryTitle"), color = Ink, fontSize = 21.sp, fontWeight = FontWeight.Bold)
            Text(tr(lang, "app.vents.galleryHelp"), color = Muted, fontSize = 14.sp)
            // Two columns keep the gallery legible on small phones; no infinite horizontal guessing.
            WindSuggestions.all.chunked(2).forEach { row ->
                Row(horizontalArrangement = Arrangement.spacedBy(12.dp)) {
                    row.forEach { preset ->
                        val selected = name == preset.name && color == preset.color && style == preset.style
                        Surface(shape = RoundedCornerShape(22.dp), color = Color(Wind("", "", preset.color, "").argb).copy(alpha = .10f),
                            modifier = Modifier.weight(1f).clip(RoundedCornerShape(22.dp))
                                .then(if (selected) Modifier.border(2.dp, Sea, RoundedCornerShape(22.dp)) else Modifier)
                                .clickable(enabled = !saving) {
                                    name = preset.name; color = preset.color; style = preset.style
                                    role = tr(lang, "app.vents.preset.${preset.name}.role")
                                    persona = tr(lang, "app.vents.preset.${preset.name}.persona")
                                    gallery = false
                                }) {
                            Column(Modifier.padding(12.dp), horizontalAlignment = Alignment.CenterHorizontally, verticalArrangement = Arrangement.spacedBy(6.dp)) {
                                WindFace(size = 62, accent = Color(Wind("", "", preset.color, "").argb), style = windStyleOf(preset.style))
                                Text(preset.name, color = Ink, fontWeight = FontWeight.SemiBold, fontSize = 14.sp)
                                Text(tr(lang, "app.vents.preset.${preset.name}.role"), color = Muted, fontSize = 12.sp)
                            }
                        }
                    }
                }
            }
            TextButton(onClick = { gallery = false }, modifier = Modifier.fillMaxWidth()) { Text(tr(lang, "app.vents.startFromScratch")) }
            } else {
            Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(14.dp)) {
                WindFace(size = 60, accent = Color(Wind("", "", color, "").argb), style = windStyleOf(style))
                Column { Text(tr(lang, "app.vents.makeYours"), color = Ink, fontWeight = FontWeight.SemiBold); Text(tr(lang, "app.vents.detailsHelp"), color = Muted, fontSize = 13.sp) }
            }
            TextButton(onClick = { gallery = true }, enabled = !saving) { Text(tr(lang, "app.vents.changePreset")) }
            OutlinedTextField(name, { name = it }, enabled = !saving, label = { Text(tr(lang, "app.company.windName")) }, singleLine = true, modifier = Modifier.fillMaxWidth())
            OutlinedTextField(role, { role = it }, enabled = !saving, label = { Text(tr(lang, "app.company.windRole")) }, modifier = Modifier.fillMaxWidth())
            OutlinedTextField(persona, { persona = it }, enabled = !saving, label = { Text(tr(lang, "app.vents.personality")) }, minLines = 3, modifier = Modifier.fillMaxWidth())
            OutlinedButton(onClick = {
                suggesting = true; error = null
                onSuggest(name, role, { suggesting = false; error = tr(lang, "app.vents.suggestError") }) { p, s ->
                    persona = p; style = windStyleOf(s).encode(); suggesting = false
                }
            }, enabled = name.isNotBlank() && !suggesting && !saving, modifier = Modifier.fillMaxWidth()) {
                if (suggesting) CircularProgressIndicator(Modifier.size(18.dp), strokeWidth = 2.dp) else Icon(Icons.Rounded.AutoAwesome, null, Modifier.size(18.dp))
                Spacer(Modifier.width(8.dp)); Text(tr(lang, "app.company.windSuggest"))
            }
            TextButton(onClick = { customize = !customize }, modifier = Modifier.fillMaxWidth()) {
                Text(tr(lang, "app.vents.customize")); Spacer(Modifier.weight(1f)); Icon(Icons.Rounded.ExpandMore, null)
            }
            if (customize) {
                Text(tr(lang, "app.company.windColor"), color = Muted)
                Row(Modifier.horizontalScroll(rememberScrollState()), horizontalArrangement = Arrangement.spacedBy(6.dp)) {
                    WindSuggestions.all.map { it.color }.distinct().forEachIndexed { i, c ->
                        Box(Modifier.size(48.dp).clip(CircleShape).clickable(enabled = !saving) { color = c }
                            .semantics { contentDescription = tr(lang, "app.vents.colorChoice", (i + 1).toString()) }, contentAlignment = Alignment.Center) {
                            Box(Modifier.size(34.dp).background(Color(Wind("", "", c, "").argb), CircleShape)
                                .then(if (color == c) Modifier.border(2.dp, Ink, CircleShape) else Modifier))
                        }
                    }
                }
                val face = windStyleOf(style)
                FaceOptions(lang, "app.company.windFaceEyes", WindEyes.entries.map { it.key }, face.eyes.key,
                    { style = face.copy(eyes = WindEyes.entries.first { e -> e.key == it }).encode() },
                    { "app.company.windEyes_$it" }, { face.copy(eyes = WindEyes.entries.first { e -> e.key == it }) }, color)
                FaceOptions(lang, "app.company.windFaceMouth", WindMouth.entries.map { it.key }, face.mouth.key,
                    { style = face.copy(mouth = WindMouth.entries.first { e -> e.key == it }).encode() },
                    { "app.company.windMouth_$it" }, { face.copy(mouth = WindMouth.entries.first { e -> e.key == it }) }, color)
                FaceOptions(lang, "app.company.windFaceShape", WindShape.entries.map { it.key }, face.shape.key,
                    { style = face.copy(shape = WindShape.entries.first { e -> e.key == it }).encode() },
                    { "app.company.windShape_$it" }, { face.copy(shape = WindShape.entries.first { e -> e.key == it }) }, color)
            }
            }
        }
    }
}

@Composable
private fun FaceOptions(lang: String, label: String, options: List<String>, selected: String, pick: (String) -> Unit,
    optionLabel: (String) -> String, face: (String) -> WindStyle, color: String) {
    Column(verticalArrangement = Arrangement.spacedBy(8.dp)) {
        Text(tr(lang, label), color = Muted, fontSize = 13.sp)
        Row(Modifier.horizontalScroll(rememberScrollState()), horizontalArrangement = Arrangement.spacedBy(8.dp)) {
            options.forEach { option ->
                Column(Modifier.clip(RoundedCornerShape(14.dp)).background(if (option == selected) Sea.copy(alpha = .12f) else Color.Transparent)
                    .clickable { pick(option) }.padding(8.dp), horizontalAlignment = Alignment.CenterHorizontally) {
                    WindFace(size = 42, accent = Color(Wind("", "", color, "").argb), style = face(option))
                    Text(tr(lang, optionLabel(option)), fontSize = 11.sp, color = Ink)
                }
            }
        }
    }
}
