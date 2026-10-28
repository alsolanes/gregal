package xyz.solanes.gregal

import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test

/**
 * Les tres animacions de la marea i la línia d'horitzó són espec (port
 * literal de `internal/tui/anim.go` i `theme.go`): si aquests tests fallen,
 * el mòbil ha deixat de parlar el mateix idioma que el TUI i la web.
 */
class MareaTest {

    @Test
    fun onadaTeCrestaAlCentre() {
        // frame 0 amb marge 6 → centre a -6, fora de la línia: mar plana.
        assertEquals(" ".repeat(10), tideLine(10, 0))
        // El centre és frame%period - marge: amb frame = marge+10 el cim cau
        // just a la columna 10 i la cresta s'obre ≋≈~· cap als costats.
        val l = tideLine(20, tideMargin + 10)
        assertEquals(20, l.length)
        assertEquals('≋', l[10])
        assertEquals('≈', l[9])
        assertEquals('~', l[8])
        assertEquals('·', l[7])
        assertEquals('·', l[13])
        assertEquals(' ', l[0])
    }

    @Test
    fun onadaTeMargeAbansDeTornar() {
        // El període és amplada + 2*marge. Amb frame 2 el cim encara és a
        // -4 (fora): tota la línia plana. Al frame següent el primer glif
        // de la cresta (·) ja entra per l'esquerra.
        val period = 20 + 2 * tideMargin
        assertEquals(32, period)
        assertEquals(" ".repeat(20), tideLine(20, 2))
        assertEquals('·', tideLine(20, 3)[0])
        assertEquals('≋', tideLine(20, 6)[0])
    }

    @Test
    fun ampladaZeroNoPeta() {
        assertEquals("", tideLine(0, 3))
        assertEquals("", tideGauge(50, 0, 0))
        assertEquals("", waveLineChars(3))
    }

    @Test
    fun mesuradorAmbVuitens() {
        // 0% → tot buit; 100% → tot ple.
        assertEquals("·".repeat(8), tideGauge(0, 8, 0))
        assertEquals("█".repeat(8), tideGauge(100, 8, 0))
        // 50% → 4 plens i 4 buits (frame parell, sense batec).
        assertEquals("████····", tideGauge(50, 8, 0))
        // Vuitens: 53% de 8 cel·les = 33/64 → 4 plenes i el primer octau (▏).
        assertEquals("████▏···", tideGauge(53, 8, 0))
        // 56% = 35/64 → 4 plenes i tres octaus (▍).
        assertEquals("████▍···", tideGauge(56, 8, 0))
    }

    @Test
    fun mesuradorBategaAl80() {
        // Per sobre del 80% l'última cel·la plena alterna █/▓ (frame senar).
        val parell = tideGauge(90, 8, 0)
        val senar = tideGauge(90, 8, 1)
        assertTrue(parell.startsWith("███████"))
        assertEquals('▓', senar[6])
        // I per sota del 80% no batega mai.
        assertEquals(tideGauge(70, 8, 0), tideGauge(70, 8, 1))
    }

    @Test
    fun mesuradorEnganxaElsLimits() {
        assertEquals(tideGauge(0, 8, 0), tideGauge(-40, 8, 0))
        assertEquals(tideGauge(100, 8, 0), tideGauge(300, 8, 0))
    }

    @Test
    fun liniaHoritzoEsmoreixCapAlsExtrems() {
        val l = waveLineChars(21)
        assertEquals(21, l.length)
        // Cim al centre, calma a les vores.
        assertEquals('≋', l[10])
        assertEquals(' ', l[0])
        assertEquals(' ', l[20])
        // I els passos intermedis hi són.
        assertTrue(l.contains('≈'))
        assertTrue(l.contains('~'))
    }

    @Test
    fun rellotgeDelTornEnCurt() {
        assertEquals("0s", elapsedShort(0))
        assertEquals("7s", elapsedShort(7_400))
        assertEquals("59s", elapsedShort(59_999))
        assertEquals("1m00s", elapsedShort(60_000))
        assertEquals("2m04s", elapsedShort(124_000))
        assertEquals("1h05m", elapsedShort(3_900_000))
        // Un rellotge cap enrere no pot ensenyar un número negatiu.
        assertEquals("0s", elapsedShort(-500))
    }

    @Test
    fun percentatgeDeContext() {
        assertEquals(0, contextPct(500, 0))       // rol sense finestra
        assertEquals(0, contextPct(0, 32_000))
        assertEquals(50, contextPct(16_000, 32_000))
        assertEquals(100, contextPct(99_999, 32_000)) // no passa del 100
        assertEquals(20, contextPct(6_400, 32_000))
    }

