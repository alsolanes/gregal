package xyz.solanes.gregal

import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Surface
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.ui.Modifier
import androidx.compose.ui.test.*
import androidx.compose.ui.test.junit4.createComposeRule
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config
import org.robolectric.annotation.GraphicsMode
import org.robolectric.annotation.LooperMode

@RunWith(RobolectricTestRunner::class)
@Config(sdk = [34], qualifiers = "w393dp-h851dp")
@GraphicsMode(GraphicsMode.Mode.NATIVE)
@LooperMode(LooperMode.Mode.PAUSED)
class CompanyNavigationTest {
    @get:Rule val ui = createComposeRule()

    @Test
    fun firstCompletionKeepsServiceForOtherRunningWork() {
        val context = androidx.test.core.app.ApplicationProvider.getApplicationContext<android.content.Context>()
        AppVisibility.foreground = true
        try {
            GregalNotifications.workStarted(context)
            GregalNotifications.workStarted(context)
            org.junit.Assert.assertTrue(GregalNotifications.serviceForegrounded())
            GregalNotifications.workFinished(context, false)
            org.junit.Assert.assertTrue("second work still needs foreground service", GregalNotifications.serviceForegrounded())
            GregalNotifications.workFinished(context, false)
            org.junit.Assert.assertFalse(GregalNotifications.serviceForegrounded())
        } finally {
            GregalNotifications.workFinished(context, false)
            GregalNotifications.workFinished(context, false)
            GregalNotifications.serviceGone()
            AppVisibility.foreground = false
        }
    }

    private fun notificationContext(): android.content.Context {
        val context = androidx.test.core.app.ApplicationProvider.getApplicationContext<android.content.Context>()
        org.robolectric.Shadows.shadowOf(context as android.app.Application)
            .grantPermissions(android.Manifest.permission.POST_NOTIFICATIONS)
        Prefs(context).idioma = "ca"
        AppVisibility.foreground = false
        return context
    }

    private fun complete(context: android.content.Context, target: String, partial: Boolean = false, error: String = "") {
        GregalNotifications.workFinished(context, error.isEmpty(), "question", "private answer", target, "Private wind", partial, error)
    }

    @Test
    fun windAndJobNoticesAreIsolatedBySourceAndLockscreenIsGeneric() {
        val context = notificationContext()
        val prefs = Prefs(context)
        prefs.save("https://winds.example", "443", "fake-token", "test-user")
        val voice = WindNovetat(10, "Aa", "Private wind", "remind", "private message")
        val job = JobItem(id = "same-job", name = "Private job")
        val run = RunItem(id = "same-run", jobId = "same-job", jobName = "Private job", startedAt = "", finishedAt = "", status = "ok", output = "private result", error = "")
        for (mode in listOf(Prefs.TARGET_LOCAL, Prefs.TARGET_REMOTE)) {
            prefs.target = mode
            WindNotifications.veu(context, voice)
            JobNotifications.result(context, job, run)
        }
        val notices = org.robolectric.Shadows.shadowOf(context.getSystemService(android.app.NotificationManager::class.java)).allNotifications
        org.junit.Assert.assertEquals(4, notices.size)
        org.junit.Assert.assertEquals(4, notices.map { org.robolectric.Shadows.shadowOf(it.contentIntent).savedIntent.dataString }.toSet().size)
        notices.forEach {
            org.junit.Assert.assertEquals("Gregal", it.publicVersion.extras.getCharSequence(android.app.Notification.EXTRA_TITLE).toString())
            org.junit.Assert.assertFalse(it.publicVersion.extras.toString().contains("private", ignoreCase = true))
        }
    }

    @Test
    fun remoteCursorSurvivesLocalModeButNeverCrossesSavedPairings() {
        val context = notificationContext()
        val prefs = Prefs(context)
        prefs.save("https://a.example", "443", "fake-a")
        prefs.target = Prefs.TARGET_REMOTE
        WindWatch.saveTs(context, 100)
        prefs.target = Prefs.TARGET_LOCAL
        org.junit.Assert.assertTrue("saved remote watcher stays active while using local", WindWatch.wanted(context))
        org.junit.Assert.assertEquals(100L, WindWatch.lastTs(context))
        prefs.save("https://b.example", "443", "fake-b")
        org.junit.Assert.assertEquals("new server must not inherit the old high watermark", 0L, WindWatch.lastTs(context))
        WindWatch.saveTs(context, 3)
        prefs.save("https://a.example", "443", "fake-a")
        org.junit.Assert.assertEquals(100L, WindWatch.lastTs(context))
    }

