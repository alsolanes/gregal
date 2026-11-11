package xyz.solanes.gregal

import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Test
import java.io.File

/**
 * Les fixtures del contracte (docs/assistant-contract/fixtures) es llegeixen
 * amb els models @Serializable del mòbil. JVM pura: kotlinx.serialization no
 * necessita Robolectric (org.json sí).
 */
class AssistantContractTest {

    private fun fixturesDir(): File {
        var dir: File? = File("").absoluteFile
        while (dir != null) {
            val f = File(dir, "docs/assistant-contract/fixtures")
            if (f.isDirectory) return f
            dir = dir.parentFile
        }
        error("no trobo docs/assistant-contract/fixtures")
    }

    private fun fixture(name: String) = File(fixturesDir(), name).readText()

    @Test fun totesLesFixturesEsLlegeixen() {
        val files = fixturesDir().listFiles { f -> f.name.endsWith(".json") }.orEmpty()
        assertTrue("sense fixtures", files.isNotEmpty())
        val kinds = files.map { AssistantContract.parse(it.readText())::class.simpleName }.toSet()
        assertEquals(setOf("Encarrec", "Decisio", "Connexio", "Ordinador", "Missatge"), kinds)
    }

    @Test fun encarrec() {
        val e = AssistantContract.parse(fixture("encarrec-esperant.json")) as Encarrec
        assertEquals("enc-viatge-divendres", e.id)
        assertEquals("waiting_user", e.state)
        assertEquals(412L, e.runId)
        assertEquals(listOf("mail:pilot@example.test", "web"), e.allowedScope.read)
        assertEquals(emptyList<String>(), e.allowedScope.act)
        assertEquals("dec-hotel-arribada", e.pendingDecisionId)
    }

    @Test fun decisio() {
        val d = AssistantContract.parse(fixture("decisio-pendent.json")) as Decisio
        assertEquals("pending", d.state)
        assertEquals(1, d.revision)
        assertEquals("send_message", d.proposedEffect.type)
        assertEquals("res-correu-hotel-v1", d.proposedEffect.draftRef)
    }

    @Test fun connexio() {
        val c = AssistantContract.parse(fixture("connexio-caducada.json")) as Connexio
        assertEquals("needs_session", c.status)
        assertTrue(c.authRef.startsWith("auth:"))
        assertEquals(listOf("read"), c.capabilities)
    }

    @Test fun ordinadorAmbCampNul() {
        // "leaseExpiry": null → el valor per defecte, no un error.
        val o = AssistantContract.parse(fixture("ordinador-aturat.json")) as Ordinador
        assertEquals("stopped", o.health)
        assertEquals("none", o.controlOwner)
        assertEquals("", o.leaseExpiry)
    }

    @Test fun missatgeProactiu() {
        val m = AssistantContract.parse(fixture("missatge-proactiu.json")) as Missatge
        assertEquals("heartbeat", m.source)
        assertEquals(412L, m.relatedRunId)
        assertTrue(m.text.startsWith("Sobre el viatge"))
    }

    @Test fun unServidorAnticAmbMenysCampsNoTrencaRes() {
        val d = AssistantContract.parse("""{"kind":"decisio","id":"d1","camp_nou":{"x":[1,2]}}""") as Decisio
        assertEquals("d1", d.id)
        assertEquals("", d.state)
        assertEquals(ProposedEffect(), d.proposedEffect)
    }
}

/** Les lectures noves toleren el mateix que org.json: camps que falten, nuls, desconeguts i números entre cometes. */
class GregalJsonTest {
    @Test fun capFalta() {
        val st = CapState.parse("{}")
        assertEquals("Ventet", st.name)
        assertEquals(CapPet(), st.pet)
        assertEquals(0, st.briefCount)
    }

    @Test fun nulsIDesconegutsIQuotes() {
        val st = CapState.parse("""{"name":null,"color":"#112233","brief":{"done":"2","news":1,"text":null},"pet":{"level":"3","nou":true},"extra":[1]}""")
        assertEquals("Ventet", st.name)
        assertEquals("#112233", st.color)
        assertEquals(3, st.briefCount)
        assertEquals("", st.brief)
        assertEquals(3, st.pet.level)
        assertEquals(70, st.pet.happy)
    }

    @Test fun avuiSenseRes() {
        val v = AvuiJson.view("""{"now":null}""")
        assertTrue(v.now.isEmpty())
        assertEquals("22:00", v.settings.quietFrom)
    }

    @Test fun laCopiaDesadaDAbansEsLlegeix() {
        // Format escrit per la versió amb org.json.
        val raw = """[{"from":"pet","text":"Hola","speaker_id":"x","speaker_name":"Xaloc","ts":5,"kind":"","parcial":true,"id":"m1"}]"""
        val m = OfflineLogic.decodeChat(raw)!!.single()
        assertEquals("Xaloc", m.speakerName)
        assertTrue(m.parcial)
        assertEquals(OfflineLogic.decodeChat(OfflineLogic.encodeChat(listOf(m))), listOf(m))
        assertEquals(mapOf("a" to 5L), OfflineLogic.decodeShown("""{"a":5}"""))
        assertTrue(OfflineLogic.decodeOutbox("no-json").isEmpty())
    }

    @Test fun preguntesIInteraccions() {
        assertEquals(listOf(QuestionOption("Sí", "")), parseQuestionOptions("""[{"label":"Sí"},7]"""))
        assertEquals("""{"key":"k1","query":"?"}""", structuredInteractivePayload("""{"key":"k1","query":"?","text":"x"}"""))
        assertEquals("""{"key":"k2"}""", structuredInteractivePayload("""{"payload":"{\"key\":\"k2\"}"}"""))
        assertEquals("", structuredInteractivePayload("no-json"))
    }
}
