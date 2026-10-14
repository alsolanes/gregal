package demodb

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Consultes estructurades (sense SQL): taula base, columnes o agregats,
// filtres, agrupació (també per mes, trimestre, any, setmana o dia),
// ordenació i límit. Les columnes d'altres taules s'hi arriben amb noms
// amb punt (customers.segment) per un conjunt fix d'unions.

// MaxRows és el màxim de files que torna una consulta.
const MaxRows = 200

// DefaultLimit és el límit si la consulta no en porta.
const DefaultLimit = 100

// joins: taula → taula referenciada → columna clau forana.
var joins = map[string]map[string]string{
	"orders":          {"customers": "customer_id", "products": "product_id"},
	"support_tickets": {"customers": "customer_id", "products": "product_id"},
	"customers":       {"regions": "region_id"},
}

// JoinsFrom són les taules a què s'arriba des d'una taula (directament o
// en dos salts), en ordre.
func JoinsFrom(table string) []string {
	var out []string
	for to := range joins[table] {
		out = append(out, to)
		for to2 := range joins[to] {
			out = append(out, to2)
		}
	}
	sort.Strings(out)
	return out
}

// StringList accepta "x" o ["x","y"].
type StringList []string

func (s *StringList) UnmarshalJSON(b []byte) error {
	b = bytes.TrimSpace(b)
	if len(b) > 0 && b[0] == '"' {
		var one string
		if err := json.Unmarshal(b, &one); err != nil {
			return err
		}
		*s = StringList{one}
		return nil
	}
	var many []string
	if err := json.Unmarshal(b, &many); err != nil {
		return fmt.Errorf("expected a string or an array of strings")
	}
	*s = many
	return nil
}

// Filter és una condició sobre una columna (o un bucket de data).
type Filter struct {
	Col    string `json:"col"`
	Column string `json:"column,omitempty"`
	Op     string `json:"op"`
	Value  any    `json:"value"`
}

// OrderBy és una clau d'ordenació.
type OrderBy struct {
	By   string
	Desc bool
}

// OrderList accepta "expr desc", ["expr desc", …] o [{"column":"expr","direction":"desc"}].
type OrderList []OrderBy

func (o *OrderList) UnmarshalJSON(b []byte) error {
	var raw []json.RawMessage
	b = bytes.TrimSpace(b)
	if len(b) > 0 && b[0] != '[' {
		raw = []json.RawMessage{b}
	} else if err := json.Unmarshal(b, &raw); err != nil {
		return fmt.Errorf("order_by: expected an array")
	}
	for _, r := range raw {
		var s string
		if json.Unmarshal(r, &s) == nil {
			ob, err := parseOrder(s)
			if err != nil {
				return err
			}
			*o = append(*o, ob)
			continue
		}
		var obj struct {
			Column    string `json:"column"`
			Col       string `json:"col"`
			By        string `json:"by"`
			Direction string `json:"direction"`
			Dir       string `json:"dir"`
			Desc      bool   `json:"desc"`
		}
		if err := json.Unmarshal(r, &obj); err != nil {
			return fmt.Errorf("order_by: each item is \"expr [asc|desc]\" or {\"column\":…, \"direction\":\"asc|desc\"}")
		}
		by := firstNonEmpty(obj.Column, obj.Col, obj.By)
		dir := strings.ToLower(firstNonEmpty(obj.Direction, obj.Dir))
		if by == "" || (dir != "" && dir != "asc" && dir != "desc") {
			return fmt.Errorf("order_by: each item is \"expr [asc|desc]\" or {\"column\":…, \"direction\":\"asc|desc\"}")
		}
		*o = append(*o, OrderBy{By: by, Desc: obj.Desc || dir == "desc"})
	}
	return nil
}

