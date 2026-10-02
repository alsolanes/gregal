package xyz.solanes.gregal

import androidx.compose.material3.darkColorScheme
import androidx.compose.material3.lightColorScheme
import androidx.compose.runtime.Composable
import androidx.compose.runtime.ReadOnlyComposable
import androidx.compose.runtime.compositionLocalOf
import androidx.compose.ui.graphics.Color

/** Paleta Gregal. Els noms en anglès es queden (els call sites no canvien),
 *  però els valors són els de `docs/identitat.md`: abans aquest fitxer
 *  portava la paleta vella (#101418/#0C2229) i l'app semblava una altra.
 *
 *  Els hex són els mateixos que `internal/tui/theme.go` i `:root` de la web;
 *  si un canvia, han de canviar tots tres (hi ha un test a cada front). */
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
    /** Les línies fines: escuma al 10% (fosc) / escumaClar al 14% (clar). */
    val line: Color,
    val lineHi: Color,
    val abyss: Color,
) {
    // Àlies històrics: noms que ja feia servir tota la UI.
    val pillDark: Color get() = surface
    val bubbleDark: Color get() = surface
    val codeDark: Color get() = surfaceHi
}

val DarkPal = GregalPalette(
    sea = Color(0xFF7FD4C1),
    seaClara = Color(0xFFA3E4D6),
    sand = Color(0xFFF2E7C9),
    onada = Color(0xFF6CB4C7),
    algua = Color(0xFF1E3A44),
    deepTop = Color(0xFF0F172A),
    deepBottom = Color(0xFF0B1220),
    surface = Color(0xFF111C2F),
    surfaceHi = Color(0xFF1A2740),
    ink = Color(0xFFF1F5F9),
    text = Color(0xFFE2E8F0),
    muted = Color(0xFF94A3B8),
    faint = Color(0xFF475569),
    green = Color(0xFF4ADE80),
    red = Color(0xFFF97066),
    lila = Color(0xFFC4B5FD),
    lilaFons = Color(0xFF2A2440),
    line = Color(0x1A7FD4C1),   // escuma 10%
    lineHi = Color(0x337FD4C1), // escuma 20%
    abyss = Color(0xFF0B1220),
)

val LightPal = GregalPalette(
    sea = Color(0xFF0E8C74),
    seaClara = Color(0xFF0A6F5C),
    sand = Color(0xFF9A6B12),
    onada = Color(0xFF0E7490),
    algua = Color(0xFF7C8B95),
    deepTop = Color(0xFFF5F7F8),
    deepBottom = Color(0xFFEBEFF1),
    surface = Color(0xFFFFFFFF),
    surfaceHi = Color(0xFFE2E9EC),
    ink = Color(0xFF0B1220),
    text = Color(0xFF1E293B),
    muted = Color(0xFF52616B),
    faint = Color(0xFF7C8B95),
    green = Color(0xFF15803D),
    red = Color(0xFFDC2626),
    lila = Color(0xFF6D28D9),
    lilaFons = Color(0xFFEDE9FE),
    line = Color(0x240E8C74),   // escumaClar 14%
    lineHi = Color(0x470E8C74), // escumaClar 28%
    abyss = Color(0xFF0B1220),
)

val LocalPal = compositionLocalOf { DarkPal }

/** Schemes Material derivats de la paleta (per als components M3). */
fun GregalPalette.scheme() = if (this == DarkPal) darkColorScheme(
    primary = sea, onPrimary = abyss, secondary = sand,
    background = deepTop, surface = surface, surfaceVariant = surfaceHi,
    onBackground = ink, onSurface = ink, error = red,
) else lightColorScheme(
    primary = sea, onPrimary = Color.White, secondary = sand,
    background = deepTop, surface = surface, surfaceVariant = surfaceHi,
    onBackground = ink, onSurface = ink, error = red,
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
