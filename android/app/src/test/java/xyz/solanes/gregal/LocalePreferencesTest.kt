package xyz.solanes.gregal

import android.content.Context
import android.content.res.Configuration
import android.os.LocaleList
import androidx.test.core.app.ApplicationProvider
import org.junit.Assert.*
import org.junit.Before
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config
import java.util.Locale

@RunWith(RobolectricTestRunner::class)
@Config(sdk = [34])
class LocalePreferencesTest {
    private val app: Context get() = ApplicationProvider.getApplicationContext()
    @Before fun clear() { app.getSharedPreferences("gregal", 0).edit().clear().commit() }
    private fun prefs(tag: String, secondary: String = ""): Prefs {
        val config = Configuration(app.resources.configuration)
        config.setLocales(LocaleList.forLanguageTags(listOf(tag, secondary).filter { it.isNotEmpty() }.joinToString(",")))
        return Prefs(app.createConfigurationContext(config))
    }
    @Test fun primaryCatalanRegionsDefaultToCatalan() {
        assertEquals("ca", prefs("ca-ES").idioma)
        assertEquals("ca", prefs("ca-AD").idioma)
    }
    @Test fun everyOtherPrimaryLanguageDefaultsToEnglish() {
        for (tag in listOf("en-US", "es-ES", "fr-FR")) assertEquals("en", prefs(tag, "ca-ES").idioma)
    }
    @Test fun explicitChoiceSurvivesDeviceLanguageChanges() {
        prefs("ca-ES").idioma = "en"
        assertEquals("en", prefs("ca-AD").idioma)
        prefs("en-US").idioma = "ca"
        assertEquals("ca", prefs("fr-FR").idioma)
        prefs("en-US").idioma = "system"
        assertEquals("ca", prefs("ca-AD").idioma)
        assertEquals("en", prefs("es-ES").idioma)
    }
}
