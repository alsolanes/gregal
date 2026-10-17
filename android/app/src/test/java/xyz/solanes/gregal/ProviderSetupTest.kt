package xyz.solanes.gregal

import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test

class ProviderSetupTest {
    private val json = """
        {"providers":[{"name":"openai","key":"(buida)"}],
         "catalog":[
           {"name":"ollama","label":"Ollama","url":"http://localhost:11434/v1","local":true,"note":"cal tenir `ollama serve` en marxa","default_model":""},
           {"name":"openai","label":"OpenAI","url":"https://api.openai.com/v1","local":false,"note":"clau a platform.openai.com/api-keys","default_model":"gpt-4.1-mini"},
           {"name":"zen","label":"OpenCode Go","local":false,"note":"clau a opencode.ai/zen","default_model":"glm-5.3-flash"}
         ]}
    """.trimIndent()

    @Test
    fun parsesTheServerCatalog() {
        val c = parseCatalog(json)
        assertEquals(listOf("ollama", "openai", "zen"), c.map { it.name })
        assertTrue(c[0].local)
        assertEquals("OpenAI", c[1].label)
        assertEquals("gpt-4.1-mini", c[1].defaultModel)
    }

    @Test
    fun setupOffersOnlyCloudProviders() {
        // Ollama i companyia escolten a localhost de l'ordinador: des del mòbil no hi són.
        assertEquals(listOf("openai", "zen"), setupChoices(parseCatalog(json)).map { it.name })
    }

    @Test
    fun badCatalogIsEmptyNotACrash() {
        assertTrue(parseCatalog("").isEmpty())
        assertTrue(parseCatalog("{\"providers\":[]}").isEmpty())
        assertTrue(parseCatalog("not json").isEmpty())
    }

    @Test
    fun keyDomainComesFromTheNote() {
        assertEquals("platform.openai.com/api-keys", keyDomain("clau a platform.openai.com/api-keys"))
        assertEquals("", keyDomain("cal tenir el servidor en marxa"))
    }

    @Test
    fun localUrlsNeedNoKey() {
        assertTrue(isLocalUrl("http://localhost:11434/v1"))
        assertTrue(isLocalUrl("http://127.0.0.1:8080/v1"))
        assertFalse(isLocalUrl("https://api.openai.com/v1"))
    }

    @Test
    fun approvalArgsShowRealNewlines() {
        val p = prettyArgs("{\"path\":\"hello.txt\",\"content\":\"hi there\\nsecond line\"}", "en")
        assertEquals("path: hello.txt\ncontent: hi there\nsecond line", p)
    }

    @Test
    fun fontScaleIsCappedOnlyAboveTheLimit() {
        assertEquals(1.0f, cappedFontScale(1.0f, 1.3f))
        assertEquals(1.3f, cappedFontScale(2.0f, 1.3f))
    }
}
