package xyz.solanes.gregal

import androidx.test.core.app.ApplicationProvider
import org.json.JSONObject
import org.junit.Assert.*
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config

/**
 * La narració que el model escriu abans d'una eina va dins el treball del torn,
 * com a la web, i hi porta les pastilles de les crides; la resposta final
 * continua sent un missatge de la conversa.
 */
@RunWith(RobolectricTestRunner::class)
@Config(sdk = [34])
class NarrationWorkTest {
    private fun call(vm: GregalViewModel, name: String, vararg args: Any) {
        val m = GregalViewModel::class.java.declaredMethods.first { it.name == name && it.parameterCount == args.size }
        m.isAccessible = true
        m.invoke(vm, *args)
    }
    private fun event(vm: GregalViewModel, kind: String, o: JSONObject) = call(vm, "onEvent", kind, o, "")

    @Test fun narrationBeforeAToolGoesIntoTheWorkGroup() {
        val vm = GregalViewModel(ApplicationProvider.getApplicationContext())
        call(vm, "beginTurn")
        event(vm, "token", JSONObject().put("text", "Busco on es valida la contrasenya."))
        event(vm, "tool_call", JSONObject().put("id", "c1").put("name", "read").put("args", JSONObject().put("path", "src/login.go")))
        event(vm, "tool_result", JSONObject().put("id", "c1").put("name", "read").put("output", "package login"))
        event(vm, "token", JSONObject().put("text", "Arreglat."))
        call(vm, "endTurn", false)

        val msgs = vm.msgs.value
        assertTrue(msgs.none { it.kind == "text" && it.text.contains("Busco on es valida") })
        val steps = msgs.single { it.kind == "work" }.work!!.steps
        assertEquals(listOf(StepKind.Note, StepKind.Tool), steps.map { it.kind })
        assertEquals("Busco on es valida la contrasenya.", steps[0].detail)
        // La resposta final, després del treball.
        assertEquals("Arreglat.", msgs.last().text)
        assertEquals("work", msgs[msgs.size - 2].kind)
    }
}
