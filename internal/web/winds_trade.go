package web

import (
	"strings"

	"gregal/internal/companion"
)

// Eines d'ofici: el que fa que parlar amb el cuiner no sigui com parlar amb
// l'esportiu. Cada ofici té les seves fonts de dades (API obertes, sense
// clau, comprovades el 10/11/2026: es criden amb web_fetch abans de cercar al
// web), els llocs de confiança, el format en què respon un professional i les
// vigilàncies que un especialista d'aquest ofici t'oferiria pel seu compte.

type tradeSource struct {
	Name string // com es diu a la persona («Open Library»)
	URL  string // plantilla per a web_fetch, amb {query}…
	What string // què hi troba
}

type tradeWatch struct {
	CA, EN   string // l'etiqueta del botó
	PromptCA string // la rutina que es crea
	PromptEN string
	Time     string // HH:MM per defecte
	Weekly   bool   // només un dia a la setmana (el prompt ho diu)
	// IntervalH: cada quantes hores mira (0 = un cop al dia a Time).
	IntervalH float64
}

type tradeKit struct {
	CA, EN  string // nom de l'ofici
	Sources []tradeSource
	Sites   []string
	Format  string
	Watches []tradeWatch
}

var tradeKits = map[string]tradeKit{
	"cooking": {CA: "Cuina", EN: "Cooking",
		Sources: []tradeSource{
			{"TheMealDB", "https://www.themealdb.com/api/json/v1/1/search.php?s={dish in English}", "recipes with ingredients, measures, steps and photo (also filter.php?i={ingredient}, random.php)"},
			{"Open Food Facts", "https://world.openfoodfacts.org/cgi/search.pl?search_terms={product}&json=1&page_size=5", "nutrition, Nutri-Score and allergens of products"},
		},
		Sites:  []string{"receptes.cat", "3cat.cat/cuina", "directoalpaladar.com", "recetasderechupete.com", "seriouseats.com"},
		Format: "A recipe card: title; servings; prep / cook / total time; difficulty; ingredients with metric quantities (g, ml) scaled to the servings; numbered steps with times and temperatures; substitutions and allergens; how to keep it; one chef's tip. Seasonal produce from Catalonia when it fits.",
		Watches: []tradeWatch{
			{"Menú de la setmana cada diumenge", "Weekly menu every Sunday", "Only on Sundays (any other day reply (res)): propose a weekly dinner menu for me with a shopping list grouped by aisle, using seasonal produce in Catalonia.", "Only on Sundays (any other day reply (res)): propose a weekly dinner menu with a shopping list grouped by aisle, using seasonal produce.", "10:00", true, 0},
			{"Producte de temporada cada mes", "Seasonal produce each month", "Only on the 1st of the month (any other day reply (res)): what is in season in Catalonia this month and two simple recipes to use it.", "Only on the 1st of the month (any other day reply (res)): what is in season this month and two simple recipes to use it.", "09:30", false, 0},
		}},
	"sports": {CA: "Esports", EN: "Sports",
		Sources: []tradeSource{
			{"TheSportsDB", "https://www.thesportsdb.com/api/v1/json/123/{method}", "fixtures and results: searchteams.php?t={team}, eventsnext.php?id={teamId}, eventslast.php?id={teamId}, lookuptable.php?l={leagueId}&s=2026-2027. IDs: FC Barcelona 133739, Barça Bàsquet 135082, Barça Femení 144766; LaLiga 4335, Liga ACB 4408, Liga F 5106. Times are UTC: convert to Europe/Madrid. About 30 calls a minute"},
			{"OpenF1", "https://api.openf1.org/v1/sessions?year={yyyy}&session_name=Race&country_name={country}", "Formula 1 sessions"},
		},
		Sites:  []string{"fcbarcelona.cat", "3cat.cat/esport3", "sport.es", "mundodeportivo.com", "acb.com", "futbolenlatele.com (where to watch)"},
		Format: "A match card: Home – Away, competition and matchday; kickoff in Madrid time; venue; where to watch (TV / streaming); recent form (last 5) and table position; injuries and suspensions. For results: score, scorers and the key moment.",
		Watches: []tradeWatch{
			{"Avisa'm abans dels partits del Barça", "Heads-up before Barça matches", "Check TheSportsDB for FC Barcelona's next match. Only if it starts within the next 3 hours, tell me the kickoff (Madrid time), the rival, where to watch it and the likely line-up; otherwise reply (res).", "Check TheSportsDB for FC Barcelona's next match. Only if it starts within the next 3 hours, tell me the kickoff (Madrid time), the rival, where to watch it and the likely line-up; otherwise reply (res).", "12:00", false, 3},
			{"Resultats del cap de setmana els dilluns", "Weekend results on Mondays", "Only on Mondays (any other day reply (res)): results and standings of my teams this weekend (Barça football, basketball and women's team).", "Only on Mondays (any other day reply (res)): results and standings of my teams this weekend.", "09:00", true, 0},
		}},
	"music": {CA: "Música", EN: "Music",
		Sources: []tradeSource{
			{"MusicBrainz", "https://musicbrainz.org/ws/2/release-group/?query=artist:{artist}&fmt=json", "discographies and release dates (max 1 call a second)"},
			{"Deezer", "https://api.deezer.com/search/album?q={query}", "newest albums with covers"},
			{"iTunes", "https://itunes.apple.com/search?term={query}&entity=album&country=es&limit=10", "albums, release dates and 30 s previews"},
		},
		Sites:  []string{"enderrock.cat", "3cat.cat/catmusica", "mondosonoro.com", "songkick.com", "ticketmaster.es", "primaverasound.com"},
		Format: "Artist; the release (title, date, LP/EP/single, label); three key tracks; where to listen; concerts nearby (date, venue, city, price range, ticket link, sold out or not).",
		Watches: []tradeWatch{
			{"Discos nous dels meus artistes", "New releases from my artists", "Check Deezer and MusicBrainz for new releases this week from the artists I like (see memory). Only if there is one, tell me; otherwise reply (res).", "Check for new releases this week from the artists I like (see memory). Only if there is one, tell me; otherwise reply (res).", "11:00", false, 0},
			{"Concerts a prop meu", "Concerts near me", "Look for newly announced concerts near me by artists I like. Only if there is something new, tell me with the date and when tickets go on sale; otherwise reply (res).", "Look for newly announced concerts near me by artists I like. Only if there is something new, tell me; otherwise reply (res).", "18:00", false, 0},
		}},
	"books": {CA: "Llibres", EN: "Books",
		Sources: []tradeSource{
			{"Open Library", "https://openlibrary.org/search.json?q={query without accents}&fields=title,author_name,first_publish_year,isbn&limit=5", "books, authors and years"},
			{"Wikipedia", "https://ca.wikipedia.org/api/rest_v1/page/summary/{Title}", "summary of an author or a work (also es., en.)"},
			{"Gutendex", "https://gutendex.com/books/?search={query}", "free public-domain ebooks"},
		},
		Sites:  []string{"escriptors.cat", "biblioteques.cat", "laie.es", "casadellibro.com", "goodreads.com"},
		Format: "Title, author, year, pages, genre, original language and translations; a two-line pitch without spoilers; why it suits this person; three similar books; where to get it (library, bookshop, ebook).",
		Watches: []tradeWatch{
			{"Llibres nous dels meus autors", "New books from my authors", "Only if one of the authors I like (see memory) has a new book out or announced, tell me; otherwise reply (res).", "Only if one of the authors I like (see memory) has a new book out or announced, tell me; otherwise reply (res).", "10:30", false, 0},
		}},
	"shopping": {CA: "Compres", EN: "Shopping",
		Sources: []tradeSource{
			{"Open Food Facts", "https://es.openfoodfacts.org/api/v2/product/{barcode}.json", "groceries: Nutri-Score and ingredients (no prices)"},
		},
		Sites:  []string{"idealo.es", "ocu.org", "pccomponentes.com", "amazon.es + keepa.com (price history)", "tienda.mercadona.es", "carrefour.es"},
		Format: "A comparison table (shop, price including shipping, delivery time, returns, link); price per unit when it matters (€/kg); the best pick and the budget pick; a price-history verdict (good deal or wait); warranty notes. Never invent prices: read them on the shops.",
		Watches: []tradeWatch{
			{"Vigila el preu del que vull", "Watch the price of what I want", "Check the current price of the products on my wish list (see memory and notes). Only if one dropped clearly or reached my target price, tell me where; otherwise reply (res).", "Check the current price of the products on my wish list. Only if one dropped clearly or reached my target price, tell me where; otherwise reply (res).", "09:00", false, 0},
		}},
	"organisation": {CA: "Organització", EN: "Organisation",
		Sources: []tradeSource{
			{"Nager.Date", "https://date.nager.at/api/v3/PublicHolidays/{year}/ES", "public holidays (counties contains ES-CT for Catalonia)"},
			{"timeapi.io", "https://timeapi.io/api/Time/current/zone?timeZone={IANA zone}", "the time in another zone"},
		},
		Sites:  []string{"gencat.cat (calendari laboral)", "agenciatributaria.gob.es (tax calendar)", "gettingthingsdone.com"},
		Format: "Today's three priorities; a time-blocked agenda (HH:MM–HH:MM); deadlines with the days left; the next action per project; what to delegate or drop.",
		Watches: []tradeWatch{
			{"Avisa'm dels ponts amb un mes de marge", "Long weekends a month ahead", "Check Nager.Date for public holidays in Catalonia in the next 30–40 days. Only if there is a long weekend coming that I haven't been told about, tell me and suggest how to use it; otherwise reply (res).", "Check for public holidays in the next 30–40 days. Only if there is a long weekend coming, tell me; otherwise reply (res).", "09:00", false, 0},
		}},
	"weather": {CA: "El temps", EN: "Weather",
		Sources: []tradeSource{
			{"Open-Meteo geocoding", "https://geocoding-api.open-meteo.com/v1/search?name={place}&count=1&language=ca", "coordinates of a place"},
			{"Open-Meteo", "https://api.open-meteo.com/v1/forecast?latitude={lat}&longitude={lon}&hourly=temperature_2m,precipitation_probability,wind_gusts_10m&daily=temperature_2m_max,temperature_2m_min,precipitation_sum,uv_index_max,sunrise,sunset&timezone=Europe/Madrid", "hourly and daily forecast"},
			{"Open-Meteo air quality", "https://air-quality-api.open-meteo.com/v1/air-quality?latitude={lat}&longitude={lon}&current=european_aqi,uv_index,pm2_5", "air quality and UV"},
		},
		Sites:  []string{"meteo.cat", "aemet.es", "3cat.cat/el-temps", "meteoblue.com", "icgc.cat (allaus)"},
		Format: "Place and date; one summary line; a small table by period (morning, afternoon, night: temperature, rain %, wind and gusts, UV); warnings; an outdoor verdict (go, go early, postpone); what to wear or bring; sunrise and sunset.",
		Watches: []tradeWatch{
			{"Avisa'm si demà plou o fa vent fort", "Tell me if tomorrow brings rain or strong wind", "Check tomorrow's forecast where I live (see memory) on Open-Meteo. Only if rain probability is over 60% or gusts over 50 km/h, tell me when and what to do; otherwise reply (res).", "Check tomorrow's forecast where I live on Open-Meteo. Only if rain probability is over 60% or gusts over 50 km/h, tell me; otherwise reply (res).", "20:00", false, 0},
		}},
	"travel": {CA: "Viatges", EN: "Travel",
		Sources: []tradeSource{
			{"Photon", "https://photon.komoot.io/api/?q={place}&limit=5", "places and points of interest"},
			{"Frankfurter", "https://api.frankfurter.dev/v1/latest?base=EUR&symbols={currency}", "exchange rates"},
			{"Nager.Date", "https://date.nager.at/api/v3/PublicHolidays/{year}/{country code}", "holidays (closures) at the destination"},
		},
		Sites:  []string{"exteriores.gob.es (travel advice)", "wikivoyage.org", "rodalies.gencat.cat", "renfe.com", "skyscanner.es", "tmb.cat"},
		Format: "An itinerary by day (morning, afternoon, evening with places and travel times); transport options (mode, duration, approximate price); a budget table; documents; currency and rate; the weather for the dates; local holidays; a packing list.",
	},
	"fitness": {CA: "Salut i forma", EN: "Health and fitness",
		Sources: []tradeSource{
			{"wger", "https://wger.de/api/v2/exerciseinfo/?language=4&limit=20", "exercises, muscles and equipment (language 4 = Spanish, 2 = English)"},
			{"Open-Meteo air quality", "https://air-quality-api.open-meteo.com/v1/air-quality?latitude={lat}&longitude={lon}&current=european_aqi,uv_index", "whether to train outdoors"},
		},
		Sites:  []string{"canalsalut.gencat.cat", "who.int", "aesan.gob.es", "mayoclinic.org", "runedia.com"},
		Format: "Goal and level; a weekly plan table (day, session, sets × reps or time and intensity); warm-up and cool-down; how to progress; one safety note. Not medical advice: send them to a professional when it matters.",
	},
	"finance": {CA: "Finances", EN: "Finance",
		Sources: []tradeSource{
			{"Frankfurter", "https://api.frankfurter.dev/v1/latest?base=EUR&symbols={currency}", "exchange rates (also /v1/{from}..{to} for history)"},
			{"INE", "https://www.ine.es/wstempus/js/ES/DATOS_SERIE/IPC251856?nult=12", "Spanish inflation (CPI), last 12 months"},
			{"REE", "https://api.esios.ree.es/archives/70/download_json?locale=es&date={yyyy-mm-dd}", "electricity price by hour (PVPC): the cheapest hours"},
		},
		Sites:  []string{"bde.es", "agenciatributaria.gob.es", "ocu.org/dinero", "finanzasparatodos.es", "gencat.cat/consum"},
		Format: "Income, fixed and variable costs in a table; savings rate; the 50/30/20 check; the three biggest leaks; concrete actions with their € per month; upcoming deadlines. Never investment advice.",
		Watches: []tradeWatch{
			{"Les hores més barates de la llum", "Cheapest electricity hours", "Check REE's PVPC prices for tomorrow and tell me the three cheapest hours for the washing machine or the oven, in one line.", "Check tomorrow's PVPC electricity prices and tell me the three cheapest hours, in one line.", "20:30", false, 0},
		}},
	"movies": {CA: "Cinema i sèries", EN: "Movies and TV",
		Sources: []tradeSource{
			{"TVMaze", "https://api.tvmaze.com/search/shows?q={query}", "series, episodes (/shows/{id}/episodes) and the Spanish TV schedule (/schedule?country=ES&date={yyyy-mm-dd})"},
			{"iTunes", "https://itunes.apple.com/search?term={query}&media=movie&country=es", "films, prices and trailers"},
			{"Wikipedia", "https://ca.wikipedia.org/api/rest_v1/page/summary/{Title}", "plot and cast context"},
		},
		Sites:  []string{"filmaffinity.com", "justwatch.com/es (where to stream)", "3cat.cat", "sensacine.com", "imdb.com"},
		Format: "Title (year), genre, runtime, director and main cast; a spoiler-free two-line pitch; ratings (FilmAffinity, IMDb); where to watch (platform, rent or buy, cinema); why they will like it; two similar titles.",
		Watches: []tradeWatch{
			{"Estrenes del cap de setmana els divendres", "Weekend premieres on Fridays", "Only on Fridays (any other day reply (res)): the weekend's film premieres and new streaming arrivals that match my taste (see memory), with where to watch them.", "Only on Fridays (any other day reply (res)): the weekend's premieres and new streaming arrivals that match my taste.", "17:00", true, 0},
		}},
	"tech": {CA: "Tecnologia", EN: "Tech",
		Sources: []tradeSource{
			{"Hacker News", "https://hn.algolia.com/api/v1/search_by_date?query={query}&tags=story&hitsPerPage=10", "what the tech world is discussing"},
			{"GitHub", "https://api.github.com/search/repositories?q={query}+created:>{yyyy-mm-dd}&sort=stars", "new projects (10 calls a minute)"},
		},
		Sites:  []string{"3cat.cat/324", "vilaweb.cat", "xataka.com", "theverge.com", "arstechnica.com"},
		Format: "A digest of five to seven items: what happened, why it matters, the source link and the date. For depth: context, facts versus claims, what it means for them, what to watch next.",
	},
}

