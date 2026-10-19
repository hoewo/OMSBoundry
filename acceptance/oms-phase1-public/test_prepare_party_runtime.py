import importlib.util
from pathlib import Path
import unittest


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


if __name__ == "__main__":
    unittest.main()
