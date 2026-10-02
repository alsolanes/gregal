package tools

// office_open: obre un document amb el programa associat del sistema
// (Word/Excel/PowerPoint a Windows, o el visor que toqui). No bloqueja:
// el programa s'engega en segon pla i l'eina torna de seguida.
//
// ATENCIÓ Windows: mentre el document és obert al programa, el fitxer
// queda bloquejat i office_edit no hi pot escriure (l'Excel en exclusiu;
// Word/PowerPoint deneguen escriptura). office_read sí que funciona.
// OfficeWritable ho detecta perquè l'edició falli ràpid amb un missatge
// clar en comptes d'un "accés denegat" al final de la feina.

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

// OfficeOpen obre path amb el programa associat. Accepta moderns i antics
// (.doc/.xls/.ppt els obre el Word/Excel sense problema, encara que
// office_read no els llegeixi).
func OfficeOpen(path string) (string, error) {
	if strings.TrimSpace(path) == "" {
		return "", fmt.Errorf("ruta buida")
	}
	fi, err := os.Stat(path)
	if err != nil {
		return "", fmt.Errorf("no s'ha pogut obrir: %v", err)
	}
	if fi.IsDir() {
		return "", fmt.Errorf("és un directori, no un document: %s", path)
	}
	kind := OfficeKind(path)
	if kind == "" {
		return "", fmt.Errorf("no és un document office: %s", path)
	}
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		// start necessita un títol buit quan la ruta va entre cometes.
		cmd = exec.Command("cmd", "/c", "start", "", path)
	case "darwin":
		cmd = exec.Command("open", path)
	default:
		cmd = exec.Command("xdg-open", path)
	}
	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("no s'ha pogut obrir amb el programa associat: %v", err)
	}
	_ = cmd.Process.Release()
	names := map[string]string{
		"docx": "Word", "doc": "Word",
		"xlsx": "Excel", "xls": "Excel",
		"pptx": "PowerPoint", "ppt": "PowerPoint",
	}
	app, ok := names[kind]
	if !ok {
		app = "el programa associat"
	}
	return fmt.Sprintf("obert %s amb %s (mentre sigui obert no s'hi pot escriure: tanca'l per editar)", path, app), nil
}

// OfficeWritable diu si podem reescriure el fitxer ara mateix. A Windows un
// document obert al Word/Excel/PowerPoint queda bloquejat i el rename final
// de zipRewrite petaria amb "accés denegat": millor fallar aquí, abans de
// fer feina, amb un missatge que digui què ha de fer l'usuari.
func OfficeWritable(path string) error {
	f, err := os.OpenFile(path, os.O_WRONLY, 0)
	if err != nil {
		if os.IsNotExist(err) {
			return err
		}
		if kind := OfficeKind(path); kind != "" {
			return fmt.Errorf("fitxer bloquejat (segurament obert al Word/Excel/PowerPoint): desa'l i tanca'l abans d'editar %s", path)
		}
		return fmt.Errorf("no es pot escriure: %v", err)
	}
	return f.Close()
}
