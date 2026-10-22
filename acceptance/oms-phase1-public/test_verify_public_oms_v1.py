import importlib.util
from pathlib import Path
import unittest


SCRIPT = Path(__file__).with_name("verify-public-oms-v1.py")
SPEC = importlib.util.spec_from_file_location("verify_public_oms_v1", SCRIPT)
MODULE = importlib.util.module_from_spec(SPEC)
assert SPEC.loader is not None
SPEC.loader.exec_module(MODULE)


class DirectoryAuthorityTest(unittest.TestCase):
    def test_requires_directory_authority_for_contract_and_money(self):
        ready = {
            "party_runtime_mode": "directory",
            "directory_authority_established": True,
        }
        MODULE.require_directory_authority("contract-ledger", ready)
        MODULE.require_directory_authority("money-flow-ledger", ready)

        for application in ("contract-ledger", "money-flow-ledger"):
            with self.assertRaisesRegex(RuntimeError, "authority is not established"):
                MODULE.require_directory_authority(
                    application,
                    {
                        "party_runtime_mode": "local",
                        "directory_authority_established": False,
                    },
                )

    def test_does_not_apply_to_other_owner_documents(self):
        MODULE.require_directory_authority("project-workspace", {"items": []})


if __name__ == "__main__":
    unittest.main()
