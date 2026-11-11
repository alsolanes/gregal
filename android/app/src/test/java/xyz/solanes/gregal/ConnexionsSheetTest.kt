package xyz.solanes.gregal

import androidx.compose.material3.MaterialTheme
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.test.*
import androidx.compose.ui.test.junit4.createComposeRule
import org.json.JSONObject
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config
import org.robolectric.annotation.GraphicsMode
import org.robolectric.annotation.LooperMode
import java.io.File

/** B5: el full de comptes connectats (dades del contracte, sense servidor). */
@RunWith(RobolectricTestRunner::class)
@Config(sdk = [34], qualifiers = "w360dp-h780dp")
@GraphicsMode(GraphicsMode.Mode.NATIVE)
@LooperMode(LooperMode.Mode.PAUSED)
class ConnexionsSheetTest {
    @get:Rule val ui = createComposeRule()

    private val sample = JSONObject("""
        {"connexions":[
          {"kind":"connexio","id":"con-a1","owner":"anna","method":"api","service":"mcp","provider":"notes","accountLabel":"anna@example.org",
           "status":"connected","capabilities":["read","send"],"allowed":["read"],"effective":["read"],"authRef":"auth:x","lastCheckedAt":"2026-11-11T09:05:00Z"},
          {"kind":"connexio","id":"con-b2","owner":"anna","method":"api","service":"mcp","provider":"notes","accountLabel":"personal",
           "status":"needs_session","capabilities":["read","send"],"allowed":["read","send"],"effective":["read","send"],"authRef":"auth:y","error":"Cal tornar a iniciar sessió"},
          {"kind":"connexio","id":"con-c3","owner":"anna","method":"api","service":"mail-imap","accountLabel":"anna@example.org",
           "status":"disconnected","capabilities":["read"],"allowed":["read"],"effective":["read"],"authRef":"auth:z"}
        ],
        "catalogue":[{"service":"calendar-ics","method":"api","how":"form"},{"service":"mcp","provider":"notes","method":"api","how":"oauth"},
                     {"service":"browser","method":"browser","how":"viewer","available":false}]}
    """.trimIndent())

    @Test fun parsesConnectionsAndCatalogue() {
        val st = parseConnexions(sample)
        assertEquals(3, st.items.size)
        val a = st.items[0]
        assertEquals("con-a1", a.id)
        assertEquals(listOf("read", "send"), a.capabilities)
        assertEquals(listOf("read"), a.effective)
        assertEquals("needs_session", st.items[1].status)
        assertEquals(listOf("notes"), st.catalogue.filter { it.service == "mcp" }.map { it.provider })
        assertFalse(st.catalogue.first { it.service == "browser" }.available)
    }

    @Test fun twoAccountsOfTheSameConnectorAreDistinguishable() {
        val st = parseConnexions(sample)
        val notes = st.items.filter { it.provider == "notes" }
        assertEquals(2, notes.size)
        assertTrue(notes[0].id != notes[1].id && notes[0].accountLabel != notes[1].accountLabel)
    }

    @Test fun actionsFollowTheRealStatus() {
        val st = parseConnexions(sample)
        assertEquals(listOf("check", "allowSend", "disconnect", "remove"), connexioActions(st.items[0]))
        assertEquals(listOf("reconnect", "check", "readOnly", "disconnect", "remove"), connexioActions(st.items[1]))
        // Desconnectat: no es comprova ni es desconnecta altra vegada.
        assertEquals(listOf("remove"), connexioActions(st.items[2]))
    }

    @Test fun permissionsSayWhatReallyHolds() {
        val st = parseConnexions(sample)
        assertEquals("Pot llegir", connexioPermsText("ca", st.items[0]))
        assertEquals("Can read, can act with your confirmation", connexioPermsText("en", st.items[1]))
        assertEquals("en pausa: no pot fer res", connexioPermsText("ca", st.items[0].copy(effective = emptyList())))
        assertEquals("Connector · notes", connexioServiceText("ca", st.items[0]))
        assertEquals("Correu (IMAP)", connexioServiceText("ca", st.items[2]))
    }

    @Test fun connectWithoutClientIdSaysSo() {
        assertTrue(connexioConnectError("ca", "HTTP 502: {\"code\":\"needs_client_id\"}").contains("administrador"))
        assertTrue(connexioConnectError("en", "timeout").startsWith("Could not do it"))
    }

    @Test fun copyExistsInBothLanguages() {
        val keys = listOf("title", "help", "open", "openHelp", "empty", "available", "connecting", "connected", "needs_session", "error",
            "disconnected", "notChecked", "checkedAt", "svc.mcp", "svc.calendar-ics", "svc.mail-imap", "svc.browser", "canRead", "canPrepare",
            "canSend", "paused", "check", "reconnect", "disconnect", "remove", "allowSend", "readOnly", "connectOf", "browserHelp", "noBrowser",
            "needsClientId", "opening", "done", "failed", "close", "waitTitle", "waitSub", "waitOpen", "cancel")
        for (k in keys) {
            val ca = connexioCopy("ca", k)
            val en = connexioCopy("en", k)
            assertTrue("falta $k", ca != k && en != k)
        }
    }

    // La mateixa forma que la fixture del contracte: el parse no hi perd res.
    @Test fun readsTheContractFixture() {
        val f = File("../../docs/assistant-contract/fixtures/connexio-caducada.json")
        if (!f.exists()) return
        val st = parseConnexions(JSONObject().put("connexions", org.json.JSONArray().put(JSONObject(f.readText()))))
        assertEquals("needs_session", st.items.single().status)
        assertTrue("reconnect" !in connexioActions(st.items.single())) // IMAP: es torna a connectar amb el formulari
    }

    @Test fun waitCardOpensConnectionsOrCancels() {
        var cancelled = false
        ui.setContent {
            MaterialTheme {
                ConnexioWaitCard(Conn("", "", ""), "ca",
                    GregalApi.WindAsk(key = "wa-1", type = "connection", name = "connector", kind = "connect", what = "notes", reason = "Per llegir les notes"),
                    Color(0xFF8193FF), onCancel = { cancelled = true })
            }
        }
        ui.onNodeWithText("Cal connectar un compte").assertIsDisplayed()
        ui.onNodeWithText("Per llegir les notes").assertIsDisplayed()
        ui.onNodeWithTag("connexio-wait-open").assertHasClickAction()
        ui.onNodeWithTag("connexio-wait-cancel").performClick()
        assertTrue(cancelled)
    }
}
