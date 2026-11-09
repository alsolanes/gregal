package xyz.solanes.gregal

import androidx.activity.compose.BackHandler
import androidx.compose.foundation.clickable
import androidx.compose.foundation.horizontalScroll
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.text.ClickableText
import androidx.compose.foundation.text.KeyboardActions
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.rounded.ArrowBack
import androidx.compose.material.icons.rounded.Add
import androidx.compose.material.icons.rounded.ArrowDropDown
import androidx.compose.material.icons.rounded.AutoAwesome
import androidx.compose.material.icons.rounded.Delete
import androidx.compose.material.icons.rounded.Edit
import androidx.compose.material.icons.rounded.Menu
import androidx.compose.material.icons.rounded.PushPin
import androidx.compose.material.icons.rounded.Refresh
import androidx.compose.material.icons.rounded.Search
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.LocalClipboardManager
import androidx.compose.ui.platform.LocalUriHandler
import androidx.compose.ui.text.AnnotatedString
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.input.ImeAction
import androidx.compose.ui.text.input.KeyboardCapitalization
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import org.json.JSONArray
import org.json.JSONObject
import java.net.URLDecoder
import java.net.URLEncoder
import java.text.Normalizer

// Notes: el quadern de l'usuari, compartit amb els {vents} (GET/POST
// /api/notes, notes.go). Llista amb cerca, quaderns i etiquetes; la nota es
// llegeix en markdown (Md.kt) amb les tasques marcables i els [[enllaços]]
// a altres notes; l'editor desa amb el control de conflicte del servidor.

/** Una nota; a la llista no porta el cos. */
data class Note(
    val id: String,
    val title: String,
    val body: String = "",
    val notebook: String = "",
    val tags: List<String> = emptyList(),
    val hashtags: List<String> = emptyList(),
    val pinned: Boolean = false,
    val private: Boolean = false,
    val locked: Boolean = false,
    val created: String = "",
    val updated: String = "",
    val by: String = "",
    val editedBy: String = "",
    val excerpt: String = "",
    val snippet: String = "",
    val tasksOpen: Int = 0,
    val tasksTotal: Int = 0,
    val noRemind: Boolean = false,
    /** Carpeta del projecte de Gregal associat (com a /api/workspaces); buit = cap. */
    val project: String = "",
) {
    val allTags: List<String> get() = (tags + hashtags).distinctBy { foldNote(it) }
}

data class NoteCount(val name: String, val count: Int)

/** Una tasca amb data d'una nota (l'agenda). index = ordre de la tasca dins la nota. */
data class AgendaItem(val noteId: String, val noteTitle: String, val index: Int, val text: String, val due: String, val time: String, val remindAt: String = "")

data class NotesPage(
    val notes: List<Note>, val tags: List<NoteCount>, val notebooks: List<NoteCount>, val total: Int,
    val agenda: List<AgendaItem> = emptyList(),
    /** Notes per projecte (name = la ruta). */
    val projects: List<NoteCount> = emptyList(),
)

data class NoteDetail(val note: Note, val backlinks: List<Pair<String, String>>)

/** Filtres de la llista. project: "" = tots, "-" = sense projecte, o la ruta. */
data class NotesQuery(val q: String = "", val tag: String = "", val notebook: String = "", val project: String = "") {
    /** El projecte que hereta una nota nova feta amb aquest filtre. */
    val newProject: String? get() = project.takeIf { it.isNotBlank() && it != "-" }
}

/** El que es desa des de l'editor. */
data class NoteInput(
    val id: String, val title: String, val body: String, val notebook: String,
    val tags: List<String>, val private: Boolean, val locked: Boolean, val base: String,
    val noRemind: Boolean = false,
    /** null = no el toca (es queda el que tenia); "" = cap projecte. */
    val project: String? = null,
)

/** El que torna /api/notes/ai: una proposta, que no es desa fins que l'usuari l'aplica. */
data class NoteAIResult(val body: String, val changed: Boolean)

/** Les accions fixes del menú d'IA (a més de "custom", amb instrucció). */
val NOTE_AI_ACTIONS = listOf("tidy", "organize", "summarize", "tasks")

object NotesJson {
    private fun JSONArray?.strs(): List<String> = this?.let { a -> (0 until a.length()).map { a.optString(it) }.filter { it.isNotBlank() } } ?: emptyList()

    fun note(x: JSONObject) = Note(
        x.optString("id"), x.optString("title"), x.optString("body"), x.optString("notebook"),
        x.optJSONArray("tags").strs(), x.optJSONArray("hashtags").strs(),
        x.optBoolean("pinned"), x.optBoolean("private"), x.optBoolean("locked"),
        x.optString("created"), x.optString("updated"), x.optString("by"), x.optString("edited_by"),
        x.optString("excerpt"), x.optString("snippet"), x.optInt("tasks_open"), x.optInt("tasks_total"),
        x.optBoolean("no_remind"), x.optString("project"),
    )

    private fun counts(a: JSONArray?): List<NoteCount> = a?.let { arr ->
        (0 until arr.length()).mapNotNull { i -> arr.optJSONObject(i)?.let { NoteCount(it.optString("name"), it.optInt("count")) } }
    } ?: emptyList()

    fun page(raw: String): NotesPage {
        val o = JSONObject(raw)
        val arr = o.optJSONArray("notes") ?: JSONArray()
        val ag = o.optJSONArray("agenda") ?: JSONArray()
        return NotesPage((0 until arr.length()).mapNotNull { arr.optJSONObject(it)?.let(::note) },
            counts(o.optJSONArray("tags")), counts(o.optJSONArray("notebooks")), o.optInt("total"),
            (0 until ag.length()).mapNotNull { i -> ag.optJSONObject(i)?.let {
                AgendaItem(it.optString("note_id"), it.optString("note_title"), it.optInt("index"), it.optString("text"), it.optString("due"), it.optString("time"), it.optString("remind_at"))
            } }, counts(o.optJSONArray("projects")))
    }

