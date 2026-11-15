package xyz.solanes.gregal

import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.ColorScheme
import androidx.compose.material3.Shapes
import androidx.compose.material3.darkColorScheme
import androidx.compose.material3.lightColorScheme
import androidx.compose.runtime.Composable
import androidx.compose.runtime.ReadOnlyComposable
import androidx.compose.runtime.compositionLocalOf
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.lerp
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp

/** Paleta Gregal. Els noms en anglès es queden (els call sites no canvien),
 *  però els valors són els de `docs/identitat.md` ("Paleta" / "Paleta clara").
 *
 *  La font de veritat és `internal/tema/tema.go` (`Fosc()`/`Clar()`), d'on
 *  surten la web i el TUI; si canvia, aquest fitxer i `PaletaTest` també. */
data class GregalPalette(
    val sea: Color,
    val seaClara: Color,
    val sand: Color,
    val onada: Color,
    val algua: Color,
    val deepTop: Color,
    val deepBottom: Color,
    val surface: Color,
    val surfaceHi: Color,
    val ink: Color,
    val text: Color,
    val muted: Color,
    val faint: Color,
    val green: Color,
    val red: Color,
    val lila: Color,
    val lilaFons: Color,
    /** Les línies fines: gris fred al 10/20% (fosc) / navy al 14/28% (clar);
     *  no l'accent, que ha de ser escàs. */
    val line: Color,
    val lineHi: Color,
    val abyss: Color,
    /** Text i icones damunt l'accent (`sea`): fosc al tema fosc, blanc al clar. */
    val onSea: Color,
    /** Si la paleta és fosca. Abans es preguntava `== DarkPal`, que deixava de
     *  ser cert en afegir més temes: ara ho diu la pròpia paleta. */
    val dark: Boolean = false,
) {
    // Àlies històrics: noms que ja feia servir tota la UI.
    val pillDark: Color get() = surface
    val bubbleDark: Color get() = surface
    val codeDark: Color get() = surfaceHi
    /** Enviar: en fosc, llima amb el glif de mar; en clar, el cercle de mar amb el glif llima. */
    val sendBg: Color get() = sea
    val sendFg: Color get() = if (dark) onSea else Color(0xFFD4F25A)
    /** La teva bombolla: blanc trencat en fosc, mar en clar (com el tauler). */
    val userBubble: Color get() = if (dark) Color(0xFFEEF1EC) else Color(0xFF17323D)
    val onUserBubble: Color get() = if (dark) Color(0xFF141C18) else Color(0xFFF2F5EF)
    /** Colors de les sèries dels gràfics: vius en tots dos temes (girar el to
     *  de l'accent donava grisos amb el mar del tema clar). */
    val chart: List<Color> get() = if (dark)
        listOf(Color(0xFFD4F25A), Color(0xFF7FD6C2), Color(0xFFF2B48E), Color(0xFFC9B2FF), Color(0xFF8FC7FF), Color(0xFFFF9FB2))
        else listOf(Color(0xFF0E7C66), Color(0xFFC4622D), Color(0xFF5E8000), Color(0xFF7444D6), Color(0xFF2F6FD6), Color(0xFFC2335E))
}

val DarkPal = GregalPalette(
    sea = Color(0xFFD4F25A),        // escuma: blau Gregal
    seaClara = Color(0xFFE4F98F),   // escumaClara
    sand = Color(0xFFF2B48E),       // arena (avís)
    onada = Color(0xFF7FD6C2),      // aqua juganer
    algua = Color(0xFF2B3531),
    deepTop = Color(0xFF121715),    // fons
    deepBottom = Color(0xFF0D110F), // abisme: sidebar/panells
    surface = Color(0xFF1A211E),    // night
    surfaceHi = Color(0xFF242D29),  // superficieAlta
    ink = Color(0xFFF2F5EF),
    text = Color(0xFFDDE4DF),
    muted = Color(0xFFA8B3AD),
    faint = Color(0xFF909C96),
    green = Color(0xFF86E3A6),
    red = Color(0xFFFF8F86),
    lila = Color(0xFFC9B2FF),
    lilaFons = Color(0xFF2A2645),
    line = Color(0x1ACFE3DA),       // gris fred #C9D1E6 al 10%
    lineHi = Color(0x33CFE3DA),     // gris fred al 20%
    abyss = Color(0xFF0D110F),
    onSea = Color(0xFF121715),      // sobre l'accent: text fosc (fons)
    dark = true,
)

