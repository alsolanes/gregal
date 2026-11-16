package web

import (
	"context"
	"strings"
	"testing"
)

func fakeCardImages(t *testing.T, found map[string]string) *[]string {
	t.Helper()
	var asked []string
	old := cardImageSearch
	cardImageSearch = func(_ context.Context, q string, _ int) (string, error) {
		asked = append(asked, q)
		if u, ok := found[q]; ok {
			return "1. " + q + "\n   image: " + u + "\n   page: https://commons.wikimedia.org/x\n", nil
		}
		return "No images found.", nil
	}
	t.Cleanup(func() { cardImageSearch = old })
	return &asked
}

func TestCardWithoutPhotoGetsOneFromImageQuery(t *testing.T) {
	asked := fakeCardImages(t, map[string]string{"Montserrat monastery": "https://upload.wikimedia.org/m.jpg"})
	s := &Server{}
	in := "Aquí el tens.\n\n```gregal-plan {\"title\":\"Un matí a Montserrat\",\"image_query\":\"Montserrat monastery\",\"items\":[{\"time\":\"08:36\",\"title\":\"R5 des d'Espanya\"}]}\n```\nBon viatge."
	out := s.windCardImages(in)
	if !strings.Contains(out, `"image":"https://upload.wikimedia.org/m.jpg"`) || strings.Contains(out, "image_query") {
		t.Fatalf("photo not set or query left:\n%s", out)
	}
	if !strings.HasPrefix(out, "Aquí el tens.") || !strings.HasSuffix(out, "Bon viatge.") || !strings.Contains(out, "```gregal-plan\n{") {
		t.Fatalf("text around the card changed:\n%s", out)
	}
	if strings.Join(*asked, "|") != "Montserrat monastery" {
		t.Fatalf("asked %v", *asked)
	}
}

func TestCardPhotoFallsBackToTitleAndInventedURLsGo(t *testing.T) {
	fakeCardImages(t, map[string]string{"Cigrons amb espinacs": "https://upload.wikimedia.org/c.jpg"})
	s := &Server{}
	out := s.windCardImages("```gregal-recipe\n{\"title\":\"Cigrons amb espinacs\",\"image\":\"/api/winds/image?id=placeholder\",\"steps\":[\"Bull\"]}\n```")
	if !strings.Contains(out, "https://upload.wikimedia.org/c.jpg") || strings.Contains(out, "placeholder") {
		t.Fatalf("%s", out)
	}
}

func TestCardKeepsItsPhotoAndOptionsOnlyAskWhenTold(t *testing.T) {
	asked := fakeCardImages(t, map[string]string{})
	s := &Server{}
	keep := "```gregal-plan\n{\"title\":\"Girona\",\"image\":\"https://e.com/g.jpg\",\"items\":[{\"title\":\"Tren\"}]}\n```"
	if out := s.windCardImages(keep); !strings.Contains(out, "https://e.com/g.jpg") {
		t.Fatalf("%s", out)
	}
	s.windCardImages("```gregal-result\n{\"title\":\"Portàtils\",\"rows\":[{\"lead\":\"A\"}]}\n```")
	if len(*asked) != 0 {
		t.Fatalf("searched for %v", *asked)
	}
	// Sense foto enlloc: la targeta queda igual (sense camps nous) i el text no es perd.
	out := s.windCardImages("Res:\n```gregal-plan\n{\"title\":\"Lloc rar\",\"items\":[{\"title\":\"X\"}]}\n```")
	if strings.Contains(out, `"image"`) || !strings.HasPrefix(out, "Res:") {
		t.Fatalf("%s", out)
	}
	if bad := "```gregal-plan\nno és json\n```"; s.windCardImages(bad) != bad {
		t.Fatal("broken JSON must stay as written")
	}
}

func TestTextPhotoLineGetsARealPhotoOrGoes(t *testing.T) {
	asked := fakeCardImages(t, map[string]string{"Sagrada Familia Barcelona": "https://upload.wikimedia.org/s.jpg"})
	s := &Server{}
	in := "Gaudí va ser l'arquitecte del modernisme.\n\n![Sagrada Família](image: Sagrada Familia Barcelona)\n\nVes-hi primer.\n\n![Casa rara](image:Casa que no existeix)\n\nFi."
	out := s.windCardImages(in)
	if !strings.Contains(out, "![Sagrada Família](https://upload.wikimedia.org/s.jpg)") {
		t.Fatalf("photo not resolved:\n%s", out)
	}
	if strings.Contains(out, "image:") || strings.Contains(out, "Casa rara") || strings.Contains(out, "\n\n\n") {
		t.Fatalf("unresolved photo line must go cleanly:\n%s", out)
	}
	if !strings.HasSuffix(out, "Ves-hi primer.\n\nFi.") {
		t.Fatalf("text around changed:\n%s", out)
	}
	if len(*asked) != 2 {
		t.Fatalf("asked %v", *asked)
	}
	if plain := "Sense fotos. [enllaç](https://e.com)"; s.windCardImages(plain) != plain {
		t.Fatal("plain text must stay as written")
	}
}