    @Test
    fun sameConversationIdOnLocalAndServerKeepsSeparateNotificationsAndIntents() {
        val context = notificationContext()
        val prefs = Prefs(context)
        prefs.save("https://winds.example", "443", "not-a-real-token", "test-user")
        prefs.target = Prefs.TARGET_LOCAL
        complete(context, "wind:shared")
        prefs.target = Prefs.TARGET_REMOTE
        complete(context, "wind:shared")
        val notifications = org.robolectric.Shadows.shadowOf(context.getSystemService(android.app.NotificationManager::class.java)).allNotifications
        org.junit.Assert.assertEquals("same target on different instances must not overwrite", 2, notifications.size)
        val intents = notifications.map { org.robolectric.Shadows.shadowOf(it.contentIntent).savedIntent }
        org.junit.Assert.assertEquals(2, intents.map { it.dataString }.toSet().size)
        org.junit.Assert.assertTrue(notifications.any { it.extras.getCharSequence(android.app.Notification.EXTRA_TITLE).toString().contains("Local") })
        org.junit.Assert.assertTrue(notifications.any { it.extras.getCharSequence(android.app.Notification.EXTRA_TITLE).toString().contains("Servidor") })
    }

    @Test
    fun completedWindAndRotllanaOpenTheirOwnConversationWithoutOverwritingEachOther() {
        val context = notificationContext()
        complete(context, "wind:xaloc")
        complete(context, "rot:project")
        val notifications = org.robolectric.Shadows.shadowOf(context.getSystemService(android.app.NotificationManager::class.java)).allNotifications
        org.junit.Assert.assertEquals(2, notifications.size)
        org.junit.Assert.assertTrue("a received chat reply is not proof that a background task finished", notifications.all {
            it.extras.getCharSequence(android.app.Notification.EXTRA_TITLE).toString().contains("Resposta rebuda")
        })
        val intents = notifications.map { org.robolectric.Shadows.shadowOf(it.contentIntent).savedIntent }
        org.junit.Assert.assertTrue(intents.any { it.getStringExtra(WindNotifications.EXTRA_WIND) == "xaloc" })
        org.junit.Assert.assertTrue(intents.any { it.getStringExtra(WindNotifications.EXTRA_ROT) == "project" })
        org.junit.Assert.assertTrue(intents.all { !it.hasExtra(GregalNotifications.EXTRA_SCREEN) })
    }

    @Test
    fun partialAndFailedNotificationsStateOutcomeWithoutLeakingProjectOnLockscreen() {
        val context = notificationContext()
        complete(context, "wind:partial", partial = true)
        complete(context, "rot:failed", error = "private backend error")
        val notifications = org.robolectric.Shadows.shadowOf(context.getSystemService(android.app.NotificationManager::class.java)).allNotifications
        val titles = notifications.map { it.extras.getCharSequence(android.app.Notification.EXTRA_TITLE).toString() }
        org.junit.Assert.assertTrue(titles.any { it.contains("Resposta parcial") })
        org.junit.Assert.assertTrue(titles.any { it.contains("No s'ha pogut acabar") })
        for (n in notifications) {
            org.junit.Assert.assertEquals(android.app.Notification.VISIBILITY_PRIVATE, n.visibility)
            org.junit.Assert.assertNotNull(n.publicVersion)
            val redacted = n.publicVersion.extras.toString()
            org.junit.Assert.assertFalse(redacted.contains("Private wind"))
            org.junit.Assert.assertFalse(redacted.contains("private answer"))
            org.junit.Assert.assertFalse(redacted.contains("private backend error"))
        }
    }

    @Test
    fun workProgressIsCompactSilentTargetedAndIgnoresRepeatedOrLateStages() {
        val context = notificationContext()
        try {
            GregalNotifications.workStarted(context, "wind:xaloc", "Xaloc")
            GregalNotifications.serviceForegrounded()
            GregalNotifications.workUpdated(context, "wind:xaloc", "Cercant fonts")
            val manager = org.robolectric.Shadows.shadowOf(context.getSystemService(android.app.NotificationManager::class.java))
            val n = manager.getNotification(GregalNotifications.WORK_ID)
            org.junit.Assert.assertEquals("Cercant fonts", n.extras.getCharSequence(android.app.Notification.EXTRA_TEXT).toString())
            org.junit.Assert.assertEquals("xaloc", org.robolectric.Shadows.shadowOf(n.contentIntent).savedIntent.getStringExtra(WindNotifications.EXTRA_WIND))
            org.junit.Assert.assertTrue(n.flags and android.app.Notification.FLAG_ONLY_ALERT_ONCE != 0)
            org.junit.Assert.assertEquals(android.app.Notification.GROUP_ALERT_SUMMARY, n.groupAlertBehavior)
            org.junit.Assert.assertNotNull(n.publicVersion)
            GregalNotifications.workUpdated(context, "wind:xaloc", "Cercant fonts")
            org.junit.Assert.assertSame("identical stage is not reposted", n, manager.getNotification(GregalNotifications.WORK_ID))
            AppVisibility.foreground = true
            complete(context, "wind:xaloc")
            context.getSystemService(android.app.NotificationManager::class.java).cancelAll()
            GregalNotifications.workUpdated(context, "wind:xaloc", "late stage")
            org.junit.Assert.assertTrue(manager.allNotifications.isEmpty())
        } finally {
            AppVisibility.foreground = true
            complete(context, "wind:xaloc")
            GregalNotifications.serviceGone()
            AppVisibility.foreground = false
        }
    }

