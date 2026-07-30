#!/usr/bin/env python3
"""V2 validation helpers — assert transaction summaries and cron monitors."""
import json
import sys
from pathlib import Path


def load(path=None):
    if path:
        return json.loads(Path(path).read_text())
    return json.load(sys.stdin)


def check_transactions(path=None):
    rows = load(path)
    print("transaction_count", len(rows))
    assert len(rows) >= 1, rows
    for r in rows:
        print(
            f"  name={r['name']!r} count={r['count']} "
            f"p95={r['p95_ms']:.2f} p99={r['p99_ms']:.2f}"
        )
        assert r["count"] >= 1
        assert r["p95_ms"] >= 0
        assert r["p99_ms"] >= r["p95_ms"] - 1e-6
    print("TRANSACTIONS_OK")


def check_transaction_detail(path=None):
    d = load(path)
    samples = d.get("samples") or []
    print("detail_name", d.get("name"), "samples", len(samples))
    assert len(samples) >= 1, d
    has_spans = any((s.get("spans") or []) for s in samples)
    print("has_spans", has_spans)
    assert has_spans or samples[0].get("duration_ms", 0) >= 0
    print("TRANSACTION_DETAIL_OK")


def check_crons(path=None):
    rows = load(path)
    if isinstance(rows, dict):
        rows = [rows]
    print("cron_count", len(rows))
    assert len(rows) >= 1, rows
    m = rows[0]
    assert m.get("token")
    assert m.get("schedule_sec", 0) > 0
    assert m.get("status") in ("ok", "late", "missed", "unknown")
    print("CRONS_OK", m["name"], m["status"], m["token"][:8])


def main():
    cmd = sys.argv[1] if len(sys.argv) > 1 else ""
    path = sys.argv[2] if len(sys.argv) > 2 else None
    if cmd == "transactions":
        check_transactions(path)
    elif cmd == "transaction_detail":
        check_transaction_detail(path)
    elif cmd == "crons":
        check_crons(path)
    else:
        print("usage: v2_validate.py transactions|transaction_detail|crons [file]", file=sys.stderr)
        sys.exit(2)


if __name__ == "__main__":
    main()
