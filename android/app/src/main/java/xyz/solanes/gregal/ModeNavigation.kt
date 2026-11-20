package xyz.solanes.gregal

/** The two top-level areas; auxiliary screens keep the currently selected area. */
internal enum class AppArea { Vents, Code }

internal fun Screen.appArea(): AppArea? = when (this) {
    Screen.Company, Screen.Ventet -> AppArea.Vents
    Screen.Chat, Screen.Sessions, Screen.Carpeta, Screen.Work -> AppArea.Code
    else -> null
}

internal fun activeAppArea(screen: Screen, simpleVents: Boolean): AppArea =
    screen.appArea() ?: if (simpleVents) AppArea.Vents else AppArea.Code

internal fun parseRememberedScreen(name: String, area: AppArea, fallback: Screen): Screen {
    val parsed = runCatching { Screen.valueOf(name) }.getOrNull()
    return parsed?.takeIf { it.appArea() == area } ?: fallback
}

/** Restore only a route that belongs to the requested area; stale prefs fall back safely. */
internal fun modeDestination(area: AppArea, lastVents: Screen, lastCode: Screen): Screen {
    val remembered = if (area == AppArea.Vents) lastVents else lastCode
    if (remembered.appArea() == area) return remembered
    return if (area == AppArea.Vents) Screen.Company else Screen.Chat
}