    @Test
    fun crestaQueLliscaTePausa() {
        // Una cresta de 3 columnes com a molt en un frame.
        for (f in 0 until 40) {
            val ranges = shimmerRanges(12, f)
            val total = ranges.sumOf { it.last - it.first + 1 }
            assertTrue("frame $f: $ranges", total <= 3)
        }
        // I amb la pausa, hi ha frames sense cresta (no és un estrobo).
        assertTrue((0 until 12 + 10).any { shimmerRanges(12, it).isEmpty() })
        // Alguna vegada hi és.
        assertTrue((0 until 12 + 10).any { shimmerRanges(12, it).isNotEmpty() })
    }

    @Test
    fun crestaButiraNoPeta() {
        assertTrue(shimmerRanges(0, 0).isEmpty())
    }
}

/** El manifest OTA i les defenses de l'actualitzador (portat de Marea). */
class OtaTest {

    @Test
    fun llegeixElManifest() {
        val json = """
            {"versionCode":30,"versionName":"0.9.1",
             "downloadUrl":"https://releases.example.org/gregal/dist/gregal-android-v0.9.1.apk",
             "changelog":"paleta + marea","sizeBytes":10372206,
             "sha256":"8fc6540d40427a6fc1e7478c7035dde1e81ab5acd03608ce43ca288429437d3e"}
        """.trimIndent()
        val u = parseUpdateText(json)
        assertEquals(30, u.versionCode)
        assertEquals("0.9.1", u.versionName)
        assertTrue(u.downloadUrl.endsWith("gregal-android-v0.9.1.apk"))
        assertEquals("paleta + marea", u.changelog)
        assertEquals(10372206L, u.midaDeclarada())
        assertTrue(manifestPlausible(u))
    }

    @Test
    fun nomeSEnsenyaSiEsMesNou() {
        val u = parseUpdateText("""{"versionCode":30,"versionName":"0.9.1"}""")
        assertTrue(u.hiHaNova(29))
        assertFalse(u.hiHaNova(30))
        // Un manifest antic (o el mateix) no pot oferir "actualització".
        assertFalse(u.hiHaNova(31))
    }

    @Test
    fun manifestIncompletNoPeta() {
        val u = parseUpdateText("{}")
        assertEquals(0, u.versionCode)
        assertFalse(u.hiHaNova(1))
        assertEquals("", u.downloadUrl)
    }

    @Test
    fun manifestAmbAccentsICometes() {
        val u = parseUpdateText("""{"changelog":"Nova versió \"marea\" — àgil"}""")
        assertEquals("Nova versió \"marea\" — àgil", u.changelog)
    }

    @Test
    fun aliasSizeITambeAcceptat() {
        // Alguns desplegaments anomenen el camp `size`: s'ha d'entendre igual.
        val u = parseUpdateText("""{"size":1234}""")
        assertEquals(1234L, u.midaDeclarada())
    }

    private val manifestAmbAbis = """
        {"versionCode":42,"versionName":"0.10.1",
         "apks":{
           "arm64-v8a":{"downloadUrl":"https://h.example/Gregal-Android-0.10.1-arm64-v8a.apk","sha256":"${"a".repeat(64)}","sizeBytes":28000000},
           "x86_64":{"downloadUrl":"https://h.example/Gregal-Android-0.10.1-x86_64.apk","sha256":"${"b".repeat(64)}","sizeBytes":29000000}
         },
         "downloadUrl":"https://h.example/Gregal-Android-0.10.1.apk","sha256":"${"c".repeat(64)}","sizeBytes":42000000,
         "changelog":"x"}
    """.trimIndent()

    @Test
    fun triaLApkDeLAbiDelMobil() {
        // Un mòbil arm64 (amb armeabi de compatibilitat) baixa el seu APK.
        val arm = parseUpdateText(manifestAmbAbis, listOf("arm64-v8a", "armeabi-v7a", "armeabi"))
        assertEquals("https://h.example/Gregal-Android-0.10.1-arm64-v8a.apk", arm.downloadUrl)
        assertEquals("a".repeat(64), arm.sha256)
        assertEquals(28000000L, arm.midaDeclarada())
        assertEquals(42, arm.versionCode)
        assertEquals("x", arm.changelog)
        // L'emulador, el seu.
        val x86 = parseUpdateText(manifestAmbAbis, listOf("x86_64", "x86"))
        assertEquals("https://h.example/Gregal-Android-0.10.1-x86_64.apk", x86.downloadUrl)
        assertEquals(29000000L, x86.midaDeclarada())
    }

