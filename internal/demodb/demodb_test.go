package demodb

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"runtime"
	"strings"
	"testing"
	"time"

	"gregal/internal/mcp"
)

var testDB = Generate()

func TestGenerateDeterministicAndSmall(t *testing.T) {
	want := map[string]int{"regions": 8, "customers": 800, "products": 60, "orders": 6000, "marketing_spend": 120, "support_tickets": 1500}
	for name, n := range want {
		if got := len(testDB.Table(name).Rows); got != n {
			t.Errorf("%s: %d rows, want %d", name, got, n)
		}
	}
	other := Generate()
	for _, name := range testDB.Order {
		a, b := testDB.Tables[name].Rows, other.Tables[name].Rows
		if fmt.Sprint(a) != fmt.Sprint(b) {
			t.Fatalf("%s is not deterministic", name)
		}
	}
	// Mida i temps d'arrencada.
	runtime.GC()
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	t0 := time.Now()
	db := Generate()
	elapsed := time.Since(t0)
	runtime.GC()
	runtime.ReadMemStats(&after)
	runtime.KeepAlive(db)
	heap := int64(after.HeapAlloc) - int64(before.HeapAlloc)
	t.Logf("generate: %s, live heap ≈ %.2f MB", elapsed, float64(heap)/1e6)
	if heap > 5<<20 {
		t.Errorf("dataset uses %d bytes, want < 5 MB", heap)
	}
	if elapsed > 2*time.Second { // marge ampli per a màquines de CI lentes
		t.Errorf("generate took %s", elapsed)
	}
	// Totes les comandes dins dels 24 mesos i amb client ja donat d'alta.
	o, c := testDB.Table("orders"), testDB.Table("customers")
	for _, r := range o.Rows {
		d := r[o.Col("order_date")].(time.Time)
		if !d.Before(Anchor) || d.Before(Anchor.AddDate(-2, 0, 0)) {
			t.Fatalf("order date out of range: %v", d)
		}
		cust := c.Rows[r[o.Col("customer_id")].(int64)-1]
		if cust[c.Col("signup_date")].(time.Time).After(d) {
			t.Fatalf("order %v before customer signup", r[0])
		}
	}
}

func run(t *testing.T, js string) *Result {
	t.Helper()
	q, err := ParseQuery([]byte(js))
	if err != nil {
		t.Fatal(err)
	}
	res, err := testDB.Run(q)
	if err != nil {
		t.Fatalf("%s: %v", js, err)
	}
	return res
}

func TestQueryMonthlyRevenue(t *testing.T) {
	res := run(t, `{"table":"orders","select":["month(order_date) as month","sum(revenue) as revenue","count(*) as orders"],"filters":[{"col":"status","op":"=","value":"completed"}],"group_by":["month(order_date)"]}`)
	if res.RowCount != 24 || res.Truncated {
		t.Fatalf("want 24 months, got %d (%v)", res.RowCount, res.Truncated)
	}
	if res.Rows[0][0] != "2024-10" || res.Rows[23][0] != "2026-09" {
		t.Fatalf("months not in order: %v … %v", res.Rows[0][0], res.Rows[23][0])
	}
	// Estacionalitat: el desembre supera el febrer.
	byMonth := map[string]float64{}
	for _, r := range res.Rows {
		byMonth[r[0].(string)] = r[1].(float64)
	}
	if byMonth["2025-12"] <= byMonth["2025-02"] {
		t.Errorf("no Q4 peak: dec %v feb %v", byMonth["2025-12"], byMonth["2025-02"])
	}
	// La suma per mesos quadra amb el total.
	total := run(t, `{"table":"orders","select":["sum(revenue)"],"filters":[{"col":"status","op":"=","value":"completed"}]}`)
	sum := 0.0
	for _, v := range byMonth {
		sum += v
	}
	if d := sum - total.Rows[0][0].(float64); d > 1 || d < -1 {
		t.Errorf("monthly sum %v != total %v", sum, total.Rows[0][0])
	}
}

