#!/usr/bin/env python3
"""Puxa o workflow api_osps_pcp na live, de 50 em 50, e grava num JSON."""

from __future__ import annotations

import json
import ssl
import sys
import time
import urllib.error
import urllib.parse
import urllib.request
from pathlib import Path

URL = "https://eace.org.br/api/1.1/wf/api_osps_pcp"
PAGE = 50
TIMEOUT = 180
SAIDA = Path("osps_pcp.json")


def get_page(index: int, total: int) -> list[dict]:
    qs = urllib.parse.urlencode({"index": index, "total": total})
    req = urllib.request.Request(
        f"{URL}?{qs}",
        headers={"Accept": "application/json"},
        method="GET",
    )
    ctx = ssl.create_default_context()
    with urllib.request.urlopen(req, timeout=TIMEOUT, context=ctx) as resp:
        envelope = json.loads(resp.read().decode("utf-8"))
    data = (envelope.get("response") or {}).get("data")
    if data is None:
        raise RuntimeError(f"resposta sem data: {envelope!r}"[:400])
    if isinstance(data, str):
        items = json.loads(data)
    else:
        items = data
    if not isinstance(items, list):
        raise RuntimeError(f"data não é lista: {type(items)}")
    return items


def main() -> int:
    todos: list[dict] = []
    index = 0
    pagina = 1
    while True:
        t0 = time.time()
        print(f"página {pagina}: index={index} total={PAGE} ...", flush=True)
        try:
            items = get_page(index, PAGE)
        except urllib.error.HTTPError as e:
            print(f"HTTP {e.code}: {e.read()[:400]!r}", file=sys.stderr)
            return 1
        except Exception as e:
            print(f"falhou: {e}", file=sys.stderr)
            return 1
        dt = time.time() - t0
        print(f"  {len(items)} itens em {dt:.1f}s", flush=True)
        if not items:
            break
        todos.extend(items)
        SAIDA.write_text(json.dumps(todos, ensure_ascii=False, indent=2), encoding="utf-8")
        print(f"  gravado {SAIDA} ({len(todos)} no total)", flush=True)
        if len(items) < PAGE:
            break
        index += PAGE
        pagina += 1
    print(f"fim: {len(todos)} itens em {SAIDA}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