    @Test
    fun sensApkPerAquestaAbiBaixaLUniversal() {
        // ABI que no publiquem, o cap ABI coneguda: l'universal de dalt.
        for (abis in listOf(listOf("armeabi-v7a"), emptyList())) {
            val u = parseUpdateText(manifestAmbAbis, abis)
            assertEquals("https://h.example/Gregal-Android-0.10.1.apk", u.downloadUrl)
            assertEquals("c".repeat(64), u.sha256)
            assertEquals(42000000L, u.midaDeclarada())
        }
        // Manifest antic (sense `apks`): igual que abans.
        val vell = parseUpdateText("""{"versionCode":40,"downloadUrl":"https://h.example/a.apk"}""", listOf("arm64-v8a"))
        assertEquals("https://h.example/a.apk", vell.downloadUrl)
    }

    @Test
    fun laCacheTambeTriaPerAbi() {
        val dir = createTempDir()
        try {
            desaCache(dir, manifestAmbAbis, 1000L)
            java.io.File(dir, "gregal-update.json").setLastModified(System.currentTimeMillis())
            val c = llegeixCache(dir, System.currentTimeMillis(), listOf("x86_64"))
            assertEquals("https://h.example/Gregal-Android-0.10.1-x86_64.apk", c?.downloadUrl)
        } finally {
            dir.deleteRecursively()
        }
    }

    @Test
    fun nomesEsDescarregaDelMateixHost() {
        val manifest = "https://releases.example.org/gregal/dist/gregal-android-version.json"
        assertTrue(isTrustedDownloadUrl(manifest, "https://releases.example.org/gregal/dist/x.apk"))
        // Altre host = no.
        assertFalse(isTrustedDownloadUrl(manifest, "https://malicioso.example/x.apk"))
        // Sense https = no.
        assertFalse(isTrustedDownloadUrl(manifest, "http://releases.example.org/x.apk"))
        assertFalse(isTrustedDownloadUrl("http://releases.example.org/v.json", "http://releases.example.org/x.apk"))
        // Brossa = no peta.
        assertFalse(isTrustedDownloadUrl(manifest, "no-és-una-url"))
    }

    @Test
    fun manifestImplausibleEsRebutja() {
        // sha256 que no és de 64 hexos → no ens refiem del fitxer.
        val shaDolent = parseUpdateText("""{"sha256":"deadbeef"}""")
        assertFalse(manifestPlausible(shaDolent))
        // Mida absurda (més gran que el límit dur) → fora.
        assertFalse(manifestPlausible(parseUpdateText("""{"sizeBytes":999999999999}""")))
        assertFalse(manifestPlausible(parseUpdateText("""{"sizeBytes":-5}""")))
        // Sense mida ni hash, no hi ha res a comprovar: passa (i el
        // descarregador no podrà verificar, que és pitjor però no fatal).
        assertTrue(manifestPlausible(parseUpdateText("""{"versionCode":1}""")))
    }

    @Test
    fun versioSilenciadaNoTornaAAvisar() {
        // S'amaga quan la disponible és <= la silenciada — mai comparant amb
        // la instal·lada, o instal·lar-la a mà apagaria tots els avisos.
        assertTrue(avisAmagat(latest = 30, skipped = 30))
        assertTrue(avisAmagat(latest = 29, skipped = 30))
        assertFalse(avisAmagat(latest = 31, skipped = 30))
        assertFalse(avisAmagat(latest = 30, skipped = 0))
    }

    @Test
    fun cacheDelManifestCaduca() {
        val dir = java.io.File(System.getProperty("java.io.tmpdir"), "gregal-test-cache").apply {
            deleteRecursively(); mkdirs()
        }
        try {
            val json = """{"versionCode":30,"versionName":"0.9.1"}"""
            val ara = System.currentTimeMillis()
            desaCache(dir, json, ara)
            val fresca = llegeixCache(dir, ara)
            assertEquals(30, fresca?.versionCode)
            assertEquals("0.9.1", fresca?.versionName)
            // Set hores i un minut després, la caché ja no val.
            assertNull(llegeixCache(dir, ara + 6 * 60 * 60 * 1000L + 60_000L))
            netejaCache(dir)
            assertNull(llegeixCache(dir, ara))
        } finally {
            dir.deleteRecursively()
        }
    }

    @Test
    fun ventsSActualitzaDesDeGitHubIlaNormalNo() {
        val vents = androidOtaUrl("xyz.solanes.gregal.vents")
        assertTrue(vents.startsWith("https://github.com/alsolanes/gregal/releases/latest/download/"))
        assertEquals(ANDROID_OTA_URL, androidOtaUrl("xyz.solanes.gregal"))
        // El manifest i l'APK de la release viuen al mateix host.
        assertTrue(isTrustedDownloadUrl(vents, "https://github.com/alsolanes/gregal/releases/download/v1.23.0/Gregal-Vents-1.35.0.apk"))
    }
}
