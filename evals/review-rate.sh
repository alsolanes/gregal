#!/bin/bash
# Taxa del revisor (D2): llegeix ~/.config/gregal/verify-log.jsonl i resumeix.
# Ús: ./evals/review-rate.sh [N]  (N = últims N veredictes; 0 = tots)
set -u
LOG=${GREGAL_VERIFY_LOG:-$HOME/.config/gregal/verify-log.jsonl}
[ -f "$LOG" ] || { echo "sense dades ($LOG)"; exit 0; }
python3 - "$LOG" "${1:-0}" <<'PYEOF'
import json, sys
rows = [json.loads(l) for l in open(sys.argv[1]) if l.strip()]
n = int(sys.argv[2])
if n > 0:
    rows = rows[-n:]
ap = sum(1 for r in rows if r.get("verdict") == "APROVAT")
print(f"veredictes: {len(rows)} · APROVAT: {ap} ({100*ap//len(rows) if rows else 0}%) · CAL REVISAR: {len(rows)-ap}")
by_model = {}
for r in rows:
    m = by_model.setdefault(r.get("model", "?"), [0, 0])
    m[r.get("verdict") == "APROVAT"] += 1
for m, (no, yes) in sorted(by_model.items()):
    print(f"  {m}: {yes} APROVAT / {no} CAL REVISAR")
PYEOF