func TestCardWithARepeatedRestartIsMended(t *testing.T) {
	fakeCardImages(t, map[string]string{"Girona old town": "https://upload.wikimedia.org/g.jpg"})
	s := &Server{}
	// Tal com el va tornar deepseek: comença un element, el deixa a mitges i el torna a escriure sencer.
	in := "```gregal-plan {\"title\":\"Girona\",\"image_query\":\"Girona old town\",\"items\":[{\"time\":\"10:00\",\"title\":\"Banys\"},{\"time\":\"15:30\",\"title\":\"Cases de l'Onyar\",\"detail\":\"Fotos des del Pont Eiffel, {\"time\":\"15:30\",\"title\":\"Cases de l'Onyar\",\"detail\":\"Fotos des del Pont Eiffel, 20 min\"},{\"time\":\"16:15\",\"title\":\"Plaça\"}]}\n```"
	out := s.windCardImages(in)
	if !strings.Contains(out, "https://upload.wikimedia.org/g.jpg") || strings.Count(out, "Cases de l'Onyar") != 1 || !strings.Contains(out, "Eiffel, 20 min") || !strings.Contains(out, "Plaça") {
		t.Fatalf("card not mended:\n%s", out)
	}
}

func fakeSubject(t *testing.T, subject string) *int {
	t.Helper()
	calls := 0
	old := subjectPicker
	subjectPicker = func(_ *Server, _ context.Context, _, _ string, _ bool) string { calls++; return cleanSubject(subject) }
	t.Cleanup(func() { subjectPicker = old })
	return &calls
}

func TestAnswerAboutSomethingGetsItsPhotoAfterTheFirstParagraph(t *testing.T) {
	fakeCardImages(t, map[string]string{"Sagrada Familia Barcelona": "https://upload.wikimedia.org/s.jpg"})
	fakeSubject(t, "\"Sagrada Familia Barcelona\".")
	s := &Server{}
	in := "Antoni Gaudí (1852-1926) va ser l'arquitecte del modernisme català, amb arcs parabòlics i trencadís.\n\nComença per la **Sagrada Família**: és la que explica totes les altres, i ves-hi a primera hora."
	out := s.windSubjectPhoto("Qui era Gaudí?", in)
	want := "trencadís.\n\n![Sagrada Familia Barcelona](https://upload.wikimedia.org/s.jpg)\n\nComença"
	if !strings.Contains(out, want) {
		t.Fatalf("photo not after the first paragraph:\n%s", out)
	}
}

func TestSubjectPhotoOnlyWhenItFits(t *testing.T) {
	fakeCardImages(t, map[string]string{"Girona": "https://upload.wikimedia.org/g.jpg"})
	calls := fakeSubject(t, "NONE")
	s := &Server{}
	long := strings.Repeat("Un consell general sobre com organitzar-te la setmana. ", 5)
	if out := s.windSubjectPhoto("com m'organitzo?", long); out != long {
		t.Fatalf("NONE must leave it alone:\n%s", out)
	}
	// Curta, amb foto ja posada o amb targeta: ni es pregunta.
	*calls = 0
	s.windSubjectPhoto("hola", "Hola! Com va?")
	s.windSubjectPhoto("Girona", long+"\n\n![Girona](https://e.com/g.jpg)")
	s.windSubjectPhoto("pla", long+"\n\n```gregal-plan\n{}\n```")
	if *calls != 0 {
		t.Fatalf("asked the picker %d times", *calls)
	}
	if cleanSubject("Here is the subject: a very long sentence that goes on") != "" {
		t.Fatal("chatty picker output must be ignored")
	}
}


