package xyz.solanes.gregal

import android.app.Notification
import android.content.Context
import android.graphics.Bitmap
import android.graphics.Canvas
import android.graphics.LinearGradient
import android.graphics.Paint
import android.graphics.Shader
import android.graphics.drawable.Icon
import java.util.Locale

/**
 * L'aspecte d'un avís segons el que porta: el resum del matí surt amb un sol,
 * un recordatori amb una campana, el temps amb un núvol… El glif va a la barra
 * d'estat i, en gran, sobre una insígnia rodona amb el color de la mena.
 * La tria és pura (testeable); només [badge] i [apply] toquen Android.
 */
enum class NotifKind(val glyph: Int, val top: Long, val bottom: Long) {
    BRIEFING(R.drawable.ic_notif_sun, 0xFFFFC56B, 0xFFF08A4B),
    REMINDER(R.drawable.ic_notif_bell, 0xFF9AA8FF, 0xFF6B6FE8),
    WEATHER(R.drawable.ic_notif_rain, 0xFF8FD0F5, 0xFF4B95D9),
    NEWS(R.drawable.ic_notif_news, 0xFFB5BDCC, 0xFF6F7A90),
    SPORTS(R.drawable.ic_notif_ball, 0xFF7FDBA8, 0xFF2FA374),
    MUSIC(R.drawable.ic_notif_music, 0xFFF5A3C2, 0xFFD45F8D),
    BOOKS(R.drawable.ic_notif_book, 0xFFE3BF8A, 0xFFB07F45),
    CALENDAR(R.drawable.ic_notif_calendar, 0xFF94B4FF, 0xFF5577E0),
    SHOPPING(R.drawable.ic_notif_bag, 0xFFFFB199, 0xFFE0705A),
    DONE(R.drawable.ic_notif_check, 0xFF8EE0B5, 0xFF3BAA79),
    FAILED(R.drawable.ic_notif_alert, 0xFFFF9B95, 0xFFD9534F),
    SPARK(R.drawable.ic_notif_spark, 0xFFC3B6FF, 0xFF8B76E8),
}

object NotifLook {
    private val keys: List<Pair<NotifKind, List<String>>> = listOf(
        NotifKind.WEATHER to listOf("pluja", "plourà", "temps", "graus", "°c", "núvol", "sol i", "vent fort", "tempesta", "weather", "rain", "forecast", "lluvia", "tiempo"),
        NotifKind.SPORTS to listOf("partit", "gol", "barça", "lliga", "champions", "classificació", "match", "league", "fútbol", "futbol", "bàsquet", "tennis", "football"),
        NotifKind.CALENDAR to listOf("reunió", "agenda", "calendari", "cita", "meeting", "calendar", "appointment", "reunión"),
        NotifKind.MUSIC to listOf("concert", "disc", "cançó", "àlbum", "album", "música", "music", "song", "spotify"),
        NotifKind.BOOKS to listOf("llibre", "novel·la", "lectura", "book", "novel", "libro"),
        NotifKind.SHOPPING to listOf("preu", "oferta", "rebaixa", "compra", "descompte", "price", "deal", "discount", "precio"),
        NotifKind.NEWS to listOf("notícies", "notícia", "titulars", "news", "headlines", "noticias"),
    )
    private val morning = listOf("bon dia", "good morning", "buenos días", "buenos dias", "resum del matí", "morning brief", "resumen de la mañana", "avui tens", "today you have")

    /**
     * [source]: d'on ve (job, remind, heartbeat, feina:…, football, corner);
     * [title] i [text]: el nom de la feina i el que diu. [hour]: l'hora local
     * (el butlletí d'un vent abans de migdia és el resum del matí).
     */
    fun of(source: String, title: String, text: String, failed: Boolean = false, hour: Int = -1): NotifKind {
        if (failed) return NotifKind.FAILED
        val s = source.lowercase(Locale.ROOT)
        if (s == "remind") return NotifKind.REMINDER
        // Una feina de la colla, o la resposta que vas demanar i no vas veure (app tancada).
        if (s.startsWith("feina:") || s == "resposta" || s.startsWith("resposta:")) return NotifKind.DONE
        if (s == "football") return NotifKind.SPORTS
        // El títol mana: «Previsió del temps» no és el resum encara que digui «bon dia».
        val named = title.lowercase(Locale.ROOT)
        keys.firstOrNull { (_, words) -> words.any { named.contains(it) } }?.let { return it.first }
        val head = (title + "\n" + text.take(400)).lowercase(Locale.ROOT)
        if (morning.any { head.contains(it) }) return NotifKind.BRIEFING
        if ((s == "job" || s == "daily") && hour in 5..11) return NotifKind.BRIEFING
        keys.firstOrNull { (_, words) -> words.any { head.contains(it) } }?.let { return it.first }
        return if (s == "job" || s == "daily") NotifKind.BRIEFING else NotifKind.SPARK
    }

    /** La insígnia gran: un cercle amb el degradat de la mena i el glif en blanc. */
    fun badge(context: Context, kind: NotifKind, px: Int = 128): Bitmap? = try {
        val bmp = Bitmap.createBitmap(px, px, Bitmap.Config.ARGB_8888)
        val c = Canvas(bmp)
        val r = px / 2f
        val fill = Paint(Paint.ANTI_ALIAS_FLAG).apply {
            shader = LinearGradient(0f, 0f, px * .4f, px.toFloat(), kind.top.toInt(), kind.bottom.toInt(), Shader.TileMode.CLAMP)
        }
        c.drawCircle(r, r, r, fill)
        // Una llum suau a dalt: volum, com les cares dels vents.
        val sheen = Paint(Paint.ANTI_ALIAS_FLAG).apply {
            shader = LinearGradient(0f, 0f, 0f, px * .6f, 0x40FFFFFF, 0x00FFFFFF, Shader.TileMode.CLAMP)
        }
        c.drawCircle(r, r, r, sheen)
        context.getDrawable(kind.glyph)?.let { d ->
            val g = (px * .5f).toInt()
            val o = (px - g) / 2
            d.setBounds(o, o, o + g, o + g)
            d.draw(c)
        }
        bmp
    } catch (_: Exception) {
        null
    }

    /** Glif a la barra d'estat, insígnia gran i color de la mena. */
    fun apply(context: Context, builder: Notification.Builder, kind: NotifKind): NotifKind {
        builder.setSmallIcon(kind.glyph).setColor(kind.bottom.toInt())
        badge(context, kind)?.let { builder.setLargeIcon(Icon.createWithBitmap(it)) }
        return kind
    }
}