val LightPal = GregalPalette(
    sea = Color(0xFF17323D),
    seaClara = Color(0xFF0D232C),
    sand = Color(0xFF9A4E1E),
    onada = Color(0xFF0A6E62),
    algua = Color(0xFFD2DBD3),
    deepTop = Color(0xFFEEF1EC),
    deepBottom = Color(0xFFE4E9E3),
    surface = Color(0xFFFFFFFF),
    surfaceHi = Color(0xFFDEE5DE),
    ink = Color(0xFF10202A),
    text = Color(0xFF1C2D35),
    muted = Color(0xFF4C5F5C),
    faint = Color(0xFF52645E),
    green = Color(0xFF2F6B1F),
    red = Color(0xFFC2332E),
    lila = Color(0xFF7444D6),
    lilaFons = Color(0xFFEEE8FD),
    line = Color(0x2410202A),       // navy #1E2A4A al 14%
    lineHi = Color(0x4710202A),     // navy al 28%
    abyss = Color(0xFFE4E9E3),
    onSea = Color(0xFFFFFFFF),      // sobre l'accent: blanc
    dark = false,
)

val LocalPal = compositionLocalOf { DarkPal }

/** Schemes Material derivats de la paleta (per als components M3).
 *
 *  Els contenidors (menús, diàlegs, xips seleccionats, fulls) també surten de
 *  la paleta: sense això Material posava els seus grisos liles de referència
 *  i un menú o un xip «Tots» destacaven com d'una altra app. */
fun GregalPalette.scheme(): ColorScheme {
    val low = lerp(deepTop, surface, 0.5f)
    val high = lerp(surface, surfaceHi, 0.5f)
    // En fosc, els contenidors són neutres (la llima barrejada amb el grafit
    // feia oliva); l'accent queda per al text i les icones.
    val secondaryBox = if (dark) lerp(surface, ink, 0.10f) else lerp(surface, sea, 0.14f)
    val primaryBox = if (dark) lerp(surface, ink, 0.14f) else lerp(surface, sea, 0.18f)
    val errorBox = lerp(surface, red, if (dark) 0.20f else 0.12f)
    return if (dark) darkColorScheme(
        primary = sea, onPrimary = onSea, secondary = onada, onSecondary = deepTop,
        tertiary = lila, outline = Color(0xFF66726C), outlineVariant = algua, // outline = boxIdle (3:1)
        background = deepTop, surface = surface, surfaceVariant = surfaceHi,
        onBackground = ink, onSurface = ink, onSurfaceVariant = muted, error = red,
        primaryContainer = primaryBox, onPrimaryContainer = ink,
        secondaryContainer = secondaryBox, onSecondaryContainer = ink,
        tertiaryContainer = lilaFons, onTertiaryContainer = ink,
        errorContainer = errorBox, onErrorContainer = ink,
        surfaceTint = sea, inverseSurface = ink, inverseOnSurface = deepTop, inversePrimary = seaClara,
        surfaceDim = deepBottom, surfaceBright = surfaceHi,
        surfaceContainerLowest = deepBottom, surfaceContainerLow = low, surfaceContainer = surface,
        surfaceContainerHigh = high, surfaceContainerHighest = surfaceHi,
    ) else lightColorScheme(
        primary = sea, onPrimary = onSea, secondary = onada, onSecondary = Color.White,
        tertiary = lila, outline = Color(0xFF7A8A86), outlineVariant = algua, // outline = boxIdle (3:1)
        background = deepTop, surface = surface, surfaceVariant = surfaceHi,
        onBackground = ink, onSurface = ink, onSurfaceVariant = muted, error = red,
        primaryContainer = primaryBox, onPrimaryContainer = ink,
        secondaryContainer = secondaryBox, onSecondaryContainer = ink,
        tertiaryContainer = lilaFons, onTertiaryContainer = ink,
        errorContainer = errorBox, onErrorContainer = ink,
        surfaceTint = sea, inverseSurface = ink, inverseOnSurface = surface, inversePrimary = seaClara,
        surfaceDim = deepBottom, surfaceBright = surface,
        surfaceContainerLowest = surface, surfaceContainerLow = surface, surfaceContainer = surface,
        surfaceContainerHigh = high, surfaceContainerHighest = surfaceHi,
    )
}

// ─── Escala tipogràfica i de formes ───────────────────────────────────
// Pocs graons amb nom en lloc de mides soltes a cada pantalla. Cada estil
// porta la seva interlínia: un Text amb només `fontSize = 13.sp` heretava
// els 24 sp de bodyLarge i els paràgrafs petits quedaven esbarriats.

