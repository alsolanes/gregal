package xyz.solanes.gregal.fcm

import android.content.Context
import com.google.android.gms.tasks.Tasks
import com.google.firebase.FirebaseApp
import com.google.firebase.FirebaseOptions
import com.google.firebase.messaging.FirebaseMessaging
import com.google.firebase.messaging.FirebaseMessagingService
import com.google.firebase.messaging.RemoteMessage
import xyz.solanes.gregal.BuildConfig
import xyz.solanes.gregal.PushTransport
import xyz.solanes.gregal.VentsPush
import java.util.concurrent.TimeUnit

/*
 * Només es compila amb la configuració de Firebase (gregal.fcm.* a Gradle,
 * fora del repositori). Sense google-services.json: FirebaseApp s'inicia a
 * mà amb les opcions de BuildConfig. Sense GMS al dispositiu, token() torna
 * null i l'app continua amb el sondeig.
 */

internal object Fcm {
    fun app(ctx: Context): FirebaseApp? = try {
        FirebaseApp.getApps(ctx).firstOrNull() ?: FirebaseApp.initializeApp(
            ctx.applicationContext,
            FirebaseOptions.Builder()
                .setProjectId(BuildConfig.FCM_PROJECTID)
                .setApplicationId(BuildConfig.FCM_APPID)
                .setApiKey(BuildConfig.FCM_APIKEY)
                .setGcmSenderId(BuildConfig.FCM_SENDERID)
                .build(),
        )
    } catch (_: Throwable) { null }
}

/** El transport que VentsPush troba per reflexió. */
class FcmTransport : PushTransport {
    override fun token(ctx: Context): String? = try {
        if (Fcm.app(ctx) == null) null
        else Tasks.await(FirebaseMessaging.getInstance().token, 20, TimeUnit.SECONDS)?.takeIf { it.isNotBlank() }
    } catch (_: Throwable) { null }
}

/** Rep els push: només dades (k, id). Mai text; el contingut es llegeix autenticat. */
class FcmService : FirebaseMessagingService() {
    override fun onCreate() {
        Fcm.app(this)
        super.onCreate()
    }

    override fun onNewToken(token: String) = VentsPush.onNewToken(this, token)

    override fun onMessageReceived(message: RemoteMessage) = VentsPush.onMessage(this, message.data)
}