// La foto ha de ser del que diu la targeta: si la cerca de «Sípia amb pèsols»
// només troba farcellets de col, la recepta queda sense foto.
func TestCardPhotoMustMatchTheDish(t *testing.T) {
	old := cardImageSearch
	cardImageSearch = func(_ context.Context, q string, _ int) (string, error) {
		return "1. Cabbage rolls (Sarma)\n   image: https://upload.wikimedia.org/Sarma_dish.jpg\n   page: https://commons.wikimedia.org/x\n" +
			"2. Stuffed peppers\n   image: https://upload.wikimedia.org/Peppers.jpg\n   page: https://commons.wikimedia.org/y\n", nil
	}
	t.Cleanup(func() { cardImageSearch = old })
	s := &Server{}
	out := s.windCardImages("```gregal-recipe\n{\"title\":\"Sípia amb pèsols\",\"steps\":[\"Sofregeix\"]}\n```")
	if strings.Contains(out, "upload.wikimedia.org") {
		t.Fatalf("a wrong photo is worse than none:\n%s", out)
	}
	// I si el segon resultat sí que és sípia amb pèsols, es queda aquest.
	cardImageSearch = func(_ context.Context, q string, _ int) (string, error) {
		return "1. Sarma\n   image: https://upload.wikimedia.org/Sarma.jpg\n   page: x\n" +
			"2. Cuttlefish with peas\n   image: https://upload.wikimedia.org/Sepia_amb_pesols.jpg\n   page: y\n", nil
	}
	out = s.windCardImages("```gregal-recipe\n{\"title\":\"Sípia amb pèsols\",\"image_query\":\"cuttlefish with peas\",\"steps\":[\"Sofregeix\"]}\n```")
	if !strings.Contains(out, "Sepia_amb_pesols.jpg") {
		t.Fatalf("the matching photo should win:\n%s", out)
	}
}

func TestRelevantImageIgnoresFillerWords(t *testing.T) {
	out := "1. Paella valenciana\n   image: https://e.org/p.jpg\n   page: x\n"
	if relevantImage("recepta amb arròs", out) != "" {
		t.Fatal("«recepta» and «amb» say nothing about the photo")
	}
	if relevantImage("Paella de marisc", out) != "https://e.org/p.jpg" {
		t.Fatal("paella matches")
	}
}

func TestImageNeedsTheMainWord(t *testing.T) {
	peas := "1. Green peas\n   image: https://e.org/peas.jpg\n   page: x\n"
	if relevantImage("cuttlefish with peas", peas) != "" {
		t.Fatal("a photo of peas alone is not the dish")
	}
	abbey := "1. Santa Maria de Montserrat Abbey\n   image: https://e.org/m.jpg\n   page: x\n"
	if relevantImage("Montserrat monastery", abbey) == "" {
		t.Fatal("the abbey is Montserrat")
	}
}

// L'adreça d'una foto sola en una línia (o un enllaç a una foto) es mostra com
// a foto; dins d'una frase o d'un bloc de codi, no es toca.
func TestImageLinksOnTheirOwnLineBecomePhotos(t *testing.T) {
	in := "Mira:\n\nhttps://upload.wikimedia.org/a/b/Montserrat.jpg\n\n- [El monestir](https://e.org/m.webp?w=800)\n\nAquí hi ha [una foto](https://e.org/x.png) que comento.\n\n```\nhttps://e.org/codi.png\n```"
	out := embedImageLinks(in)
	for _, want := range []string{"![](https://upload.wikimedia.org/a/b/Montserrat.jpg)", "- ![El monestir](https://e.org/m.webp?w=800)",
		"Aquí hi ha [una foto](https://e.org/x.png) que comento.", "```\nhttps://e.org/codi.png\n```"} {
		if !strings.Contains(out, want) {
			t.Fatalf("falta %q a:\n%s", want, out)
		}
	}
	if same := "Una web: https://e.org/pagina.html"; embedImageLinks(same) != same {
		t.Fatal("una adreça que no és una foto es queda com era")
	}
}

