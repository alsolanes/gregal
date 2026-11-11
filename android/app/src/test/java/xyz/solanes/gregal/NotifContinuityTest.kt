package xyz.solanes.gregal

import android.content.Intent
import androidx.test.core.app.ApplicationProvider
import org.junit.Assert.*
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner

/** B7: política de permís, deduplicació push + sondeig, obrir per message_id i stream. */
@RunWith(RobolectricTestRunner::class)
class NotifContinuityTest {
    private fun n(id: String, msg: String = "", ts: Long = 100, wind: String = "xaloc", owner: String = "", read: Boolean = false) =
        WindNovetat(ts, wind, "Xaloc", "remind", "text", messageId = msg, id = id, owner = owner, read = read)

    // ---- política del permís ----

    @Test fun `mai a l'arrencada ni sense motiu`() {
        assertFalse(NotifPermissionPolicy.shouldAsk(34, granted = false, reason = null, state = NotifAskState()))
    }

    @Test fun `una vegada en context`() {
        assertTrue(NotifPermissionPolicy.shouldAsk(34, false, NotifAskReason.REMINDER, NotifAskState()))
        val after = NotifPermissionPolicy.afterResult(NotifAskState(), granted = true)
        // Concedit i després revocat des del sistema: no es torna a demanar sol.
        assertFalse(NotifPermissionPolicy.shouldAsk(34, false, NotifAskReason.REMINDER, after))
    }

    @Test fun `denegat es durable entre arrencades`() {
        val denied = NotifPermissionPolicy.afterResult(NotifAskState(), granted = false)
        assertTrue(denied.denied)
        for (reason in listOf(null, NotifAskReason.REMINDER, NotifAskReason.WATCH))
            assertFalse(NotifPermissionPolicy.shouldAsk(34, false, reason, denied))
        // Només un botó explícit de l'usuari el torna a intentar.
        assertTrue(NotifPermissionPolicy.shouldAsk(34, false, NotifAskReason.USER, denied))
    }

    @Test fun `concedit o Android antic no demana res`() {
        assertFalse(NotifPermissionPolicy.shouldAsk(34, granted = true, reason = NotifAskReason.REMINDER, state = NotifAskState()))
        assertFalse(NotifPermissionPolicy.shouldAsk(32, granted = false, reason = NotifAskReason.REMINDER, state = NotifAskState()))
    }

    @Test fun `la denegacio es desa a preferencies i sobreviu`() {
        val ctx = ApplicationProvider.getApplicationContext<android.content.Context>()
        NotifPermission.save(ctx, NotifPermissionPolicy.afterResult(NotifPermission.state(ctx), granted = false))
        assertEquals(NotifAskState(asked = 1, denied = true), NotifPermission.state(ctx))
        // Amb l'app en segon pla mai no s'obre res, sigui quin sigui el motiu.
        AppVisibility.foreground = false
        assertFalse(NotifPermission.requestFor(ctx, NotifAskReason.REMINDER))
    }

    // ---- deduplicació ----

    @Test fun `push i despres sondeig no avisen dues vegades`() {
        val ctx = ApplicationProvider.getApplicationContext<android.content.Context>()
        val push = NoticeLedger.claim(ctx, "o1", listOf(n("n-aaaa1", "m-1")), me = "aleix")
        assertEquals(1, push.size)
        val poll = NoticeLedger.claim(ctx, "o1", listOf(n("n-aaaa1", "m-1"), n("n-bbbb2", "m-2", ts = 200)), me = "aleix")
        assertEquals(listOf("n-bbbb2"), poll.map { it.id })
        // Un altre origen (servidor/compte) té el seu propi registre.
        assertEquals(1, NoticeLedger.claim(ctx, "o2", listOf(n("n-aaaa1", "m-1")), me = "").size)
    }

    @Test fun `vista dins l'app no sona despres`() {
        val ctx = ApplicationProvider.getApplicationContext<android.content.Context>()
        NoticeLedger.mark(ctx, "o3", listOf("n-cccc3"))
        assertTrue(NoticeLedger.claim(ctx, "o3", listOf(n("n-cccc3")), me = "").isEmpty())
    }

