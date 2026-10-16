package ooxml

import (
	"fmt"
	"strings"
)

// Tema de presentació propi: 16:9, màster amb estils de text coherents
// (mides 40/20/18/16 pt, vinyetes amb el color d'accent), 9 layouts amb
// marcadors de posició i fons fosc per a portada i seccions (estructura
// «fosc–clar–fosc»). Tot es construeix a partir d'un Theme.

const (
	nsP = "http://schemas.openxmlformats.org/presentationml/2006/main"

	SlideW = 12192000 // 13,333 polzades
	SlideH = 6858000  // 7,5 polzades
	emuIn  = 914400

	ctPresMain    = "application/vnd.openxmlformats-officedocument.presentationml.presentation.main+xml"
	ctSlide       = "application/vnd.openxmlformats-officedocument.presentationml.slide+xml"
	ctSlideLayout = "application/vnd.openxmlformats-officedocument.presentationml.slideLayout+xml"
	ctSlideMaster = "application/vnd.openxmlformats-officedocument.presentationml.slideMaster+xml"
	ctNotesMaster = "application/vnd.openxmlformats-officedocument.presentationml.notesMaster+xml"
	ctNotesSlide  = "application/vnd.openxmlformats-officedocument.presentationml.notesSlide+xml"
	ctTheme       = "application/vnd.openxmlformats-officedocument.theme+xml"
	ctPresProps   = "application/vnd.openxmlformats-officedocument.presentationml.presProps+xml"
	ctViewProps   = "application/vnd.openxmlformats-officedocument.presentationml.viewProps+xml"
	ctTableStyles = "application/vnd.openxmlformats-officedocument.presentationml.tableStyles+xml"
	ctChart       = "application/vnd.openxmlformats-officedocument.drawingml.chart+xml"
)

const xmlHead = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\n"

// Geometria (EMU) compartida pels layouts.
var (
	mX     = 457200 * 1  // marge lateral 0,5"
	contW  = SlideW - 2*mX
	titleY = 380000
	titleH = 1000000
	bodyY  = 1560000
	bodyH  = SlideH - bodyY - 560000
	gap    = 457200 // 0,5"
	halfW  = (contW - gap) / 2
)

func themeXML(t Theme, name string) string {
	shade := func(hex string) string { return hex }
	return xmlHead + `<a:theme xmlns:a="` + nsA + `" name="` + Esc(name) + `"><a:themeElements>` +
		`<a:clrScheme name="Gregal ` + t.Name + `">` +
		`<a:dk1><a:srgbClr val="` + t.Text + `"/></a:dk1><a:lt1><a:srgbClr val="FFFFFF"/></a:lt1>` +
		`<a:dk2><a:srgbClr val="` + t.Dark + `"/></a:dk2><a:lt2><a:srgbClr val="` + t.Light + `"/></a:lt2>` +
		`<a:accent1><a:srgbClr val="` + t.Primary + `"/></a:accent1><a:accent2><a:srgbClr val="` + t.Accent + `"/></a:accent2>` +
		`<a:accent3><a:srgbClr val="` + t.Accent2 + `"/></a:accent3><a:accent4><a:srgbClr val="` + t.Muted + `"/></a:accent4>` +
		`<a:accent5><a:srgbClr val="` + shade(t.Dark) + `"/></a:accent5><a:accent6><a:srgbClr val="A5A5A5"/></a:accent6>` +
		`<a:hlink><a:srgbClr val="` + t.Accent + `"/></a:hlink><a:folHlink><a:srgbClr val="` + t.Muted + `"/></a:folHlink></a:clrScheme>` +
		`<a:fontScheme name="Gregal"><a:majorFont><a:latin typeface="` + t.HeadingFont + `"/><a:ea typeface=""/><a:cs typeface=""/></a:majorFont>` +
		`<a:minorFont><a:latin typeface="` + t.BodyFont + `"/><a:ea typeface=""/><a:cs typeface=""/></a:minorFont></a:fontScheme>` +
		`<a:fmtScheme name="Gregal"><a:fillStyleLst><a:solidFill><a:schemeClr val="phClr"/></a:solidFill><a:solidFill><a:schemeClr val="phClr"/></a:solidFill><a:solidFill><a:schemeClr val="phClr"/></a:solidFill></a:fillStyleLst>` +
		`<a:lnStyleLst><a:ln w="6350"><a:solidFill><a:schemeClr val="phClr"/></a:solidFill></a:ln><a:ln w="12700"><a:solidFill><a:schemeClr val="phClr"/></a:solidFill></a:ln><a:ln w="19050"><a:solidFill><a:schemeClr val="phClr"/></a:solidFill></a:ln></a:lnStyleLst>` +
		`<a:effectStyleLst><a:effectStyle><a:effectLst/></a:effectStyle><a:effectStyle><a:effectLst/></a:effectStyle><a:effectStyle><a:effectLst/></a:effectStyle></a:effectStyleLst>` +
		`<a:bgFillStyleLst><a:solidFill><a:schemeClr val="phClr"/></a:solidFill><a:solidFill><a:schemeClr val="phClr"/></a:solidFill><a:solidFill><a:schemeClr val="phClr"/></a:solidFill></a:bgFillStyleLst></a:fmtScheme>` +
		`</a:themeElements><a:objectDefaults/><a:extraClrSchemeLst/></a:theme>`
}