// «Mostra'm gràfics i imatges sobre la rotura»: el model fa un gràfic i una
// web amb un dibuix, però cap foto. Has demanat imatges: hi va la foto real,
// abans del gràfic, encara que hi hagi targetes.
func TestAskedForImagesGetsAPhotoEvenWithCharts(t *testing.T) {
	fakeCardImages(t, map[string]string{"Abdominal external oblique muscle": "https://upload.wikimedia.org/oblic.png"})
	var askedFlag bool
	old := subjectPicker
	subjectPicker = func(_ *Server, _ context.Context, _, _ string, asked bool) string {
		askedFlag = asked
		return "Abdominal external oblique muscle"
	}
	t.Cleanup(func() { subjectPicker = old })
	s := &Server{}
	in := "Aquí tens la fitxa visual de la rotura de l'oblic, amb els graus i els dies de baixa reals dels casos coneguts de l'NBA.\n\n**Quants dies.** Les dades:\n\n```chart\n{\"type\":\"bar\",\"labels\":[\"a\"],\"series\":[{\"name\":\"Dies\",\"values\":[3]}]}\n```\n\n```gregal-web\n{\"title\":\"Oblic\",\"url\":\"/p/0123456789ab/o/index.html\"}\n```"
	out := s.windSubjectPhoto("mostra'm gràfics i imatges sobre aquesta rotura", in)
	if !askedFlag {
		t.Fatal("the picker must know images were asked for")
	}
	photo := "![Abdominal external oblique muscle](https://upload.wikimedia.org/oblic.png)"
	if !strings.Contains(out, photo) || strings.Index(out, photo) > strings.Index(out, "```chart") {
		t.Fatalf("photo missing or after the chart:\n%s", out)
	}
	// Sense demanar imatges, amb un gràfic no s'hi afegeix res (com abans).
	if got := s.windSubjectPhoto("quants dies de baixa?", in); got != in {
		t.Fatalf("without asking, charts keep it as it was:\n%s", got)
	}
	if !askedForImages("Ensenya'm FOTOS del Montseny") || askedForImages("com em dic?") {
		t.Fatal("askedForImages")
	}
}

