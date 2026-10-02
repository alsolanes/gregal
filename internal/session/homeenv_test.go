package session

import "testing"

// setHomeTest aïlla el directori d'usuari durant un test. Posar-hi només HOME
// no serveix a Windows: os.UserHomeDir() hi llegeix USERPROFILE, i els tests
// acabaven llegint —i sobreescrivint— els fitxers de debò de $HOME.
func setHomeTest(t *testing.T, dir string) {
	t.Helper()
	t.Setenv("HOME", dir)
	t.Setenv("USERPROFILE", dir)
}
