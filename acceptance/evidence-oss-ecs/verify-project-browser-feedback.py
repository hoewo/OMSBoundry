#!/usr/bin/env python3
"""Capture and verify a single browser command on the retained test Project.

SQL is read-only; this never executes a business command or replaces a DB.
Rows/IDs are retained in root-only evidence, not public output.
"""
import importlib.util
import json
import os
from pathlib import Path
import sys

PROJECT_ID = "b6e26691-cfd6-4ef4-ba53-8d9f9a8c2cc2"
ROOT = Path("/opt/omsbizfoundry/acceptance/evidence-oss-transition-20261001")
BEFORE = ROOT / "project-browser-feedback-before.json"
AFTER = ROOT / "project-browser-feedback-after.json"
spec = importlib.util.spec_from_file_location("probe", Path(__file__).with_name("rollout-project-feedback.py"))
probe = importlib.util.module_from_spec(spec)
spec.loader.exec_module(probe)


def collect():
    tables = json.loads(probe.sql("project_workspace", "SELECT json_agg(tablename ORDER BY tablename) FROM pg_tables WHERE schemaname='public';"))
    assert len(tables) == 8
    untouched = {}
    counts = {}
    for table in tables:
        ident = '"' + table.replace('"', '""') + '"'
        if table == "projects":
            predicate = "t.id <> '" + PROJECT_ID + "'::uuid"
        elif table == "project_commands":
            predicate = "NOT EXISTS (SELECT 1 FROM project_history_events h WHERE h.project_id='" + PROJECT_ID + "'::uuid AND h.tenant_id=t.tenant_id AND h.command_id=t.command_id)"
        else:
            predicate = "coalesce(to_jsonb(t)->>'project_id','') <> '" + PROJECT_ID + "'"
        untouched[table] = probe.sql("project_workspace", "SELECT count(*)::text || ':' || md5(coalesce(string_agg(to_jsonb(t)::text,E'\\n' ORDER BY to_jsonb(t)::text),'')) FROM public." + ident + " t WHERE " + predicate + ";")
        counts[table] = int(probe.sql("project_workspace", "SELECT count(*) FROM public." + ident + ";"))
    row = json.loads(probe.sql("project_workspace", "SELECT to_jsonb(p) FROM projects p WHERE id='" + PROJECT_ID + "';"))
    assert row["project_number"] == "OA-VERIFY-20261001-BROWSER-1634"
    history = json.loads(probe.sql("project_workspace", "SELECT json_agg(to_jsonb(h) ORDER BY project_revision,event_sequence) FROM project_history_events h WHERE project_id='" + PROJECT_ID + "';"))
    return {"other_project_facts": untouched, "counts": counts, "project": row, "history": history}


def write(path, value):
    with os.fdopen(os.open(path, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600), "w") as target:
        json.dump(value, target)


def main(mode):
    assert os.geteuid() == 0
    value = collect()
    if mode == "before":
        assert not BEFORE.exists() and not AFTER.exists()
        assert value["project"]["revision"] == 2 and value["project"]["status"] == "pending_completion"
        assert len(value["history"]) == 2
        write(BEFORE, value)
        print(json.dumps({"phase": "before", "revision": 2, "history_events": 2, "tables": 8}))
        return
    before = json.loads(BEFORE.read_text())
    assert value["other_project_facts"] == before["other_project_facts"]
    if mode == "after":
        assert not AFTER.exists()
        assert value["project"]["revision"] == 3 and value["project"]["status"] == "pending_completion"
        assert value["history"][:2] == before["history"] and len(value["history"]) == 3
        assert value["history"][-1]["event_type"] == "project_initiation_pending"
        changed = {key: value["counts"][key] - count for key, count in before["counts"].items()}
        assert changed == {key: (1 if key in ("project_commands", "project_history_events") else 0) for key in changed}
        value["result"] = "pass"
        write(AFTER, value)
        print(json.dumps({"result": "pass", "scope": "one browser incomplete-initiation command", "revision": 3,
                          "history_events": 3, "new_commands": 1, "other_project_facts_unchanged": True}))
    elif mode == "frozen":
        after = json.loads(AFTER.read_text())
        assert all(value[key] == after[key] for key in value)
        print(json.dumps({"result": "pass", "scope": "no command replay during expiry/permission refresh", "revision": 3, "project_facts_unchanged": True}))
    else:
        raise AssertionError("unknown phase")


if __name__ == "__main__":
    try:
        assert len(sys.argv) == 2
        main(sys.argv[1])
    except Exception as error:
        trace = error.__traceback__
        while trace.tb_next:
            trace = trace.tb_next
        raise SystemExit("browser feedback postcheck failed: " + type(error).__name__ + " at line " + str(trace.tb_lineno))