func parseOrder(s string) (OrderBy, error) {
	s = strings.TrimSpace(s)
	low := strings.ToLower(s)
	switch {
	case strings.HasSuffix(low, " desc"):
		return OrderBy{By: strings.TrimSpace(s[:len(s)-5]), Desc: true}, nil
	case strings.HasSuffix(low, " asc"):
		return OrderBy{By: strings.TrimSpace(s[:len(s)-4])}, nil
	case s == "":
		return OrderBy{}, fmt.Errorf("order_by: empty item")
	}
	return OrderBy{By: s}, nil
}

func firstNonEmpty(xs ...string) string {
	for _, x := range xs {
		if strings.TrimSpace(x) != "" {
			return strings.TrimSpace(x)
		}
	}
	return ""
}

// Query és una consulta estructurada.
type Query struct {
	Table   string     `json:"table"`
	Select  StringList `json:"select"`
	Filters []Filter   `json:"filters"`
	GroupBy StringList `json:"group_by"`
	OrderBy OrderList  `json:"order_by"`
	Limit   *int       `json:"limit"`
}

// Result és el que torna una consulta.
type Result struct {
	Table     string   `json:"table"`
	Columns   []string `json:"columns"`
	Rows      [][]any  `json:"rows"`
	RowCount  int      `json:"row_count"`
	TotalRows int      `json:"total_rows"`
	Truncated bool     `json:"truncated"`
}

// ParseQuery llegeix una consulta de JSON (camps desconeguts = error).
func ParseQuery(raw []byte) (Query, error) {
	var q Query
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&q); err != nil {
		return q, fmt.Errorf("invalid query: %v (fields: table, select, filters, group_by, order_by, limit)", err)
	}
	return q, nil
}

// --- expressions ---

type hop struct {
	fk     int // columna clau forana a la taula d'origen
	target *Table
}

type colRef struct {
	hops  []hop
	table *Table
	idx   int
	typ   string
	canon string // taula.columna
}

func (c colRef) value(row []any) any {
	for _, h := range c.hops {
		id, ok := row[h.fk].(int64)
		if !ok || id < 1 || int(id) > len(h.target.Rows) {
			return nil
		}
		row = h.target.Rows[id-1]
	}
	return row[c.idx]
}

var buckets = map[string]bool{"day": true, "week": true, "month": true, "quarter": true, "year": true}
var aggs = map[string]bool{"sum": true, "avg": true, "min": true, "max": true, "count": true, "count_distinct": true}

type scalar struct {
	ref    colRef
	bucket string
}

func (s scalar) typ() string {
	if s.bucket != "" {
		return TString
	}
	return s.ref.typ
}

func (s scalar) canon() string {
	if s.bucket != "" {
		return s.bucket + "(" + s.ref.canon + ")"
	}
	return s.ref.canon
}

func (s scalar) value(row []any) any {
	v := s.ref.value(row)
	if s.bucket == "" || v == nil {
		return v
	}
	t := v.(time.Time)
	switch s.bucket {
	case "day":
		return t.Format("2006-01-02")
	case "week":
		y, w := t.ISOWeek()
		return fmt.Sprintf("%d-W%02d", y, w)
	case "month":
		return t.Format("2006-01")
	case "quarter":
		return fmt.Sprintf("%d-Q%d", t.Year(), (int(t.Month())-1)/3+1)
	default:
		return strconv.Itoa(t.Year())
	}
}

type item struct {
	name   string // nom de la columna de sortida
	canon  string
	scalar *scalar
	agg    string
	arg    *scalar // nil = count(*)
}

func (it item) outType() string {
	switch it.agg {
	case "":
		return it.scalar.typ()
	case "count", "count_distinct":
		return TInt
	case "sum", "avg":
		return TFloat
	}
	return it.arg.typ()
}

var callRe = regexp.MustCompile(`^([A-Za-z_]+)\s*\(\s*(.*?)\s*\)$`)
var asRe = regexp.MustCompile(`(?i)^(.*?)\s+as\s+([A-Za-z_][A-Za-z0-9_]*)$`)

