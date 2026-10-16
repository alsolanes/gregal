package officerender

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"os"
	"strconv"
)

// ContactSheet compon una graella de miniatures numerades (una sola
// imatge = poc context per al model) i la desa com a PNG.
func ContactSheet(pngs []string, labels []int, cols, thumbW int, out string) error {
	if len(pngs) == 0 {
		return fmt.Errorf("cap imatge")
	}
	if cols <= 0 {
		cols = 3
	}
	if len(pngs) < cols {
		cols = len(pngs)
	}
	var thumbs []image.Image
	maxH := 0
	for _, p := range pngs {
		img, err := loadPNG(p)
		if err != nil {
			return err
		}
		t := Scale(img, thumbW)
		thumbs = append(thumbs, t)
		if h := t.Bounds().Dy(); h > maxH {
			maxH = h
		}
	}
	const pad, labelH = 16, 26
	rows := (len(thumbs) + cols - 1) / cols
	W := cols*thumbW + (cols+1)*pad
	H := rows*(maxH+labelH) + (rows+1)*pad
	dst := image.NewRGBA(image.Rect(0, 0, W, H))
	draw.Draw(dst, dst.Bounds(), &image.Uniform{color.RGBA{0xE6, 0xE8, 0xEC, 0xFF}}, image.Point{}, draw.Src)
	for i, t := range thumbs {
		r, c := i/cols, i%cols
		x := pad + c*(thumbW+pad)
		y := pad + r*(maxH+labelH+pad)
		n := i + 1
		if i < len(labels) {
			n = labels[i]
		}
		drawNumber(dst, x, y+4, strconv.Itoa(n), color.RGBA{0x33, 0x33, 0x33, 0xFF})
		b := t.Bounds()
		rect := image.Rect(x, y+labelH, x+b.Dx(), y+labelH+b.Dy())
		// vora fina
		draw.Draw(dst, rect.Inset(-1), &image.Uniform{color.RGBA{0xB0, 0xB4, 0xBA, 0xFF}}, image.Point{}, draw.Src)
		draw.Draw(dst, rect, t, b.Min, draw.Src)
	}
	f, err := os.Create(out)
	if err != nil {
		return err
	}
	defer f.Close()
	return png.Encode(f, dst)
}

func loadPNG(p string) (image.Image, error) {
	f, err := os.Open(p)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	return img, err
}

// Scale redueix (mitjana de caixa) una imatge a l'amplada w.
func Scale(src image.Image, w int) image.Image {
	b := src.Bounds()
	if b.Dx() <= w || w <= 0 {
		return src
	}
	h := b.Dy() * w / b.Dx()
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	fx := float64(b.Dx()) / float64(w)
	fy := float64(b.Dy()) / float64(h)
	for y := 0; y < h; y++ {
		y0 := b.Min.Y + int(float64(y)*fy)
		y1 := b.Min.Y + int(float64(y+1)*fy)
		if y1 <= y0 {
			y1 = y0 + 1
		}
		for x := 0; x < w; x++ {
			x0 := b.Min.X + int(float64(x)*fx)
			x1 := b.Min.X + int(float64(x+1)*fx)
			if x1 <= x0 {
				x1 = x0 + 1
			}
			var r, g, bl, a, n uint32
			for yy := y0; yy < y1; yy++ {
				for xx := x0; xx < x1; xx++ {
					cr, cg, cb, ca := src.At(xx, yy).RGBA()
					r, g, bl, a = r+cr, g+cg, bl+cb, a+ca
					n++
				}
			}
			dst.Set(x, y, color.RGBA64{uint16(r / n), uint16(g / n), uint16(bl / n), uint16(a / n)})
		}
	}
	return dst
}

// SavePNG desa una imatge.
func SavePNG(img image.Image, out string) error {
	f, err := os.Create(out)
	if err != nil {
		return err
	}
	defer f.Close()
	return png.Encode(f, img)
}

// Font de 5×7 per als números de pàgina (sense dependències).
var digits = map[rune][7]string{
	'0': {"01110", "10001", "10011", "10101", "11001", "10001", "01110"},
	'1': {"00100", "01100", "00100", "00100", "00100", "00100", "01110"},
	'2': {"01110", "10001", "00001", "00010", "00100", "01000", "11111"},
	'3': {"11110", "00001", "00001", "01110", "00001", "00001", "11110"},
	'4': {"00010", "00110", "01010", "10010", "11111", "00010", "00010"},
	'5': {"11111", "10000", "11110", "00001", "00001", "10001", "01110"},
	'6': {"00110", "01000", "10000", "11110", "10001", "10001", "01110"},
	'7': {"11111", "00001", "00010", "00100", "01000", "01000", "01000"},
	'8': {"01110", "10001", "10001", "01110", "10001", "10001", "01110"},
	'9': {"01110", "10001", "10001", "01111", "00001", "00010", "01100"},
}

func drawNumber(dst *image.RGBA, x, y int, s string, c color.Color) {
	const scale = 3
	for _, r := range s {
		g, ok := digits[r]
		if !ok {
			continue
		}
		for row, line := range g {
			for col, bit := range line {
				if bit != '1' {
					continue
				}
				for dy := 0; dy < scale; dy++ {
					for dx := 0; dx < scale; dx++ {
						dst.Set(x+col*scale+dx, y+row*scale+dy, c)
					}
				}
			}
		}
		x += 6 * scale
	}
}
