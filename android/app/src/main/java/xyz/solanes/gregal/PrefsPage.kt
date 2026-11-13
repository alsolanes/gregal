package xyz.solanes.gregal

/** Native settings routes. Each child has an explicit parent for toolbar/system Back. */
enum class PrefsPage(val ca: String, val en: String) {
    Index("Preferències", "Preferences"),
    General("General", "General"),
    Appearance("Aparença", "Appearance"),
    Theme("Mode clar/fosc", "Light/dark mode"),
    Palette("Paleta", "Palette"),
    Names("Noms dels ajudants", "Helper names"),
    About("Versió i actualitzacions", "Version and updates"),
    Notifications("Avisos", "Notifications"),
    Models("Models", "Models"),
    Provider("Proveïdor i clau", "Provider and key"),
    Permissions("Permisos", "Permissions"),
    Connections("Connexions", "Connections"),
    Accounts("Comptes connectats", "Connected accounts"),
    CalendarMail("Calendari i correu", "Calendar and email"),
    Calendar("Google Calendar", "Google Calendar"),
    Mail("Gmail", "Gmail"),
    Briefing("Resum del matí", "Morning briefing"),
    Server("Servidor", "Server");

    fun title(lang: String) = if (lang == "en") en else ca

    val parent: PrefsPage get() = when (this) {
        Accounts, CalendarMail -> Connections
        Calendar, Mail, Briefing -> CalendarMail
        Theme, Palette, Names -> Appearance
        About, Notifications -> General
        Provider -> Models
        else -> Index
    }

    fun available(coding: Boolean, admin: Boolean) = when (this) {
        Permissions -> coding
        Server -> admin
        else -> true
    }
}