    fun detail(raw: String): NoteDetail {
        val o = JSONObject(raw)
        val back = o.optJSONArray("backlinks") ?: JSONArray()
        return NoteDetail(note(o.optJSONObject("note") ?: JSONObject()),
            (0 until back.length()).mapNotNull { i -> back.optJSONObject(i)?.let { it.optString("id") to it.optString("title") } })
    }

    fun input(n: NoteInput): JSONObject = JSONObject().put("action", "save").put("id", n.id).put("title", n.title)
        .put("body", n.body).put("notebook", n.notebook).put("tags", JSONArray(n.tags))
        .put("private", n.private).put("locked", n.locked).put("no_remind", n.noRemind).put("base", n.base)
        .also { o -> n.project?.let { o.put("project", it) } }

    /** El text de l'editor va sencer: pot no estar desat encara. */
    fun aiRequest(action: String, instruction: String, title: String, body: String): JSONObject =
        JSONObject().put("action", action).put("instruction", instruction).put("title", title).put("body", body)

    fun aiResult(o: JSONObject) = NoteAIResult(o.optString("body"), o.optBoolean("changed"))
}

private fun enc(s: String) = URLEncoder.encode(s, "UTF-8")

fun GregalApi.getNotes(q: NotesQuery): NotesPage {
    val p = buildList {
        if (q.q.isNotBlank()) add("q=" + enc(q.q.trim()))
        if (q.tag.isNotBlank()) add("tag=" + enc(q.tag))
        if (q.notebook.isNotBlank()) add("notebook=" + enc(q.notebook))
        if (q.project.isNotBlank()) add("project=" + enc(q.project))
    }
    return NotesJson.page(get("/api/notes" + if (p.isEmpty()) "" else "?" + p.joinToString("&")).toString())
}

fun GregalApi.getNote(id: String): NoteDetail = NotesJson.detail(get("/api/notes?id=" + enc(id)).toString())

/** Desa; null si la nota ha canviat mentre s'editava (409). */
fun GregalApi.saveNote(n: NoteInput): Note? = try {
    NotesJson.note(post("/api/notes", NotesJson.input(n)))
} catch (e: ApiException) {
    if (e.message.orEmpty().startsWith("HTTP 409")) null else throw e
}

fun GregalApi.pinNote(id: String, pinned: Boolean) { post("/api/notes", JSONObject().put("action", "pin").put("id", id).put("pinned", pinned)) }

fun GregalApi.deleteNote(id: String) { post("/api/notes", JSONObject().put("action", "delete").put("id", id)) }

/** POST /api/notes/ai: proposa la nota reescrita; no desa res. */
fun GregalApi.noteAI(action: String, instruction: String, title: String, body: String): NoteAIResult =
    NotesJson.aiResult(post("/api/notes/ai", NotesJson.aiRequest(action, instruction, title, body)))

// ---------------------------------------------------------------- pures

/** Minúscules i sense accents, com el servidor. */
fun foldNote(s: String): String =
    Normalizer.normalize(s, Normalizer.Form.NFD).replace(Regex("\\p{Mn}+"), "").lowercase().trim()

private val noteLinkRe = Regex("\\[\\[([^\\[\\]\\n|]{1,120})(?:\\|([^\\[\\]\\n]{0,120}))?\\]\\]")
const val NOTE_LINK_SCHEME = "gregal-note:"

/** [[Títol|text]] → [text](gregal-note:Títol), fora del codi: Md.kt ja en fa un enllaç. */
fun linkifyNotes(body: String): String {
    var inFence = false
    return body.split("\n").joinToString("\n") { line ->
        if (line.trimStart().startsWith("```")) inFence = !inFence
        if (inFence) line
        else line.split("`").mapIndexed { i, part ->
            if (i % 2 == 1) part
            else noteLinkRe.replace(part) { m ->
                val title = m.groupValues[1].trim()
                val label = m.groupValues[2].trim().ifBlank { title }.replace("[", "(").replace("]", ")")
                "[$label]($NOTE_LINK_SCHEME${enc(title)})"
            }
        }.joinToString("`")
    }
}

/** El títol d'un enllaç gregal-note:, o null si és un enllaç normal. */
fun noteLinkTitle(url: String): String? =
    if (url.startsWith(NOTE_LINK_SCHEME)) URLDecoder.decode(url.removePrefix(NOTE_LINK_SCHEME), "UTF-8") else null

private val taskRe = Regex("^(\\s*[-*+] \\[)([ xX])(\\].*)$")

/** Marca o desmarca la tasca número i (0 = la primera), saltant el codi. */
fun toggleNoteTask(body: String, index: Int): String {
    var n = -1
    var inFence = false
    return body.split("\n").joinToString("\n") { line ->
        if (line.trimStart().startsWith("```")) inFence = !inFence
        val m = if (inFence) null else taskRe.find(line)
        if (m == null) line else {
            n++
            if (n == index) m.groupValues[1] + (if (m.groupValues[2] == " ") "x" else " ") + m.groupValues[3] else line
        }
    }
}

/** Un tros de la nota: text markdown o una tasca marcable. */
sealed interface NoteBlock {
    data class Text(val md: String) : NoteBlock
    data class Task(val index: Int, val done: Boolean, val text: String, val indent: Int) : NoteBlock
}

/** Parteix el cos en markdown i tasques (les de dins de codi són text). */
fun noteBlocks(body: String): List<NoteBlock> {
    val out = mutableListOf<NoteBlock>()
    val buf = StringBuilder()
    var inFence = false
    var n = 0
    fun flush() {
        if (buf.isNotBlank()) out += NoteBlock.Text(buf.toString().trim('\n'))
        buf.clear()
    }
    for (line in body.split("\n")) {
        if (line.trimStart().startsWith("```")) inFence = !inFence
        val m = if (inFence) null else taskRe.find(line)
        if (m != null) {
            flush()
            val lead = m.groupValues[1].takeWhile { it == ' ' || it == '\t' }.length
            out += NoteBlock.Task(n++, m.groupValues[2] != " ", m.groupValues[3].removePrefix("]").trim(), lead / 2)
        } else buf.append(line).append('\n')
    }
    flush()
    return out
}

