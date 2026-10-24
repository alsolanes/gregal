package xyz.solanes.gregal

import org.json.JSONObject
import org.junit.Assert.assertEquals
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config

/** La proposta de colla arriba del servidor amb vents per reutilitzar i vents nous. */
@RunWith(RobolectricTestRunner::class)
@Config(sdk = [34])
class CrewProposalTest {
    @Test fun `llegeix la proposta i els vents nous porten la seva cara`() {
        val p = CrewProposal.parse(JSONObject("""{"plan":"Ratxa construeix","reuse":[{"id":"w-1","name":"Gregal","role":"Organització","why":"planifica"}],
            "create":[{"name":"Ratxa","role":"Pa i massa mare","kit":"","persona":"Fa pa.","why":"cal un forner","color":"#F7B2D9","style":"bon"}]}"""))
        assertEquals("Ratxa construeix", p.plan)
        assertEquals("w-1", p.reuse.single().id)
        val w = p.create.single().wind
        assertEquals("Ratxa", w.name)
        assertEquals("#F7B2D9", w.color)
        assertEquals(WindStyle(WindEyes.WIDE, WindMouth.OPEN, WindShape.CLOUD), w.faceStyle)
        assertEquals("Pa i massa mare", JSONObject(p.create.single().json().toString()).getString("role"))
    }
}
