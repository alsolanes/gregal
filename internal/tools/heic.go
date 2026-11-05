//go:build !android

package tools

import (
	"bytes"
	"fmt"
	"image"
	"image/jpeg"

	"github.com/gen2brain/heic"
	"golang.org/x/image/draw"
)

// Les fotos de l'iPhone (i de molts Android) són HEIC/HEIF. Cap model les
// accepta i els navegadors (fora de Safari) no les saben pintar, així que es
// converteixen a JPEG al servidor, en un sol lloc (DataURLForBytes): així val
// per a la web, l'escriptori, l'app d'Android i Telegram alhora.

// heicMaxSide és el costat màxim després de convertir: una foto de 12 MP
// passa a ~2,5 MP, prou per veure-hi detall i molt menys pes per al model.
const heicMaxSide = 2560

// IsHEIF mira la caixa ftyp de l'ISO BMFF: «ftyp» a 4..8 i una marca HEIF.
func IsHEIF(raw []byte) bool {
	if len(raw) < 12 || string(raw[4:8]) != "ftyp" {
		return false
	}
	switch string(raw[8:12]) {
	case "heic", "heix", "hevc", "hevx", "heim", "heis", "hevm", "hevs", "mif1", "msf1":
		return true
	}
	return false
}

// HEIFToJPEG descodifica una imatge HEIC/HEIF i la torna en JPEG (qualitat
// 85), reduïda si fa falta.
func HEIFToJPEG(raw []byte) ([]byte, error) {
	img, err := heic.Decode(bytes.NewReader(raw))
	if err != nil {
		return nil, fmt.Errorf("no puc llegir la foto HEIC: %w", err)
	}
	b := img.Bounds()
	if w, h := b.Dx(), b.Dy(); w > heicMaxSide || h > heicMaxSide {
		scale := float64(heicMaxSide) / float64(max(w, h))
		dst := image.NewRGBA(image.Rect(0, 0, max(1, int(float64(w)*scale)), max(1, int(float64(h)*scale))))
		draw.CatmullRom.Scale(dst, dst.Bounds(), img, b, draw.Src, nil)
		img = dst
	}
	var out bytes.Buffer
	if err := jpeg.Encode(&out, img, &jpeg.Options{Quality: 85}); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}