type planner struct {
	db   *DB
	base *Table
}

func (p *planner) resolveCol(name string) (colRef, error) {
	name = strings.TrimSpace(name)
	tbl, col := p.base.Name, name
	if a, b, ok := strings.Cut(name, "."); ok {
		tbl, col = strings.TrimSpace(a), strings.TrimSpace(b)
	}
	if col == "" {
		return colRef{}, fmt.Errorf("empty column name")
	}
	var hops []hop
	cur := p.base
	if tbl != p.base.Name {
		path := p.path(tbl)
		if path == nil {
			if p.db.Table(tbl) == nil {
				return colRef{}, fmt.Errorf("unknown table %q in %q", tbl, name)
			}
			reach := JoinsFrom(p.base.Name)
			hint := "none"
			if len(reach) > 0 {
				hint = strings.Join(reach, ", ")
			}
			return colRef{}, fmt.Errorf("cannot join %s to %s (tables reachable from %s: %s)", p.base.Name, tbl, p.base.Name, hint)
		}
		for _, to := range path {
			fk := cur.Col(joins[cur.Name][to])
			next := p.db.Table(to)
			hops = append(hops, hop{fk: fk, target: next})
			cur = next
		}
	}
	idx := cur.Col(col)
	if idx < 0 {
		return colRef{}, fmt.Errorf("unknown column %q in table %s (columns: %s)", col, cur.Name, colNames(cur))
	}
	return colRef{hops: hops, table: cur, idx: idx, typ: cur.Columns[idx].Type, canon: cur.Name + "." + col}, nil
}

// path troba les taules a recórrer des de la base (fins a dos salts).
func (p *planner) path(target string) []string {
	for to := range joins[p.base.Name] {
		if to == target {
			return []string{to}
		}
	}
	for to := range joins[p.base.Name] {
		for to2 := range joins[to] {
			if to2 == target {
				return []string{to, to2}
			}
		}
	}
	return nil
}

func colNames(t *Table) string {
	n := make([]string, len(t.Columns))
	for i, c := range t.Columns {
		n[i] = c.Name
	}
	return strings.Join(n, ", ")
}

func (p *planner) parseScalar(expr string) (*scalar, error) {
	expr = strings.TrimSpace(expr)
	if m := callRe.FindStringSubmatch(expr); m != nil {
		fn := strings.ToLower(m[1])
		if !buckets[fn] {
			if aggs[fn] {
				return nil, fmt.Errorf("aggregate %s() is not allowed here", fn)
			}
			return nil, fmt.Errorf("unknown function %s() (date buckets: day, week, month, quarter, year)", fn)
		}
		ref, err := p.resolveCol(m[2])
		if err != nil {
			return nil, err
		}
		if ref.typ != TDate {
			return nil, fmt.Errorf("%s() needs a date column; %s is %s", fn, ref.canon, ref.typ)
		}
		return &scalar{ref: ref, bucket: fn}, nil
	}
	if strings.ContainsAny(expr, "()*") {
		return nil, fmt.Errorf("cannot parse expression %q", expr)
	}
	ref, err := p.resolveCol(expr)
	if err != nil {
		return nil, err
	}
	return &scalar{ref: ref}, nil
}

func (p *planner) parseItem(text string) (item, error) {
	text = strings.TrimSpace(text)
	name := ""
	if m := asRe.FindStringSubmatch(text); m != nil {
		text, name = strings.TrimSpace(m[1]), m[2]
	}
	if text == "" {
		return item{}, fmt.Errorf("empty select item")
	}
	if m := callRe.FindStringSubmatch(text); m != nil && aggs[strings.ToLower(m[1])] {
		fn := strings.ToLower(m[1])
		it := item{agg: fn}
		if m[2] == "*" {
			if fn != "count" {
				return item{}, fmt.Errorf("only count(*) accepts *")
			}
			it.canon = "count(*)"
		} else {
			arg, err := p.parseScalar(m[2])
			if err != nil {
				return item{}, err
			}
			if (fn == "sum" || fn == "avg") && arg.typ() != TInt && arg.typ() != TFloat {
				return item{}, fmt.Errorf("%s() needs a numeric column; %s is %s", fn, arg.canon(), arg.typ())
			}
			it.arg = arg
			it.canon = fn + "(" + arg.canon() + ")"
		}
		if name == "" {
			name = text
		}
		it.name = name
		return it, nil
	}
	sc, err := p.parseScalar(text)
	if err != nil {
		return item{}, err
	}
	if name == "" {
		name = text
	}
	return item{name: name, canon: sc.canon(), scalar: sc}, nil
}

