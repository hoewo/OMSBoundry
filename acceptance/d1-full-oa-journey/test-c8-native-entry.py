#!/usr/bin/env python3
"""Exercise the actual C8 cookie assertion without a server or IAM writes."""
import base64
import json
from pathlib import Path
import re
import subprocess
import tempfile
import unittest


class NativeEntryAssertion(unittest.TestCase):
    def setUp(self):
        source = Path(__file__).with_name("verify-c8").read_text()
        self.helper = re.search(r"^assert_evidence_actions\(\) \{\n.*?^\}", source, re.M | re.S).group()
        self.directory = tempfile.TemporaryDirectory(prefix="c8-native-entry-")
        self.cookies = Path(self.directory.name) / "cookies"

    def tearDown(self):
        self.directory.cleanup()

    def check(self, actions, expected, *, tenant="tenant-a", file_id="file-a", audience="evidence-service", cookie_name="oms_cap_evidence_service"):
        claims = {"tid": tenant, "aud": audience, "actions": actions,
                  "scope": {"kind": "resources", "resources": [{"type": "file", "id": file_id}]}}
        payload = base64.urlsafe_b64encode(json.dumps(claims).encode()).decode().rstrip("=")
        self.cookies.write_text("#HttpOnly_fixture\tFALSE\t/evidence\tFALSE\t0\t" + cookie_name + "\theader." + payload + ".signature\n")
        result = subprocess.run(["sh", "-eu", "-c", self.helper + '\nparticipant_cookies=$1; tenant_id=tenant-a; material_file=file-a; assert_evidence_actions "$2"', "fixture", str(self.cookies), json.dumps(expected)], capture_output=True)
        return result.returncode

    def test_three_native_action_sets(self):
        for actions in (["evidence-service.metadata.read"], ["evidence-service.download"], ["evidence-service.metadata.read", "evidence-service.download"]):
            with self.subTest(actions=actions):
                self.assertEqual(self.check(actions, list(reversed(actions))), 0)

    def test_view_workaround_rejected(self):
        self.assertNotEqual(self.check(["evidence-service.download", "evidence-service.view"], ["evidence-service.download"]), 0)

    def test_wrong_action_rejected(self):
        self.assertNotEqual(self.check(["evidence-service.upload"], ["evidence-service.download"]), 0)

    def test_identity_and_scope_rejected(self):
        for overrides in ({"tenant": "neighbor"}, {"file_id": "neighbor"}, {"audience": "project-workspace"}, {"cookie_name": "unrelated"}):
            with self.subTest(overrides=overrides):
                self.assertNotEqual(self.check(["evidence-service.download"], ["evidence-service.download"], **overrides), 0)


if __name__ == "__main__":
    unittest.main()