// sp construeix un marcador de posició (o una forma) amb geometria.
func phSp(id int, name, phType, idx string, x, y, cx, cy int, extra string) string {
	ph := `<p:ph`
	if phType != "" {
		ph += ` type="` + phType + `"`
	}
	if idx != "" {
		ph += ` idx="` + idx + `"`
	}
	ph += `/>`
	geo := ""
	if cx > 0 {
		geo = fmt.Sprintf(`<a:xfrm><a:off x="%d" y="%d"/><a:ext cx="%d" cy="%d"/></a:xfrm>`, x, y, cx, cy)
	}
	if extra == "" {
		extra = `<a:bodyPr/><a:lstStyle/><a:p><a:endParaRPr lang="ca-ES"/></a:p>`
	}
	return fmt.Sprintf(`<p:sp><p:nvSpPr><p:cNvPr id="%d" name="%s"/><p:cNvSpPr><a:spLocks noGrp="1"/></p:cNvSpPr><p:nvPr>%s</p:nvPr></p:nvSpPr><p:spPr>%s</p:spPr><p:txBody>%s</p:txBody></p:sp>`,
		id, Esc(name), ph, geo, extra)
}

const spTreeHead = `<p:nvGrpSpPr><p:cNvPr id="1" name=""/><p:cNvGrpSpPr/><p:nvPr/></p:nvGrpSpPr><p:grpSpPr><a:xfrm><a:off x="0" y="0"/><a:ext cx="0" cy="0"/><a:chOff x="0" y="0"/><a:chExt cx="0" cy="0"/></a:xfrm></p:grpSpPr>`

func lvlStyle(lvl, sz int, color, extra string) string {
	return fmt.Sprintf(`<a:lvl%dpPr%s><a:defRPr sz="%d"><a:solidFill><a:schemeClr val="%s"/></a:solidFill></a:defRPr></a:lvl%dpPr>`, lvl, extra, sz, color, lvl)
}

