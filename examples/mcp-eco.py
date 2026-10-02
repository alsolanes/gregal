#!/usr/bin/python3
"""Micro-servidor MCP (stdio) per verificar Gregal end-to-end. Eina: eco."""
import json
import sys


def reply(id_, result):
    sys.stdout.write(json.dumps({"jsonrpc": "2.0", "id": id_, "result": result}) + "\n")
    sys.stdout.flush()


for line in sys.stdin:
    line = line.strip()
    if not line:
        continue
    try:
        msg = json.loads(line)
    except json.JSONDecodeError:
        continue
    method = msg.get("method")
    id_ = msg.get("id")
    if method == "initialize":
        reply(id_, {"protocolVersion": "2024-11-05", "capabilities": {},
                    "serverInfo": {"name": "eco", "version": "0.1"}})
    elif method == "tools/list":
        reply(id_, {"tools": [{
            "name": "eco",
            "description": "Retorna el text rebut amb el prefix ECO:. Útil per verificar que el pont MCP funciona.",
            "inputSchema": {"type": "object",
                            "properties": {"text": {"type": "string"}},
                            "required": ["text"]}}]})
    elif method == "tools/call":
        text = (msg.get("params", {}).get("arguments", {}) or {}).get("text", "")
        reply(id_, {"content": [{"type": "text", "text": "ECO: " + str(text)}]})
    # notificacions (sense id): sense resposta
