package transcript

import "gregal/internal/brand"

// ActiveBrand és la identitat del branding actiu per a una exportació HTML
// (nil = cap branding actiu).
func ActiveBrand() *Brand {
	p := brand.ActivePack()
	if p == nil {
		return nil
	}
	return &Brand{Name: p.Name, Logo: p.Logo, Accent: p.Colors.Primary, Font: p.BodyFontCSS(), HeadingFont: p.HeadingFontCSS()}
}