// masterLogo desa el logo del tema per al màster i en torna la forma: a
// baix a l'esquerra, a la mateixa alçada que el número de diapositiva, i
// per tant a totes les diapositives que mostren les formes del màster.
func masterLogo(pk *Package, master string, t Theme) string {
	const h = 304800
	cx, cy, ok := logoSize(t, h)
	if !ok {
		return ""
	}
	if maxW := int64(contW) / 4; cx > maxW {
		cy, cx = cy*maxW/cx, maxW
	}
	part := "ppt/media/brand-logo." + t.LogoExt
	pk.Put(part, t.Logo)
	rid := pk.AddRel(master, RelImage, part)
	return fmt.Sprintf(`<p:pic><p:nvPicPr><p:cNvPr id="9" name="Logo"/><p:cNvPicPr><a:picLocks noChangeAspect="1"/></p:cNvPicPr><p:nvPr userDrawn="1"/></p:nvPicPr><p:blipFill><a:blip r:embed="%s"/><a:stretch><a:fillRect/></a:stretch></p:blipFill><p:spPr><a:xfrm><a:off x="%d" y="%d"/><a:ext cx="%d" cy="%d"/></a:xfrm><a:prstGeom prst="rect"><a:avLst/></a:prstGeom></p:spPr></p:pic>`,
		rid, mX, SlideH-457200, cx, cy)
}

func masterXML(t Theme, layoutRIDs []string, logo string) string {
	var ids strings.Builder
	for i, r := range layoutRIDs {
		ids.WriteString(fmt.Sprintf(`<p:sldLayoutId id="%d" r:id="%s"/>`, 2147483649+i, r))
	}
	bullet := func(lvl, marL, indent, sz int, ch string) string {
		return fmt.Sprintf(`<a:lvl%dpPr marL="%d" indent="%d" algn="l" defTabSz="914400" rtl="0" eaLnBrk="1" latinLnBrk="0" hangingPunct="1"><a:lnSpc><a:spcPct val="100000"/></a:lnSpc><a:spcBef><a:spcPts val="1200"/></a:spcBef><a:buClr><a:schemeClr val="accent2"/></a:buClr><a:buFont typeface="Arial"/><a:buChar char="%s"/><a:defRPr sz="%d" kern="1200"><a:solidFill><a:schemeClr val="tx1"/></a:solidFill><a:latin typeface="+mn-lt"/><a:ea typeface="+mn-ea"/><a:cs typeface="+mn-cs"/></a:defRPr></a:lvl%dpPr>`,
			lvl, marL, indent, ch, sz, lvl)
	}
	var body strings.Builder
	body.WriteString(bullet(1, 342900, -342900, 2200, "•"))
	body.WriteString(bullet(2, 742950, -285750, 1900, "–"))
	body.WriteString(bullet(3, 1143000, -228600, 1700, "•"))
	for l := 4; l <= 9; l++ {
		body.WriteString(bullet(l, 1085850+(l-3)*400000, -228600, 1600, "•"))
	}
	return xmlHead + `<p:sldMaster xmlns:a="` + nsA + `" xmlns:r="` + nsR + `" xmlns:p="` + nsP + `"><p:cSld>` +
		`<p:bg><p:bgRef idx="1001"><a:schemeClr val="bg1"/></p:bgRef></p:bg><p:spTree>` + spTreeHead +
		phSp(2, "Title Placeholder 1", "title", "", mX, titleY, contW, titleH, `<a:bodyPr vert="horz" lIns="0" tIns="45720" rIns="0" bIns="45720" rtlCol="0" anchor="b"><a:normAutofit/></a:bodyPr><a:lstStyle/><a:p><a:r><a:rPr lang="ca-ES"/><a:t>Títol</a:t></a:r></a:p>`) +
		phSp(3, "Text Placeholder 2", "body", "1", mX, bodyY, contW, bodyH, `<a:bodyPr vert="horz" lIns="0" tIns="45720" rIns="0" bIns="45720" rtlCol="0"><a:normAutofit/></a:bodyPr><a:lstStyle/><a:p><a:pPr lvl="0"/><a:r><a:rPr lang="ca-ES"/><a:t>Text</a:t></a:r></a:p>`) +
		phSp(4, "Slide Number Placeholder 3", "sldNum", "12", SlideW-mX-914400, SlideH-457200, 914400, 304800, `<a:bodyPr vert="horz" lIns="0" tIns="0" rIns="0" bIns="0" anchor="ctr"/><a:lstStyle><a:lvl1pPr algn="r"><a:defRPr sz="1100"><a:solidFill><a:schemeClr val="accent4"/></a:solidFill></a:defRPr></a:lvl1pPr></a:lstStyle><a:p><a:fld id="{B6F15528-21DE-4FAA-801E-634DDDAF4B2B}" type="slidenum"><a:rPr lang="ca-ES"/><a:t>‹#›</a:t></a:fld><a:endParaRPr lang="ca-ES"/></a:p>`) +
		logo +
		`</p:spTree></p:cSld>` +
		`<p:clrMap bg1="lt1" tx1="dk1" bg2="lt2" tx2="dk2" accent1="accent1" accent2="accent2" accent3="accent3" accent4="accent4" accent5="accent5" accent6="accent6" hlink="hlink" folHlink="folHlink"/>` +
		`<p:sldLayoutIdLst>` + ids.String() + `</p:sldLayoutIdLst>` +
		`<p:txStyles><p:titleStyle><a:lvl1pPr algn="l" defTabSz="914400" rtl="0" eaLnBrk="1" latinLnBrk="0" hangingPunct="1"><a:lnSpc><a:spcPct val="90000"/></a:lnSpc><a:spcBef><a:spcPct val="0"/></a:spcBef><a:buNone/><a:defRPr sz="3600" b="1" kern="1200"><a:solidFill><a:schemeClr val="tx2"/></a:solidFill><a:latin typeface="+mj-lt"/><a:ea typeface="+mj-ea"/><a:cs typeface="+mj-cs"/></a:defRPr></a:lvl1pPr></p:titleStyle>` +
		`<p:bodyStyle>` + body.String() + `</p:bodyStyle>` +
		`<p:otherStyle><a:defPPr><a:defRPr lang="ca-ES"/></a:defPPr><a:lvl1pPr marL="0" algn="l" defTabSz="914400" rtl="0" eaLnBrk="1" latinLnBrk="0" hangingPunct="1"><a:defRPr sz="1800" kern="1200"><a:solidFill><a:schemeClr val="tx1"/></a:solidFill><a:latin typeface="+mn-lt"/><a:ea typeface="+mn-ea"/><a:cs typeface="+mn-cs"/></a:defRPr></a:lvl1pPr></p:otherStyle>` +
		`</p:txStyles></p:sldMaster>`
}