/** Estils de text de Gregal, de més gran a més petit. */
object GregalType {
    /** La salutació de la portada (inici dels Vents i xat buit). */
    val hero = TextStyle(fontSize = 34.sp, lineHeight = 40.sp, fontWeight = FontWeight.SemiBold, letterSpacing = (-0.9).sp)
    /** Títols de pantalla i de full. */
    val title = TextStyle(fontSize = 20.sp, lineHeight = 26.sp, fontWeight = FontWeight.SemiBold, letterSpacing = (-0.2).sp)
    /** El títol d'una targeta o d'una fila destacada. */
    val heading = TextStyle(fontSize = 16.sp, lineHeight = 22.sp, fontWeight = FontWeight.SemiBold)
    /** Text de lectura: missatges i descripcions. */
    val body = TextStyle(fontSize = 15.sp, lineHeight = 21.sp)
    /** La segona línia d'una fila, ajudes sota un camp. */
    val bodySmall = TextStyle(fontSize = 13.sp, lineHeight = 18.sp)
    /** Hores, peus i etiquetes petites. */
    val caption = TextStyle(fontSize = 12.sp, lineHeight = 16.sp)
    /** Capçaleres de secció en majúscules. */
    val overline = TextStyle(fontSize = 11.sp, lineHeight = 14.sp, fontWeight = FontWeight.SemiBold, letterSpacing = 0.8.sp)
    /** El cos dels missatges del fil (bombolles de l'assistent i teves). */
    val chatBody = TextStyle(fontSize = 16.sp, lineHeight = 23.sp)
    /** Hores, rebuts i separadors del fil. */
    val chatMeta = TextStyle(fontSize = 12.sp, lineHeight = 16.sp)
}

/** Radis de cantonada: cinc graons i la pastilla. */
object GregalShape {
    /** Codi, miniatures i etiquetes petites. */
    val xs = RoundedCornerShape(8.dp)
    /** Botons, camps i xips. */
    val sm = RoundedCornerShape(12.dp)
    /** Files i targetes (el més comú). */
    val md = RoundedCornerShape(14.dp)
    /** Targetes grans, fulls i diàlegs propis. */
    val lg = RoundedCornerShape(20.dp)
    /** El compositor. */
    val xl = RoundedCornerShape(28.dp)
    val pill = RoundedCornerShape(50)
    /** Radi de les bombolles del fil. */
    val bubbleRadius = 22.dp
    /** La cantonada de la cua: només a l'última bombolla d'una ratxa. */
    val bubbleTailRadius = 6.dp
    /** Bombolla sense cua (dins d'una ratxa). */
    val bubble = RoundedCornerShape(bubbleRadius)
    /** Bombolla amb cua a baix: a l'esquerra (assistent) o a la dreta (tu). */
    fun bubbleTail(mine: Boolean) = if (mine)
        RoundedCornerShape(bubbleRadius, bubbleRadius, bubbleTailRadius, bubbleRadius)
    else RoundedCornerShape(bubbleRadius, bubbleRadius, bubbleRadius, bubbleTailRadius)
    /** Targetes de resultat i botons de proposta del fil. */
    val card = RoundedCornerShape(15.dp)
}

/** Mides fixes del fil: controls circulars i amples màxims de bombolla. */
object GregalChat {
    /** Botons circulars de la capçalera (menú, ordinador): target tàctil sencer. */
    val control = 44.dp
    /** L'avatar centrat de la capçalera. */
    val headerAvatar = 44
    /** Botó d'enviar del compositor. */
    val send = 40.dp
    /** La franja «2 feines en marxa» sobre el compositor. */
    val workStrip = 32.dp
    /** Separació dins una ratxa i entre parlants. */
    val gapInRun = 3.dp
    val gapBetween = 14.dp
    /** Ample màxim de la bombolla de l'assistent: 84 % de la pantalla, fins a 520 dp. */
    const val assistantWidth = 0.84f
    val assistantMax = 520.dp
    /** Ample màxim de la teva bombolla: 78 %. */
    const val userWidth = 0.78f
    val bubblePadH = 14.dp
    val bubblePadV = 10.dp
}

/** Les formes de Material alineades amb [GregalShape]. */
fun gregalShapes() = Shapes(
    extraSmall = GregalShape.xs, small = GregalShape.sm, medium = GregalShape.md,
    large = GregalShape.lg, extraLarge = GregalShape.xl,
)

