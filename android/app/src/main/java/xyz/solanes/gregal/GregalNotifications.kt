package xyz.solanes.gregal

import android.app.Notification
import android.app.NotificationChannel
import android.app.NotificationManager
import android.app.PendingIntent
import android.app.Person
import android.app.Service
import android.content.Context
import android.content.Intent
import android.content.pm.PackageManager
import android.graphics.drawable.Icon
import android.os.Build
import android.os.IBinder

/** Estat mínim per no avisar si l'usuari ja està llegint el streaming. */
object AppVisibility {
    @Volatile var foreground: Boolean = false
}

/**
 * Manté el procés prioritari mentre el servidor envia SSE. Sense aquest servei,
 * Android pot matar la connexió quan l'app queda en segon pla.
 */
class GregalWorkService : Service() {
    override fun onStartCommand(intent: Intent?, flags: Int, startId: Int): Int {
        GregalNotifications.ensureChannel(this)
        startForeground(GregalNotifications.WORK_ID, GregalNotifications.working(this))
        return START_NOT_STICKY
    }

    override fun onBind(intent: Intent?): IBinder? = null
}

/** Notificacions locals: no hi ha cap token ni text del projecte a la pantalla bloquejada. */
object GregalNotifications {
    const val CHANNEL_ID = "gregal_answers"
    const val WORK_ID = 431
    private const val READY_ID = 432

    private fun text(context: Context, key: String): String = tr(Prefs(context).idioma, key)

    fun ensureChannel(context: Context) {
        if (Build.VERSION.SDK_INT < Build.VERSION_CODES.O) return
        val channel = NotificationChannel(
            CHANNEL_ID, text(context, "app.notifications.channel"), NotificationManager.IMPORTANCE_DEFAULT
        ).apply {
            description = text(context, "app.notifications.channelDescription")
        }
        context.getSystemService(NotificationManager::class.java).createNotificationChannel(channel)
    }

    fun workStarted(context: Context) {
        ensureChannel(context)
        val intent = Intent(context, GregalWorkService::class.java)
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) context.startForegroundService(intent)
        else context.startService(intent)
    }

    fun workFinished(context: Context, hasAnswer: Boolean, userText: String = "", answer: String = "") {
        context.stopService(Intent(context, GregalWorkService::class.java))
        if (!hasAnswer || AppVisibility.foreground || !canNotify(context)) return
        ensureChannel(context)
        val b = Notification.Builder(context, CHANNEL_ID)
            .setSmallIcon(R.drawable.ic_stat_gregal)
            .setContentTitle(text(context, "app.notifications.readyTitle"))
            .setContentIntent(openApp(context))
            .setAutoCancel(true)
            .setVisibility(Notification.VISIBILITY_PRIVATE)
        val excerpt = answer.trim().take(280)
        if (excerpt.isNotEmpty() && Build.VERSION.SDK_INT >= Build.VERSION_CODES.P) {
            // Estil conversa quan està desbloquejat; a la pantalla bloquejada
            // segueix sent genèric (PRIVATE): ni resposta ni dades del projecte.
            val me = Person.Builder().setName(text(context, "app.notifications.you")).build()
            val gregal = Person.Builder().setName("Gregal").build()
            val now = System.currentTimeMillis()
            b.style = Notification.MessagingStyle(gregal)
                .addMessage(userText.trim().take(160).ifEmpty { "…" }, now, me)
                .addMessage(excerpt, now, gregal)
        } else {
            b.setContentText(text(context, "app.notifications.readyBody"))
        }
        try {
            b.setLargeIcon(Icon.createWithResource(context, R.mipmap.ic_gregal))
        } catch (_: Exception) {
        }
        context.getSystemService(NotificationManager::class.java).notify(READY_ID, b.build())
    }

    internal fun working(context: Context): Notification = Notification.Builder(context, CHANNEL_ID)
        .setSmallIcon(R.drawable.ic_stat_gregal)
        .setContentTitle(text(context, "app.notifications.workingTitle"))
        .setContentText(text(context, "app.notifications.workingBody"))
        .setContentIntent(openApp(context))
        .setUsesChronometer(true)
        .setOngoing(true)
        .setVisibility(Notification.VISIBILITY_PRIVATE)
        .build()

    private fun openApp(context: Context): PendingIntent = PendingIntent.getActivity(
        context, 0,
        Intent(context, MainActivity::class.java).apply {
            flags = Intent.FLAG_ACTIVITY_SINGLE_TOP or Intent.FLAG_ACTIVITY_CLEAR_TOP
        },
        PendingIntent.FLAG_UPDATE_CURRENT or PendingIntent.FLAG_IMMUTABLE,
    )

    private fun canNotify(context: Context): Boolean =
        Build.VERSION.SDK_INT < 33 ||
            context.checkSelfPermission(android.Manifest.permission.POST_NOTIFICATIONS) ==
            PackageManager.PERMISSION_GRANTED
}
