package xyz.solanes.gregal

import androidx.compose.material3.darkColorScheme
import androidx.compose.material3.lightColorScheme
import androidx.compose.runtime.Composable
import androidx.compose.runtime.ReadOnlyComposable
import androidx.compose.runtime.compositionLocalOf
import androidx.compose.ui.graphics.Color

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
}

val DarkPal = GregalPalette(
    sea = Color(0xFF8193FF),        // escuma: blau Gregal
    seaClara = Color(0xFFA3B0FF),   // escumaClara
    sand = Color(0xFFF5C451),       // arena (avís)
    onada = Color(0xFF5CE1D6),      // aqua juganer
    algua = Color(0xFF2C3240),
    deepTop = Color(0xFF13151B),    // fons
    deepBottom = Color(0xFF0E1015), // abisme: sidebar/panells
    surface = Color(0xFF1A1D25),    // night
    surfaceHi = Color(0xFF242833),  // superficieAlta
    ink = Color(0xFFF3F5FA),
    text = Color(0xFFE4E7EF),
    muted = Color(0xFFA3AABA),
    faint = Color(0xFF8890A2),
    green = Color(0xFF6EDB95),
    red = Color(0xFFFF8A85),
    lila = Color(0xFFC4A8FF),
    lilaFons = Color(0xFF2A2245),
    line = Color(0x1AC9D1E6),       // gris fred #C9D1E6 al 10%
    lineHi = Color(0x33C9D1E6),     // gris fred al 20%
    abyss = Color(0xFF0E1015),
    onSea = Color(0xFF13151B),      // sobre l'accent: text fosc (fons)
    dark = true,
)

val LightPal = GregalPalette(
    sea = Color(0xFF4F5BE8),
    seaClara = Color(0xFF3D47C7),
    sand = Color(0xFF9A5B00),
    onada = Color(0xFF0A776F),
    algua = Color(0xFFD5DAE4),
    deepTop = Color(0xFFF6F7FA),
    deepBottom = Color(0xFFEEF0F5),
    surface = Color(0xFFFFFFFF),
    surfaceHi = Color(0xFFE5E8F0),
    ink = Color(0xFF0E1220),
    text = Color(0xFF1F2433),
    muted = Color(0xFF545D71),
    faint = Color(0xFF5F6879),
    green = Color(0xFF187A41),
    red = Color(0xFFC92A35),
    lila = Color(0xFF7444D6),
    lilaFons = Color(0xFFEEE8FD),
    line = Color(0x241E2A4A),       // navy #1E2A4A al 14%
    lineHi = Color(0x471E2A4A),     // navy al 28%
    abyss = Color(0xFFEEF0F5),
    onSea = Color(0xFFFFFFFF),      // sobre l'accent: blanc
    dark = false,
)

val LocalPal = compositionLocalOf { DarkPal }

/** Schemes Material derivats de la paleta (per als components M3). */
fun GregalPalette.scheme() = if (dark) darkColorScheme(
    primary = sea, onPrimary = onSea, secondary = onada, onSecondary = deepTop,
    tertiary = lila, outline = Color(0xFF626B80), outlineVariant = algua, // outline = boxIdle (3:1)
    background = deepTop, surface = surface, surfaceVariant = surfaceHi,
    onBackground = ink, onSurface = ink, onSurfaceVariant = muted, error = red,
) else lightColorScheme(
    primary = sea, onPrimary = onSea, secondary = onada, onSecondary = Color.White,
    tertiary = lila, outline = Color(0xFF7E879B), outlineVariant = algua, // outline = boxIdle (3:1)
    background = deepTop, surface = surface, surfaceVariant = surfaceHi,
    onBackground = ink, onSurface = ink, onSurfaceVariant = muted, error = red,
)

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

/** Calma: verd salvi i molt aire. Pensat per llegir, no per lluir. */
val CalmaClar = paleta(
    dark = false,
    accent = Color(0xFF1F6F5C), accentSoft = Color(0xFF17594A),
    aqua = Color(0xFF0F766E), sand = Color(0xFFA16207),
    bg = Color(0xFFF6F8F6), abyss = Color(0xFFEDF1EE),
    surface = Color(0xFFFFFFFF), surfaceHi = Color(0xFFE6EDE8),
    ink = Color(0xFF111C17), text = Color(0xFF1D2A24),
    muted = Color(0xFF55635C), faint = Color(0xFF6B7A72),
    green = Color(0xFF146C43), red = Color(0xFFB4232E),
    lila = Color(0xFF6D5BD0), lilaFons = Color(0xFFEAE7FA),
    onAccent = Color(0xFFFFFFFF),
)
val CalmaFosc = paleta(
    dark = true,
    accent = Color(0xFF4FD1A5), accentSoft = Color(0xFF7FE3C1),
    aqua = Color(0xFF5CE1D6), sand = Color(0xFFF5C451),
    bg = Color(0xFF0C1210), abyss = Color(0xFF080D0B),
    surface = Color(0xFF141B18), surfaceHi = Color(0xFF1D2521),
    ink = Color(0xFFEAF1EC), text = Color(0xFFDCE6E0),
    muted = Color(0xFF9BA9A1), faint = Color(0xFF7E8C85),
    green = Color(0xFF6EDB95), red = Color(0xFFFF8A85),
    lila = Color(0xFFC4B5FD), lilaFons = Color(0xFF202A3A),
    onAccent = Color(0xFF08120E),
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