/** El punt d'estat d'un servidor: respon (verd), no respon (vermell), canviant (sorra) o sense saber-ho. */
@Composable
@ReadOnlyComposable
fun reachDot(reach: Boolean?, switching: Boolean = false): Color = when {
    switching -> Sand
    reach == true -> Green
    reach == false -> Red
    else -> Muted
}

// ─── Temes ────────────────────────────────────────────────────────────
// Un tema és un parell de paletes (clara i fosca); el mode (sistema/clar/fosc)
// es tria a part, així cada tema té les dues cares i cap combinació queda morta.

/** Paleta construïda a partir del que de debò canvia entre temes: el fons, la
 *  superfície, el text i l'accent. Les línies es deriven del text. */
private fun paleta(
    dark: Boolean,
    accent: Color, accentSoft: Color, aqua: Color, sand: Color,
    bg: Color, abyss: Color, surface: Color, surfaceHi: Color,
    ink: Color, text: Color, muted: Color, faint: Color,
    green: Color, red: Color, lila: Color, lilaFons: Color,
    onAccent: Color,
) = GregalPalette(
    sea = accent, seaClara = accentSoft, sand = sand, onada = aqua, algua = surfaceHi,
    deepTop = bg, deepBottom = abyss, surface = surface, surfaceHi = surfaceHi,
    ink = ink, text = text, muted = muted, faint = faint, green = green, red = red,
    lila = lila, lilaFons = lilaFons,
    line = text.copy(alpha = if (dark) 0.10f else 0.14f),
    lineHi = text.copy(alpha = if (dark) 0.20f else 0.28f),
    abyss = abyss, onSea = onAccent, dark = dark,
)

/** Nit: negre neutre i un sol accent violeta. El més silenciós de tots. */
val NitFosc = paleta(
    dark = true,
    accent = Color(0xFFA78BFA), accentSoft = Color(0xFFC4B5FD),
    aqua = Color(0xFF7DD3FC), sand = Color(0xFFFBBF24),
    bg = Color(0xFF000000), abyss = Color(0xFF000000),
    surface = Color(0xFF0C0D11), surfaceHi = Color(0xFF16171E),
    ink = Color(0xFFF2F3F8), text = Color(0xFFE3E5EE),
    muted = Color(0xFF9AA1B2), faint = Color(0xFF7B8296),
    green = Color(0xFF34D399), red = Color(0xFFFB7185),
    lila = Color(0xFFC4B5FD), lilaFons = Color(0xFF241E3D),
    onAccent = Color(0xFF0B0C10),
)
val NitClar = paleta(
    dark = false,
    accent = Color(0xFF6D4AE0), accentSoft = Color(0xFF5738C6),
    aqua = Color(0xFF0E7490), sand = Color(0xFFA16207),
    bg = Color(0xFFF8F8FB), abyss = Color(0xFFF0F1F6),
    surface = Color(0xFFFFFFFF), surfaceHi = Color(0xFFEAEBF2),
    ink = Color(0xFF15161C), text = Color(0xFF23252E),
    muted = Color(0xFF5A5F70), faint = Color(0xFF6E7488),
    green = Color(0xFF15803D), red = Color(0xFFBE123C),
    lila = Color(0xFF6D4AE0), lilaFons = Color(0xFFEDE9FE),
    onAccent = Color(0xFFFFFFFF),
)

/** Calma: la variant càlida de l'escriptori (grafit càlid i ambre), la
 *  mateixa que `:root[data-theme="calma"]` de `calma.css`. Abans aquí era un
 *  verd salvi i el mateix nom volia dir dues coses segons on miraves. */
val CalmaClar = paleta(
    dark = false,
    accent = Color(0xFFA2581C), accentSoft = Color(0xFF8A4A17),
    aqua = Color(0xFF0F766E), sand = Color(0xFF92620A),
    bg = Color(0xFFFAF8F5), abyss = Color(0xFFF2EEE9),
    surface = Color(0xFFFFFFFF), surfaceHi = Color(0xFFEEE9E3),
    ink = Color(0xFF1C1916), text = Color(0xFF2A2622),
    muted = Color(0xFF645D55), faint = Color(0xFF77706A),
    green = Color(0xFF2E7A3A), red = Color(0xFFB4232E),
    lila = Color(0xFF7A4BC0), lilaFons = Color(0xFFF1EAF7),
    onAccent = Color(0xFFFFFFFF),
)
val CalmaFosc = paleta(
    dark = true,
    accent = Color(0xFFE8A96B), accentSoft = Color(0xFFF2BE88),
    aqua = Color(0xFF7FC8B8), sand = Color(0xFFE8B75A),
    bg = Color(0xFF1B1A19), abyss = Color(0xFF151413),
    surface = Color(0xFF252322), surfaceHi = Color(0xFF2F2C2A),
    ink = Color(0xFFF2EFE9), text = Color(0xFFDCD7D0),
    muted = Color(0xFFA29D94), faint = Color(0xFF948F87),
    green = Color(0xFF8FCB8A), red = Color(0xFFEE8E80),
    lila = Color(0xFFC9A8E8), lilaFons = Color(0xFF2E2638),
    onAccent = Color(0xFF1B1A19),
)

