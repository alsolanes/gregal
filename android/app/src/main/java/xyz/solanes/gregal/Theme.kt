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
)

val LocalPal = compositionLocalOf { DarkPal }

/** Schemes Material derivats de la paleta (per als components M3). */
fun GregalPalette.scheme() = if (this == DarkPal) darkColorScheme(
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

/** Tria la paleta: 'clar', 'fosc' o 'sistema' (defecte, com la web). */
fun paletteFor(tema: String, systemDark: Boolean): GregalPalette = when (tema) {
    "clar" -> LightPal
    "fosc" -> DarkPal
    else -> if (systemDark) DarkPal else LightPal
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
