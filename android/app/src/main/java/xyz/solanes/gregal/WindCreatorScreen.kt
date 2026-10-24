package xyz.solanes.gregal

import androidx.activity.compose.BackHandler
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.horizontalScroll
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.selection.selectable
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.rounded.ArrowBack
import androidx.compose.material.icons.rounded.AutoAwesome
import androidx.compose.material.icons.rounded.ExpandMore
import androidx.compose.material.icons.rounded.ExpandLess
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp

// Solid mascot colors; old pastel presets remain available in the optional gallery.
private val windEditorColors = listOf("#20BE92", "#1A9BEE", "#27292F", "#FF9F24", "#EB4EB4", "#EF5965", "#8C6CE8", "#EACF87")

internal data class WindDraft(val name: String, val color: String, val role: String, val style: String, val persona: String, val id: String)

@Composable
internal fun WindCreatorScreen(vm: GregalViewModel, lang: String, edit: Wind?) {
    WindEditor(lang, edit,
        onBack = { vm.showWindCreator(false) },
        onSuggest = { name, role, error, done -> vm.suggestWind(name, role, error, done) },
        onSave = { draft, done -> vm.createWind(draft.name, draft.color, draft.role, draft.style, draft.persona, draft.id, done) })
}

/** A visual editor first; templates and personality are optional second-level choices. */
@OptIn(ExperimentalMaterial3Api::class)
@Composable
internal fun WindEditor(
    lang: String,
    edit: Wind?,
    onBack: () -> Unit,
    onSuggest: (String, String, () -> Unit, (String, String) -> Unit) -> Unit,
    onSave: (WindDraft, (Boolean) -> Unit) -> Unit,
) {
    var name by rememberSaveable(edit?.id) { mutableStateOf(edit?.name.orEmpty()) }
    var color by rememberSaveable(edit?.id) { mutableStateOf(edit?.color ?: windEditorColors.first()) }
    val colorChoices = remember(edit?.id) { (windEditorColors + listOfNotNull(edit?.color)).distinct() }
    var role by rememberSaveable(edit?.id) { mutableStateOf(edit?.role.orEmpty()) }
    var style by rememberSaveable(edit?.id) { mutableStateOf(windStyleOf(edit?.style).encode()) }
    var persona by rememberSaveable(edit?.id) { mutableStateOf(edit?.persona.orEmpty()) }
    var gallery by rememberSaveable(edit?.id) { mutableStateOf(false) }
    var customize by rememberSaveable(edit?.id) { mutableStateOf(false) }
    var personalityOpen by rememberSaveable(edit?.id) { mutableStateOf(false) }
    var suggesting by remember(edit?.id) { mutableStateOf(false) }
    var saving by remember(edit?.id) { mutableStateOf(false) }
    var error by remember(edit?.id) { mutableStateOf<String?>(null) }
    val navigateBack: () -> Unit = {
        if (!saving) {
            if (gallery) gallery = false else onBack()
        }
    }
    BackHandler { navigateBack() }
    Scaffold(containerColor = Superficie,
        topBar = { TopAppBar(
            title = { Text(tr(lang, if (edit == null) "app.company.windNewTitle" else "app.company.windEditTitle")) },
            navigationIcon = { IconButton(onClick = navigateBack, enabled = !saving) { Icon(Icons.Rounded.ArrowBack, tr(lang, "app.vents.back")) } },
            colors = TopAppBarDefaults.topAppBarColors(containerColor = Superficie)) },
        bottomBar = {
            if (!gallery) Surface(color = Superficie) {
                Column(Modifier.navigationBarsPadding().imePadding().padding(horizontal = 20.dp, vertical = 12.dp)) {
                    error?.let { Text(it, color = MaterialTheme.colorScheme.error, modifier = Modifier.padding(bottom = 8.dp)) }
                    Button(onClick = {
                        saving = true; error = null
                        onSave(WindDraft(name.trim(), color, role.trim(), style, persona, edit?.id.orEmpty())) { success ->
                            saving = false
                            if (success) onBack() else error = tr(lang, "app.vents.saveError")
                        }
                    }, enabled = !gallery && name.isNotBlank() && !saving && !suggesting, modifier = Modifier.fillMaxWidth().heightIn(min = 50.dp),
                        shape = RoundedCornerShape(28.dp),
                        colors = ButtonDefaults.buttonColors(containerColor = Ink, contentColor = Superficie)) {
                        if (saving) { CircularProgressIndicator(Modifier.size(18.dp), strokeWidth = 2.dp, color = Superficie); Spacer(Modifier.width(8.dp)) }
                        Text(tr(lang, if (saving) "app.vents.saving" else if (edit != null) "app.vents.saveChanges" else "app.company.windSave"))
                    }
                }
            }
        }) { padding ->
        // Separate scroll positions: returning from the optional gallery restores the editor.
        val editorScroll = rememberScrollState()
        val galleryScroll = rememberScrollState()
        Column(Modifier.fillMaxSize().padding(padding).verticalScroll(if (gallery) galleryScroll else editorScroll)
            .padding(horizontal = 20.dp, vertical = 8.dp), verticalArrangement = Arrangement.spacedBy(12.dp)) {
            if (gallery) {
            Text(tr(lang, "app.vents.galleryTitle"), color = Ink, fontSize = 21.sp, fontWeight = FontWeight.Bold)
            Text(tr(lang, "app.vents.galleryHelp"), color = Muted, fontSize = 14.sp)
            // Plantilles per feina: rol+persona fets; només cal nom i carona.
            Text(tr(lang, "app.vents.tplSection"), color = Ink, fontSize = 16.sp, fontWeight = FontWeight.SemiBold)
            Text(tr(lang, "app.vents.tplSectionHelp"), color = Muted, fontSize = 13.sp)
            WindTemplates.all.forEach { tpl ->
                Surface(shape = RoundedCornerShape(18.dp), color = Color(Wind("", "", tpl.color, "").argb).copy(alpha = .10f),
                    modifier = Modifier.fillMaxWidth().clip(RoundedCornerShape(18.dp))
                        .clickable(enabled = !saving) {
                            role = tr(lang, "app.vents.tpl.${tpl.id}.role")
                            // A visual/template change must not erase a persona already written.
                            if (persona.isBlank()) persona = tr(lang, "app.vents.tpl.${tpl.id}.persona")
                            color = tpl.color; style = tpl.style
                            gallery = false
                        }) {
                    Row(Modifier.padding(12.dp), verticalAlignment = Alignment.CenterVertically,
                        horizontalArrangement = Arrangement.spacedBy(12.dp)) {
                        WindFace(size = 46, accent = Color(Wind("", "", tpl.color, "").argb), style = windStyleOf(tpl.style))
                        Column(Modifier.weight(1f)) {
                            Text(tr(lang, "app.vents.tpl.${tpl.id}.title"), color = Ink, fontWeight = FontWeight.SemiBold, fontSize = 15.sp)
                            Text(tr(lang, "app.vents.tpl.${tpl.id}.desc"), color = Muted, fontSize = 13.sp)
                        }
                    }
                }
            }
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
                                    if (persona.isBlank()) persona = tr(lang, "app.vents.preset.${preset.name}.persona")
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
            // The face owns the canvas: no halo, duplicate title, or preset wall before creation.
            val face = windStyleOf(style)
            val controlsEnabled = !saving && !suggesting
            Box(Modifier.fillMaxWidth().height(156.dp).testTag("wind-editor-preview"), contentAlignment = Alignment.Center) {
                WindFace(size = 144, accent = Color(Wind("", "", color, "").argb),
                    style = face, tappable = !saving)
            }
            OutlinedTextField(name, { name = it }, enabled = controlsEnabled,
                label = { Text(tr(lang, "app.company.windName")) }, singleLine = true,
                shape = RoundedCornerShape(16.dp), modifier = Modifier.fillMaxWidth())
            OutlinedTextField(role, { role = it }, enabled = controlsEnabled,
                label = { Text(tr(lang, "app.company.windRole")) }, maxLines = 2,
                shape = RoundedCornerShape(16.dp), modifier = Modifier.fillMaxWidth())
            FaceOptions(lang, "app.company.windFaceShape", WindShape.entries.map { it.key }, face.shape.key,
                { style = face.copy(shape = WindShape.entries.first { e -> e.key == it }).encode() },
                { "app.company.windShape_$it" }, { face.copy(shape = WindShape.entries.first { e -> e.key == it }) }, color, controlsEnabled)
            Column(verticalArrangement = Arrangement.spacedBy(4.dp)) {
                Text(tr(lang, "app.company.windColor"), color = Muted, fontSize = 13.sp)
                // Two short rows keep every swatch reachable, without horizontal scrolling.
                colorChoices.chunked(4).forEachIndexed { row, colors ->
                    Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.SpaceEvenly) {
                        colors.forEachIndexed { index, c ->
                            Box(Modifier.size(48.dp).clip(CircleShape)
                                .selectable(selected = color == c, enabled = controlsEnabled, role = Role.RadioButton, onClick = { color = c })
                                .semantics { contentDescription = tr(lang, "app.vents.colorChoice", (row * 4 + index + 1).toString()) },
                                contentAlignment = Alignment.Center) {
                                // Vora fina sempre: un color fosc no desapareix sobre el fons fosc.
                                Box(Modifier.size(34.dp).background(Color(Wind("", "", c, "").argb), CircleShape)
                                    .border(if (color == c) 2.dp else 1.dp, if (color == c) Ink else LineHi, CircleShape))
                            }
                        }
                    }
                }
            }
            EditorDisclosure(tr(lang, "app.vents.customize"), customize, !saving) { customize = !customize }
            if (customize) {
                FaceOptions(lang, "app.company.windFaceEyes", WindEyes.entries.map { it.key }, face.eyes.key,
                    { style = face.copy(eyes = WindEyes.entries.first { e -> e.key == it }).encode() },
                    { "app.company.windEyes_$it" }, { face.copy(eyes = WindEyes.entries.first { e -> e.key == it }) }, color, controlsEnabled)
                FaceOptions(lang, "app.company.windFaceMouth", WindMouth.entries.map { it.key }, face.mouth.key,
                    { style = face.copy(mouth = WindMouth.entries.first { e -> e.key == it }).encode() },
                    { "app.company.windMouth_$it" }, { face.copy(mouth = WindMouth.entries.first { e -> e.key == it }) }, color, controlsEnabled)
            }
            EditorDisclosure(tr(lang, "app.vents.personality"), personalityOpen, !saving) { personalityOpen = !personalityOpen }
            if (personalityOpen) {
                // Collapsing this section never clears the existing or AI-written persona.
                OutlinedTextField(persona, { persona = it }, enabled = controlsEnabled,
                    label = { Text(tr(lang, "app.vents.personality")) }, minLines = 3,
                    shape = RoundedCornerShape(16.dp), modifier = Modifier.fillMaxWidth())
                OutlinedButton(onClick = {
                    suggesting = true; error = null
                    onSuggest(name, role, { suggesting = false; error = tr(lang, "app.vents.suggestError") }) { p, s ->
                        persona = p; style = windStyleOf(s).encode(); suggesting = false
                    }
                }, enabled = name.isNotBlank() && controlsEnabled, modifier = Modifier.fillMaxWidth()) {
                    if (suggesting) CircularProgressIndicator(Modifier.size(18.dp), strokeWidth = 2.dp)
                    else Icon(Icons.Rounded.AutoAwesome, null, Modifier.size(18.dp))
                    Spacer(Modifier.width(8.dp)); Text(tr(lang, "app.company.windSuggest"))
                }
            }
            TextButton(onClick = { gallery = true }, enabled = controlsEnabled, modifier = Modifier.fillMaxWidth()) {
                Text(tr(lang, "app.vents.changePreset"))
            }
            }
        }
    }
}