    @Test fun `mateix missatge amb altre id tampoc`() {
        val announced = NoticeDedupe.record(emptyMap(), listOf(n("", "m-7")), now = 1000)
        assertTrue(NoticeDedupe.announceable(listOf(n("n-dddd4", "m-7")), announced, "").isEmpty())
    }

    @Test fun `llegides i d'altres comptes no s'anuncien`() {
        val list = listOf(n("n-1aaa", owner = "carla"), n("n-2aaa", read = true), n("n-3aaa", owner = "Aleix"), n("n-4aaa"))
        assertEquals(listOf("n-3aaa", "n-4aaa"), NoticeDedupe.announceable(list, emptyMap(), "aleix").map { it.id })
    }

    @Test fun `el registre oblida les velles`() {
        val old = NoticeDedupe.record(emptyMap(), listOf(n("n-old1")), now = 0)
        val now = NoticeDedupe.KEEP_MS + 1
        assertFalse("n-old1" in NoticeDedupe.record(old, listOf(n("n-new1")), now))
    }

    @Test fun `vents amb l'ultima llegida treuen l'avis`() {
        val list = listOf(n("a", ts = 1, read = false), n("b", ts = 2, read = true),
            n("c", ts = 5, wind = "mestral", read = true), n("d", ts = 6, wind = "mestral", read = false))
        assertEquals(setOf("xaloc"), NoticeDedupe.readWinds(list))
    }

    // ---- obrir per message_id ----

    @Test fun `la notificacio porta el missatge i el consum el demana al fil`() {
        val ctx = ApplicationProvider.getApplicationContext<android.content.Context>()
        val i = Intent().putExtra(NotificationNavigation.EXTRA_TARGET, "wind:xaloc")
            .putExtra(NotificationNavigation.EXTRA_ORIGIN, "remote-x")
            .putExtra(NotificationNavigation.EXTRA_MESSAGE, "m-42")
        val dest = NotificationNavigation.consume(i)!!
        assertEquals(NotificationDestination("remote-x", "wind:xaloc", "m-42"), dest)
        assertFalse(i.hasExtra(NotificationNavigation.EXTRA_MESSAGE))
        val req = MessageFocus.request.value!!
        assertEquals("m-42", req.messageId)
        assertEquals(2, MessageFocusLogic.indexFor(req, "wind:xaloc", listOf("m-1", "h-2", "m-42"), req.at))
        MessageFocus.done(req)
        assertNull(MessageFocus.request.value)
        assertNotNull(ctx)
    }

    @Test fun `focus nomes al seu fil i mentre es viu`() {
        val req = FocusRequest("wind:xaloc", "m-1", at = 1000)
        assertNull(MessageFocusLogic.indexFor(req, "wind:mestral", listOf("m-1"), 1000))
        assertNull(MessageFocusLogic.indexFor(req, "wind:xaloc", listOf("m-2"), 1000)) // encara no carregat
        assertNull(MessageFocusLogic.indexFor(req, "wind:xaloc", listOf("m-1"), 1000 + MessageFocusLogic.TTL_MS + 1))
        assertTrue(MessageFocusLogic.expired(req, 1000 + MessageFocusLogic.TTL_MS + 1))
        assertEquals(0, MessageFocusLogic.indexFor(req, "wind:xaloc", listOf("m-1"), 1500))
    }

    @Test fun `avis sense missatge no deixa cap focus`() {
        NotificationNavigation.consume(Intent().putExtra(NotificationNavigation.EXTRA_TARGET, "wind:xaloc"))
        assertNull(MessageFocus.request.value)
    }

    // ---- push ----

    @Test fun `el push nomes accepta l'id opac`() {
        assertEquals("n-abc123", PushLogic.novetatId(mapOf("k" to "novetat", "id" to "n-abc123")))
        assertNull(PushLogic.novetatId(mapOf("k" to "altra", "id" to "n-abc123")))
        assertNull(PushLogic.novetatId(mapOf("k" to "novetat", "id" to "../api/x")))
        assertNull(PushLogic.novetatId(mapOf("k" to "novetat")))
    }

