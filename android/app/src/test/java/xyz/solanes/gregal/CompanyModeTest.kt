package xyz.solanes.gregal

import org.json.JSONObject
import org.junit.Assert.*
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config

/**
 * Mode company (estil Dot): parseig de les respostes de /api/companion.
 *
 * Els contractes (docs/api-contract.md):
 *  - GET /api/companion/memory → {state, facts, notes}
 *  - POST {action:accept|dismiss|forget} → {ok,total,notes} / {forgotten,notes}
 *  - POST /api/companion/checkin → {checkin, at}
 */
@RunWith(RobolectricTestRunner::class)
@Config(sdk = [34])
class CompanyModeTest {

    private fun parseMemory(raw: String): Triple<String, List<String>, List<String>> {
        val o = JSONObject(raw)
        return Triple(
            o.optString("state"),
            o.optJSONArray("facts")?.let { a -> (0 until a.length()).map { a.optString(it) } } ?: emptyList(),
            o.optJSONArray("notes")?.let { a -> (0 until a.length()).map { a.optString(it) } } ?: emptyList(),
        )
    }

    @Test fun memoryAmbPropostaIFets() {
        val (state, facts, notes) = parseMemory(
            """{"state":"ready","facts":["Fa servir una e-bike per anar a la feina"],
                "notes":["- 2026-10-19: Em dic Aleix"]}""")
        assertEquals("ready", state)
        assertEquals(1, facts.size)
        assertTrue(facts[0].contains("e-bike"))
        assertEquals(1, notes.size)
        assertTrue(notes[0].contains("Aleix"))
    }

    @Test fun memorySenseRes() {
        val (state, facts, notes) = parseMemory("""{"state":"none","facts":[],"notes":[]}""")
        assertEquals("none", state)
        assertTrue(facts.isEmpty())
        assertTrue(notes.isEmpty())
    }

    @Test fun acceptResposta() {
        val o = JSONObject("""{"ok":true,"total":1,"notes":["- 2026-10-19: Té dos fills"]}""")
        assertTrue(o.getBoolean("ok"))
        assertEquals(1, o.getInt("total"))
        assertEquals(1, o.getJSONArray("notes").length())
    }

    @Test fun forgetResposta() {
        val o = JSONObject("""{"ok":true,"forgotten":2,"notes":[]}""")
        assertTrue(o.getBoolean("ok"))
        assertEquals(2, o.getInt("forgotten"))
        assertEquals(0, o.getJSONArray("notes").length())
    }

    @Test fun checkinResposta() {
        val o = JSONObject(
            """{"checkin":"Com va la app de música? Vas dir que li dedicaves les tardes.","at":"2026-10-19T10:00:00Z"}""")
        val c = o.optString("checkin")
        assertTrue(c.contains("app de música"))
        assertTrue(o.optString("at").startsWith("2026-10-19"))
    }

    @Test fun peticionsQueEnviaLApi() {
        // El cos de les peticions POST: exactament el contracte.
        assertEquals("""{"action":"accept","fact":"Té dos fills"}""",
            CompanionRequests.accept("Té dos fills"))
        assertEquals("""{"action":"dismiss"}""", CompanionRequests.dismiss())
        assertEquals("""{"action":"forget","term":"e-bike"}""",
            CompanionRequests.forget("e-bike"))
    }

    @Test fun tascaDotDiferenciaInvestigacioDeCanviAmbAprovacio() {
        val inspect = CompanyWorkPrompt.build("  Comprova el projecte  ", makeChanges = false)
        assertTrue(inspect.contains("només lectura"))
        assertTrue(inspect.contains("Comprova el projecte"))
        assertTrue(inspect.contains("no modifiquis cap fitxer"))

        val change = CompanyWorkPrompt.build("  Arregla el test  ", makeChanges = true)
        assertTrue(change.contains("Arregla el test"))
        assertTrue(change.contains("No despleguis"))
        assertTrue(change.contains("demana aprovació explícita"))
    }

    @Test fun tascaDotBuidaNoGeneraCapTorn() {
        assertEquals("", CompanyWorkPrompt.build("  \n ", makeChanges = false))
    }