// --- filtres ---

type cond struct {
	s    *scalar
	op   string
	vals []any
}

func toNum(v any) (float64, bool) {
	switch x := v.(type) {
	case float64:
		return x, true
	case int64:
		return float64(x), true
	case int:
		return float64(x), true
	case json.Number:
		f, err := x.Float64()
		return f, err == nil
	case string:
		f, err := strconv.ParseFloat(strings.TrimSpace(x), 64)
		return f, err == nil
	}
	return 0, false
}

func parseDate(v any) (time.Time, bool) {
	s, ok := v.(string)
	if !ok {
		return time.Time{}, false
	}
	s = strings.TrimSpace(s)
	for _, layout := range []string{"2006-01-02", time.RFC3339} {
		if t, err := time.Parse(layout, s); err == nil {
			return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC), true
		}
	}
	return time.Time{}, false
}

// coerce converteix un valor del filtre al tipus de l'expressió.
func coerce(typ string, v any, col string) (any, error) {
	switch typ {
	case TInt, TFloat:
		f, ok := toNum(v)
		if !ok {
			return nil, fmt.Errorf("filter on %s expects a number, got %v", col, v)
		}
		return f, nil
	case TDate:
		t, ok := parseDate(v)
		if !ok {
			return nil, fmt.Errorf("filter on %s expects a date \"YYYY-MM-DD\", got %v", col, v)
		}
		return t, nil
	default:
		switch x := v.(type) {
		case string:
			return x, nil
		case float64:
			return strconv.FormatFloat(x, 'f', -1, 64), nil
		case bool:
			return strconv.FormatBool(x), nil
		}
		return nil, fmt.Errorf("filter on %s expects a string, got %v", col, v)
	}
}

func (p *planner) parseFilter(f Filter) (cond, error) {
	col := firstNonEmpty(f.Col, f.Column)
	if col == "" {
		return cond{}, fmt.Errorf("filter needs \"col\"")
	}
	s, err := p.parseScalar(col)
	if err != nil {
		return cond{}, fmt.Errorf("filter: %v", err)
	}
	op := strings.ToLower(strings.TrimSpace(f.Op))
	switch op {
	case "==", "eq":
		op = "="
	case "<>", "ne":
		op = "!="
	case "not in":
		op = "not_in"
	}
	c := cond{s: s, op: op}
	switch op {
	case "=", "!=", ">", ">=", "<", "<=":
		if f.Value == nil {
			if op == "=" || op == "!=" {
				c.vals = []any{nil}
				return c, nil
			}
			return cond{}, fmt.Errorf("filter %s %s needs a value", col, op)
		}
		v, err := coerce(s.typ(), f.Value, col)
		if err != nil {
			return cond{}, err
		}
		c.vals = []any{v}
	case "in", "not_in", "between":
		arr, ok := f.Value.([]any)
		if !ok {
			return cond{}, fmt.Errorf("filter %s %s needs an array value", col, op)
		}
		if op == "between" && len(arr) != 2 {
			return cond{}, fmt.Errorf("filter %s between needs [low, high]", col)
		}
		if len(arr) == 0 || len(arr) > 200 {
			return cond{}, fmt.Errorf("filter %s %s needs 1–200 values", col, op)
		}
		for _, x := range arr {
			v, err := coerce(s.typ(), x, col)
			if err != nil {
				return cond{}, err
			}
			c.vals = append(c.vals, v)
		}
	case "contains":
		if s.typ() != TString {
			return cond{}, fmt.Errorf("contains needs a text column; %s is %s", col, s.typ())
		}
		str, ok := f.Value.(string)
		if !ok {
			return cond{}, fmt.Errorf("filter %s contains needs a string", col)
		}
		c.vals = []any{strings.ToLower(str)}
	default:
		return cond{}, fmt.Errorf("unknown filter op %q (=, !=, >, >=, <, <=, in, not_in, between, contains)", f.Op)
	}
	return c, nil
}

