package xyz.solanes.gregal

import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test

class WindImageUrlTest {
    @Test
    fun recognisesOnlyWindImagePaths() {
        assertTrue(isWindImageUrl("/api/winds/image?id=0123456789abcdef01234567.jpg"))
        assertTrue(isWindImageUrl("/api/winds/image?id=0123456789abcdef01234567.png"))
        assertFalse(isWindImageUrl("https://example.com/a.jpg"))
        assertFalse(isWindImageUrl("/api/winds/image?id=../../secret.jpg"))
        assertFalse(isWindImageUrl("/api/winds/doc?name=a.pdf"))
    }

    @Test
    fun parsedAsAnImageBlock() {
        val blocks = richBlocks("Mira-ho:\n\n![Cartell](/api/winds/image?id=0123456789abcdef01234567.jpg)")
        assertTrue(blocks.filterIsInstance<Rich.Image>().any { isWindImageUrl(it.url) })
    }
}