// layoutDef descriu un layout propi.
type layoutDef struct {
	Name, Type string
	Dark       bool
	Shapes     func() string
}

func noBullets(sz int, color string, algn string, bold bool) string {
	b := ""
	if bold {
		b = ` b="1"`
	}
	return fmt.Sprintf(`<a:lstStyle><a:lvl1pPr marL="0" indent="0" algn="%s"><a:buNone/><a:defRPr sz="%d"%s><a:solidFill><a:schemeClr val="%s"/></a:solidFill></a:defRPr></a:lvl1pPr></a:lstStyle>`, algn, sz, b, color)
}

func layoutDefs() []layoutDef {
	ph := phSp
	body := func(lst string, anchor string) string {
		if anchor == "" {
			anchor = "t"
		}
		return `<a:bodyPr lIns="0" rIns="0" anchor="` + anchor + `"><a:normAutofit/></a:bodyPr>` + lst + `<a:p><a:pPr lvl="0"/><a:r><a:rPr lang="ca-ES"/><a:t>Text</a:t></a:r></a:p>`
	}
	title := func(lst string, anchor string) string {
		if anchor == "" {
			anchor = "b"
		}
		return `<a:bodyPr lIns="0" rIns="0" anchor="` + anchor + `"><a:normAutofit/></a:bodyPr>` + lst + `<a:p><a:r><a:rPr lang="ca-ES"/><a:t>Títol</a:t></a:r></a:p>`
	}
	sldNum := ph(9, "Slide Number", "sldNum", "12", 0, 0, 0, 0, `<a:bodyPr/><a:lstStyle/><a:p><a:fld id="{B6F15528-21DE-4FAA-801E-634DDDAF4B2B}" type="slidenum"><a:rPr lang="ca-ES"/><a:t>‹#›</a:t></a:fld><a:endParaRPr lang="ca-ES"/></a:p>`)
	return []layoutDef{
		{Name: "Title Slide", Type: "title", Dark: true, Shapes: func() string {
			return ph(2, "Title 1", "ctrTitle", "", mX+457200, 1900000, contW-914400, 1700000, title(noBullets(4400, "bg1", "l", true), "b")) +
				ph(3, "Subtitle 2", "subTitle", "1", mX+457200, 3750000, contW-914400, 1100000, body(noBullets(2000, "accent2", "l", false), "t"))
		}},
		{Name: "Title and Content", Type: "obj", Shapes: func() string {
			return ph(2, "Title 1", "title", "", 0, 0, 0, 0, "") + ph(3, "Content 2", "body", "1", 0, 0, 0, 0, "") + sldNum
		}},
		{Name: "Section Header", Type: "secHead", Dark: true, Shapes: func() string {
			return ph(2, "Title 1", "title", "", mX+457200, 2300000, contW-914400, 1300000, title(noBullets(4000, "bg1", "l", true), "b")) +
				ph(3, "Text 2", "body", "1", mX+457200, 3700000, contW-914400, 900000, body(noBullets(1800, "accent2", "l", false), "t"))
		}},
		{Name: "Two Content", Type: "twoObj", Shapes: func() string {
			return ph(2, "Title 1", "title", "", 0, 0, 0, 0, "") +
				ph(3, "Content 2", "body", "1", mX, bodyY, halfW, bodyH, body("<a:lstStyle/>", "t")) +
				ph(4, "Content 3", "body", "2", mX+halfW+gap, bodyY, halfW, bodyH, body("<a:lstStyle/>", "t")) + sldNum
		}},
		{Name: "Comparison", Type: "twoTxTwoObj", Shapes: func() string {
			hh := 520000
			return ph(2, "Title 1", "title", "", 0, 0, 0, 0, "") +
				ph(3, "Text 2", "body", "1", mX, bodyY, halfW, hh, body(noBullets(2000, "accent1", "l", true), "b")) +
				ph(4, "Content 3", "body", "2", mX, bodyY+hh+100000, halfW, bodyH-hh-100000, body("<a:lstStyle/>", "t")) +
				ph(5, "Text 4", "body", "3", mX+halfW+gap, bodyY, halfW, hh, body(noBullets(2000, "accent1", "l", true), "b")) +
				ph(6, "Content 5", "body", "4", mX+halfW+gap, bodyY+hh+100000, halfW, bodyH-hh-100000, body("<a:lstStyle/>", "t")) + sldNum
		}},
		{Name: "Big Number", Type: "obj", Shapes: func() string {
			return ph(2, "Title 1", "title", "", 0, 0, 0, 0, "") +
				ph(3, "Number 2", "body", "1", mX, 2000000, contW, 1900000, body(noBullets(8800, "accent2", "ctr", true), "b")) +
				ph(4, "Caption 3", "body", "2", mX+914400, 4000000, contW-1828800, 1200000, body(noBullets(2000, "accent4", "ctr", false), "t")) + sldNum
		}},
		{Name: "Picture with Caption", Type: "picTx", Shapes: func() string {
			return ph(2, "Title 1", "title", "", 0, 0, 0, 0, "") +
				ph(3, "Picture 2", "pic", "1", mX, bodyY, int(float64(contW)*0.58), bodyH, `<a:bodyPr/><a:lstStyle/><a:p><a:endParaRPr lang="ca-ES"/></a:p>`) +
				ph(4, "Text 3", "body", "2", mX+int(float64(contW)*0.58)+gap, bodyY, contW-int(float64(contW)*0.58)-gap, bodyH, body("<a:lstStyle/>", "t")) + sldNum
		}},
		{Name: "Title Only", Type: "titleOnly", Shapes: func() string {
			return ph(2, "Title 1", "title", "", 0, 0, 0, 0, "") + sldNum
		}},
		{Name: "Blank", Type: "blank", Shapes: func() string { return sldNum }},
	}
}