// norm porta un valor de taula a la forma comparable (float64, string, time).
func norm(v any) any {
	switch x := v.(type) {
	case int64:
		return float64(x)
	}
	return v
}

// cmp compara dos valors normalitzats; nil va primer.
func cmp(a, b any) int {
	switch {
	case a == nil && b == nil:
		return 0
	case a == nil:
		return -1
	case b == nil:
		return 1
	}
	switch x := a.(type) {
	case float64:
		y, _ := toNum(b)
		switch {
		case x < y:
			return -1
		case x > y:
			return 1
		}
		return 0
	case time.Time:
		if y, ok := b.(time.Time); ok {
			return x.Compare(y)
		}
		if y, ok := b.(string); ok {
			return strings.Compare(x.Format("2006-01-02"), y)
		}
	case string:
		return strings.Compare(x, fmt.Sprint(b))
	}
	return strings.Compare(fmt.Sprint(a), fmt.Sprint(b))
}

func (c cond) match(row []any) bool {
	v := norm(c.s.value(row))
	switch c.op {
	case "=":
		if c.vals[0] == nil {
			return v == nil
		}
		return v != nil && cmp(v, c.vals[0]) == 0
	case "!=":
		if c.vals[0] == nil {
			return v != nil
		}
		return v == nil || cmp(v, c.vals[0]) != 0
	case ">":
		return v != nil && cmp(v, c.vals[0]) > 0
	case ">=":
		return v != nil && cmp(v, c.vals[0]) >= 0
	case "<":
		return v != nil && cmp(v, c.vals[0]) < 0
	case "<=":
		return v != nil && cmp(v, c.vals[0]) <= 0
	case "between":
		return v != nil && cmp(v, c.vals[0]) >= 0 && cmp(v, c.vals[1]) <= 0
	case "in", "not_in":
		in := false
		for _, x := range c.vals {
			if v != nil && cmp(v, x) == 0 {
				in = true
				break
			}
		}
		return in == (c.op == "in")
	case "contains":
		s, ok := v.(string)
		return ok && strings.Contains(strings.ToLower(s), c.vals[0].(string))
	}
	return false
}

// --- execució ---

type aggState struct {
	n        int
	sum      float64
	min, max any
	distinct map[any]bool
}

func (a *aggState) add(fn string, v any) {
	if fn == "count_distinct" {
		if v == nil {
			return
		}
		if a.distinct == nil {
			a.distinct = map[any]bool{}
		}
		if t, ok := v.(time.Time); ok {
			v = t.Unix()
		}
		a.distinct[v] = true
		return
	}
	if v == nil {
		return
	}
	a.n++
	nv := norm(v)
	if f, ok := nv.(float64); ok {
		a.sum += f
	}
	if a.min == nil || cmp(nv, a.min) < 0 {
		a.min = nv
	}
	if a.max == nil || cmp(nv, a.max) > 0 {
		a.max = nv
	}
}

func (a *aggState) result(fn string, typ string) any {
	switch fn {
	case "count":
		return int64(a.n)
	case "count_distinct":
		return int64(len(a.distinct))
	case "sum":
		return a.sum
	case "avg":
		if a.n == 0 {
			return nil
		}
		return a.sum / float64(a.n)
	case "min":
		return denorm(a.min, typ)
	case "max":
		return denorm(a.max, typ)
	}
	return nil
}