    @Test fun `registre del token nomes quan cal`() {
        assertTrue(PushLogic.needsRegister("t", "", "o", "o", 0, 10))
        assertFalse(PushLogic.needsRegister("t", "t", "o", "o", 0, 10))
        assertTrue(PushLogic.needsRegister("t", "t", "o2", "o", 0, 10))
        assertTrue(PushLogic.needsRegister("t", "t", "o", "o", 0, PushLogic.REFRESH_MS + 1))
        assertFalse(PushLogic.needsRegister("", "t", "o", "o", 0, 10))
    }

    @Test fun `sense configuracio FCM no hi ha transport`() {
        assertNull(VentsPush.transport)
        assertFalse(VentsPush.active(ApplicationProvider.getApplicationContext()))
    }

    // ---- stream ----

    @Test fun `parseja events de novetats del registre`() {
        val e = NovetatStreamLogic.parse("""{"id":12,"kind":"novetat","payload":{"id":"n-1","message_id":"m-1","wind":"xaloc"}}""")!!
        assertEquals(NovetatEvent(12, "novetat", listOf("n-1"), "m-1", listOf("xaloc")), e)
        val r = NovetatStreamLogic.parse("""{"id":13,"kind":"novetat_read","payload":{"ids":["n-1","n-2"],"winds":["xaloc","mestral"]}}""")!!
        assertEquals(listOf("n-1", "n-2"), r.ids)
        assertEquals(listOf("xaloc", "mestral"), r.winds)
        assertNull(NovetatStreamLogic.parse("""{"id":14,"kind":"text","text":"hola"}"""))
        assertNull(NovetatStreamLogic.parse("no json"))
    }

    @Test fun `stream amb cursor i backoff fitat`() {
        assertTrue(NovetatStreamLogic.path(77).contains("after=77"))
        assertTrue(NovetatStreamLogic.path(0).contains("scope=user"))
        assertEquals(1_000L, NovetatStreamLogic.backoff(0))
        assertEquals(30_000L, NovetatStreamLogic.backoff(20))
    }

    @Test fun `el cursor nomes avança`() {
        val ctx = ApplicationProvider.getApplicationContext<android.content.Context>()
        NovetatCursor.set(ctx, "oc", 10)
        NovetatCursor.set(ctx, "oc", 4)
        assertEquals(10L, NovetatCursor.get(ctx, "oc"))
    }

    @Test fun `insignies ignoren les llegides en un altre lloc`() {
        val list = listOf(n("n-1", ts = 100, read = true), n("n-2", ts = 100, wind = "mestral"))
        assertEquals(setOf("mestral"), WindNewsLogic.unseen(list, emptyMap(), 0))
    }

    @Test fun `lectura per id del push`() {
        val got = AvuiJson.novetat("""{"novetat":{"ts":5,"wind":"xaloc","name":"Xaloc","source":"remind","text":"hola","id":"n-1","owner":"aleix","read":true,"message_id":"m-1"}}""")!!
        assertEquals("n-1", got.id); assertEquals("aleix", got.owner); assertTrue(got.read); assertEquals("m-1", got.messageId)
    }

    // ---- portabilitat: sense FCM tot funciona i es diu clar ----

    @Test fun `sense FCM enlloc diu push no configurat`() {
        assertEquals(PushState.NOT_CONFIGURED, PushStatusLogic.state(appHasFcm = false, serverMode = "polling"))
        assertEquals(PushState.NOT_CONFIGURED, PushStatusLogic.state(false, ""))
        assertTrue(PushStatusLogic.text("ca", PushState.NOT_CONFIGURED).startsWith("Push no configurat"))
        assertTrue(PushStatusLogic.text("en", PushState.NOT_CONFIGURED).startsWith("Push not configured"))
    }

    @Test fun `push actiu nomes amb app i servidor configurats`() {
        assertEquals(PushState.ACTIVE, PushStatusLogic.state(true, "fcm"))
        assertEquals(PushState.SERVER_ONLY, PushStatusLogic.state(false, "fcm"))
        assertEquals(PushState.APP_ONLY, PushStatusLogic.state(true, "polling"))
        for (s in PushState.values()) for (l in listOf("ca", "en")) assertTrue(PushStatusLogic.text(l, s).isNotBlank())
    }
}
