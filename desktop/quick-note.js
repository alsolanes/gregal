// Nota ràpida des de qualsevol lloc: Ctrl+Alt+N (Cmd+Alt+N al Mac) porta la
// finestra de Gregal al davant i obre el full de notes amb una nota nova,
// sigui a Codi o a Vents. Escrius, Esc, i tornes al que feies: més ràpid que obrir
// el Bloc de notes, i no es perd. La pàgina fa la resta (panels.js →
// gregalQuickNotes / notes-quick.js).

const QUICK_NOTE_ACCELERATOR = "CommandOrControl+Alt+N";

/** Mostra la finestra (encara que estigui minimitzada) i demana la captura. */
function showQuickNote(win) {
    if (!win || win.isDestroyed()) return false;
    if (win.isMinimized()) win.restore();
    win.show();
    win.focus();
    win.webContents.send("gregal:command", "quick-note");
    return true;
}

/**
 * Registra la drecera global. getWin torna la finestra viva (pot canviar si
 * es torna a crear). Torna false si una altra app ja té la drecera: no és
 * cap error, només que aquí no hi serà.
 */
function registerQuickNote(globalShortcut, getWin) {
    try {
        if (globalShortcut.isRegistered(QUICK_NOTE_ACCELERATOR)) return true;
        return globalShortcut.register(QUICK_NOTE_ACCELERATOR, () => showQuickNote(getWin()));
    } catch (e) {
        return false;
    }
}

module.exports = { QUICK_NOTE_ACCELERATOR, showQuickNote, registerQuickNote };