// El que torna send_file quan el vent t'envia una foto que ha baixat:
// «[foto.jpg](http://…/f/…/foto.jpg) (327 kB)» és una foto, no un enllaç.
func TestSentImageFilesBecomePhotos(t *testing.T) {
	in := "Aquí tens dues més:\n\n[evbuomwan-signatura.jpg](http://127.0.0.1:8097/f/f35cd51641ce8e20eddafdae/evbuomwan-signatura.jpg) (327 kB)\n[evbuomwan-princeton.jpg](http://127.0.0.1:8097/f/abaf7fca3496ede20cc308d6/evbuomwan-princeton.jpg) (1.3 MB)\n[informe.pdf](http://127.0.0.1:8097/f/abaf7fca3496ede20cc308d7/informe.pdf) (40 kB)"
	out := embedImageLinks(in)
	for _, want := range []string{
		"![evbuomwan signatura](http://127.0.0.1:8097/f/f35cd51641ce8e20eddafdae/evbuomwan-signatura.jpg)",
		"![evbuomwan princeton](http://127.0.0.1:8097/f/abaf7fca3496ede20cc308d6/evbuomwan-princeton.jpg)",
		"[informe.pdf](http://127.0.0.1:8097/f/abaf7fca3496ede20cc308d7/informe.pdf) (40 kB)",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("falta %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "(327 kB)") {
		t.Fatalf("the size goes with the photo:\n%s", out)
	}
}

// Comparació: cada cosa porta la seva foto (de image_query o del nom), i
// una adreça inventada fora.
func TestCompareCardGetsAPhotoPerItem(t *testing.T) {
	asked := fakeCardImages(t, map[string]string{"cheetah": "https://upload.wikimedia.org/c.jpg", "Lleopard": "https://upload.wikimedia.org/l.jpg"})
	s := &Server{}
	in := "Les diferències:\n\n```gregal-compare\n{\"title\":\"Guepard i lleopard\",\"items\":[{\"name\":\"Guepard\",\"image_query\":\"cheetah\",\"facts\":{\"Velocitat\":\"110 km/h\"}},{\"name\":\"Lleopard\",\"image\":\"/inventada.jpg\",\"facts\":{\"Velocitat\":\"58 km/h\"}}]}\n```"
	out := s.windCardImages(in)
	if !strings.Contains(out, `"image":"https://upload.wikimedia.org/c.jpg"`) || !strings.Contains(out, `"image":"https://upload.wikimedia.org/l.jpg"`) {
		t.Fatalf("each item needs its photo:\n%s", out)
	}
	if strings.Contains(out, "image_query") || strings.Contains(out, "inventada") {
		t.Fatalf("query or invented URL left:\n%s", out)
	}
	if len(*asked) != 2 {
		t.Fatalf("searches: %v", *asked)
	}
}

// Comparació amb noms en català i sense image_query: es tradueixen d'una
// vegada (Terra → Earth) i cada cosa troba la seva foto.
func TestCompareWithoutQueryAsksForEnglishNames(t *testing.T) {
	fakeCardImages(t, map[string]string{"Earth": "https://upload.wikimedia.org/earth.jpg", "Mars": "https://upload.wikimedia.org/mars.jpg"})
	var asked []string
	old := compareEnglishNames
	compareEnglishNames = func(_ *Server, _ context.Context, names []string) map[string]string {
		asked = names
		return map[string]string{"Terra": "Earth", "Mart": "Mars"}
	}
	t.Cleanup(func() { compareEnglishNames = old })
	s := &Server{}
	out := s.windCardImages("```gregal-compare\n{\"title\":\"Terra vs Mart\",\"items\":[{\"name\":\"Terra\",\"facts\":{\"Llunes\":\"1\"}},{\"name\":\"Mart\",\"facts\":{\"Llunes\":\"2\"}}]}\n```")
	if strings.Join(asked, "|") != "Terra|Mart" {
		t.Fatalf("one call with both names: %v", asked)
	}
	if !strings.Contains(out, "earth.jpg") || !strings.Contains(out, "mars.jpg") || strings.Contains(out, "image_query") {
		t.Fatalf("photos per item:\n%s", out)
	}
}

// Una resposta amb xifres clau (sense foto) encara pot rebre la foto del tema.
func TestFactsCardDoesNotBlockTheTopicPhoto(t *testing.T) {
	fakeCardImages(t, map[string]string{"Human heart": "https://upload.wikimedia.org/heart.png"})
	fakeSubject(t, "Human heart")
	s := &Server{}
	in := "El cor és una bomba muscular de la mida del puny que es contrau 60-80 vegades per minut en repòs i mou uns 5 litres de sang cada minut.\n\n```gregal-facts\n{\"facts\":[{\"value\":\"300 g\",\"label\":\"pes\"},{\"value\":\"5 L\",\"label\":\"per minut\"}]}\n```"
	if out := s.windSubjectPhoto("Com funciona el cor humà?", in); !strings.Contains(out, "heart.png") {
		t.Fatalf("the heart photo should be there:\n%s", out)
	}
	plan := "Un dia complet per Girona amb el barri vell, les muralles i el riu, a peu i sense presses, amb dinar al centre.\n\n```gregal-plan\n{\"title\":\"Girona\",\"items\":[{\"title\":\"Muralles\"}]}\n```"
	if out := s.windSubjectPhoto("pla per Girona", plan); out != plan {
		t.Fatalf("a plan card keeps it as it was:\n%s", out)
	}
}

// El model tanca la llista d'items abans d'hora («}],"pick":true},{…»): la
// targeta es repara i cada cosa troba la seva foto; les xifres també es reparen.
func TestBrokenCardJSONIsRepaired(t *testing.T) {
	fakeCardImages(t, map[string]string{"Cheetah": "https://upload.wikimedia.org/c.jpg", "Leopard": "https://upload.wikimedia.org/l.jpg"})
	s := &Server{}
	in := "```gregal-compare\n{\"title\":\"Guepard vs. lleopard\",\"items\":[{\"name\":\"Guepard\",\"image_query\":\"Cheetah\",\"facts\":{\"Pes\":\"40-60 kg\"}],\"pick\":true},{\"name\":\"Lleopard\",\"image_query\":\"Leopard\",\"facts\":{\"Pes\":\"30-90 kg\",}}],\"verdict\":\"Dos felins.\"}\n```"
	out := s.windCardImages(in)
	if !strings.Contains(out, "c.jpg") || !strings.Contains(out, "l.jpg") || !strings.Contains(out, `"verdict":"Dos felins."`) {
		t.Fatalf("not repaired:\n%s", out)
	}
	facts := s.windCardImages("```gregal-facts\n{\"facts\":[{\"value\":\"30 m\",\"label\":\"llarg\"},{\"value\":\"150 t\",\"label\":\"pes\"}\n```")
	if !strings.Contains(facts, `{"facts":[{"label":"llarg","value":"30 m"},{"label":"pes","value":"150 t"}]}`) {
		t.Fatalf("facts not closed:\n%s", facts)
	}
	for in, want := range map[string]string{
		`{"a":[1,2}`:          `{"a":[1,2]}`,
		`{"a":"x}]y","b":1,}`: `{"a":"x}]y","b":1}`,
		`{"a":{"b":"c\"]"}`:   `{"a":{"b":"c\"]"}}`,
	} {
		if got := balanceJSON(in); got != want {
			t.Errorf("balanceJSON(%s) = %s, want %s", in, got, want)
		}
	}
}