/** Agenda repartida: vençudes, avui, els propers 7 dies i més endavant. Pura. */
fun agendaGroups(items: List<AgendaItem>, today: java.time.LocalDate = java.time.LocalDate.now()): Map<String, List<AgendaItem>> {
    val t = today.toString()
    val week = today.plusDays(7).toString()
    return items.filter { it.due.isNotBlank() }.groupBy {
        when {
            it.due < t -> "overdue"
            it.due == t -> "today"
            it.due <= week -> "week"
            else -> "later"
        }
    }
}

/** Plantilla de reunió: data d'avui i una tasca amb termini d'aquí a una setmana. Pura. */
fun meetingTemplate(lang: String, today: java.time.LocalDate = java.time.LocalDate.now()): NoteInput {
    val ca = lang != "en"
    val next = today.plusDays(7)
    val title = (if (ca) "Reunió " else "Meeting ") + "%d/%d".format(today.dayOfMonth, today.monthValue)
    val body = if (ca) "**Data:** $today · **Amb:** \n\n## Notes\n- \n\n## Decisions\n- \n\n## Tasques\n- [ ]  @$next\n"
    else "**Date:** $today · **With:** \n\n## Notes\n- \n\n## Decisions\n- \n\n## Action items\n- [ ]  @$next\n"
    return NoteInput("", title, body, if (ca) "Reunions" else "Meetings", emptyList(), false, false, "")
}

const val QUICK_NOTE_ACTION = "xyz.solanes.gregal.QUICK_NOTE"

/** El text d'una captura: compartit (assumpte + text) o la drecera (buit). null = no és captura. Pura. */
fun noteCaptureText(action: String?, subject: String?, text: String?): String? = when (action) {
    QUICK_NOTE_ACTION -> ""
    android.content.Intent.ACTION_SEND -> {
        val body = text.orEmpty().trim()
        val head = subject.orEmpty().trim()
        when {
            body.isEmpty() && head.isEmpty() -> null
            head.isEmpty() || body.startsWith(head) -> body
            body.isEmpty() -> head
            else -> "# $head\n\n$body"
        }
    }
    else -> null
}

/** La drecera «Nota ràpida» de la icona (mantenir premut), en l'idioma de l'app. */
fun publishQuickNoteShortcut(ctx: android.content.Context, lang: String) {
    runCatching {
        val intent = android.content.Intent(ctx, MainActivity::class.java).setAction(QUICK_NOTE_ACTION)
            .addFlags(android.content.Intent.FLAG_ACTIVITY_SINGLE_TOP)
        val info = androidx.core.content.pm.ShortcutInfoCompat.Builder(ctx, "quick-note")
            .setShortLabel(tr(lang, "notes.shortcut"))
            .setLongLabel(tr(lang, "notes.shortcutLong"))
            .setIcon(androidx.core.graphics.drawable.IconCompat.createWithResource(ctx, R.mipmap.ic_gregal))
            .setIntent(intent)
            .build()
        androidx.core.content.pm.ShortcutManagerCompat.pushDynamicShortcut(ctx, info)
    }
}

/** «música, #Pendents ; musica» → [música, Pendents]. */
fun parseNoteTags(s: String): List<String> {
    val seen = mutableSetOf<String>()
    return s.split(',', ';', '\n').mapNotNull { raw ->
        val t = raw.trim().removePrefix("#").trim().split(Regex("\\s+")).filter { it.isNotEmpty() }.joinToString("-")
        if (t.isEmpty() || !seen.add(foldNote(t))) null else t
    }
}

/** El nom curt d'un projecte: l'últim tros de la ruta (/ o \). Pura. */
fun noteProjectName(path: String): String =
    path.trimEnd('/', '\\').substringAfterLast('/').substringAfterLast('\\').ifBlank { path }

/**
 * Els projectes que es poden triar, (ruta, nom): els de /api/workspaces que
 * encara hi són, després els que ja tenen notes i, si cal, l'actual; sense
 * repetir. El nom ve de /api/workspaces quan hi és. Pura.
 */
fun noteProjectChoices(workspaces: List<WorkspaceInfo>, counts: List<NoteCount>, current: String = ""): List<Pair<String, String>> {
    val names = workspaces.filter { it.path.isNotBlank() }.associate { it.path to it.name.ifBlank { noteProjectName(it.path) } }
    val out = LinkedHashMap<String, String>()
    for (w in workspaces) if (!w.missing && w.path.isNotBlank()) out.putIfAbsent(w.path, names.getValue(w.path))
    for (path in counts.map { it.name } + current) {
        if (path.isNotBlank() && path != "-") out.putIfAbsent(path, names[path] ?: noteProjectName(path))
    }
    return out.toList()
}

/** El servidor diu que no hi ha cap model configurat (400 de /api/notes/ai). Pura. */
fun noteAINeedsModel(error: String): Boolean = error.startsWith("HTTP 400") && error.contains("model", ignoreCase = true)

/** Línia de metadades: projecte, quadern, tasques, quan i qui. */
fun noteMeta(lang: String, n: Note): String = listOfNotNull(
    n.project.ifBlank { null }?.let { "📁 " + noteProjectName(it) },
    n.notebook.ifBlank { null }?.let { "📓 $it" },
    if (n.tasksTotal > 0) "☑ ${n.tasksTotal - n.tasksOpen}/${n.tasksTotal}" else null,
    noteDay(lang, n.updated).ifBlank { null },
    n.editedBy.ifBlank { null }?.let { tr(lang, "notes.by", it) },
).joinToString(" · ")

fun noteDay(lang: String, iso: String): String = runCatching {
    savedDay(lang, java.time.OffsetDateTime.parse(iso).toEpochSecond())
}.getOrDefault("")

// ---------------------------------------------------------------- pantalla

