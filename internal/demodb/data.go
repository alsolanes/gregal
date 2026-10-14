// Package demodb és una base de dades de demostració minúscula i
// determinista (llavor fixa): una empresa fictícia, «Northwind Outfitters»,
// amb clients, productes, comandes, màrqueting i suport. Viu tota en
// memòria (uns pocs MB) i es genera en arrencar en menys de 200 ms.
// `gregal mcp demo` l'exposa com a servidor MCP de només lectura perquè
// qualsevol pugui provar consultes i dashboards sense connectar res seu.
package demodb

import (
	"fmt"
	"math"
	"math/rand/v2"
	"sort"
	"time"
)

// Anchor és el «dia d'avui» de la demo: totes les dates en depenen, i
// per això els resultats són sempre els mateixos.
var Anchor = time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)

// Tipus de columna.
const (
	TInt    = "integer"
	TFloat  = "number"
	TString = "string"
	TDate   = "date"
)

// Column és una columna d'una taula.
type Column struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Description string `json:"description"`
}

// Table és una taula: files de valors int64, float64, string, time.Time
// (dates, UTC a mitjanit) o nil.
type Table struct {
	Name        string
	Description string
	Columns     []Column
	Rows        [][]any
	colIdx      map[string]int
}

// Col torna l'índex d'una columna (-1 si no hi és).
func (t *Table) Col(name string) int {
	if i, ok := t.colIdx[name]; ok {
		return i
	}
	return -1
}

// DB és el conjunt de taules.
type DB struct {
	Tables map[string]*Table
	Order  []string
}

// Table torna una taula pel nom (nil si no hi és).
func (db *DB) Table(name string) *Table { return db.Tables[name] }

func newTable(name, desc string, cols ...Column) *Table {
	t := &Table{Name: name, Description: desc, Columns: cols, colIdx: map[string]int{}}
	for i, c := range cols {
		t.colIdx[c.Name] = i
	}
	return t
}

func (db *DB) add(t *Table) {
	db.Tables[t.Name] = t
	db.Order = append(db.Order, t.Name)
}

func r2(x float64) float64 { return math.Round(x*100) / 100 }

// pick tria un índex segons els pesos.
func pick(rng *rand.Rand, w []float64) int {
	total := 0.0
	for _, x := range w {
		total += x
	}
	r := rng.Float64() * total
	for i, x := range w {
		r -= x
		if r < 0 {
			return i
		}
	}
	return len(w) - 1
}

