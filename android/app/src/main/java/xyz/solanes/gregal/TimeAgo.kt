package xyz.solanes.gregal

import java.time.LocalDate
import java.time.LocalTime
import java.time.ZoneId

/** Etiqueta de dia per a divisors ("Avui", "Ahir" o "dd-MM"). */
fun dayLabel(at: Long, nowMs: Long = System.currentTimeMillis(), lang: String = "ca"): String {
    if (at <= 0L) return ""
    val zone = ZoneId.systemDefault()
    val day = java.time.Instant.ofEpochMilli(at).atZone(zone).toLocalDate()
    val today = java.time.Instant.ofEpochMilli(nowMs).atZone(zone).toLocalDate()
    return when (day) {
        today -> tr(lang, "app.sess.today")
        today.minusDays(1) -> tr(lang, "app.sess.yesterday")
        else -> "%02d-%02d".format(day.dayOfMonth, day.monthValue)
    }
}

/** Hora curta "HH:mm" per a bombolles. */
fun hourMinute(at: Long): String {
    if (at <= 0L) return ""
    val t = java.time.Instant.ofEpochMilli(at).atZone(ZoneId.systemDefault()).toLocalTime()
    return "%02d:%02d".format(t.hour, t.minute)
}

/** Clau de dia per agrupar (nombre de dies des d'època, zona local). */
fun dayKey(at: Long): Long {
    if (at <= 0L) return -1L
    val zone = ZoneId.systemDefault()
    return java.time.Instant.ofEpochMilli(at).atZone(zone).toLocalDate().toEpochDay()
}

/** Marca curta per a ISO-8601 del servidor ("2026-09-13 08:00"). */
fun shortStamp(iso: String): String {
    val t = iso.trim()
    if (t.length >= 16 && t[4] == '-' && t[10] == 'T') {
        return t.substring(0, 10) + " " + t.substring(11, 16)
    }
    return t.take(16)
}
/** Temps relatiu per a segells "dd-MM HH:mm" del servidor (pur, testeable). */
fun ago(moment: String, nowMs: Long = System.currentTimeMillis(), lang: String = "ca"): String {
    val m = moment.trim()
    return try {
        val parts = m.split(' ', limit = 2)
        if (parts.size != 2) return m
        val dp = parts[0].split('-', limit = 2)
        val tp = parts[1].split(':', limit = 2)
        if (dp.size != 2 || tp.size != 2) return m
        val day = dp[0].toInt()
        val month = dp[1].toInt()
        val hour = tp[0].toInt()
        val min = tp[1].toInt()
        var date = LocalDate.now().withMonth(month).withDayOfMonth(day)
        val zone = ZoneId.systemDefault()
        var stamp = date.atTime(LocalTime.of(hour, min)).atZone(zone).toInstant().toEpochMilli()
        if (stamp > nowMs + 60_000L) {
            date = date.minusYears(1)
            stamp = date.atTime(LocalTime.of(hour, min)).atZone(zone).toInstant().toEpochMilli()
        }
        val mins = (nowMs - stamp) / 60_000L
        when {
            mins < 1 -> tr(lang, "app.time.now")
            mins < 60 -> tr(lang, "app.time.minutes", mins)
            mins < 24 * 60 -> tr(lang, "app.time.hours", mins / 60)
            mins < 48 * 60 -> tr(lang, "app.time.yesterday")
            else -> m
        }
    } catch (_: Exception) {
        m
    }
}
