package demodb

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"gregal/internal/mcp"
)

// Eines, recursos i prompts del servidor MCP de demostració. Tot és de
// només lectura i la sortida queda acotada (MaxRows files).

// SchemaURI és el recurs amb el README de l'esquema.
const SchemaURI = "demo://schema"

// ServeOptions són les opcions de mcp.Serve per a `gregal mcp demo`.
func ServeOptions(db *DB, version string, log func(string, ...any)) mcp.ServeOptions {
	return mcp.ServeOptions{
		Name: "gregal-demo", Version: version, Log: log,
		Tools: Tools(db),
		Resources: []mcp.ServerResource{{
			URI: SchemaURI, Name: "Demo database schema", MimeType: "text/markdown",
			Description: "Tables, columns, joins and query examples of the demo database.",
			Text:        db.Readme(),
		}},
		Prompts: Prompts(),
	}
}

func jsonText(v any) (string, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// Tools són les tres eines de la demo.
func Tools(db *DB) []mcp.ServerTool {
	str := func(desc string) map[string]any { return map[string]any{"type": "string", "description": desc} }
	strs := func(desc string) map[string]any {
		return map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": desc}
	}
	return []mcp.ServerTool{
		{
			Name:        "list_tables",
			Description: "List the tables of the demo database (a fictional retailer, Northwind Outfitters; dates up to " + Anchor.AddDate(0, 0, -1).Format("2006-01-02") + ") with row counts and descriptions. Read-only.",
			InputSchema: map[string]any{"type": "object", "properties": map[string]any{}},
			Handler: func(context.Context, map[string]any) (string, error) {
				type row struct {
					Name        string   `json:"name"`
					Rows        int      `json:"rows"`
					Description string   `json:"description"`
					Joins       []string `json:"joins,omitempty"`
				}
				var out []row
				for _, n := range db.Order {
					t := db.Tables[n]
					out = append(out, row{Name: n, Rows: len(t.Rows), Description: t.Description, Joins: JoinsFrom(n)})
				}
				return jsonText(map[string]any{"tables": out, "today": Anchor.Format("2006-01-02")})
			},
		},
		{
			Name:        "describe_table",
			Description: "Describe one demo table: columns with types and meaning, joinable tables (use dotted columns like customers.segment) and 5 sample rows. Read-only.",
			InputSchema: map[string]any{"type": "object", "properties": map[string]any{
				"table": str("Table name (see list_tables)."),
			}, "required": []string{"table"}},
			Handler: func(_ context.Context, args map[string]any) (string, error) {
				name := mcp.ArgString(args, "table")
				t := db.Table(name)
				if t == nil {
					return "", fmt.Errorf("unknown table %q (tables: %s)", name, strings.Join(db.Order, ", "))
				}
				sample := make([][]any, 0, 5)
				for i := 0; i < 5 && i < len(t.Rows); i++ {
					r := make([]any, len(t.Rows[i]))
					for j, v := range t.Rows[i] {
						r[j] = out(v)
					}
					sample = append(sample, r)
				}
				cols := make([]string, len(t.Columns))
				for i, c := range t.Columns {
					cols[i] = c.Name
				}
				return jsonText(map[string]any{
					"table": t.Name, "description": t.Description, "rows": len(t.Rows),
					"columns": t.Columns, "joins": JoinsFrom(t.Name),
					"sample": map[string]any{"columns": cols, "rows": sample},
				})
			},
		},
		{
			Name: "query",
			Description: "Run a structured, read-only query on the demo database (no SQL). Returns JSON {columns, rows, total_rows, truncated}; at most 200 rows. " +
				"select: columns or aggregates sum(x), avg(x), min(x), max(x), count(*), count(x), count_distinct(x), each optionally \"… as name\". " +
				"Date buckets usable in select, group_by and filters: day(col), week(col), month(col) → \"2025-03\", quarter(col) → \"2025-Q1\", year(col). " +
				"Columns of joined tables use dots: from orders → customers.segment, products.category, regions.name; from support_tickets → customers.*, products.*, regions.*; from customers → regions.*. " +
				"filters: [{col, op, value}] with op =, !=, >, >=, <, <=, in, not_in, between ([low, high]), contains; dates as \"YYYY-MM-DD\". " +
				"order_by: [\"name desc\"] referencing an output column. Aggregated results default to group-key order. " +
				"Example: {\"table\":\"orders\",\"select\":[\"month(order_date) as month\",\"sum(revenue) as revenue\"],\"filters\":[{\"col\":\"status\",\"op\":\"=\",\"value\":\"completed\"}],\"group_by\":[\"month(order_date)\"]}",
			InputSchema: map[string]any{"type": "object", "properties": map[string]any{
				"table":  str("Base table: regions, customers, products, orders, marketing_spend or support_tickets."),
				"select": strs("Output columns/aggregates; default: all columns, or group keys + count(*) when grouping."),
				"filters": map[string]any{"type": "array", "description": "Conditions combined with AND.", "items": map[string]any{
					"type": "object", "properties": map[string]any{
						"col":   str("Column, dotted joined column or date bucket."),
						"op":    map[string]any{"type": "string", "enum": []string{"=", "!=", ">", ">=", "<", "<=", "in", "not_in", "between", "contains"}},
						"value": map[string]any{"description": "Value; an array for in, not_in and between."},
					}, "required": []string{"col", "op", "value"},
				}},
				"group_by": strs("Columns or date buckets to group by."),
				"order_by": strs("Sort keys like \"revenue desc\" (output column names or expressions)."),
				"limit":    map[string]any{"type": "integer", "minimum": 1, "maximum": MaxRows, "description": "Maximum rows (default 100, max 200)."},
			}, "required": []string{"table"}},
			Handler: func(_ context.Context, args map[string]any) (string, error) {
				raw, err := json.Marshal(args)
				if err != nil {
					return "", err
				}
				q, err := ParseQuery(raw)
				if err != nil {
					return "", err
				}
				res, err := db.Run(q)
				if err != nil {
					return "", err
				}
				return jsonText(res)
			},
		},
	}
}

// Prompts són els prompts de la demo (construir un dashboard).
func Prompts() []mcp.ServerPrompt {
	return []mcp.ServerPrompt{
		{
			Name:        "sales_dashboard",
			Description: "Build a sales dashboard from the demo database.",
			Text:        func(map[string]string) (string, error) { return SalesDashboardPrompt, nil },
		},
		{
			Name:        "support_kpis",
			Description: "Build a support KPI dashboard from the demo database.",
			Text:        func(map[string]string) (string, error) { return SupportPrompt, nil },
		},
	}
}

// SalesDashboardPrompt demana el dashboard de vendes (també el fa servir
// el botó de la pàgina d'inici, en la seva versió web).
const SalesDashboardPrompt = `Use the demo database tools (list_tables, describe_table, query) to build a sales dashboard for Northwind Outfitters. Query the real data, then answer with one gregal-plot chart per metric:
1. Monthly revenue over the last 24 months (line; completed orders, group by month(order_date)).
2. Revenue by product category (bar; products.category, sorted descending).
3. Revenue by region (bar; regions.name).
4. Support tickets by priority (bar; support_tickets grouped by priority).
Add a one-sentence insight under each chart. Suggest saving the charts to a dashboard named "Demo sales" with the plot's Add to dashboard button; do not claim they are saved.`

// SupportPrompt demana el dashboard de suport.
const SupportPrompt = `Use the demo database tools (list_tables, describe_table, query) to build a support KPI dashboard from the support_tickets table. Query the real data, then answer with one gregal-plot chart per metric:
1. Tickets per month (line; month(created_date)).
2. Tickets by priority and status (bar).
3. Average resolution hours by priority (bar; resolved and closed tickets).
4. Average CSAT per month (line).
Add a one-sentence insight under each chart and summarise open/pending backlog. Suggest saving the charts to a dashboard named "Demo support" with the plot's Add to dashboard button; do not claim they are saved.`

// Readme és el recurs demo://schema: l'esquema en markdown.
func (db *DB) Readme() string {
	var b strings.Builder
	b.WriteString("# Gregal demo database\n\nA fictional European retailer, Northwind Outfitters. Deterministic data: every run returns the same results. ")
	fmt.Fprintf(&b, "Today is %s; orders cover the previous 24 months.\n\n", Anchor.Format("2006-01-02"))
	for _, n := range db.Order {
		t := db.Tables[n]
		fmt.Fprintf(&b, "## %s (%d rows)\n\n%s\n\n", n, len(t.Rows), t.Description)
		for _, c := range t.Columns {
			fmt.Fprintf(&b, "- `%s` (%s): %s\n", c.Name, c.Type, c.Description)
		}
		if j := JoinsFrom(n); len(j) > 0 {
			fmt.Fprintf(&b, "\nJoinable: %s (dotted columns, e.g. `%s.%s`).\n", strings.Join(j, ", "), j[0], db.Tables[j[0]].Columns[1].Name)
		}
		b.WriteString("\n")
	}
	b.WriteString("## Query examples (query)\n\n")
	b.WriteString("- Monthly revenue: `{\"table\":\"orders\",\"select\":[\"month(order_date) as month\",\"sum(revenue) as revenue\"],\"filters\":[{\"col\":\"status\",\"op\":\"=\",\"value\":\"completed\"}],\"group_by\":[\"month(order_date)\"]}`\n")
	b.WriteString("- Top categories: `{\"table\":\"orders\",\"select\":[\"products.category as category\",\"sum(revenue) as revenue\"],\"group_by\":[\"products.category\"],\"order_by\":[\"revenue desc\"]}`\n")
	b.WriteString("- Revenue by region and segment: `{\"table\":\"orders\",\"select\":[\"regions.name as region\",\"customers.segment as segment\",\"sum(revenue) as revenue\"],\"group_by\":[\"regions.name\",\"customers.segment\"]}`\n")
	b.WriteString("- Tickets by priority: `{\"table\":\"support_tickets\",\"select\":[\"priority\",\"count(*) as tickets\",\"avg(resolution_hours) as avg_hours\"],\"group_by\":[\"priority\"]}`\n")
	return b.String()
}