// Generate construeix la base de dades. Sempre la mateixa: llavor fixa.
func Generate() *DB {
	rng := rand.New(rand.NewPCG(20261001, 42))
	db := &DB{Tables: map[string]*Table{}}
	start := Anchor.AddDate(-2, 0, 0) // 24 mesos d'històric
	days := int(Anchor.Sub(start).Hours() / 24)

	// --- regions ---
	regions := newTable("regions", "Sales regions (8).",
		Column{"region_id", TInt, "Primary key"},
		Column{"name", TString, "Region name"},
		Column{"country", TString, "Country"},
		Column{"manager", TString, "Regional sales manager"},
	)
	regionData := [][3]string{
		{"Iberia", "Spain", "Laia Puig"}, {"France", "France", "Hugo Martin"},
		{"DACH", "Germany", "Lena Vogel"}, {"Benelux", "Netherlands", "Daan Visser"},
		{"Nordics", "Sweden", "Elsa Lind"}, {"UK & Ireland", "United Kingdom", "Oliver Hughes"},
		{"Italy", "Italy", "Giulia Rossi"}, {"Central Europe", "Poland", "Marek Nowak"},
	}
	regionWeight := []float64{1.0, 1.3, 1.6, 0.8, 0.7, 1.5, 1.0, 0.6}
	for i, r := range regionData {
		regions.Rows = append(regions.Rows, []any{int64(i + 1), r[0], r[1], r[2]})
	}
	db.add(regions)

	// --- products ---
	products := newTable("products", "Product catalogue (60) with list price and unit cost.",
		Column{"product_id", TInt, "Primary key"},
		Column{"name", TString, "Product name"},
		Column{"category", TString, "Electronics, Home, Outdoor, Office, Apparel or Toys"},
		Column{"price", TFloat, "List price (EUR)"},
		Column{"cost", TFloat, "Unit cost (EUR)"},
		Column{"launch_date", TDate, "Date the product went on sale"},
	)
	type catDef struct {
		name   string
		items  []string
		lo, hi float64 // preu
		margin float64 // cost/preu mitjà
	}
	cats := []catDef{
		{"Electronics", []string{"Wireless Earbuds", "Smart Speaker", "4K Monitor", "Action Camera", "E-Reader", "Power Bank", "Bluetooth Keyboard", "Noise-Cancelling Headphones", "Smartwatch", "USB-C Hub"}, 25, 420, 0.68},
		{"Home", []string{"Espresso Maker", "Air Purifier", "Cast Iron Pan", "Linen Bedding Set", "LED Desk Lamp", "Robot Vacuum", "Kettle", "Knife Set", "Throw Blanket", "Storage Baskets"}, 15, 380, 0.55},
		{"Outdoor", []string{"Trail Backpack", "Camping Tent", "Sleeping Bag", "Hiking Poles", "Insulated Bottle", "Camping Stove", "Headlamp", "Hammock", "Folding Chair", "Rain Shell"}, 12, 320, 0.52},
		{"Office", []string{"Ergonomic Chair", "Standing Desk", "Notebook Pack", "Gel Pens", "Monitor Arm", "Desk Organizer", "Whiteboard", "Paper Shredder", "Label Printer", "Laptop Stand"}, 6, 520, 0.6},
		{"Apparel", []string{"Merino T-Shirt", "Down Jacket", "Running Shoes", "Wool Socks", "Fleece Hoodie", "Rain Boots", "Chino Trousers", "Beanie", "Gloves", "Softshell Vest"}, 10, 260, 0.45},
		{"Toys", []string{"Building Blocks", "Puzzle 1000", "RC Car", "Board Game", "Plush Bear", "Science Kit", "Wooden Train", "Drawing Set", "Kite", "Card Game"}, 8, 140, 0.5},
	}
	type prod struct {
		cat         int
		price, cost float64
		pop         float64
		launch      time.Time
	}
	var prods []prod
	var catProds [][]int // índexs de prods per categoria
	for ci, c := range cats {
		var ids []int
		for k, item := range c.items {
			price := r2(c.lo + (c.hi-c.lo)*math.Pow(rng.Float64(), 1.6))
			price = math.Floor(price) + 0.99
			cost := r2(price * (c.margin + (rng.Float64()-0.5)*0.12))
			launch := start.AddDate(0, 0, -400+rng.IntN(300))
			if k >= 8 { // els dos últims de cada categoria són novetats
				launch = start.AddDate(0, 0, 120+rng.IntN(420))
			}
			p := prod{cat: ci, price: price, cost: cost, pop: 1 / math.Pow(float64(k+1), 0.7), launch: launch}
			ids = append(ids, len(prods))
			prods = append(prods, p)
			products.Rows = append(products.Rows, []any{int64(len(prods)), item, c.name, price, cost, launch})
		}
		catProds = append(catProds, ids)
	}
	db.add(products)

	// --- customers (ordenats per data d'alta: id = ordre d'alta) ---
	customers := newTable("customers", "Customers (800) with segment, region and signup date.",
		Column{"customer_id", TInt, "Primary key"},
		Column{"name", TString, "Customer name"},
		Column{"segment", TString, "Consumer, SMB or Enterprise"},
		Column{"region_id", TInt, "Foreign key to regions"},
		Column{"signup_date", TDate, "Date the customer signed up"},
	)
	first := []string{"Alba", "Marc", "Nora", "Pau", "Clara", "Jan", "Emma", "Leo", "Iris", "Max", "Sofia", "Lucas", "Mia", "Noah", "Ada", "Oscar", "Lia", "Teo", "Vera", "Hugo"}
	last := []string{"Ferrer", "Schmidt", "Dubois", "Jansen", "Berg", "Smith", "Bianchi", "Kowalski", "Serra", "Weber", "Moreau", "Bakker", "Nilsson", "Taylor", "Romano", "Novak"}
	companyA := []string{"Blue", "North", "Bright", "Iron", "Green", "Swift", "Atlas", "Nova", "Delta", "Pine"}
	companyB := []string{"Labs", "Works", "Logistics", "Retail", "Studio", "Systems", "Foods", "Partners", "Group", "Media"}
	segments := []string{"Consumer", "SMB", "Enterprise"}
	segWeight := []float64{0.62, 0.3, 0.08}
	const nCustomers = 800
	signups := make([]time.Time, nCustomers)
	histStart := start.AddDate(-3, 0, 0)
	span := int(Anchor.Sub(histStart).Hours() / 24)
	for i := range signups {
		// Més altes recents: creixement.
		d := int(float64(span) * math.Pow(rng.Float64(), 0.75))
		signups[i] = histStart.AddDate(0, 0, d)
	}
	sort.Slice(signups, func(a, b int) bool { return signups[a].Before(signups[b]) })
	type cust struct {
		seg, region int
	}
	custs := make([]cust, nCustomers)
	for i := 0; i < nCustomers; i++ {
		seg := pick(rng, segWeight)
		reg := pick(rng, regionWeight)
		var name string
		if seg == 0 {
			name = first[rng.IntN(len(first))] + " " + last[rng.IntN(len(last))]
		} else {
			name = companyA[rng.IntN(len(companyA))] + " " + companyB[rng.IntN(len(companyB))]
			if seg == 2 {
				name += " Corp"
			}
		}
		custs[i] = cust{seg: seg, region: reg}
		customers.Rows = append(customers.Rows, []any{int64(i + 1), name, segments[seg], int64(reg + 1), signups[i]})
	}
	db.add(customers)
	// signedBefore: quants clients s'havien donat d'alta abans d'una data.
	signedBefore := func(d time.Time) int {
		return sort.Search(nCustomers, func(i int) bool { return signups[i].After(d) })
	}

	// --- pesos diaris: tendència, estacionalitat, dia de la setmana, promos ---
	promo := func(d time.Time) (string, float64) {
		// Black Friday (cap de setmana després del 4t dijous de novembre).
		if d.Month() == time.November {
			bf := blackFriday(d.Year())
			if !d.Before(bf) && d.Before(bf.AddDate(0, 0, 4)) {
				return "Black Friday", 0.25
			}
		}
		if d.Month() == time.March && d.Day() >= 10 && d.Day() <= 20 {
			return "Spring Sale", 0.15
		}
		if d.Month() == time.July && d.Day() >= 1 && d.Day() <= 10 {
			return "Summer Sale", 0.2
		}
		return "", 0
	}
	seasonal := []float64{0.85, 0.78, 0.92, 0.95, 1.0, 1.02, 1.05, 0.95, 1.0, 1.05, 1.3, 1.55} // gen..des
	weights := make([]float64, days)
	for i := 0; i < days; i++ {
		d := start.AddDate(0, 0, i)
		w := (1 + 0.32*float64(i)/365) * seasonal[d.Month()-1]
		switch d.Weekday() {
		case time.Saturday, time.Sunday:
			w *= 0.8
		case time.Monday:
			w *= 1.08
		}
		if name, _ := promo(d); name != "" {
			w *= map[string]float64{"Black Friday": 3.2, "Spring Sale": 1.5, "Summer Sale": 1.7}[name]
		}
		weights[i] = w * (0.85 + 0.3*rng.Float64())
	}
	cum := make([]float64, days)
	acc := 0.0
	for i, w := range weights {
		acc += w
		cum[i] = acc
	}
	drawDay := func() time.Time {
		x := rng.Float64() * acc
		i := sort.SearchFloat64s(cum, x)
		if i >= days {
			i = days - 1
		}
		return start.AddDate(0, 0, i)
	}

	// --- orders (una línia per comanda: quantitat inclosa) ---
	orders := newTable("orders", "Orders (6000) over the last 24 months; one product line per order, quantity included. revenue = quantity × unit_price × (1 − discount).",
		Column{"order_id", TInt, "Primary key"},
		Column{"order_date", TDate, "Order date"},
		Column{"customer_id", TInt, "Foreign key to customers"},
		Column{"product_id", TInt, "Foreign key to products"},
		Column{"channel", TString, "Web, Store or Partner"},
		Column{"status", TString, "completed, returned or cancelled"},
		Column{"promo", TString, "Promotion name, or empty"},
		Column{"quantity", TInt, "Units"},
		Column{"unit_price", TFloat, "List price at order time (EUR)"},
		Column{"discount", TFloat, "Discount rate 0–1"},
		Column{"revenue", TFloat, "Net revenue (EUR)"},
		Column{"cost", TFloat, "Cost of goods (EUR)"},
		Column{"profit", TFloat, "revenue − cost (EUR)"},
	)
	const nOrders = 6000
	dates := make([]time.Time, nOrders)
	for i := range dates {
		dates[i] = drawDay()
	}
	sort.Slice(dates, func(a, b int) bool { return dates[a].Before(dates[b]) })
	channels := []string{"Web", "Store", "Partner"}
	statuses := []string{"completed", "returned", "cancelled"}
	for i, d := range dates {
		// Categoria segons la temporada.
		m := d.Month()
		cw := []float64{1.0, 0.9, 0.75, 0.6, 0.85, 0.45}
		switch {
		case m >= 5 && m <= 8:
			cw[2] *= 2.1 // Outdoor a l'estiu
			cw[4] *= 0.8
		case m == 11 || m == 12:
			cw[0] *= 1.7 // Electronics i Toys per Nadal
			cw[5] *= 2.6
			cw[4] *= 1.3
		case m == 9 || m == 1:
			cw[3] *= 1.8 // Office: tornada i any nou
		}
		if m == 10 || m == 11 || m == 12 || m == 1 || m == 2 {
			cw[4] *= 1.25 // Apparel a l'hivern
		}
		ci := pick(rng, cw)
		var pw []float64
		var cand []int
		for _, pi := range catProds[ci] {
			if prods[pi].launch.After(d) {
				continue
			}
			cand = append(cand, pi)
			pw = append(pw, prods[pi].pop)
		}
		if len(cand) == 0 {
			cand, pw = catProds[ci][:1], []float64{1}
		}
		pi := cand[pick(rng, pw)]
		p := prods[pi]
		n := signedBefore(d)
		if n < 1 {
			n = 1
		}
		ci2 := rng.IntN(n)
		c := custs[ci2]
		qty := 1 + rng.IntN(3)
		switch c.seg {
		case 1:
			qty = 1 + rng.IntN(6)
		case 2:
			qty = 3 + rng.IntN(18)
		}
		promoName, disc := promo(d)
		if promoName == "" && rng.Float64() < 0.18 {
			disc = 0.05
		}
		if c.seg == 2 {
			disc = math.Max(disc, 0.1)
		}
		web := 0.45 + 0.15*float64(d.Sub(start).Hours()/24)/float64(days)
		ch := pick(rng, []float64{web, 0.85 - web, 0.15})
		st := pick(rng, []float64{0.92, 0.05, 0.03})
		revenue := r2(float64(qty) * p.price * (1 - disc))
		cost := r2(float64(qty) * p.cost)
		if st == 2 { // cancel·lada: sense ingressos
			revenue, cost = 0, 0
		}
		orders.Rows = append(orders.Rows, []any{
			int64(i + 1), d, int64(ci2 + 1), int64(pi + 1), channels[ch], statuses[st], promoName,
			int64(qty), p.price, disc, revenue, cost, r2(revenue - cost),
		})
	}
	db.add(orders)

	// --- marketing_spend (mensual per canal) ---
	mkt := newTable("marketing_spend", "Monthly marketing spend per channel (24 months × 5 channels).",
		Column{"month", TDate, "First day of the month"},
		Column{"channel", TString, "Search, Social, Email, Display or Events"},
		Column{"spend", TFloat, "Spend (EUR)"},
		Column{"impressions", TInt, "Ad impressions"},
		Column{"clicks", TInt, "Clicks"},
		Column{"conversions", TInt, "Attributed orders"},
	)
	mch := []string{"Search", "Social", "Email", "Display", "Events"}
	mbase := []float64{9000, 6500, 1800, 4200, 3000}
	cpm := []float64{9, 6, 1.5, 4, 25}
	ctr := []float64{0.045, 0.012, 0.03, 0.004, 0.02}
	cvr := []float64{0.035, 0.018, 0.05, 0.01, 0.03}
	for k := 0; k < 24; k++ {
		month := start.AddDate(0, k, 0)
		for ci, name := range mch {
			s := mbase[ci] * (1 + 0.02*float64(k)) * seasonal[month.Month()-1] * (0.85 + 0.3*rng.Float64())
			if name == "Events" && month.Month() != time.March && month.Month() != time.October {
				s *= 0.25
			}
			if name == "Social" && k >= 12 {
				s *= 1.35 // aposta per social el segon any
			}
			imp := int64(s / cpm[ci] * 1000)
			clicks := int64(float64(imp) * ctr[ci] * (0.8 + 0.4*rng.Float64()))
			conv := int64(float64(clicks) * cvr[ci] * (0.8 + 0.4*rng.Float64()))
			mkt.Rows = append(mkt.Rows, []any{month, name, r2(s), imp, clicks, conv})
		}
	}
	db.add(mkt)

	// --- support_tickets ---
	tickets := newTable("support_tickets", "Support tickets (1500) with priority, status, resolution time and satisfaction.",
		Column{"ticket_id", TInt, "Primary key"},
		Column{"created_date", TDate, "Date the ticket was opened"},
		Column{"customer_id", TInt, "Foreign key to customers"},
		Column{"product_id", TInt, "Foreign key to products"},
		Column{"category", TString, "Shipping, Billing, Product defect, Returns, Account or How-to"},
		Column{"priority", TString, "low, medium, high or urgent"},
		Column{"status", TString, "open, pending, resolved or closed"},
		Column{"resolution_hours", TFloat, "Hours to resolution (null while open or pending)"},
		Column{"csat", TInt, "Satisfaction 1–5 (null if not rated)"},
	)
	tcat := []string{"Shipping", "Billing", "Product defect", "Returns", "Account", "How-to"}
	tcw := []float64{0.28, 0.16, 0.18, 0.14, 0.1, 0.14}
	prio := []string{"low", "medium", "high", "urgent"}
	prw := []float64{0.35, 0.4, 0.19, 0.06}
	baseHours := []float64{40, 22, 9, 3}
	const nTickets = 1500
	tdates := make([]time.Time, nTickets)
	for i := range tdates {
		tdates[i] = drawDay()
	}
	sort.Slice(tdates, func(a, b int) bool { return tdates[a].Before(tdates[b]) })
	for i, d := range tdates {
		n := signedBefore(d)
		if n < 1 {
			n = 1
		}
		cu := rng.IntN(n)
		pi := rng.IntN(len(prods))
		cat := pick(rng, tcw)
		pr := pick(rng, prw)
		if tcat[cat] == "Product defect" && pr < 2 && rng.Float64() < 0.4 {
			pr++
		}
		age := Anchor.Sub(d).Hours() / 24
		var status string
		switch {
		case age < 7 && rng.Float64() < 0.6:
			status = "open"
		case age < 21 && rng.Float64() < 0.3:
			status = "pending"
		case rng.Float64() < 0.55:
			status = "closed"
		default:
			status = "resolved"
		}
		var hours, csat any
		if status == "resolved" || status == "closed" {
			// Millora amb el temps: l'equip creix.
			improve := 1 - 0.35*(1-age/float64(days))
			h := baseHours[pr] * improve * math.Exp(rng.NormFloat64()*0.6)
			hours = r2(math.Max(0.25, h))
			if rng.Float64() < 0.7 {
				score := 5 - int(h/(baseHours[pr]*0.9)) - rng.IntN(2)
				csat = int64(min(5, max(1, score)))
			}
		}
		tickets.Rows = append(tickets.Rows, []any{int64(i + 1), d, int64(cu + 1), int64(pi + 1), tcat[cat], prio[pr], status, hours, csat})
	}
	db.add(tickets)
	return db
}

// blackFriday és el divendres després del quart dijous de novembre.
func blackFriday(year int) time.Time {
	d := time.Date(year, 11, 1, 0, 0, 0, 0, time.UTC)
	for d.Weekday() != time.Thursday {
		d = d.AddDate(0, 0, 1)
	}
	return d.AddDate(0, 0, 22)
}

// Summary és una línia per taula (per als tests i el README).
func (db *DB) Summary() string {
	s := ""
	for _, n := range db.Order {
		s += fmt.Sprintf("%s: %d rows\n", n, len(db.Tables[n].Rows))
	}
	return s
}