func TestQueryJoinsOrderLimit(t *testing.T) {
	res := run(t, `{"table":"orders","select":["products.category as category","sum(revenue) as revenue"],"group_by":["products.category"],"order_by":["revenue desc"],"limit":3}`)
	if res.RowCount != 3 || !res.Truncated || res.TotalRows != 6 {
		t.Fatalf("got %+v", res)
	}
	if res.Rows[0][1].(float64) < res.Rows[1][1].(float64) {
		t.Errorf("not sorted desc: %v", res.Rows)
	}
	// Dos salts: orders → customers → regions.
	reg := run(t, `{"table":"orders","select":["regions.name as region","count(*) as n"],"group_by":["regions.name"]}`)
	if reg.RowCount != 8 {
		t.Fatalf("want 8 regions, got %d", reg.RowCount)
	}
	n := int64(0)
	for _, r := range reg.Rows {
		n += r[1].(int64)
	}
	if n != 6000 {
		t.Errorf("region counts sum to %d", n)
	}
	// Filtres: in, between, contains, sobre una columna unida.
	f := run(t, `{"table":"orders","select":["count(*)"],"filters":[{"col":"customers.segment","op":"in","value":["Enterprise"]},{"col":"order_date","op":"between","value":["2025-01-01","2025-12-31"]},{"col":"products.name","op":"contains","value":"desk"}]}`)
	if f.Rows[0][0].(int64) <= 0 {
		t.Errorf("filtered count = %v", f.Rows[0][0])
	}
	// Sense agregats: columnes, ordre per una columna oculta i límit per defecte.
	plain := run(t, `{"table":"products","select":["name","price"],"order_by":["cost desc"]}`)
	if plain.RowCount != 60 || len(plain.Columns) != 2 {
		t.Fatalf("got %d rows %v", plain.RowCount, plain.Columns)
	}
	all := run(t, `{"table":"orders","limit":1000}`)
	if all.RowCount != MaxRows || !all.Truncated || all.TotalRows != 6000 {
		t.Errorf("limit not capped: %d %v %d", all.RowCount, all.Truncated, all.TotalRows)
	}
	// Tiquets per prioritat amb nuls a resolution_hours.
	tk := run(t, `{"table":"support_tickets","select":["priority","count(*) as tickets","avg(resolution_hours) as avg_hours","count(resolution_hours) as resolved","count_distinct(customer_id) as customers"],"group_by":["priority"],"order_by":["tickets desc"]}`)
	if tk.RowCount != 4 {
		t.Fatalf("priorities: %v", tk.Rows)
	}
	var tickets, resolved int64
	for _, r := range tk.Rows {
		tickets += r[1].(int64)
		resolved += r[3].(int64)
		if r[3].(int64) > r[1].(int64) || r[2] == nil {
			t.Errorf("null handling: %v", r)
		}
	}
	if tickets != 1500 || resolved >= tickets {
		t.Errorf("count(*) %d vs count(resolution_hours) %d", tickets, resolved)
	}
	// Bucket als filtres.
	q := run(t, `{"table":"orders","select":["quarter(order_date) as q","count(*)"],"filters":[{"col":"year(order_date)","op":"=","value":"2025"}],"group_by":["quarter(order_date)"]}`)
	if q.RowCount != 4 || q.Rows[0][0] != "2025-Q1" {
		t.Errorf("quarters: %v", q.Rows)
	}
}

func TestQueryValidation(t *testing.T) {
	bad := map[string]string{
		`{"table":"nope"}`:                      "unknown table",
		`{}`:                                    "table is required",
		`{"table":"orders","select":["bogus"]}`: "unknown column",
		`{"table":"orders","select":["sum(channel)"]}`:                                          "numeric",
		`{"table":"orders","select":["channel","count(*)"]}`:                                    "must be in group_by",
		`{"table":"orders","select":["month(channel)"]}`:                                        "date column",
		`{"table":"products","select":["regions.name"]}`:                                        "cannot join",
		`{"table":"orders","filters":[{"col":"revenue","op":"~","value":1}]}`:                   "unknown filter op",
		`{"table":"orders","filters":[{"col":"revenue","op":">","value":"abc"}]}`:               "expects a number",
		`{"table":"orders","filters":[{"col":"order_date","op":">","value":"x"}]}`:              "YYYY-MM-DD",
		`{"table":"orders","filters":[{"col":"revenue","op":"between","value":[1]}]}`:           "low, high",
		`{"table":"orders","limit":0}`:                                                          "limit",
		`{"table":"orders","select":["channel"],"order_by":["revenue"],"group_by":["channel"]}`: "output column",
		`{"table":"orders","select":["drop(table)"]}`:                                           "unknown function",
	}
	for js, want := range bad {
		q, err := ParseQuery([]byte(js))
		if err == nil {
			_, err = testDB.Run(q)
		}
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%s: got %v, want error containing %q", js, err, want)
		}
	}
	if _, err := ParseQuery([]byte(`{"table":"orders","sql":"select 1"}`)); err == nil {
		t.Error("unknown field accepted")
	}
	// Formes tolerants: group_by com a string i order_by com a objecte.
	res := run(t, `{"table":"orders","select":["channel","count(*) as n"],"group_by":"channel","order_by":[{"column":"n","direction":"desc"}]}`)
	if res.RowCount != 3 || res.Rows[0][0] != "Web" {
		t.Errorf("tolerant forms: %v", res.Rows)
	}
}

