#!/bin/sh
# build.sh — binaris de la CLI/backend per a totes les plataformes + checksums.
# Ús: ./build.sh [versió]   (defecte: tag git)
# Surt a dist/gregal-<os>-<arch>[.exe] i dist/SHA256SUMS, amb els mateixos
# noms que els fitxers de la release de GitHub: és el que fa servir la CI
# (.github/workflows/release.yml) i el que pots córrer en local.
set -eu
cd "$(dirname "$0")"

VER=${1:-$(git describe --tags --always --dirty 2>/dev/null || echo v0.0.0-dev)}
if command -v go >/dev/null 2>&1; then
	GO=go
else
	GO=$HOME/sdk/go/bin/go
fi

rm -rf dist
mkdir -p dist
build() {
	ext=""
	[ "$1" = "windows" ] && ext=".exe"
	out="dist/gregal-$1-$2$ext"
	# -trimpath i -buildvcs=false eviten que la ruta local o la metadata
	# accidental del checkout facin variar el binari d'una mateixa font.
	CGO_ENABLED=0 GOOS=$1 GOARCH=$2 "$GO" build -trimpath -buildvcs=false -ldflags "-X main.version=$VER" -o "$out" .
	echo "built $out ($VER)"
}

build linux amd64
build linux arm64
build windows amd64
build windows arm64
build darwin amd64
build darwin arm64
(cd dist && sha256sum gregal-* > SHA256SUMS)
echo "checksums: dist/SHA256SUMS"
if [ -n "${GREGAL_SIGNING_KEY:-}" ]; then
	if ! command -v gpg >/dev/null 2>&1; then
		echo "GREGAL_SIGNING_KEY definit però gpg no és instal·lat" >&2
		exit 1
	fi
	gpg --batch --local-user "$GREGAL_SIGNING_KEY" --armor --detach-sign --output "dist/SHA256SUMS.asc" "dist/SHA256SUMS"
	echo "signatura: dist/SHA256SUMS.asc"
else
	echo "avís: defineix GREGAL_SIGNING_KEY per signar SHA256SUMS"
fi
