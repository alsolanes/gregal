#!/bin/bash
# Evals del Gregal (Fase D v1.0): 6 tasques reals amb asserts, mètriques per tasca.
# Ús: GREGAL_CONFIG=~/bench-agents/bench-zen.yaml ./evals/run.sh [t1 t2 ...]
# Requereix: gregal al PATH, python3+pytest, ZEN_API_KEY a l'entorn si el config és cloud.
# Tot corre en còpies sota evals/results/ (les fixtures de tasks/ no es toquen).
set -u
HERE=$(cd "$(dirname "$0")" && pwd)
GREGAL=${GREGAL:-$HOME/.local/bin/gregal}
CFG=${GREGAL_CONFIG:-$HERE/example.yaml}
OUT=$HERE/results
mkdir -p "$OUT"
rm -f "$OUT/summary.jsonl"

tasks=${1:-"t1 t2 t3 t4 t5 t6"}
for t in $tasks; do
  D=$OUT/$t
  rm -rf "$D"; cp -r "$HERE/tasks/$t" "$D"
  log=$OUT/${t}.log
  t0=$(date +%s)
  (cd "$D" && timeout 600 "$GREGAL" -config "$CFG" -p "$(cat "$HERE/tasks/$t.prompt")" \
    -mode code -auto-approve -max-steps 25 -output json >"$log" 2>"$D/.stderr") || true
  dt=$(($(date +%s) - t0))
  if [ -f "$HERE/tasks/$t.assert" ]; then
    (cd "$D" && bash "$HERE/tasks/$t.assert" "$log" "$HERE/tasks/$t.expected" >"$OUT/${t}.assert.log" 2>&1) && verdict=PASS || verdict=FAIL
  else
    verdict=SENSE-ASSERT
  fi
  python3 - "$log" "$t" "$verdict" "$dt" >>"$OUT/summary.jsonl" <<'PYEOF'
import json, sys
log, t, verdict, dt = sys.argv[1], sys.argv[2], sys.argv[3], int(sys.argv[4])
try:
    d = json.load(open(log))
except Exception as e:
    print(json.dumps({"task": t, "verdict": "ERROR", "detail": "json il·legible: %s" % e, "secs": dt}))
    raise SystemExit
print(json.dumps({"task": t, "verdict": verdict,
    "steps": d.get("steps"), "tools": d.get("tools_used"),
    "up": d.get("up_tokens"), "down": d.get("down_tokens"),
    "cost_usd": d.get("cost_usd"), "fallback": d.get("fallback_model") or "",
    "routed": (d.get("routed_from") or "") + ("->" + d.get("routed_to") if d.get("routed_to") else ""),
    "secs": dt}))
PYEOF
done

echo "== RESUM =="
python3 - "$OUT/summary.jsonl" <<'PYEOF'
import json, sys
print(f"{'tasca':6} {'veredicte':7} {'passos':7} {'tokens':8} {'eines':20} {'rutejat':12} {'s':>4}")
for line in open(sys.argv[1]):
    d = json.loads(line)
    tok = (d.get('up') or 0) + (d.get('down') or 0)
    print(f"{d['task']:6} {d['verdict']:7} {str(d.get('steps')):7} {tok:<8} {str(d.get('tools'))[:20]:20} {d.get('routed',''):12} {d.get('secs'):>4}")
PYEOF
