package xyz.solanes.gregal

import org.junit.Assert.assertEquals
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner

@RunWith(RobolectricTestRunner::class)
/** Un avís programat al servidor ha d'arribar a l'hora, no a la propera passada de 15 minuts. */
class WindWatchDueTest {
    private val now = 1_000_000_000L

    @Test
    fun withoutRemindersTheNormalIntervalStands() {
        assertEquals(WindWatch.INTERVAL_MS, WindWatch.nextCheckDelay(WindWatch.INTERVAL_MS, emptyList(), now))
    }

    @Test
    fun aReminderInTwoMinutesBringsTheCheckForward() {
        val due = now + 2 * 60_000L
        assertEquals(2 * 60_000L + WindWatch.DUE_GRACE_MS, WindWatch.nextCheckDelay(WindWatch.INTERVAL_MS, listOf(due), now))
    }

    @Test
    fun aFarReminderDoesNotDelayTheRegularCheck() {
        val due = now + 60 * 60_000L
        assertEquals(WindWatch.INTERVAL_MS, WindWatch.nextCheckDelay(WindWatch.INTERVAL_MS, listOf(due), now))
    }

    @Test
    fun passedRemindersAreIgnoredAndTheCheckNeverFiresInstantly() {
        assertEquals(WindWatch.INTERVAL_MS, WindWatch.nextCheckDelay(WindWatch.INTERVAL_MS, listOf(now - 120_000L), now))
        // Just a l'hora: espera el marge, però mai menys de 5 s.
        assertEquals(5_000L, WindWatch.nextCheckDelay(WindWatch.INTERVAL_MS, listOf(now - 28_000L), now))
    }

    @Test
    fun theReplyCarriesTheExactReminderTime() {
        val api = GregalApi("http://x", "")
        val m = GregalApi::class.java.getDeclaredMethod("windReplyOf", org.json.JSONObject::class.java).apply { isAccessible = true }
        val o = org.json.JSONObject("""{"reply":"Fet","actions":[{"kind":"remind","at":"02/11 16:53","at_iso":"2026-11-02T16:53:27+01:00","text":"Apaga el foc"}]}""")
        val r = m.invoke(api, o) as WindReply
        assertEquals(listOf("02/11 16:53"), r.reminds)
        assertEquals(listOf(java.time.OffsetDateTime.parse("2026-11-02T16:53:27+01:00").toInstant().toEpochMilli()), r.remindAt)
    }
}