func layoutXML(l layoutDef) string {
	bg := ""
	if l.Dark {
		bg = `<p:bg><p:bgPr><a:solidFill><a:schemeClr val="tx2"/></a:solidFill><a:effectLst/></p:bgPr></p:bg>`
	}
	return xmlHead + `<p:sldLayout xmlns:a="` + nsA + `" xmlns:r="` + nsR + `" xmlns:p="` + nsP + `" type="` + l.Type + `" preserve="1"><p:cSld name="` + Esc(l.Name) + `">` + bg +
		`<p:spTree>` + spTreeHead + l.Shapes() + `</p:spTree></p:cSld><p:clrMapOvr><a:masterClrMapping/></p:clrMapOvr></p:sldLayout>`
}

func notesMasterXML() string {
	return xmlHead + `<p:notesMaster xmlns:a="` + nsA + `" xmlns:r="` + nsR + `" xmlns:p="` + nsP + `"><p:cSld><p:bg><p:bgRef idx="1001"><a:schemeClr val="bg1"/></p:bgRef></p:bg><p:spTree>` + spTreeHead +
		phSp(2, "Slide Image Placeholder 1", "sldImg", "", 381000, 685800, 6096000, 3429000, `<a:bodyPr/><a:lstStyle/><a:p><a:endParaRPr lang="ca-ES"/></a:p>`) +
		phSp(3, "Notes Placeholder 2", "body", "1", 685800, 4343400, 5486400, 4114800, `<a:bodyPr/><a:lstStyle/><a:p><a:r><a:rPr lang="ca-ES"/><a:t>Notes</a:t></a:r></a:p>`) +
		`</p:spTree></p:cSld><p:clrMap bg1="lt1" tx1="dk1" bg2="lt2" tx2="dk2" accent1="accent1" accent2="accent2" accent3="accent3" accent4="accent4" accent5="accent5" accent6="accent6" hlink="hlink" folHlink="folHlink"/>` +
		`<p:notesStyle><a:lvl1pPr marL="0" algn="l" defTabSz="914400" rtl="0" eaLnBrk="1" latinLnBrk="0" hangingPunct="1"><a:defRPr sz="1200" kern="1200"><a:solidFill><a:schemeClr val="tx1"/></a:solidFill><a:latin typeface="+mn-lt"/><a:ea typeface="+mn-ea"/><a:cs typeface="+mn-cs"/></a:defRPr></a:lvl1pPr></p:notesStyle></p:notesMaster>`
}