@Composable
private fun EditorDisclosure(label: String, expanded: Boolean, enabled: Boolean, onClick: () -> Unit) {
    TextButton(onClick = onClick, enabled = enabled, modifier = Modifier.fillMaxWidth().heightIn(min = 48.dp)) {
        Text(label, color = Ink)
        Spacer(Modifier.weight(1f))
        Icon(if (expanded) Icons.Rounded.ExpandLess else Icons.Rounded.ExpandMore, null, tint = Muted)
    }
}

@Composable
private fun FaceOptions(lang: String, label: String, options: List<String>, selected: String, pick: (String) -> Unit,
    optionLabel: (String) -> String, face: (String) -> WindStyle, color: String, enabled: Boolean) {
    Column(verticalArrangement = Arrangement.spacedBy(4.dp)) {
        Text(tr(lang, label), color = Muted, fontSize = 13.sp)
        Row(Modifier.horizontalScroll(rememberScrollState()), horizontalArrangement = Arrangement.spacedBy(8.dp)) {
            options.forEach { option ->
                Column(Modifier.clip(RoundedCornerShape(14.dp)).background(if (option == selected) Sea.copy(alpha = .12f) else Color.Transparent)
                    .selectable(selected = option == selected, enabled = enabled, role = Role.RadioButton, onClick = { pick(option) })
                    .padding(8.dp), horizontalAlignment = Alignment.CenterHorizontally) {
                    WindFace(size = 42, accent = Color(Wind("", "", color, "").argb), style = face(option))
                    Text(tr(lang, optionLabel(option)), fontSize = 11.sp, color = Ink)
                }
            }
        }
    }
}