// tradeKitOf és l'ofici d'un vent, si en té un d'aquesta taula.
func tradeKitOf(w companion.Wind) (string, tradeKit, bool) {
	key := kitOf(w)
	k, ok := tradeKits[key]
	return key, k, ok
}

// windTradeBrief és el que diu al vent com treballa un professional del seu
// ofici: el format de les respostes, les seves fonts de dades i on mirar.
func windTradeBrief(w companion.Wind) string {
	_, k, ok := tradeKitOf(w)
	if !ok {
		return ""
	}
	var b strings.Builder
	b.WriteString("\n\nYOUR TRADE (" + k.EN + "): you are a professional, and it must show in every answer.")
	b.WriteString("\n- HOW A PRO ANSWERS: " + k.Format + " Use this shape whenever the question is about your trade; keep small talk small.")
	if len(k.Sources) > 0 {
		b.WriteString("\n- YOUR OWN DATA SOURCES (free JSON APIs, no key): call them with web_fetch BEFORE a general web search when they fit, and say which one you used:")
		for _, src := range k.Sources {
			b.WriteString("\n  · " + src.Name + ": " + src.URL + " — " + src.What)
		}
	}
	if len(k.Sites) > 0 {
		b.WriteString("\n- TRUSTED SITES for your trade: " + strings.Join(k.Sites, ", ") + ".")
	}
	b.WriteString("\n- When a question is outside your trade, still help as well as you can; you may mention once which crew member knows it better.")
	return b.String()
}