func denorm(v any, typ string) any {
	if f, ok := v.(float64); ok && typ == TInt {
		return int64(f)
	}
	return v
}

// out dona format de sortida a un valor (dates ISO, decimals a 2 xifres).
func out(v any) any {
	switch x := v.(type) {
	case time.Time:
		return x.Format("2006-01-02")
	case float64:
		if math.IsNaN(x) || math.IsInf(x, 0) {
			return nil
		}
		return r2(x)
	}
	return v
}

// Run executa una consulta validant-ho tot.
func (db *DB) Run(q Query) (*Result, error) {
	name := strings.TrimSpace(q.Table)
	base := db.Table(name)
	if base == nil {
		if name == "" {
			return nil, fmt.Errorf("table is required (tables: %s)", strings.Join(db.Order, ", "))
		}
		return nil, fmt.Errorf("unknown table %q (tables: %s)", name, strings.Join(db.Order, ", "))
	}
	p := &planner{db: db, base: base}
	if len(q.Select) > 50 || len(q.GroupBy) > 5 || len(q.Filters) > 30 || len(q.OrderBy) > 5 {
		return nil, fmt.Errorf("query too large (max 50 select items, 30 filters, 5 group_by, 5 order_by)")
	}
	limit := DefaultLimit
	if q.Limit != nil {
		if *q.Limit < 1 {
			return nil, fmt.Errorf("limit must be between 1 and %d", MaxRows)
		}
		limit = min(*q.Limit, MaxRows)
	}
	var conds []cond
	for _, f := range q.Filters {
		c, err := p.parseFilter(f)
		if err != nil {
			return nil, err
		}
		conds = append(conds, c)
	}
	var groups []*scalar
	for _, g := range q.GroupBy {
		s, err := p.parseScalar(g)
		if err != nil {
			return nil, fmt.Errorf("group_by: %v", err)
		}
		groups = append(groups, s)
	}
	var items []item
	for _, s := range q.Select {
		it, err := p.parseItem(s)
		if err != nil {
			return nil, fmt.Errorf("select: %v", err)
		}
		items = append(items, it)
	}
	aggMode := len(groups) > 0
	for _, it := range items {
		if it.agg != "" {
			aggMode = true
		}
	}
	if len(items) == 0 {
		if aggMode {
			for i, g := range groups {
				items = append(items, item{name: q.GroupBy[i], canon: g.canon(), scalar: g})
			}
			items = append(items, item{name: "count(*)", canon: "count(*)", agg: "count"})
		} else {
			for i, c := range base.Columns {
				items = append(items, item{name: c.Name, canon: base.Name + "." + c.Name, scalar: &scalar{ref: colRef{table: base, idx: i, typ: c.Type, canon: base.Name + "." + c.Name}}})
			}
		}
	}
	seen := map[string]bool{}
	for _, it := range items {
		if seen[it.name] {
			return nil, fmt.Errorf("select: duplicate output column %q (use \"expr as name\")", it.name)
		}
		seen[it.name] = true
	}
	groupIdx := map[string]int{}
	for i, g := range groups {
		groupIdx[g.canon()] = i
	}
	if aggMode {
		for _, it := range items {
			if it.agg == "" {
				if _, ok := groupIdx[it.canon]; !ok {
					return nil, fmt.Errorf("select: %q must be in group_by or wrapped in an aggregate (sum, avg, min, max, count, count_distinct)", it.name)
				}
			}
		}
	}

	// Ordenació: per nom de sortida, per expressió canònica o (sense
	// agregar) per qualsevol columna, com a columna oculta.
	type sortKey struct {
		col  int
		desc bool
	}
	var keys []sortKey
	extra := []*scalar{}
	for _, ob := range q.OrderBy {
		found := -1
		for i, it := range items {
			if it.name == ob.By {
				found = i
				break
			}
		}
		if found < 0 {
			if it, err := p.parseItem(ob.By); err == nil {
				for i, x := range items {
					if x.canon == it.canon {
						found = i
						break
					}
				}
				if found < 0 && !aggMode && it.scalar != nil {
					extra = append(extra, it.scalar)
					found = len(items) + len(extra) - 1
				}
			}
		}
		if found < 0 {
			names := make([]string, len(items))
			for i, it := range items {
				names[i] = it.name
			}
			return nil, fmt.Errorf("order_by %q must name an output column (%s)", ob.By, strings.Join(names, ", "))
		}
		keys = append(keys, sortKey{col: found, desc: ob.Desc})
	}

	var rows [][]any
	if !aggMode {
		for _, r := range base.Rows {
			ok := true
			for _, c := range conds {
				if !c.match(r) {
					ok = false
					break
				}
			}
			if !ok {
				continue
			}
			row := make([]any, 0, len(items)+len(extra))
			for _, it := range items {
				row = append(row, it.scalar.value(r))
			}
			for _, s := range extra {
				row = append(row, s.value(r))
			}
			rows = append(rows, row)
		}
	} else {
		type group struct {
			keys   []any
			states []aggState
		}
		index := map[string]*group{}
		var order []*group
		for _, r := range base.Rows {
			ok := true
			for _, c := range conds {
				if !c.match(r) {
					ok = false
					break
				}
			}
			if !ok {
				continue
			}
			kv := make([]any, len(groups))
			var kb strings.Builder
			for i, g := range groups {
				kv[i] = g.value(r)
				fmt.Fprintf(&kb, "%v\x00", kv[i])
			}
			g := index[kb.String()]
			if g == nil {
				g = &group{keys: kv, states: make([]aggState, len(items))}
				index[kb.String()] = g
				order = append(order, g)
			}
			for i, it := range items {
				if it.agg == "" {
					continue
				}
				if it.arg == nil {
					g.states[i].n++
					continue
				}
				g.states[i].add(it.agg, it.arg.value(r))
			}
		}
		// Sense group_by però amb agregats: una fila encara que no n'hi hagi cap.
		if len(groups) == 0 && len(order) == 0 {
			order = append(order, &group{states: make([]aggState, len(items))})
		}
		for _, g := range order {
			row := make([]any, len(items))
			for i, it := range items {
				if it.agg == "" {
					row[i] = g.keys[groupIdx[it.canon]]
					continue
				}
				typ := TFloat
				if it.arg != nil {
					typ = it.arg.typ()
				}
				row[i] = g.states[i].result(it.agg, typ)
			}
			rows = append(rows, row)
		}
		if len(keys) == 0 {
			// Per defecte, en ordre de les claus d'agrupació (sèries temporals).
			for _, g := range groups {
				for i, it := range items {
					if it.agg == "" && it.canon == g.canon() {
						keys = append(keys, sortKey{col: i})
						break
					}
				}
			}
		}
	}
	if len(keys) > 0 {
		sort.SliceStable(rows, func(a, b int) bool {
			for _, k := range keys {
				c := cmp(norm(rows[a][k.col]), norm(rows[b][k.col]))
				if c == 0 {
					continue
				}
				if k.desc {
					return c > 0
				}
				return c < 0
			}
			return false
		})
	}
	res := &Result{Table: base.Name, TotalRows: len(rows)}
	for _, it := range items {
		res.Columns = append(res.Columns, it.name)
	}
	if len(rows) > limit {
		rows = rows[:limit]
		res.Truncated = true
	}
	res.Rows = make([][]any, len(rows))
	for i, r := range rows {
		o := make([]any, len(items))
		for j := range items {
			o[j] = out(r[j])
		}
		res.Rows[i] = o
	}
	res.RowCount = len(res.Rows)
	return res, nil
}
