package xyz.solanes.gregal

import android.graphics.Bitmap
import androidx.compose.foundation.layout.*
import androidx.compose.material3.MaterialTheme
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.ui.Modifier
import androidx.compose.ui.test.junit4.createAndroidComposeRule
import androidx.compose.ui.unit.dp
import androidx.compose.ui.test.onNodeWithTag
import androidx.compose.ui.test.onRoot
import org.junit.Assert.assertTrue
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config
import org.robolectric.annotation.GraphicsMode
import org.robolectric.annotation.LooperMode
import java.io.File

/** Un torn de Codi tal com es veu al mòbil: pregunta, treball i resposta. Captures a `build/code-chat-look/`. */
@RunWith(RobolectricTestRunner::class)
@Config(sdk = [34], qualifiers = "w360dp-h780dp-xhdpi")
@GraphicsMode(GraphicsMode.Mode.NATIVE)
@LooperMode(LooperMode.Mode.PAUSED)
class CodeChatLookTest {
    @get:Rule val ui = createAndroidComposeRule<androidx.activity.ComponentActivity>()

    private fun tool(id: String, name: String, args: String, state: StepState = StepState.Ok, out: String = "ok"): WorkStep {
        val sum = summarizeTool(name, args, "ca")
        return WorkStep(id, StepKind.Tool, sum.label, sum.target, sum.meta, out, state, name, exploreTool(name, args), args)
    }

    private val work = WorkGroup(listOf(
        WorkStep("n1", StepKind.Note, "Miro on es valida el login.", detail = "Miro on es valida el login."),
        tool("1", "read", """{"path":"internal/auth/login.go"}"""),
        tool("2", "grep", """{"pattern":"ValidToken"}"""),
        WorkStep("n2", StepKind.Note, "Hi falta comprovar la caducitat; ho afegeixo i passo els tests.", detail = "Hi falta comprovar la caducitat; ho afegeixo i passo els tests."),
        tool("3", "edit", """{"path":"internal/auth/login.go","old_string":"if ok {","new_string":"if ok && !expired(t) {\n"}"""),
        tool("4", "bash", """{"command":"go test ./internal/auth"}"""),
    ), started = 1L, ended = 14_000L)

    private val answer = """
Ja està: el login rebutja els tokens caducats.

- `internal/auth/login.go`: `ValidToken` comprova la caducitat abans d'acceptar-lo.
- Els tests de `internal/auth` passen.

```go
if ok && !expired(t) {
	return user, nil
}
```
""".trimIndent()

    private fun capture(name: String, pal: GregalPalette) {
        ui.mainClock.autoAdvance = false
        ui.setContent {
            CompositionLocalProvider(LocalPal provides pal, LocalWindMotion provides false) {
                MaterialTheme(colorScheme = pal.scheme()) {
                    GregalBg {
                        Column(Modifier.fillMaxSize().padding(horizontal = 12.dp, vertical = 16.dp), verticalArrangement = Arrangement.spacedBy(10.dp)) {
                            MsgRow(ChatMsg("user", "Fes que el login rebutgi els tokens caducats"), 0)
                            WorkGroupCard(work, null, "", "ca")
                            MsgRow(ChatMsg("assistant", answer), 1)
                        }
                    }
                }
            }
        }
        ui.mainClock.advanceTimeBy(800)
        val view = ui.activity.window.decorView
        val bmp = Bitmap.createBitmap(view.width, view.height, Bitmap.Config.ARGB_8888)
        ui.runOnUiThread { view.draw(android.graphics.Canvas(bmp)) }
        File("build/code-chat-look/$name.png").also { it.parentFile?.mkdirs() }.outputStream().use { bmp.compress(Bitmap.CompressFormat.PNG, 100, it) }
    }

    @Test fun dark() = capture("dark", DarkPal)

    /** La bombolla de l'usuari queda a la dreta i no ocupa tota l'amplada, encara que el text sigui llarg. */
    @Test fun `la bombolla de l'usuari queda a la dreta i deixa marge a l'esquerra`() {
        capture("dark", DarkPal)
        val root = ui.onRoot().fetchSemanticsNode().boundsInRoot
        val bubble = ui.onNodeWithTag("user-bubble").fetchSemanticsNode().boundsInRoot
        assertTrue("a la dreta: ${bubble.right} de ${root.right}", root.right - bubble.right < 40f * ui.density.density)
        assertTrue("amb marge a l'esquerra: ${bubble.left}", bubble.left > root.width * 0.14f)
    }
    @Test fun light() = capture("light", LightPal)
}
