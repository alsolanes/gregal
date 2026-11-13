package xyz.solanes.gregal

import org.junit.Assert.*
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner

@RunWith(RobolectricTestRunner::class)
class PrefsNavigationTest {
    private fun vm(): GregalViewModel {
        val app = androidx.test.core.app.ApplicationProvider.getApplicationContext<android.app.Application>()
        return GregalViewModel(app).also { it.go(Screen.Company); it.resetBackStackForTest() }
    }

    @Test fun connectionsShortcutAndSystemBackStayWithinPreferencesBeforeLeaving() {
        val vm = vm()
        vm.openPreferences(PrefsPage.Connections)
        vm.openPreferences(PrefsPage.CalendarMail)
        vm.openPreferences(PrefsPage.Mail)
        assertEquals(listOf(Screen.Company), vm.backStack.value)
        for (expected in listOf(PrefsPage.CalendarMail, PrefsPage.Connections, PrefsPage.Index)) {
            assertTrue(vm.back())
            assertEquals(Screen.Prefs, vm.screen.value)
            assertEquals(expected, vm.prefsPage.value)
        }
        assertTrue(vm.back())
        assertEquals(Screen.Company, vm.screen.value)
    }

    @Test fun reopeningPreferencesStartsAtTheIndex() {
        val vm = vm()
        vm.openPreferences(PrefsPage.Appearance)
        vm.go(Screen.Chat)
        vm.go(Screen.Prefs)
        assertEquals(PrefsPage.Index, vm.prefsPage.value)
        vm.openPreferences(PrefsPage.Models)
        vm.openPreferences()
        assertEquals(PrefsPage.Index, vm.prefsPage.value)
    }

    @Test fun personalSettingsDoNotDependOnServerAdministration() {
        assertFalse(PrefsPage.Server.available(coding = true, admin = false))
        assertFalse(PrefsPage.Permissions.available(coding = false, admin = true))
        for (page in listOf(PrefsPage.General, PrefsPage.Appearance, PrefsPage.Models, PrefsPage.Connections)) {
            assertTrue(page.available(coding = false, admin = false))
        }
        assertTrue(PrefsPage.Server.available(coding = false, admin = true))
    }
}