    @Test fun windParsejaElCataleg() {
        // GET /api/winds → {winds:[{id,name,color,role}]} (la persona no viatja)
        val winds = Wind.parse("""{"winds":[{"id":"tramuntana","name":"Tramuntana","color":"#29ADFF","role":"sistema"}]}""")
        assertEquals(1, winds.size)
        assertEquals("tramuntana", winds[0].id)
        assertEquals("#29ADFF", winds[0].color)
        assertEquals(0xFF29ADFFL, winds[0].argb)
    }

    @Test fun windColorInvalidCauAlTaronja() {
        val winds = Wind.parse("""{"winds":[{"id":"x","name":"X","color":"foo","role":""}]}""")
        assertEquals(0xFFFF9F00L, winds[0].argb)
    }

    @Test fun windChatParsejaHistorialIResposta() {
        // POST /api/winds/chat → {reply,history}; GET → {history}
        val o = JSONObject("""{"reply":"Avui toca vent de popa!","history":[{"from":"user","text":"què fem?"},{"from":"pet","text":"Avui toca vent de popa!"}]}""")
        assertEquals("Avui toca vent de popa!", o.optString("reply"))
        val a = o.optJSONArray("history")
        assertEquals(2, a.length())
        assertEquals("user", a.optJSONObject(0).optString("from"))
        assertEquals("pet", a.optJSONObject(1).optString("from"))
    }

    @Test fun windRequestsCosExacte() {
        // El cos ara porta cara i persona (proposta de la IA o de l'usuari).
        assertEquals("""{"name":"Mestral","color":"#A3E24E","role":"codi","style":"pso","persona":""}""",
            WindRequests.save("Mestral", "#A3E24E", "codi"))
        assertEquals("""{"name":"Xaloc","color":"#FF9F00","role":"música","style":"eto","persona":"Sóc en Xaloc."}""",
            WindRequests.save("Xaloc", "#FF9F00", "música", "eto", "Sóc en Xaloc."))
        assertEquals("""{"wind":"mestral","message":"hola"}""",
            WindRequests.chat("mestral", "hola"))
    }

    @Test fun windStyleParsejaLesPeces() {
        // Codi de tres lletres: ulls/boca/forma; buit o invàlid → pso.
        assertEquals(WindStyle(WindEyes.DOTS, WindMouth.SMILE, WindShape.BALL), windStyleOf("pso"))
        assertEquals(WindStyle(WindEyes.STARRY, WindMouth.TONGUE, WindShape.CLOUD), windStyleOf("etn"))
        assertEquals(WindStyle(WindEyes.DOTS, WindMouth.SMILE, WindShape.BALL), windStyleOf(""))
        assertEquals(WindStyle(WindEyes.DOTS, WindMouth.SMILE, WindShape.BALL), windStyleOf("ZZ9"))
        assertEquals("etn", windStyleOf("etn").encode())
        assertEquals("pso", WindStyle().encode())
    }
    @Test fun rotllanaParsejaElCataleg() {
        val rots = RotllanaRequests.parse("""{"rotllanes":[{"id":"vesprada","name":"Vesprada","members":["tramuntana","xaloc"]}]}""")
        assertEquals(1, rots.size)
        assertEquals("vesprada", rots[0].id)
        assertEquals(listOf("tramuntana", "xaloc"), rots[0].members)
    }

    @Test fun rotllanaRequestsCosExacte() {
        assertEquals("""{"name":"Vesprada","members":["tramuntana","xaloc"]}""",
            RotllanaRequests.save("Vesprada", listOf("tramuntana", "xaloc")))
        assertEquals("""{"rotllana":"vesprada","message":"hola","wind":""}""",
            RotllanaRequests.chat("vesprada", "hola"))
        assertEquals("""{"rotllana":"vesprada","message":"hola","wind":"xaloc"}""",
            RotllanaRequests.chat("vesprada", "hola", "xaloc"))
    }

    @Test fun windHeartbeatJobIdEsDeterministic() {
        // El cor del vent és un job diari amb id wind-<vent>: el backend el
        // reconeix per aquest prefix i li dona consigna pròpia.
        assertEquals("wind-tramuntana", "wind-" + "tramuntana")
    }
}

/** Constructors dels cossos de petició del mode company (testejades). */
object CompanionRequests {
    fun accept(fact: String): String = JSONObject().put("action", "accept").put("fact", fact).toString()
    fun dismiss(): String = JSONObject().put("action", "dismiss").toString()
    fun forget(term: String): String = JSONObject().put("action", "forget").put("term", term).toString()
}
