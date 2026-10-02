#!/bin/sh
# build.sh — binaris gregal versionats + checksums a dist/.
# Ús: ./build.sh [versió]   (defecte: tag git o v1.5.0-dev)
# Linux/macOS: binari sol · Windows: .zip amb l'.exe.
set -eu
cd "$(dirname "$0")"

VER=${1:-$(git describe --tags --always --dirty 2>/dev/null || echo v1.5.0-dev)}
if command -v go >/dev/null 2>&1; then
	GO=go
else
	GO=$HOME/sdk/go/bin/go
fi

mkdir -p dist
build() {
	out="dist/gregal-$VER-$1-$2"
	# -trimpath i -buildvcs=false eviten que la ruta local o la metadata
	# accidental del checkout facin variar el binari d'una mateixa font.
	ldflags="-X main.version=$VER"
	if [ "$1" = "windows" ]; then
		GOOS=$1 GOARCH=$2 "$GO" build -trimpath -buildvcs=false -ldflags "$ldflags" -o "$out.exe" .
		if command -v zip >/dev/null 2>&1; then
			(cd dist && rm -f "gregal-$VER-$1-$2.zip" && zip -q "gregal-$VER-$1-$2.zip" "gregal-$VER-$1-$2.exe" && rm "gregal-$VER-$1-$2.exe")
		else
			# Git Bash a Windows no porta zip: Compress-Archive fa el mateix.
			(cd dist && rm -f "gregal-$VER-$1-$2.zip" && powershell -NoProfile -Command "Compress-Archive -Path 'gregal-$VER-$1-$2.exe' -DestinationPath 'gregal-$VER-$1-$2.zip' -Force" && rm "gregal-$VER-$1-$2.exe")
		fi
		echo "built dist/gregal-$VER-$1-$2.zip"
	else
		GOOS=$1 GOARCH=$2 "$GO" build -trimpath -buildvcs=false -ldflags "$ldflags" -o "$out" .
		echo "built $out"
	fi
}

build linux amd64
build linux arm64
build windows amd64
build windows arm64
build darwin amd64
build darwin arm64
(cd dist && sha256sum gregal-"$VER"-* > SHA256SUMS)
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
