package xyz.solanes.gregal

import org.junit.Assert.assertEquals
import org.junit.Test

class ModeNavigationTest {
    @Test fun switchesBackToLastScreenInEachArea() {
        assertEquals(Screen.Ventet, modeDestination(AppArea.Vents, Screen.Ventet, Screen.Carpeta))
        assertEquals(Screen.Carpeta, modeDestination(AppArea.Code, Screen.Ventet, Screen.Carpeta))
    }

    @Test fun invalidRememberedScreenFallsBackToAreaHome() {
        assertEquals(Screen.Company, modeDestination(AppArea.Vents, Screen.Chat, Screen.Carpeta))
        assertEquals(Screen.Work, modeDestination(AppArea.Code, Screen.Company, Screen.Work))
        assertEquals(Screen.Company, parseRememberedScreen("Prefs", AppArea.Vents, Screen.Company))
        assertEquals(Screen.Chat, parseRememberedScreen("Unknown", AppArea.Code, Screen.Chat))
    }

    @Test fun auxiliaryScreensDoNotReplaceTheRememberedArea() {
        assertEquals(null, Screen.Prefs.appArea())
        assertEquals(AppArea.Vents, activeAppArea(Screen.Prefs, true))
        assertEquals(AppArea.Code, activeAppArea(Screen.Prefs, false))
        assertEquals(AppArea.Code, activeAppArea(Screen.Work, true))
        assertEquals(AppArea.Vents, activeAppArea(Screen.Company, false))
    }
}
