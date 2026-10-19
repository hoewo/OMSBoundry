#!/usr/bin/env python3
"""Read-only checks for two synthetic owner selections and fixture activation.

This does not issue commands, grant permissions, edit People or restore data.
Only non-IAM-linked synthetic Persons are accepted for the browser fixture.
"""
import importlib.util
import json
import os
from pathlib import Path
import sys

ROOT = Path("/opt/omsbizfoundry/acceptance/evidence-oss-transition-20261001")
BEFORE = ROOT / "project-picker-browser-before.json"
AFTER = ROOT / "project-picker-browser-after.json"
EXPECTED = {"business_owner": "c8e3d6aa-f797-4847-b766-4a2d4cb9a78e",
            "production_owner": "ce9e936b-6ede-42af-9f70-b448ac1f4a8c"}
spec = importlib.util.spec_from_file_location("feedback", Path(__file__).with_name("verify-project-browser-feedback.py"))
feedback = importlib.util.module_from_spec(spec)
spec.loader.exec_module(feedback)


def collect():
    value = feedback.collect()
    value["people_facts"] = feedback.probe.facts("people_groups")
    ids = ",".join("'" + person + "'" for person in EXPECTED.values())
    value["persons"] = json.loads(feedback.probe.sql("people_groups", "SELECT json_agg(to_jsonb(p) ORDER BY id) FROM persons p WHERE id IN (" + ids + ");"))
    assert len(value["persons"]) == 2
    for person in value["persons"]:
        assert person["tenant_id"] == value["project"]["tenant_id"]
        assert person["status"] == "active" and person["revision"] == 1
        assert person["iam_subject_id"] is None
        assert person["reference_code"].startswith("D1-P-") and person["display_name"].startswith("D1 Person ")
    value["assignments"] = json.loads(feedback.probe.sql("project_workspace", "SELECT coalesce(json_agg(to_jsonb(a) ORDER BY role_key),'[]') FROM project_assignments a WHERE project_id='" + feedback.PROJECT_ID + "';"))
    return value


def main(mode):
    assert os.geteuid() == 0
    value = collect()
    if mode == "before":
        assert not BEFORE.exists() and not AFTER.exists()
        assert value["project"]["revision"] == 3 and value["project"]["status"] == "pending_completion"
        assert len(value["history"]) == 3 and not value["assignments"]
        feedback.write(BEFORE, value)
        print(json.dumps({"phase": "before", "revision": 3, "synthetic_persons_without_iam": 2, "people_tables": value["people_facts"]["tables"]}))
        return
    before = json.loads(BEFORE.read_text())
    assert value["other_project_facts"] == before["other_project_facts"]
    assert value["people_facts"] == before["people_facts"] and value["persons"] == before["persons"]
    if mode == "after":
        assert not AFTER.exists()
        assert value["project"]["revision"] == 6 and value["project"]["status"] == "active"
        stable_fields = set(value["project"]) - {"revision", "status", "updated_at"}
        assert all(value["project"][key] == before["project"][key] for key in stable_fields)
        assert len(value["history"]) == 6 and value["history"][:3] == before["history"]
        assert [item["event_type"] for item in value["history"][3:]] == ["assignment_started", "assignment_started", "project_activated"]
        assert len(value["assignments"]) == 2
        persons = {person["id"]: person for person in value["persons"]}
        for assignment in value["assignments"]:
            assert assignment["subject_kind"] == "person" and assignment["subject_id"] == EXPECTED[assignment["role_key"]]
            assert assignment["source_revision"] == 1 and assignment["ended_at"] is None
            assert assignment["display_snapshot"] == persons[assignment["subject_id"]]["display_name"]
        delta = {key: value["counts"][key] - count for key, count in before["counts"].items()}
        assert delta == {key: (3 if key in ("project_commands", "project_history_events") else 2 if key == "project_assignments" else 0) for key in delta}
        feedback.write(AFTER, value)
        print(json.dumps({"result": "pass", "scope": "two owner picker commands and fixture initiation", "revision": 6, "status": "active", "new_commands": 3, "new_assignments": 2, "other_projects_unchanged": True, "people_fourteen_tables_unchanged": True}))
    elif mode == "frozen":
        assert value == json.loads(AFTER.read_text())
        print(json.dumps({"result": "pass", "scope": "browser refresh without command replay", "revision": 6, "project_and_people_facts_unchanged": True}))
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
        raise SystemExit("picker postcheck failed: " + type(error).__name__ + " at line " + str(trace.tb_lineno))