/** Paper: cru càlid i terracota. Res de blaus. */
val PaperClar = paleta(
    dark = false,
    accent = Color(0xFFB4532A), accentSoft = Color(0xFF94401F),
    aqua = Color(0xFF0F766E), sand = Color(0xFF92620A),
    bg = Color(0xFFFAF7F2), abyss = Color(0xFFF2EDE4),
    surface = Color(0xFFFFFFFF), surfaceHi = Color(0xFFEFE9DE),
    ink = Color(0xFF1A1613), text = Color(0xFF2A2420),
    muted = Color(0xFF6B625A), faint = Color(0xFF7E746B),
    green = Color(0xFF15703F), red = Color(0xFFB4232E),
    lila = Color(0xFF7A4BC0), lilaFons = Color(0xFFF0E7DC),
    onAccent = Color(0xFFFFFFFF),
)
val PaperFosc = paleta(
    dark = true,
    accent = Color(0xFFE8825A), accentSoft = Color(0xFFF0A183),
    aqua = Color(0xFF5CE1D6), sand = Color(0xFFF5C451),
    bg = Color(0xFF14110F), abyss = Color(0xFF0E0C0A),
    surface = Color(0xFF1C1815), surfaceHi = Color(0xFF272220),
    ink = Color(0xFFF5EFE7), text = Color(0xFFEDE5DB),
    muted = Color(0xFFA8A09A), faint = Color(0xFF8A827C),
    green = Color(0xFF6EDB95), red = Color(0xFFFF8A85),
    lila = Color(0xFFC4B5FD), lilaFons = Color(0xFF2A2245),
    onAccent = Color(0xFF1C1815),
)

/** Grafit: gris neutre i menta. El més «d'eina» i el més modern. */
val GrafitFosc = paleta(
    dark = true,
    accent = Color(0xFF35D6A4), accentSoft = Color(0xFF6FE8C2),
    aqua = Color(0xFF7DD3FC), sand = Color(0xFFFBBF24),
    bg = Color(0xFF0E1011), abyss = Color(0xFF090B0C),
    surface = Color(0xFF1B1F21), surfaceHi = Color(0xFF272C2E),
    ink = Color(0xFFEDF1F2), text = Color(0xFFE2E7E9),
    muted = Color(0xFF98A2A6), faint = Color(0xFF7C8689),
    green = Color(0xFF34D399), red = Color(0xFFFB7185),
    lila = Color(0xFFC4B5FD), lilaFons = Color(0xFF1E2A30),
    onAccent = Color(0xFF06110C),
)
val GrafitClar = paleta(
    dark = false,
    accent = Color(0xFF0C7A5A), accentSoft = Color(0xFF0A6E51),
    aqua = Color(0xFF0E7490), sand = Color(0xFFA16207),
    bg = Color(0xFFF7F8F9), abyss = Color(0xFFEDEFF1),
    surface = Color(0xFFFFFFFF), surfaceHi = Color(0xFFE4E8EA),
    ink = Color(0xFF111517), text = Color(0xFF1F2528),
    muted = Color(0xFF586266), faint = Color(0xFF6C7679),
    green = Color(0xFF15803D), red = Color(0xFFBE123C),
    lila = Color(0xFF6D4AE0), lilaFons = Color(0xFFE7F5EF),
    onAccent = Color(0xFFFFFFFF),
)

/** Un tema amb nom i les dues cares. L'id viu a les preferències. */
data class GregalTema(val id: String, val clar: GregalPalette, val fosc: GregalPalette) {
    fun perMode(fosc_: Boolean): GregalPalette = if (fosc_) fosc else clar
}

/** Tots els temes, el de sempre primer (és el defecte i no ha de canviar). */
val GregalTemes: List<GregalTema> = listOf(
    GregalTema("gregal", LightPal, DarkPal),
    GregalTema("nit", NitClar, NitFosc),
    GregalTema("calma", CalmaClar, CalmaFosc),
    GregalTema("paper", PaperClar, PaperFosc),
    GregalTema("grafit", GrafitClar, GrafitFosc),
)