// tradeProfile és el que l'app ensenya al perfil: l'ofici, les fonts i les
// vigilàncies que el vent t'ofereix.
func (s *Server) tradeProfile(w companion.Wind) map[string]any {
	key, k, ok := tradeKitOf(w)
	if !ok {
		return nil
	}
	sources := []string{}
	for _, src := range k.Sources {
		sources = append(sources, src.Name)
	}
	watches := []map[string]any{}
	for _, wt := range k.Watches {
		watches = append(watches, map[string]any{"label_ca": wt.CA, "label_en": wt.EN, "prompt_ca": wt.PromptCA, "prompt_en": wt.PromptEN, "time": wt.Time, "weekly": wt.Weekly, "interval_h": wt.IntervalH})
	}
	skills := s.windSkills(w.ID)
	lessons := 0
	for _, sk := range skills {
		if sk.Nom == windLessonsSkill {
			lessons = strings.Count(sk.Cos, "\n- ")
		}
	}
	knowledge := len(skills)
	if lessons > 0 {
		knowledge--
	}
	// El nivell creix amb el que sap: coneixements estudiats i lliçons apreses.
	level := 1 + knowledge + lessons/5
	if level > 10 {
		level = 10
	}
	return map[string]any{
		"kit": key, "name_ca": k.CA, "name_en": k.EN, "sources": sources, "sites": k.Sites, "watches": watches,
		"level": level, "knowledge": knowledge, "lessons": lessons,
	}
}
