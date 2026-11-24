package xyz.solanes.gregal

import android.app.Application
import androidx.test.core.app.ApplicationProvider
import org.junit.Assert.assertTrue
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner

/**
 * El vigilant en segon pla no s'empassa la primera novetat: el cursor
 * comença quan s'arma per a un servidor (en entrar-hi), no a la primera
 * passada, que abans marcava com a vistes les novetats ja pendents.
 */
@RunWith(RobolectricTestRunner::class)
class WindWatchCursorTest {
    private val ctx = ApplicationProvider.getApplicationContext<Application>()

    @Test fun `en entrar a un servidor el cursor del vigilant comenca ara`() {
        Prefs(ctx).save("127.0.0.1", "9", "t")
        val before = System.currentTimeMillis() / 1000
        WindWatch.schedule(ctx)
        val ts = WindWatch.lastTs(ctx)
        assertTrue("cursor $ts, abans $before", ts in (before - 1)..(before + 2))
    }

    @Test fun `armar-lo de nou no fa enrere ni endavant un cursor que ja hi era`() {
        Prefs(ctx).save("127.0.0.1", "9", "t")
        WindWatch.saveTs(ctx, 1_000L)
        WindWatch.schedule(ctx)
        assertTrue(WindWatch.lastTs(ctx) == 1_000L)
    }
}
