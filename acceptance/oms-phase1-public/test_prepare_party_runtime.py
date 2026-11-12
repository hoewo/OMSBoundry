import importlib.util
import json
from pathlib import Path
import stat
import subprocess
import tempfile
import unittest
from unittest import mock


SCRIPT = Path(__file__).with_name("prepare-party-runtime.py")
SPEC = importlib.util.spec_from_file_location("prepare_party_runtime", SCRIPT)
MODULE = importlib.util.module_from_spec(SPEC)
assert SPEC and SPEC.loader
SPEC.loader.exec_module(MODULE)


def party(identifier, *, party_id="target", name="名称", kind="organization", status="active"):
    return {
        "id": party_id,
        "kind": kind,
        "legal_name": name,
        "registration_identifier": identifier,
        "status": status,
        "revision": 1,
    }


class PartyRuntimePlannerTests(unittest.TestCase):
    def test_exact_registration_wins(self):
        local = {"registration_identifier": "REG-1", "legal_name": "甲", "kind": "organization"}
        target = party("REG-1", party_id="exact")
        self.assertEqual(MODULE.select_directory_party(local, [target])["id"], "exact")

    def test_d1_fixture_suffix_matches_across_owner_prefixes(self):
        local = {
            "registration_identifier": "D1-C-20260927T070458Z-758e9889",
            "legal_name": "D1 Contract",
            "kind": "organization",
        }
        target = party("D1-PARTY-20260927T070458Z-758e9889", party_id="fixture")
        self.assertEqual(MODULE.select_directory_party(local, [target])["id"], "fixture")

    def test_unique_exact_name_without_identifier_is_allowed(self):
        local = {"registration_identifier": None, "legal_name": "北京京宏", "kind": "organization"}
        target = party(None, party_id="name", name="北京京宏")
        self.assertEqual(MODULE.select_directory_party(local, [target])["id"], "name")

    def test_unrelated_party_returns_none(self):
        local = {"registration_identifier": "NEW-1", "legal_name": "新主体", "kind": "organization"}
        self.assertIsNone(MODULE.select_directory_party(local, [party("OTHER")]))

    def test_ambiguous_fixture_match_fails_closed(self):
        local = {
            "registration_identifier": "D1-M-20260927T070458Z-758e9889",
            "legal_name": "D1 Money",
            "kind": "organization",
        }
        matches = [
            party("D1-PARTY-20260927T070458Z-758e9889", party_id="one"),
            party("D1-C-20260927T070458Z-758e9889", party_id="two"),
        ]
        with self.assertRaisesRegex(RuntimeError, "ambiguous D1 fixture"):
            MODULE.select_directory_party(local, matches)

    def test_inactive_exact_match_is_not_reused(self):
        local = {"registration_identifier": "REG-1", "legal_name": "甲", "kind": "organization"}
        self.assertIsNone(MODULE.select_directory_party(local, [party("REG-1", status="inactive")]))

    @mock.patch.object(MODULE.subprocess, "check_output")
    def test_preflight_overrides_container_tenant(self, check_output):
        check_output.return_value = '{"items": []}'
        result = MODULE.run_ops(
            "contract-container", "party-migration-preflight", "tenant-current"
        )
        self.assertEqual(result, {"items": []})
        check_output.assert_called_once_with(
            [
                "docker",
                "exec",
                "-e",
                "IAM_TENANT_ID=tenant-current",
                "contract-container",
                "/app/ops",
                "party-migration-preflight",
                "--tenant-id",
                "tenant-current",
            ],
            text=True,
        )

    @mock.patch.object(MODULE.subprocess, "run")
    def test_mapping_overrides_container_tenant(self, run):
        run.return_value = subprocess.CompletedProcess(
            args=[], returncode=0, stdout='{"recorded": 1}', stderr=""
        )
        result = MODULE.run_ops(
            "money-container", "party-migration-map", "tenant-current", {"value": 1}
        )
        self.assertEqual(result, {"recorded": 1})
        self.assertEqual(
            run.call_args.args[0][:7],
            [
                "docker",
                "exec",
                "-i",
                "-e",
                "IAM_TENANT_ID=tenant-current",
                "money-container",
                "/app/ops",
            ],
        )

    def test_login_credentials_file_must_be_owner_only(self):
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory, "login.json")
            path.write_text(json.dumps({"username": "admin", "password": "secret"}))
            path.chmod(stat.S_IRUSR | stat.S_IWUSR | stat.S_IRGRP)
            with self.assertRaisesRegex(RuntimeError, "readable only by its owner"):
                MODULE.load_login_credentials({}, str(path))

    def test_login_credentials_file_is_loaded_without_echoing(self):
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory, "login.json")
            path.write_text(json.dumps({"username": "admin", "password": "secret"}))
            path.chmod(stat.S_IRUSR | stat.S_IWUSR)
            self.assertEqual(
                MODULE.load_login_credentials({}, str(path)),
                {"username": "admin", "password": "secret"},
            )


if __name__ == "__main__":
    unittest.main()