@OptIn(ExperimentalMaterial3Api::class)
@Composable
internal fun NotesScreen(vm: GregalViewModel, onMenu: () -> Unit) {
    val lang by vm.idioma.collectAsState()
    val page by vm.notes.collectAsState()
    val open by vm.openNote.collectAsState()
    var editing by remember { mutableStateOf<NoteInput?>(null) }
    val capture by vm.noteCapture.collectAsState()
    LaunchedEffect(Unit) {
        if (page == null) vm.loadNotes()
        // Els noms dels projectes (filtre i selector) surten de /api/workspaces.
        if (vm.workspaces.value == null) vm.loadWorkspaces()
    }
    // Comparteix / drecera: l'editor ja amb el text (o buit, a punt d'escriure).
    LaunchedEffect(capture) {
        val text = vm.takeNoteCapture() ?: return@LaunchedEffect
        val q = vm.notesQuery
        editing = NoteInput("", "", text, q.notebook, emptyList(), false, false, "", project = q.newProject)
    }
    BackHandler(enabled = editing != null || open != null) {
        if (editing != null) editing = null else vm.closeNote()
    }
    val current = editing
    when {
        current != null -> NoteEditor(vm, lang, current, onDone = { editing = null })
        open != null -> NoteView(vm, lang, open!!, onEdit = { n ->
            editing = NoteInput(n.id, n.title, n.body, n.notebook, n.tags, n.private, n.locked, n.updated, n.noRemind, n.project)
        }, onNew = { title -> editing = NoteInput("", title, "", "", emptyList(), false, false, "") })
        else -> NotesList(vm, lang, page, onMenu, onNew = {
            val q = vm.notesQuery
            editing = NoteInput("", "", "", q.notebook, listOfNotNull(q.tag.ifBlank { null }), false, false, "", project = q.newProject)
        }, onMeeting = { editing = meetingTemplate(lang) })
    }
}

@OptIn(ExperimentalMaterial3Api::class)
@Composable
private fun NotesList(vm: GregalViewModel, lang: String, page: NotesPage?, onMenu: () -> Unit, onNew: () -> Unit, onMeeting: () -> Unit) {
    var quick by remember { mutableStateOf("") }
    var query by remember { mutableStateOf(vm.notesQuery.q) }
    var pinnedOnly by remember { mutableStateOf(false) }
    LaunchedEffect(query) {
        if (query != vm.notesQuery.q) {
            kotlinx.coroutines.delay(250)
            vm.loadNotes(vm.notesQuery.copy(q = query))
        }
    }
    Scaffold(containerColor = Superficie, snackbarHost = { NoticeHost(vm) }, topBar = {
        TopAppBar(
            title = { Text(tr(lang, "notes.title"), fontSize = 18.sp, fontWeight = FontWeight.SemiBold) },
            navigationIcon = { IconButton(onClick = onMenu) { Icon(Icons.Rounded.Menu, tr(lang, "app.nav.menu")) } },
            actions = { IconButton(onClick = { vm.loadNotes() }) { Icon(Icons.Rounded.Refresh, tr(lang, "auto.refresh")) } },
            colors = TopAppBarDefaults.topAppBarColors(containerColor = Superficie),
        )
    }, floatingActionButton = {
        ExtendedFloatingActionButton(onClick = onNew, icon = { Icon(Icons.Rounded.Add, null) }, text = { Text(tr(lang, "notes.new")) },
            containerColor = Sea, contentColor = Superficie)
    }) { pad ->
        LazyColumn(Modifier.fillMaxSize().padding(pad), contentPadding = PaddingValues(16.dp, 8.dp, 16.dp, 96.dp),
            verticalArrangement = Arrangement.spacedBy(10.dp)) {
            item(key = "help") { Text(tr(lang, "notes.help"), color = Muted, fontSize = 13.sp, lineHeight = 18.sp) }
            // Captura ràpida: escrius i desa, sense obrir l'editor.
            item(key = "quick") {
                OutlinedTextField(quick, { quick = it }, Modifier.fillMaxWidth(), singleLine = true,
                    placeholder = { Text(tr(lang, "notes.quick")) }, shape = MaterialTheme.shapes.medium,
                    keyboardOptions = KeyboardOptions(capitalization = KeyboardCapitalization.Sentences, imeAction = ImeAction.Done),
                    keyboardActions = KeyboardActions(onDone = {
                        val text = quick.trim()
                        if (text.isNotEmpty()) { quick = ""; vm.quickNote(text) }
                    }),
                    trailingIcon = {
                        if (quick.isNotBlank()) IconButton(onClick = { val text = quick.trim(); quick = ""; vm.quickNote(text) }) {
                            Icon(Icons.Rounded.Add, tr(lang, "notes.save"))
                        }
                    })
            }
            item(key = "shortcuts") {
                Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                    AssistChip(onClick = onMeeting, label = { Text("🗓 " + tr(lang, "notes.meeting")) })
                }
            }
            item(key = "search") {
                OutlinedTextField(query, { query = it }, Modifier.fillMaxWidth(), singleLine = true,
                    placeholder = { Text(tr(lang, "notes.search")) }, leadingIcon = { Icon(Icons.Rounded.Search, null) },
                    shape = MaterialTheme.shapes.medium)
            }
            val p = page
            if (p == null) {
                item(key = "loading") { LinearProgressIndicator(Modifier.fillMaxWidth()) }
                return@LazyColumn
            }
            if (vm.notesQuery == NotesQuery() && p.agenda.isNotEmpty()) item(key = "agenda") { AgendaCard(vm, lang, p.agenda) }
            item(key = "chips") {
                val q = vm.notesQuery
                Row(Modifier.horizontalScroll(rememberScrollState()), horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                    FilterChip(q.tag.isBlank() && q.notebook.isBlank() && q.project.isBlank() && !pinnedOnly, {
                        pinnedOnly = false; vm.loadNotes(q.copy(tag = "", notebook = "", project = ""))
                    }, { Text(tr(lang, "notes.all"), fontSize = 13.sp) })
                    ProjectFilterChip(vm, lang, p, q)
                    if (pinnedOnly || p.notes.any { it.pinned }) FilterChip(pinnedOnly, { pinnedOnly = !pinnedOnly }, { Text("📌 " + tr(lang, "notes.pinned"), fontSize = 13.sp) })
                    for (nb in p.notebooks.take(12)) FilterChip(q.notebook == nb.name, {
                        vm.loadNotes(q.copy(notebook = if (q.notebook == nb.name) "" else nb.name))
                    }, { Text("📓 " + nb.name, fontSize = 13.sp) })
                    for (t in p.tags.take(16)) FilterChip(foldNote(q.tag) == foldNote(t.name), {
                        vm.loadNotes(q.copy(tag = if (foldNote(q.tag) == foldNote(t.name)) "" else t.name))
                    }, { Text("#" + t.name, fontSize = 13.sp) })
                }
            }
            val shown = if (pinnedOnly) p.notes.filter { it.pinned } else p.notes
            if (shown.isEmpty()) item(key = "empty") {
                Surface(color = Ink.copy(alpha = 0.04f), shape = MaterialTheme.shapes.medium) {
                    Text(tr(lang, if (p.total == 0) "notes.empty" else "notes.noneHere"), color = Tinta, fontSize = 14.sp, lineHeight = 20.sp,
                        modifier = Modifier.padding(16.dp))
                }
            }
            items(shown, key = { it.id }) { n -> NoteRow(lang, n) { vm.openNote(n.id) } }
        }
    }
}

