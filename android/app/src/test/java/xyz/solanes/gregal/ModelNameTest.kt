package xyz.solanes.gregal

import org.junit.Assert.assertEquals
import org.junit.Test

/** Els mateixos casos que la web (model-name.test.cjs). */
class ModelNameTest {
    @Test fun readableNames() {
        val cases = mapOf(
            "deepseek-v4.1-flash" to "DeepSeek V4.1 Flash",
            "cloud/deepseek-v4.1-flash" to "DeepSeek V4.1 Flash",
            "zen/deepseek-v4.1-flash" to "DeepSeek V4.1 Flash",
            "claude-sonnet-5-5" to "Claude Sonnet 5.5",
            "claude-opus-5-5" to "Claude Opus 5.5",
            "claude-3-5-sonnet-20241022" to "Claude 3.5 Sonnet",
            "gpt-5.1-codex" to "GPT-5.1 Codex",
            "gpt-4o-mini" to "GPT-4o Mini",
            "llama-3.3-70b-instruct" to "Llama 3.3 70B Instruct",
            "llama3.2:3b" to "Llama 3.2 3B",
            "qwen3-coder-30b-a3b" to "Qwen3 Coder 30B A3B",
            "gemini-2.5-pro" to "Gemini 2.5 Pro",
            "mistral-large-latest" to "Mistral Large",
            "o3-mini" to "o3 Mini",
            "kimi-k2-thinking" to "Kimi K2 Thinking",
            "accounts/fireworks/models/glm-4.6" to "GLM 4.6",
        )
        for ((id, want) in cases) assertEquals(id, want, prettyModelName(id))
        assertEquals("", prettyModelName(""))
        assertEquals("", prettyModelName(null))
    }
}
