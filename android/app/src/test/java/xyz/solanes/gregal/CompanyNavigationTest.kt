package xyz.solanes.gregal

import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Surface
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.ui.Modifier
import androidx.compose.ui.test.*
import androidx.compose.ui.test.junit4.createComposeRule
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config
import org.robolectric.annotation.GraphicsMode
import org.robolectric.annotation.LooperMode

@RunWith(RobolectricTestRunner::class)
@Config(sdk = [34], qualifiers = "w393dp-h851dp")
@GraphicsMode(GraphicsMode.Mode.NATIVE)
@LooperMode(LooperMode.Mode.PAUSED)
class CompanyNavigationTest {
    @get:Rule val ui = createComposeRule()

    @Test
    fun openingPetChatReplacesCompanyScreenInsteadOfOverlayingIt() {
        val pet = Pet("mel", "Mel", "música")
        ui.setContent {
            CompositionLocalProvider(LocalPal provides LightPal) {
                MaterialTheme(colorScheme = LightPal.scheme()) {
                    Surface(Modifier.fillMaxSize()) {
                        CompanyRoute(
                            openPet = pet,
                            companyContent = { androidx.compose.material3.Text("company home") },
                            petChatContent = { androidx.compose.material3.Text("chat with ${it.name}") },
                        )
                    }
                }
            }
        }

        ui.onNodeWithText("chat with Mel").assertExists()
        ui.onNodeWithText("company home").assertDoesNotExist()
    }

    @Test
    fun noSelectedPetShowsOnlyCompanyScreen() {
        ui.setContent {
            CompositionLocalProvider(LocalPal provides LightPal) {
                MaterialTheme(colorScheme = LightPal.scheme()) {
                    Surface(Modifier.fillMaxSize()) {
                        CompanyRoute(
                            openPet = null,
                            companyContent = { androidx.compose.material3.Text("company home") },
                            petChatContent = { androidx.compose.material3.Text("chat with ${it.name}") },
                        )
                    }
                }
            }
        }

        ui.onNodeWithText("company home").assertExists()
        ui.onNodeWithText("chat with Mel").assertDoesNotExist()
    }

    @Test
    fun companyHomeShowsWindChatCard() {
        // La cara del vent i el seu xat són el centre de la pantalla Company.
        ui.setContent {
            CompositionLocalProvider(LocalPal provides LightPal) {
                MaterialTheme(colorScheme = LightPal.scheme()) {
                    Surface(Modifier.fillMaxSize()) {
                        CompanyRoute(
                            openPet = null,
                            companyContent = {
                                androidx.compose.material3.Text(tr("ca", "app.company.windEmptyTitle"))
                                androidx.compose.material3.Text(tr("ca", "app.company.windNew"))
                            },
                            petChatContent = { androidx.compose.material3.Text("chat with ${it.name}") },
                        )
                    }
                }
            }
        }

        ui.onNodeWithText("Cap vent encara").assertExists()
        ui.onNodeWithText("Nou vent").assertExists()
    }
}