@Composable
private fun AgendaCard(vm: GregalViewModel, lang: String, items: List<AgendaItem>) {
    val groups = remember(items) { agendaGroups(items) }
    Surface(color = Sea.copy(alpha = 0.08f), shape = MaterialTheme.shapes.medium, modifier = Modifier.fillMaxWidth()) {
        Column(Modifier.padding(horizontal = 12.dp, vertical = 10.dp), verticalArrangement = Arrangement.spacedBy(2.dp)) {
            Text(tr(lang, "notes.agenda").uppercase(), color = Muted, fontSize = 11.sp, fontWeight = FontWeight.SemiBold, letterSpacing = 1.2.sp)
            var shown = 0
            for (k in listOf("overdue", "today", "week", "later")) {
                val list = groups[k].orEmpty()
                if (list.isEmpty() || shown >= 10) continue
                Text(tr(lang, "notes.agenda.$k"), color = if (k == "overdue") Red else Faint,
                    fontSize = 12.sp, fontWeight = FontWeight.SemiBold, modifier = Modifier.padding(top = 6.dp))
                for (item in list.take(10 - shown)) {
                    shown++
                    key(item.noteId + ":" + item.index) {
                        var done by remember { mutableStateOf(false) }
                        Row(verticalAlignment = Alignment.CenterVertically, modifier = Modifier.fillMaxWidth().clickable { vm.openNote(item.noteId) }) {
                            Checkbox(done, { v -> done = v; vm.tickAgenda(item) })
                            Column(Modifier.weight(1f)) {
                                Text(item.text.ifBlank { item.noteTitle }, color = Tinta, fontSize = 14.sp, maxLines = 2, overflow = TextOverflow.Ellipsis)
                                val day = if (k == "today") "" else agendaDay(lang, item.due)
                                Text(listOf(day, item.time, item.noteTitle).filter { it.isNotBlank() }.joinToString(" · ") + if (item.remindAt.isNotBlank()) " · 🔔" else "",
                                    color = Muted, fontSize = 12.sp, maxLines = 1, overflow = TextOverflow.Ellipsis)
                            }
                        }
                    }
                }
            }
        }
    }
}

/** «ahir», «dj. 12/11» o la data. */
fun agendaDay(lang: String, due: String): String = runCatching {
    savedDay(lang, java.time.LocalDate.parse(due).atTime(12, 0).atZone(java.time.ZoneId.systemDefault()).toEpochSecond())
}.getOrDefault(due)

@Composable
private fun NoteRow(lang: String, n: Note, onOpen: () -> Unit) {
    Surface(color = Ink.copy(alpha = 0.04f), shape = MaterialTheme.shapes.medium, modifier = Modifier.fillMaxWidth().clickable(onClick = onOpen)) {
        Column(Modifier.padding(12.dp), verticalArrangement = Arrangement.spacedBy(4.dp)) {
            Text(listOfNotNull(if (n.pinned) "📌" else null, n.title.ifBlank { tr(lang, "notes.untitled") },
                if (n.private) "🙈" else null, if (n.locked) "🔒" else null).joinToString(" "),
                color = Ink, fontWeight = FontWeight.SemiBold, fontSize = 15.sp, maxLines = 2, overflow = TextOverflow.Ellipsis)
            val text = n.snippet.ifBlank { n.excerpt }
            if (text.isNotBlank()) Text(text, color = Tinta, fontSize = 13.sp, lineHeight = 18.sp, maxLines = 2, overflow = TextOverflow.Ellipsis)
            val tags = n.allTags.take(4)
            val meta = noteMeta(lang, n) + if (tags.isEmpty()) "" else " · #" + tags.joinToString(" #")
            Text(meta, color = Muted, fontSize = 12.sp, maxLines = 1, overflow = TextOverflow.Ellipsis)
        }
    }
}

