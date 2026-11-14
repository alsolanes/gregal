package xyz.solanes.gregal

import android.content.Context
import android.content.Intent
import android.content.pm.PackageManager
import android.os.Build
import android.os.Bundle
import androidx.activity.ComponentActivity
import androidx.activity.result.contract.ActivityResultContracts

/*
 * Una única política del permís d'avisos (Android 13+), B7.
 *
 * - Mai a l'arrencada ni «perquè sí»: només quan l'usuari fa una cosa que
 *   el justifica (demanar un recordatori, encendre la vigilància d'un vent,
 *   o connectar-se a un servidor que ja pot enviar avisos: PUSH).
 * - Una sola vegada. Si diu que no, la decisió és durable: no es torna a
 *   mostrar el diàleg ni en aquesta sessió ni en cap altra. Qui el vulgui
 *   activar després ho fa des de la targeta «Feina en segon pla» (Feines),
 *   que obre la configuració del sistema.
 * - Abans hi havia dos disparadors (MainActivity a cada arrencada nova i
 *   CompanyScreen quan hi havia jobs) i un snackbar en denegar-lo.
 */

/** Per què es vol avisar. Null = ningú no ho ha demanat (arrencada, sondeig…). */
enum class NotifAskReason { REMINDER, WATCH, USER, PUSH }

/** El que queda desat entre sessions. */
data class NotifAskState(
    /** Vegades que s'ha mostrat el diàleg del sistema. */
    val asked: Int = 0,
    /** L'usuari l'ha denegat (en qualsevol moment): no es torna a demanar sol. */
    val denied: Boolean = false,
)

object NotifPermissionPolicy {
    const val FIRST_SDK = 33

    /**
     * Si cal mostrar ara el diàleg del sistema.
     *  - sense motiu (arrencada, segon pla) → mai;
     *  - concedit o Android < 13 → no cal;
     *  - denegat abans → mai automàticament; només [NotifAskReason.USER]
     *    (un botó explícit) i encara així Android pot no mostrar-lo: llavors
     *    toca obrir la configuració;
     *  - en context i mai preguntat → sí, una vegada.
     */
    fun shouldAsk(sdk: Int, granted: Boolean, reason: NotifAskReason?, state: NotifAskState): Boolean = when {
        sdk < FIRST_SDK || granted || reason == null -> false
        reason == NotifAskReason.USER -> true
        state.denied -> false
        else -> state.asked == 0
    }

    /** Estat després de mostrar el diàleg. */
    fun afterResult(state: NotifAskState, granted: Boolean): NotifAskState =
        NotifAskState(asked = state.asked + 1, denied = !granted)
}

/** Pega Android de la política: preferències i el diàleg en una activitat transparent. */
object NotifPermission {
    private const val PREFS = "gregal_notif_permission"
    private const val KEY_ASKED = "asked"
    private const val KEY_DENIED = "denied"

    private fun prefs(ctx: Context) = ctx.getSharedPreferences(PREFS, Context.MODE_PRIVATE)

    fun state(ctx: Context): NotifAskState = prefs(ctx).let {
        NotifAskState(asked = it.getInt(KEY_ASKED, 0), denied = it.getBoolean(KEY_DENIED, false))
    }

    internal fun save(ctx: Context, s: NotifAskState) {
        prefs(ctx).edit().putInt(KEY_ASKED, s.asked).putBoolean(KEY_DENIED, s.denied).apply()
    }

    fun granted(ctx: Context): Boolean = Build.VERSION.SDK_INT < NotifPermissionPolicy.FIRST_SDK ||
        ctx.checkSelfPermission(android.Manifest.permission.POST_NOTIFICATIONS) == PackageManager.PERMISSION_GRANTED

    /**
     * L'usuari acaba de fer una cosa que vol avisos: si la política ho
     * permet i l'app és davant, surt el diàleg del sistema (una vegada).
     * Torna si s'ha mostrat.
     */
    fun requestFor(ctx: Context, reason: NotifAskReason): Boolean {
        val app = ctx.applicationContext
        if (!AppVisibility.foreground) return false
        if (!NotifPermissionPolicy.shouldAsk(Build.VERSION.SDK_INT, granted(app), reason, state(app))) return false
        if (pending) return false
        pending = true
        return try {
            app.startActivity(Intent(app, NotifPermissionActivity::class.java).addFlags(Intent.FLAG_ACTIVITY_NEW_TASK))
            true
        } catch (_: Exception) {
            pending = false
            false
        }
    }

    /** Un diàleg en curs: dos motius alhora no n'obren dos. */
    @Volatile internal var pending = false
}

/**
 * Transparent: demana el permís i es tanca. Desa el resultat de manera
 * durable; no mostra cap snackbar ni cap altre avís en denegar-lo.
 */
class NotifPermissionActivity : ComponentActivity() {
    private val ask = registerForActivityResult(ActivityResultContracts.RequestPermission()) { granted ->
        NotifPermission.save(this, NotifPermissionPolicy.afterResult(NotifPermission.state(this), granted))
        NotifPermission.pending = false
        if (granted) {
            // Ara sí que pot sonar: rearma el vigilant i el registre de push.
            try { WindWatch.schedule(applicationContext) } catch (_: Exception) {}
        }
        finish()
    }

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        if (savedInstanceState == null) {
            if (Build.VERSION.SDK_INT >= NotifPermissionPolicy.FIRST_SDK) ask.launch(android.Manifest.permission.POST_NOTIFICATIONS)
            else { NotifPermission.pending = false; finish() }
        }
    }

    override fun onDestroy() {
        if (isFinishing) NotifPermission.pending = false
        super.onDestroy()
    }
}