fun temaPerId(id: String?): GregalTema =
    GregalTemes.firstOrNull { it.id == id } ?: GregalTemes.first()

/** Tria la paleta: el tema triat (gregal per defecte) i el mode
 *  ('clar', 'fosc' o 'sistema', com la web). */
fun paletteFor(temaId: String?, mode: String, systemDark: Boolean): GregalPalette {
    val fosc = when (mode) {
        "clar" -> false
        "fosc" -> true
        else -> systemDark
    }
    return temaPerId(temaId).perMode(fosc)
}

/** Compatibilitat: el tema de sempre amb el mode (l'usaven els tests que
 *  comproven la paleta documentada). */
fun paletteFor(tema: String, systemDark: Boolean): GregalPalette =
    paletteFor("gregal", tema, systemDark)

/** Si el mode triat acaba en fosc (per a la barra d'estat i els tests). */
fun esFosc(mode: String, systemDark: Boolean): Boolean = when (mode) {
    "clar" -> false
    "fosc" -> true
    else -> systemDark
}

// Getters temàtics amb els noms de sempre: els call sites no canvien.
val Sea @Composable @ReadOnlyComposable get() = LocalPal.current.sea
val SeaClara @Composable @ReadOnlyComposable get() = LocalPal.current.seaClara
/** Fons d'un estat seleccionat o destacat. En fosc és neutre (la llima
 *  barrejada amb el grafit feia oliva); en clar, l'accent tenyit com sempre. */
fun GregalPalette.soft(alpha: Float): Color = if (dark) ink.copy(alpha = alpha * 0.7f) else sea.copy(alpha = alpha)
@Composable @ReadOnlyComposable fun SeaSoft(alpha: Float): Color = LocalPal.current.soft(alpha)
val Sand @Composable @ReadOnlyComposable get() = LocalPal.current.sand
val Onada @Composable @ReadOnlyComposable get() = LocalPal.current.onada
val Algua @Composable @ReadOnlyComposable get() = LocalPal.current.algua
val DeepTop @Composable @ReadOnlyComposable get() = LocalPal.current.deepTop
val DeepBottom @Composable @ReadOnlyComposable get() = LocalPal.current.deepBottom
// Els noms de superfície en català com al document: `Surface` xocaria amb el
// composable de Material3 que tots els fitxers importen.
val Superficie @Composable @ReadOnlyComposable get() = LocalPal.current.surface
val SuperficieAlta @Composable @ReadOnlyComposable get() = LocalPal.current.surfaceHi
val Ink @Composable @ReadOnlyComposable get() = LocalPal.current.ink
val Tinta @Composable @ReadOnlyComposable get() = LocalPal.current.text
val Muted @Composable @ReadOnlyComposable get() = LocalPal.current.muted
val Faint @Composable @ReadOnlyComposable get() = LocalPal.current.faint
val Green @Composable @ReadOnlyComposable get() = LocalPal.current.green
val Red @Composable @ReadOnlyComposable get() = LocalPal.current.red
val Lila @Composable @ReadOnlyComposable get() = LocalPal.current.lila
val LilaFons @Composable @ReadOnlyComposable get() = LocalPal.current.lilaFons
val Line @Composable @ReadOnlyComposable get() = LocalPal.current.line
val LineHi @Composable @ReadOnlyComposable get() = LocalPal.current.lineHi
val PillDark @Composable @ReadOnlyComposable get() = LocalPal.current.surface
val BubbleDark @Composable @ReadOnlyComposable get() = LocalPal.current.surface
val CodeDark @Composable @ReadOnlyComposable get() = LocalPal.current.surfaceHi
val Abyss @Composable @ReadOnlyComposable get() = LocalPal.current.abyss
val OnSea @Composable @ReadOnlyComposable get() = LocalPal.current.onSea
/** La llima del tauler de marca: ressalts (píndoles, «la millor opció») en tots dos temes. */
val Lime = Color(0xFFD4F25A)
val SendBg @Composable @ReadOnlyComposable get() = LocalPal.current.sendBg
val SendFg @Composable @ReadOnlyComposable get() = LocalPal.current.sendFg
val UserBubble @Composable @ReadOnlyComposable get() = LocalPal.current.userBubble
val OnUserBubble @Composable @ReadOnlyComposable get() = LocalPal.current.onUserBubble