@OptIn(ExperimentalMaterial3Api::class, ExperimentalLayoutApi::class)
@Composable
private fun NoteView(vm: GregalViewModel, lang: String, d: NoteDetail, onEdit: (Note) -> Unit, onNew: (String) -> Unit) {
    val n = d.note
    val uri = LocalUriHandler.current
    val clip = LocalClipboardManager.current
    var confirmDelete by remember(n.id) { mutableStateOf(false) }
    var aiBusy by remember(n.id) { mutableStateOf(false) }
    var proposal by remember(n.id) { mutableStateOf<String?>(null) }
    Scaffold(containerColor = Superficie, snackbarHost = { NoticeHost(vm) }, topBar = {
        TopAppBar(
            title = { Text(n.title, fontSize = 17.sp, fontWeight = FontWeight.SemiBold, maxLines = 1, overflow = TextOverflow.Ellipsis) },
            navigationIcon = { IconButton(onClick = { vm.closeNote() }) { Icon(Icons.AutoMirrored.Rounded.ArrowBack, tr(lang, "notes.back")) } },
            actions = {
                NoteAIButton(lang, aiBusy) { action, instruction ->
                    if (action != "custom" && n.body.isBlank()) vm.notify(tr(lang, "notes.ai.empty"))
                    else {
                        aiBusy = true
                        vm.noteAI(action, instruction, n.title, n.body) { aiBusy = false; proposal = it }
                    }
                }
                IconButton(onClick = { vm.pinNote(n, !n.pinned) }) {
                    Icon(Icons.Rounded.PushPin, tr(lang, if (n.pinned) "notes.unpin" else "notes.pin"), tint = if (n.pinned) Sea else Muted)
                }
                IconButton(onClick = { onEdit(n) }) { Icon(Icons.Rounded.Edit, tr(lang, "notes.edit")) }
                IconButton(onClick = { confirmDelete = true }) { Icon(Icons.Rounded.Delete, tr(lang, "notes.delete")) }
            },
            colors = TopAppBarDefaults.topAppBarColors(containerColor = Superficie),
        )
    }) { pad ->
        Column(Modifier.fillMaxSize().padding(pad).verticalScroll(rememberScrollState()).padding(horizontal = 18.dp, vertical = 8.dp),
            verticalArrangement = Arrangement.spacedBy(10.dp)) {
            if (aiBusy) NoteAIWorking(lang)
            Text(n.title.ifBlank { tr(lang, "notes.untitled") }, color = Ink, fontSize = 24.sp, fontFamily = FontFamily.Serif, lineHeight = 30.sp)
            val meta = listOfNotNull(
                noteMeta(lang, n).ifBlank { null },
                if (n.by.isNotBlank() && n.by != n.editedBy) tr(lang, "notes.createdBy", n.by) else null,
                if (n.private) "🙈 " + tr(lang, "notes.privateOn") else null,
                if (n.locked) "🔒 " + tr(lang, "notes.lockedOn") else null,
            ).joinToString(" · ")
            if (meta.isNotBlank()) Text(meta, color = Muted, fontSize = 12.sp, lineHeight = 16.sp)
            if (n.allTags.isNotEmpty()) FlowRow(horizontalArrangement = Arrangement.spacedBy(6.dp), verticalArrangement = Arrangement.spacedBy(6.dp)) {
                for (t in n.allTags) AssistChip(onClick = { vm.closeNote(); vm.loadNotes(NotesQuery(tag = t)) }, label = { Text("#$t", fontSize = 12.sp) })
            }
            val blocks = remember(n.body) { noteBlocks(n.body) }
            if (blocks.isEmpty()) Text(tr(lang, "notes.emptyNote"), color = Muted, fontSize = 13.sp)
            for (b in blocks) when (b) {
                is NoteBlock.Text -> MdPageBody(linkifyNotes(b.md), lang) { url ->
                    val title = noteLinkTitle(url)
                    if (title != null) vm.openNoteByTitle(title) { onNew(it) } else runCatching { uri.openUri(url) }
                }
                is NoteBlock.Task -> Row(Modifier.fillMaxWidth().padding(start = (b.indent * 18).dp).clickable { vm.toggleNoteTask(n, b.index) },
                    verticalAlignment = Alignment.CenterVertically) {
                    Checkbox(b.done, { vm.toggleNoteTask(n, b.index) })
                    val rich = remember(b.text) { renderMd(b.text, Color.Transparent) }
                    Text(rich, color = if (b.done) Muted else Tinta, fontSize = 15.sp, lineHeight = 21.sp,
                        textDecoration = if (b.done) androidx.compose.ui.text.style.TextDecoration.LineThrough else null)
                }
            }
            if (d.backlinks.isNotEmpty()) {
                HorizontalDivider(color = Line, modifier = Modifier.padding(top = 8.dp))
                Text(tr(lang, "notes.backlinks").uppercase(), color = Faint, fontSize = 11.sp, fontWeight = FontWeight.SemiBold, letterSpacing = 1.2.sp)
                FlowRow(horizontalArrangement = Arrangement.spacedBy(6.dp)) {
                    for ((id, title) in d.backlinks) AssistChip(onClick = { vm.openNote(id) }, label = { Text(title, fontSize = 12.sp) })
                }
            }
            if (!n.private) OutlinedButton(onClick = {
                clip.setText(AnnotatedString(tr(lang, "notes.askPrompt", n.title)))
                vm.notify(tr(lang, "notes.askCopied"))
                vm.go(Screen.Company)
            }, modifier = Modifier.padding(top = 8.dp)) { Text(tr(lang, "notes.ask")) }
            Spacer(Modifier.height(40.dp))
        }
    }
    if (confirmDelete) AlertDialog(
        onDismissRequest = { confirmDelete = false },
        title = { Text(tr(lang, "notes.deleteTitle")) },
        text = { Text(n.title, maxLines = 3, overflow = TextOverflow.Ellipsis) },
        confirmButton = { TextButton(onClick = { confirmDelete = false; vm.deleteNote(n.id) }) { Text(tr(lang, "notes.delete")) } },
        dismissButton = { TextButton(onClick = { confirmDelete = false }) { Text(tr(lang, "act.cancel")) } },
    )
    proposal?.let { text ->
        NoteAIProposal(lang, text, onApply = { proposal = null; vm.setNoteBody(n, text) }, onDiscard = { proposal = null })
    }
}