// TestMCPRoundTrip parla el protocol amb el servidor en procés.
func TestMCPRoundTrip(t *testing.T) {
	cr, cw := io.Pipe() // client → servidor
	sr, sw := io.Pipe() // servidor → client
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- mcp.Serve(ctx, cr, sw, ServeOptions(testDB, "test", nil)) }()
	dec := bufio.NewScanner(sr)
	dec.Buffer(make([]byte, 1<<20), 8<<20)
	id := 0
	call := func(method string, params any) map[string]any {
		t.Helper()
		id++
		b, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": id, "method": method, "params": params})
		if _, err := cw.Write(append(b, '\n')); err != nil {
			t.Fatal(err)
		}
		if !dec.Scan() {
			t.Fatalf("%s: no reply", method)
		}
		var resp map[string]any
		if err := json.Unmarshal(dec.Bytes(), &resp); err != nil {
			t.Fatal(err)
		}
		if resp["error"] != nil {
			t.Fatalf("%s: %v", method, resp["error"])
		}
		return resp["result"].(map[string]any)
	}
	init := call("initialize", map[string]any{"protocolVersion": "2025-06-18"})
	caps := init["capabilities"].(map[string]any)
	if caps["resources"] == nil || caps["prompts"] == nil || caps["tools"] == nil {
		t.Fatalf("capabilities: %v", caps)
	}
	tools := call("tools/list", map[string]any{})["tools"].([]any)
	if len(tools) != 3 {
		t.Fatalf("tools: %v", tools)
	}
	res := call("tools/call", map[string]any{"name": "query", "arguments": map[string]any{
		"table": "orders", "select": []string{"regions.name as region", "sum(revenue) as revenue"}, "group_by": []string{"regions.name"}, "order_by": []string{"revenue desc"},
	}})
	if res["isError"] == true {
		t.Fatalf("query failed: %v", res)
	}
	text := res["content"].([]any)[0].(map[string]any)["text"].(string)
	var out Result
	if err := json.Unmarshal([]byte(text), &out); err != nil || out.RowCount != 8 || out.Columns[0] != "region" {
		t.Fatalf("result %s (%v)", text, err)
	}
	bad := call("tools/call", map[string]any{"name": "query", "arguments": map[string]any{"table": "x"}})
	if bad["isError"] != true {
		t.Errorf("bad query not an error: %v", bad)
	}
	desc := call("tools/call", map[string]any{"name": "describe_table", "arguments": map[string]any{"table": "customers"}})
	if !strings.Contains(fmt.Sprint(desc), "segment") {
		t.Errorf("describe: %v", desc)
	}
	rs := call("resources/read", map[string]any{"uri": SchemaURI})
	if !strings.Contains(fmt.Sprint(rs), "support_tickets") {
		t.Errorf("schema resource: %v", rs)
	}
	ps := call("prompts/list", map[string]any{})["prompts"].([]any)
	if len(ps) != 2 {
		t.Errorf("prompts: %v", ps)
	}
	pg := call("prompts/get", map[string]any{"name": "sales_dashboard"})
	if !strings.Contains(fmt.Sprint(pg), "gregal-plot") {
		t.Errorf("prompt: %v", pg)
	}
	cw.Close()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("server did not stop")
	}
}