    @Test
    fun hashCollisionsDoNotMergeNotificationOrPendingIntentTargets() {
        val context = notificationContext()
        org.junit.Assert.assertEquals("wind:Aa".hashCode(), "wind:BB".hashCode())
        complete(context, "wind:Aa")
        complete(context, "wind:BB")
        val notifications = org.robolectric.Shadows.shadowOf(context.getSystemService(android.app.NotificationManager::class.java)).allNotifications
        org.junit.Assert.assertEquals(2, notifications.size)
        val targets = notifications.map { org.robolectric.Shadows.shadowOf(it.contentIntent).savedIntent.getStringExtra(WindNotifications.EXTRA_WIND) }.toSet()
        org.junit.Assert.assertEquals(setOf("Aa", "BB"), targets)
    }

    @Test
    fun targetedCompletionCannotStopAnotherTargetAndRemainingStageBecomesVisible() {
        val context = notificationContext()
        AppVisibility.foreground = true
        try {
            GregalNotifications.workStarted(context, "wind:a", "A")
            GregalNotifications.workStarted(context, "rot:b", "B")
            GregalNotifications.serviceForegrounded()
            GregalNotifications.workUpdated(context, "rot:b", "Revisant")
            complete(context, "wind:a")
            complete(context, "wind:a") // Late duplicate must not consume b's reference.
            org.junit.Assert.assertTrue(GregalNotifications.serviceForegrounded())
            val n = GregalNotifications.working(context)
            org.junit.Assert.assertEquals("Revisant", n.extras.getCharSequence(android.app.Notification.EXTRA_TEXT).toString())
            org.junit.Assert.assertEquals("b", org.robolectric.Shadows.shadowOf(n.contentIntent).savedIntent.getStringExtra(WindNotifications.EXTRA_ROT))
            complete(context, "rot:b")
            org.junit.Assert.assertFalse(GregalNotifications.serviceForegrounded())
        } finally {
            complete(context, "wind:a")
            complete(context, "rot:b")
            GregalNotifications.serviceGone()
            AppVisibility.foreground = false
        }
    }

    @Test
    fun foregroundOrDeniedPermissionSuppressesFinishedPartialAndFailedAlerts() {
        val context = notificationContext()
        val manager = org.robolectric.Shadows.shadowOf(context.getSystemService(android.app.NotificationManager::class.java))
        AppVisibility.foreground = true
        complete(context, "wind:ok")
        complete(context, "wind:partial", partial = true)
        complete(context, "wind:failed", error = "boom")
        org.junit.Assert.assertTrue(manager.allNotifications.isEmpty())
        AppVisibility.foreground = false
        org.robolectric.Shadows.shadowOf(context as android.app.Application).denyPermissions(android.Manifest.permission.POST_NOTIFICATIONS)
        complete(context, "wind:ok")
        complete(context, "wind:partial", partial = true)
        complete(context, "wind:failed", error = "boom")
        org.junit.Assert.assertTrue(manager.allNotifications.isEmpty())
    }

    @Test
    fun openingPetChatReplacesCompanyScreenInsteadOfOverlayingIt() {
        val pet = Pet("mel", "Mel", "música")
        ui.setContent {
            CompositionLocalProvider(LocalPal provides LightPal) {
                MaterialTheme(colorScheme = LightPal.scheme()) {
                    Surface(Modifier.fillMaxSize()) {
                        CompanyRoute(
                            openPet = pet,
                            companyContent = { androidx.compose.material3.Text("company home") },
                            petChatContent = { androidx.compose.material3.Text("chat with ${it.name}") },
                        )
                    }
                }
            }
        }

        ui.onNodeWithText("chat with Mel").assertExists()
        ui.onNodeWithText("company home").assertDoesNotExist()
    }

    @Test
    fun noSelectedPetShowsOnlyCompanyScreen() {
        ui.setContent {
            CompositionLocalProvider(LocalPal provides LightPal) {
                MaterialTheme(colorScheme = LightPal.scheme()) {
                    Surface(Modifier.fillMaxSize()) {
                        CompanyRoute(
                            openPet = null,
                            companyContent = { androidx.compose.material3.Text("company home") },
                            petChatContent = { androidx.compose.material3.Text("chat with ${it.name}") },
                        )
                    }
                }
            }
        }

        ui.onNodeWithText("company home").assertExists()
        ui.onNodeWithText("chat with Mel").assertDoesNotExist()
    }

    @Test
    fun companyHomeShowsWindChatCard() {
        // La cara del vent i el seu xat són el centre de la pantalla Company.
        ui.setContent {
            CompositionLocalProvider(LocalPal provides LightPal) {
                MaterialTheme(colorScheme = LightPal.scheme()) {
                    Surface(Modifier.fillMaxSize()) {
                        CompanyRoute(
                            openPet = null,
                            companyContent = {
                                androidx.compose.material3.Text(tr("ca", "app.company.windEmptyTitle"))
                                androidx.compose.material3.Text(tr("ca", "app.company.windNew"))
                            },
                            petChatContent = { androidx.compose.material3.Text("chat with ${it.name}") },
                        )
                    }
                }
            }
        }

        ui.onNodeWithText("Cap vent encara").assertExists()
        ui.onNodeWithText("Nou vent").assertExists()
    }
}