// NewPptxPackage crea un paquet de presentació buit (sense diapositives)
// amb el tema, el màster i els layouts propis.
func NewPptxPackage(t Theme, title string) *Package {
	pk := New()
	pk.Put(ctPart, []byte(xmlHead+`<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">`+
		`<Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>`+
		`<Default Extension="xml" ContentType="application/xml"/>`+
		`<Default Extension="png" ContentType="image/png"/><Default Extension="jpeg" ContentType="image/jpeg"/><Default Extension="jpg" ContentType="image/jpeg"/><Default Extension="gif" ContentType="image/gif"/>`+
		`</Types>`))
	pres := "ppt/presentation.xml"
	pk.SetOverride(pres, ctPresMain)
	pk.AddRel("", RelOfficeDoc, pres)
	pk.AddRel("", RelCoreProps, "docProps/core.xml")
	pk.AddRel("", RelExtProps, "docProps/app.xml")
	pk.Put("docProps/core.xml", []byte(coreProps(title, "")))
	pk.SetOverride("docProps/core.xml", ctCore)
	pk.Put("docProps/app.xml", []byte(xmlHead+`<Properties xmlns="http://schemas.openxmlformats.org/officeDocument/2006/extended-properties"><Application>Gregal</Application></Properties>`))
	pk.SetOverride("docProps/app.xml", ctApp)

	// Tema i màster.
	pk.Put("ppt/theme/theme1.xml", []byte(themeXML(t, "Gregal")))
	pk.SetOverride("ppt/theme/theme1.xml", ctTheme)
	master := "ppt/slideMasters/slideMaster1.xml"
	defs := layoutDefs()
	var rids []string
	for i, l := range defs {
		lp := fmt.Sprintf("ppt/slideLayouts/slideLayout%d.xml", i+1)
		pk.Put(lp, []byte(layoutXML(l)))
		pk.SetOverride(lp, ctSlideLayout)
		pk.AddRel(lp, RelSlideMaster, master)
		rids = append(rids, pk.AddRel(master, RelSlideLayout, lp))
	}
	pk.AddRel(master, RelTheme, "ppt/theme/theme1.xml")
	pk.Put(master, []byte(masterXML(t, rids, masterLogo(pk, master, t))))
	pk.SetOverride(master, ctSlideMaster)

	// Notes.
	pk.Put("ppt/theme/theme2.xml", []byte(themeXML(t, "Gregal Notes")))
	pk.SetOverride("ppt/theme/theme2.xml", ctTheme)
	pk.Put("ppt/notesMasters/notesMaster1.xml", []byte(notesMasterXML()))
	pk.SetOverride("ppt/notesMasters/notesMaster1.xml", ctNotesMaster)
	pk.AddRel("ppt/notesMasters/notesMaster1.xml", RelTheme, "ppt/theme/theme2.xml")

	pk.Put("ppt/presProps.xml", []byte(xmlHead+`<p:presentationPr xmlns:a="`+nsA+`" xmlns:r="`+nsR+`" xmlns:p="`+nsP+`"/>`))
	pk.SetOverride("ppt/presProps.xml", ctPresProps)
	pk.Put("ppt/viewProps.xml", []byte(xmlHead+`<p:viewPr xmlns:a="`+nsA+`" xmlns:r="`+nsR+`" xmlns:p="`+nsP+`"><p:normalViewPr><p:restoredLeft sz="15620"/><p:restoredTop sz="94660"/></p:normalViewPr><p:gridSpacing cx="76200" cy="76200"/></p:viewPr>`))
	pk.SetOverride("ppt/viewProps.xml", ctViewProps)
	pk.Put("ppt/tableStyles.xml", []byte(xmlHead+`<a:tblStyleLst xmlns:a="`+nsA+`" def="{5C22544A-7EE6-4342-B048-85BDC9FD1C3A}"/>`))
	pk.SetOverride("ppt/tableStyles.xml", ctTableStyles)

	mrid := pk.AddRel(pres, RelSlideMaster, master)
	nrid := pk.AddRel(pres, RelNotesMaster, "ppt/notesMasters/notesMaster1.xml")
	pk.AddRel(pres, RelPresProps, "ppt/presProps.xml")
	pk.AddRel(pres, RelViewProps, "ppt/viewProps.xml")
	pk.AddRel(pres, RelTheme, "ppt/theme/theme1.xml")
	pk.AddRel(pres, RelTableStyles, "ppt/tableStyles.xml")
	pk.Put(pres, []byte(xmlHead+`<p:presentation xmlns:a="`+nsA+`" xmlns:r="`+nsR+`" xmlns:p="`+nsP+`" saveSubsetFonts="1">`+
		`<p:sldMasterIdLst><p:sldMasterId id="2147483648" r:id="`+mrid+`"/></p:sldMasterIdLst>`+
		`<p:notesMasterIdLst><p:notesMasterId r:id="`+nrid+`"/></p:notesMasterIdLst>`+
		`<p:sldIdLst/>`+
		fmt.Sprintf(`<p:sldSz cx="%d" cy="%d"/><p:notesSz cx="6858000" cy="9144000"/>`, SlideW, SlideH)+
		`<p:defaultTextStyle><a:defPPr><a:defRPr lang="ca-ES"/></a:defPPr></p:defaultTextStyle></p:presentation>`))
	return pk
}
