package xyz.solanes.gregal

import android.app.Application
import android.content.Context
import androidx.test.core.app.ApplicationProvider
import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner

@RunWith(RobolectricTestRunner::class)
class ModeSwitchViewModelTest {
    @Test fun remembersBothAreasAcrossSwitchesAndViewModelRecreation() {
        val app = ApplicationProvider.getApplicationContext<Application>()
        app.getSharedPreferences("gregal", Context.MODE_PRIVATE).edit().clear().commit()

        val first = GregalViewModel(app).apply { attachPrefs(Prefs(app)) }
        first.go(Screen.Company)
        first.go(Screen.Chat)
        first.go(Screen.Work)
        first.switchArea(AppArea.Vents)
        assertEquals(Screen.Company, first.screen.value)
        assertTrue(first.simpleVents.value)
        first.go(Screen.Ventet)
        first.switchArea(AppArea.Code)
        assertEquals(Screen.Work, first.screen.value)
        assertEquals(false, first.simpleVents.value)

        val recreated = GregalViewModel(app).apply { attachPrefs(Prefs(app)) }
        recreated.switchArea(AppArea.Vents)
        assertEquals(Screen.Ventet, recreated.screen.value)
        assertTrue(recreated.simpleVents.value)
    }
}
