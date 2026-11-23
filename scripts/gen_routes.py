"""Regenerate internal/tools/routes.json from an ARTEX source tree.

Usage: python3 scripts/gen_routes.py /path/to/ARTEX-God
Only literal `mux.HandleFunc("METHOD /api/...")` routes are collected; the
generic artex_api_request tool can still reach dynamically registered routes.
"""
import json
import re
import sys
from pathlib import Path

METHOD = r"GET|POST|PATCH|PUT|DELETE|HEAD"
LITERAL = re.compile(r'HandleFunc\("(' + METHOD + r') (/api/[^"]+)",\s*(s\.\w+|func)')


def main() -> None:
    repo = Path(sys.argv[1] if len(sys.argv) > 1 else ".").resolve()
    routes = {}
    for src in sorted((repo / "server").glob("*.go")):
        if src.name.endswith("_test.go"):
            continue
        text = src.read_text(encoding="utf-8")
        for lineno, line in enumerate(text.splitlines(), 1):
            m = LITERAL.search(line)
            if not m:
                continue
            method, path, handler = m.groups()
            key = f"{method} {path}"
            if key in routes:
                continue
            routes[key] = {
                "method": method,
                "path": path,
                "handler": handler.removeprefix("s."),
                "source": f"server/{src.name}#L{lineno}",
            }
    out = Path(__file__).resolve().parent.parent / "internal" / "tools" / "routes.json"
    out.write_text(json.dumps({"routes": routes}, ensure_ascii=False, indent=1, sort_keys=True) + "\n", encoding="utf-8")
    print(f"wrote {len(routes)} routes to {out}")


if __name__ == "__main__":
    main()
