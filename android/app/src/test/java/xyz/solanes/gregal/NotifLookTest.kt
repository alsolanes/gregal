package xyz.solanes.gregal

import android.graphics.Bitmap
import android.graphics.Canvas
import android.graphics.Color
import androidx.test.core.app.ApplicationProvider
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNotEquals
import org.junit.Assert.assertNotNull
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config
import org.robolectric.annotation.GraphicsMode
import java.io.File

@RunWith(RobolectricTestRunner::class)
@Config(sdk = [34])
@GraphicsMode(GraphicsMode.Mode.NATIVE)
class NotifLookTest {
    @Test fun eachNoticeLooksLikeWhatItCarries() {
        assertEquals(NotifKind.BRIEFING, NotifLook.of("job", "Resum del matí", "Bon dia! Avui tens dues reunions."))
        assertEquals(NotifKind.BRIEFING, NotifLook.of("job", "", "Tres coses per avui.", hour = 8))
        assertEquals(NotifKind.REMINDER, NotifLook.of("remind", "", "Truca al dentista"))
        assertEquals(NotifKind.WEATHER, NotifLook.of("job", "Previsió del temps", "Bon dia, avui plourà a la tarda"))
        assertEquals(NotifKind.SPORTS, NotifLook.of("heartbeat", "", "El Barça juga avui el partit de lliga a les 21h", hour = 15))
        assertEquals(NotifKind.MUSIC, NotifLook.of("heartbeat", "", "Hi ha un concert nou dels teus a Barcelona", hour = 15))
        assertEquals(NotifKind.DONE, NotifLook.of("feina:abc", "", "Feina feta"))
        assertEquals(NotifKind.FAILED, NotifLook.of("job", "Resum del matí", "", failed = true))
        assertEquals(NotifKind.SPARK, NotifLook.of("heartbeat", "", "M'he recordat d'una cosa que et pot agradar", hour = 15))
    }

    @Test fun badgesAreDrawnInTheirOwnColours() {
        val ctx = ApplicationProvider.getApplicationContext<android.content.Context>()
        val kinds = NotifKind.entries
        val px = 96
        val sheet = Bitmap.createBitmap(px * kinds.size + 8 * (kinds.size + 1), px + 16, Bitmap.Config.ARGB_8888)
        val canvas = Canvas(sheet)
        canvas.drawColor(Color.parseColor("#1B1D24"))
        kinds.forEachIndexed { i, kind ->
            val badge = NotifLook.badge(ctx, kind, px)
            assertNotNull("badge $kind", badge)
            // El glif blanc és al centre, el color de la mena a la vora.
            assertNotEquals(badge!!.getPixel(px / 2, px / 2 - px / 5), badge.getPixel(px / 2, px - 6))
            canvas.drawBitmap(badge, (8 + i * (px + 8)).toFloat(), 8f, null)
        }
        val out = File("build/notif-look/badges.png")
        out.parentFile?.mkdirs()
        out.outputStream().use { sheet.compress(Bitmap.CompressFormat.PNG, 100, it) }
    }
}
