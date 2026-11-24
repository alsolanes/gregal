package xyz.solanes.gregal

import org.json.JSONObject
import org.junit.Assert.assertEquals
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config

/** «+ New knowledge» in a wind's profile saves with the method the server takes (docs/api-contract.md: PUT). */
@RunWith(RobolectricTestRunner::class)
@Config(sdk = [34])
class WindConeixementsTest {
    @Test fun `saving a wind's knowledge is a PUT, the server answers 405 to a POST`() {
        FakeGregal { """{"ok":true,"nom":"paella"}""" }.use { server ->
            val api = GregalApi("http://127.0.0.1:${server.port}", "token")
            val nom = api.saveConeixement("gregal", GregalApi.Coneixement("paella", "Paella", "How to cook paella", "Rice"))
            assertEquals("paella", nom)
            val req = server.requests.single()
            assertEquals("PUT /api/winds/coneixements", req.substringBefore(" {"))
            val body = JSONObject(req.substringAfter("coneixements "))
            assertEquals("gregal", body.getString("wind"))
            assertEquals("Rice", body.getString("cos"))
        }
    }
}
