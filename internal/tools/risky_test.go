package tools

import (
	"path/filepath"
	"testing"
)

// Els vents no tenen finestra d'aprovació pròpia: Risky decideix quan
// s'ha de preguntar. La feina normal del projecte no ha de molestar.
func TestRiskyAsksForInstallsDownloadsDeletesAndSystem(t *testing.T) {
	proj := t.TempDir()
	home := filepath.Dir(proj)
	outside := filepath.Join(home, "altra-cosa")
	cases := []struct {
		cmd, kind string
	}{
		// Instal·lar programari.
		{"sdkmanager --licenses", "install"},
		{"yes | sdkmanager --licenses", "install"},
		{`sdkmanager "platforms;android-34" "build-tools;34.0.0"`, "install"},
		{"apt-get install -y openjdk-21-jdk", "install"},
		{"brew install gradle", "install"},
		{"npm install -g typescript", "install"},
		{"pip install --user requests", "install"},
		{"pipx install poetry", "install"},
		{"cargo install ripgrep", "install"},
		{"sh -c 'apt-get install -y unzip'", "install"},
		// Baixar paquets i binaris.
		{"curl -fsSL -o cmdline-tools.zip https://dl.google.com/android/repository/commandlinetools-linux.zip", "download"},
		{"wget https://download.oracle.com/java/21/jdk-21_linux-x64_bin.tar.gz", "download"},
		{"curl -L -O https://example.com/app.apk", "download"},
		// Esborrar fora del projecte.
		{"rm -rf " + outside, "delete"},
		{"rm -rf ~/Android", "delete"},
		{"rm -rf /var/www/blog", "delete"},
		{"rm -rf " + proj, "delete"}, // el projecte sencer
		{"find /srv/old -name '*.log' -delete", "delete"},
		// Sistema.
		{"systemctl restart caddy", "system"},
		{"docker run -d nginx", "system"},
		{"crontab -e", "system"},
		{"chmod -R 777 /srv/www", "system"},
	}
	for _, c := range cases {
		r, ok := Risky(c.cmd, proj, home)
		if !ok || r.Kind != c.kind {
			t.Errorf("%q → %+v ok=%v, volia %s", c.cmd, r, ok, c.kind)
		}
	}
}

func TestRiskyLeavesNormalProjectWorkAlone(t *testing.T) {
	proj := t.TempDir()
	home := filepath.Dir(proj)
	for _, cmd := range []string{
		"./gradlew assembleDebug",
		"export ANDROID_HOME=$HOME/Android/Sdk && ./gradlew assembleDebug",
		"npm install",
		"npm install express",
		"pip install requests",
		"go build ./...",
		"python -m py_compile app.py",
		"node --check server.js",
		"git status",
		"curl -s http://127.0.0.1:8097/api/health",
		"curl -fsSL https://api.example.com/data.json",
		"curl -I https://example.com/",
		"sdkmanager --list",
		"rm -rf dist node_modules",
		"rm -rf " + filepath.Join(proj, "build"),
		"rm -f notes.txt",
		"find . -name '*.pyc' -delete",
		"systemctl status caddy",
		"docker ps",
		"crontab -l",
		"cat > index.html <<'EOF'\nrm -rf /important\nEOF",
		"mkdir -p site && cp -r public/* site/",
		"unzip app.zip -d app",
	} {
		if r, ok := Risky(cmd, proj, home); ok {
			t.Errorf("%q no havia de demanar permís: %+v", cmd, r)
		}
	}
}