@OptIn(ExperimentalMaterial3Api::class)
@Composable
private fun NoteEditor(vm: GregalViewModel, lang: String, start: NoteInput, onDone: () -> Unit) {
    var title by remember { mutableStateOf(start.title) }
    var body by remember { mutableStateOf(start.body) }
    var notebook by remember { mutableStateOf(start.notebook) }
    var tags by remember { mutableStateOf(start.tags.joinToString(", ")) }
    var private by remember { mutableStateOf(start.private) }
    var locked by remember { mutableStateOf(start.locked) }
    var remind by remember { mutableStateOf(!start.noRemind) }
    var project by remember { mutableStateOf(start.project.orEmpty()) }
    var saving by remember { mutableStateOf(false) }
    var conflict by remember { mutableStateOf(false) }
    var aiBusy by remember { mutableStateOf(false) }
    var proposal by remember { mutableStateOf<String?>(null) }
    fun input(force: Boolean) = NoteInput(start.id, title, body, notebook, parseNoteTags(tags), private, locked, if (force) "" else start.base, !remind, project)
    fun save(force: Boolean) {
        saving = true
        vm.saveNote(input(force)) { ok ->
            saving = false
            if (ok) onDone() else conflict = true
        }
    }
    Scaffold(containerColor = Superficie, snackbarHost = { NoticeHost(vm) }, topBar = {
        TopAppBar(
            title = { Text(tr(lang, if (start.id.isBlank()) "notes.new" else "notes.edit"), fontSize = 17.sp, fontWeight = FontWeight.SemiBold) },
            navigationIcon = { IconButton(onClick = onDone) { Icon(Icons.AutoMirrored.Rounded.ArrowBack, tr(lang, "act.cancel")) } },
            actions = {
                // A l'editor la proposta substitueix el text; es desa amb «Desa».
                NoteAIButton(lang, aiBusy) { action, instruction ->
                    if (action != "custom" && body.isBlank()) vm.notify(tr(lang, "notes.ai.empty"))
                    else {
                        aiBusy = true
                        vm.noteAI(action, instruction, title, body) { aiBusy = false; proposal = it }
                    }
                }
                TextButton(onClick = { save(false) }, enabled = !saving && (title.isNotBlank() || body.isNotBlank())) {
                    Text(tr(lang, "notes.save"), fontWeight = FontWeight.SemiBold)
                }
            },
            colors = TopAppBarDefaults.topAppBarColors(containerColor = Superficie),
        )
    }) { pad ->
        Column(Modifier.fillMaxSize().padding(pad).imePadding().verticalScroll(rememberScrollState()).padding(horizontal = 16.dp, vertical = 8.dp),
            verticalArrangement = Arrangement.spacedBy(10.dp)) {
            if (aiBusy) NoteAIWorking(lang)
            OutlinedTextField(title, { title = it }, Modifier.fillMaxWidth(), singleLine = true, label = { Text(tr(lang, "notes.fieldTitle")) },
                keyboardOptions = KeyboardOptions(capitalization = KeyboardCapitalization.Sentences))
            OutlinedTextField(body, { body = it }, Modifier.fillMaxWidth().heightIn(min = 260.dp), label = { Text(tr(lang, "notes.fieldBody")) },
                placeholder = { Text(tr(lang, "notes.bodyHint"), color = Faint) },
                textStyle = LocalTextStyle.current.copy(fontFamily = FontFamily.Monospace, fontSize = 14.sp, lineHeight = 20.sp),
                keyboardOptions = KeyboardOptions(capitalization = KeyboardCapitalization.Sentences))
            ProjectPicker(vm, lang, project) { project = it }
            OutlinedTextField(notebook, { notebook = it }, Modifier.fillMaxWidth(), singleLine = true, label = { Text(tr(lang, "notes.fieldNotebook")) })
            val notebooks = vm.notes.collectAsState().value?.notebooks.orEmpty().filter { it.name != notebook }
            if (notebooks.isNotEmpty()) Row(Modifier.horizontalScroll(rememberScrollState()), horizontalArrangement = Arrangement.spacedBy(6.dp)) {
                for (nb in notebooks.take(10)) SuggestionChip(onClick = { notebook = nb.name }, label = { Text(nb.name, fontSize = 12.sp) })
            }
            OutlinedTextField(tags, { tags = it }, Modifier.fillMaxWidth(), singleLine = true, label = { Text(tr(lang, "notes.fieldTags")) },
                placeholder = { Text(tr(lang, "notes.tagsHint"), color = Faint) })
            SwitchRow(tr(lang, "notes.remind"), remind) { remind = it }
            SwitchRow(tr(lang, "notes.private"), private) { private = it }
            SwitchRow(tr(lang, "notes.locked"), locked) { locked = it }
            Spacer(Modifier.height(40.dp))
        }
    }
    if (conflict) AlertDialog(
        onDismissRequest = { conflict = false },
        title = { Text(tr(lang, "notes.conflictTitle")) },
        text = { Text(tr(lang, "notes.conflict")) },
        confirmButton = { TextButton(onClick = { conflict = false; save(true) }) { Text(tr(lang, "notes.keepMine")) } },
        dismissButton = { TextButton(onClick = { conflict = false; onDone(); vm.openNote(start.id) }) { Text(tr(lang, "notes.seeNew")) } },
    )
    proposal?.let { text ->
        NoteAIProposal(lang, text, onApply = { proposal = null; body = text }, onDiscard = { proposal = null })
    }
}

/** Els projectes per triar: /api/workspaces més els que ja tenen notes. */
@Composable
private fun rememberProjectChoices(vm: GregalViewModel, current: String): List<Pair<String, String>> {
    val ws by vm.workspaces.collectAsState()
    val page by vm.notes.collectAsState()
    val counts = page?.projects.orEmpty()
    return remember(ws, counts, current) { noteProjectChoices(ws?.list.orEmpty(), counts, current) }
}

/** Filtre de la llista per projecte: tots, sense projecte o un de concret. */
@Composable
private fun ProjectFilterChip(vm: GregalViewModel, lang: String, page: NotesPage, q: NotesQuery) {
    val choices = rememberProjectChoices(vm, q.project)
    if (choices.isEmpty() && q.project.isBlank()) return
    var open by remember { mutableStateOf(false) }
    val label = when (q.project) {
        "" -> tr(lang, "notes.project.all")
        "-" -> tr(lang, "notes.project.none")
        else -> choices.firstOrNull { it.first == q.project }?.second ?: noteProjectName(q.project)
    }
    val counts = page.projects.associate { it.name to it.count }
    fun pick(v: String) { open = false; if (v != q.project) vm.loadNotes(q.copy(project = v)) }
    Box {
        FilterChip(q.project.isNotBlank(), { open = true }, { Text("📁 $label", fontSize = 13.sp, maxLines = 1) },
            trailingIcon = { Icon(Icons.Rounded.ArrowDropDown, tr(lang, "notes.project"), Modifier.size(18.dp)) })
        DropdownMenu(open, { open = false }) {
            DropdownMenuItem(text = { Text(tr(lang, "notes.project.all")) }, onClick = { pick("") })
            DropdownMenuItem(text = { CountLabel(tr(lang, "notes.project.none"), page.total - counts.values.sum()) }, onClick = { pick("-") })
            if (choices.isNotEmpty()) HorizontalDivider(color = Line)
            for ((path, name) in choices) DropdownMenuItem(text = { CountLabel(name, counts[path] ?: 0) }, onClick = { pick(path) })
        }
    }
}

