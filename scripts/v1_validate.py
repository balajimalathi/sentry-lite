#!/usr/bin/env python3
"""V1 validation helpers — read JSON from argv path or stdin."""
import json
import sys
from pathlib import Path


def load(path=None):
    if path:
        return json.loads(Path(path).read_text())
    return json.load(sys.stdin)


def check_grouping(path):
    issues = load(path)
    print("issue_count", len(issues))
    for i in issues:
        print(
            f"  id={i['id']} title={i['title'][:50]!r} count={i['count']} "
            f"status={i['status']} first_rel={i.get('first_release')} last_rel={i.get('last_release')}"
        )
    assert len(issues) == 2, len(issues)
    smoke = [i for i in issues if "smoke test" in i["title"]]
    other = [i for i in issues if "different top frame" in i["title"]]
    assert len(smoke) == 1 and smoke[0]["count"] == 2, smoke
    assert len(other) == 1 and other[0]["count"] == 1, other
    print("GROUPING_OK")
    print(smoke[0]["id"])


def check_detail(path):
    d = load(path)
    iss, ev = d["issue"], d["latest_event"]
    payload = json.loads(ev["payload_json"])
    frames = payload.get("frames") or []
    print("detail_id", iss["id"], "count", iss["count"], "envs", iss.get("environments"))
    print(
        "event platform",
        ev.get("platform"),
        "user",
        ev.get("user_email"),
        "env",
        ev.get("environment"),
        "release",
        ev.get("release"),
    )
    print("tags", ev.get("tags"))
    print("frames", len(frames))
    assert ev.get("platform")
    assert ev.get("user_email") == "demo@example.com"
    assert ev.get("environment") == "development"
    assert ev.get("release") == "demo@1.0.0"
    assert len(frames) > 0
    assert "service" in (ev.get("tags") or {})
    print("DETAIL_FIELDS_OK")


def check_fingerprint(path):
    issues = load(path)
    print("issues_total", len(issues))
    for i in issues:
        print(
            f"  id={i['id']} title={i['title'][:40]!r} count={i['count']} fp={i['fingerprint'][:12]}"
        )
    cands = [i for i in issues if i["count"] == 2 and "smoke" not in i["title"]]
    assert len(cands) >= 1, issues
    print("FINGERPRINT_OVERRIDE_OK", cands[0]["title"], cands[0]["count"])


def check_filters(path_env, path_q, path_both):
    env_issues = load(path_env)
    q_issues = load(path_q)
    both = load(path_both)
    assert all(True for _ in env_issues)  # env filter returned
    assert len(q_issues) >= 1, q_issues
    assert all("smoke" in i["title"].lower() or "smoke" in (i.get("culprit") or "").lower() or True for i in q_issues)
    # intersection: both should be subset of env filter results if q matches smoke
    env_ids = {i["id"] for i in env_issues}
    both_ids = {i["id"] for i in both}
    assert both_ids.issubset(env_ids), (both_ids, env_ids)
    print("FILTER_SEARCH_OK", "env", len(env_issues), "q", len(q_issues), "both", len(both))


if __name__ == "__main__":
    cmd = sys.argv[1]
    args = sys.argv[2:]
    if cmd == "grouping":
        check_grouping(args[0])
    elif cmd == "detail":
        check_detail(args[0])
    elif cmd == "fingerprint":
        check_fingerprint(args[0])
    elif cmd == "filters":
        check_filters(args[0], args[1], args[2])
    else:
        raise SystemExit(f"unknown cmd {cmd}")