@Composable
private fun CountLabel(name: String, count: Int) {
    Row(verticalAlignment = Alignment.CenterVertically) {
        Text(name, maxLines = 1, overflow = TextOverflow.Ellipsis, modifier = Modifier.weight(1f, fill = false))
        if (count > 0) Text("  $count", color = Muted, fontSize = 12.sp)
    }
}

/** El projecte de la nota a l'editor ("" = cap). */
@Composable
private fun ProjectPicker(vm: GregalViewModel, lang: String, value: String, onPick: (String) -> Unit) {
    val choices = rememberProjectChoices(vm, value)
    var open by remember { mutableStateOf(false) }
    val label = if (value.isBlank()) tr(lang, "notes.project.none")
    else choices.firstOrNull { it.first == value }?.second ?: noteProjectName(value)
    Box(Modifier.fillMaxWidth()) {
        OutlinedTextField(label, {}, Modifier.fillMaxWidth(), readOnly = true, singleLine = true,
            label = { Text(tr(lang, "notes.fieldProject")) },
            trailingIcon = { Icon(Icons.Rounded.ArrowDropDown, null) })
        // El camp és només de lectura: tota la caixa obre el menú.
        Box(Modifier.matchParentSize().clickable { open = true })
        DropdownMenu(open, { open = false }) {
            DropdownMenuItem(text = { Text(tr(lang, "notes.project.none")) }, onClick = { open = false; onPick("") })
            if (choices.isNotEmpty()) HorizontalDivider(color = Line)
            for ((path, name) in choices) DropdownMenuItem(
                text = {
                    Column {
                        Text(name, maxLines = 1, overflow = TextOverflow.Ellipsis)
                        Text(path, color = Muted, fontSize = 11.sp, maxLines = 1, overflow = TextOverflow.Ellipsis)
                    }
                },
                onClick = { open = false; onPick(path) },
            )
        }
    }
}

/** El menú d'IA: netejar, organitzar, resumir, extreure tasques o el que es demani. */
@Composable
private fun NoteAIButton(lang: String, busy: Boolean, onRun: (action: String, instruction: String) -> Unit) {
    var open by remember { mutableStateOf(false) }
    var asking by remember { mutableStateOf(false) }
    Box {
        IconButton(onClick = { open = true }, enabled = !busy) { Icon(Icons.Rounded.AutoAwesome, tr(lang, "notes.ai"), tint = if (busy) Faint else Sea) }
        DropdownMenu(open, { open = false }) {
            for (a in NOTE_AI_ACTIONS) DropdownMenuItem(
                text = {
                    Column {
                        Text(tr(lang, "notes.ai.$a"), fontWeight = FontWeight.Medium)
                        Text(tr(lang, "notes.ai.$a.help"), color = Muted, fontSize = 12.sp)
                    }
                },
                onClick = { open = false; onRun(a, "") },
            )
            HorizontalDivider(color = Line)
            DropdownMenuItem(text = { Text(tr(lang, "notes.ai.custom")) }, onClick = { open = false; asking = true })
        }
    }
    if (asking) {
        var text by remember { mutableStateOf("") }
        fun go() { val t = text.trim(); if (t.isNotEmpty()) { asking = false; onRun("custom", t) } }
        AlertDialog(
            onDismissRequest = { asking = false },
            title = { Text(tr(lang, "notes.ai.customTitle")) },
            text = {
                OutlinedTextField(text, { text = it }, Modifier.fillMaxWidth(), minLines = 2,
                    placeholder = { Text(tr(lang, "notes.ai.customHint"), color = Faint) },
                    keyboardOptions = KeyboardOptions(capitalization = KeyboardCapitalization.Sentences, imeAction = ImeAction.Done),
                    keyboardActions = KeyboardActions(onDone = { go() }))
            },
            confirmButton = { TextButton(onClick = { go() }, enabled = text.isNotBlank()) { Text(tr(lang, "notes.ai.go")) } },
            dismissButton = { TextButton(onClick = { asking = false }) { Text(tr(lang, "act.cancel")) } },
        )
    }
}

@Composable
private fun NoteAIWorking(lang: String) {
    Column(verticalArrangement = Arrangement.spacedBy(4.dp)) {
        LinearProgressIndicator(Modifier.fillMaxWidth(), color = Sea)
        Text(tr(lang, "notes.ai.working"), color = Muted, fontSize = 12.sp)
    }
}

/** La proposta de la IA, en markdown: no es desa res fins que l'usuari l'aplica. */
@Composable
private fun NoteAIProposal(lang: String, proposal: String, onApply: () -> Unit, onDiscard: () -> Unit) {
    AlertDialog(
        onDismissRequest = onDiscard,
        title = { Text(tr(lang, "notes.ai.proposal")) },
        text = {
            Column(Modifier.heightIn(max = 480.dp).verticalScroll(rememberScrollState()), verticalArrangement = Arrangement.spacedBy(8.dp)) {
                Text(tr(lang, "notes.ai.proposalHelp"), color = Muted, fontSize = 12.sp)
                MdPageBody(linkifyNotes(proposal), lang) { }
            }
        },
        confirmButton = { TextButton(onClick = onApply) { Text(tr(lang, "notes.ai.apply"), fontWeight = FontWeight.SemiBold) } },
        dismissButton = { TextButton(onClick = onDiscard) { Text(tr(lang, "notes.ai.discard")) } },
    )
}

@Composable
private fun SwitchRow(label: String, on: Boolean, set: (Boolean) -> Unit) {
    Row(Modifier.fillMaxWidth().clickable { set(!on) }.padding(vertical = 4.dp), verticalAlignment = Alignment.CenterVertically) {
        Text(label, color = Tinta, fontSize = 14.sp, lineHeight = 19.sp, modifier = Modifier.weight(1f))
        Spacer(Modifier.width(12.dp))
        Switch(on, set)
    }
}
