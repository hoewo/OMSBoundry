#!/usr/bin/env python3
"""No Docker/SSH: verify the exact-scope stop helper and safe retry refusal."""
import copy
import importlib.util
import io
from pathlib import Path
import tempfile
from types import SimpleNamespace
import unittest
from unittest.mock import patch

spec = importlib.util.spec_from_file_location("stop_restore20", Path(__file__).with_name("stop-restore20.py"))
stop = importlib.util.module_from_spec(spec)
spec.loader.exec_module(stop)


def fixture(project):
    return {service: {"Id": project + ":" + service, "Image": "fixed:" + service,
                      "State": {"Running": True, "Health": {"Status": "healthy"}},
                      "Mounts": [{"Type": "volume", "Name": project + ":data", "Destination": "/data"},
                                 {"Type": "bind", "Source": "/fixture/config", "Destination": "/config"}]}
            for service in stop.probe.SERVICES.values()}


class StopTests(unittest.TestCase):
    def setUp(self):
        self.directory = tempfile.TemporaryDirectory(prefix="restore20-stop-unit-")
        self.addCleanup(self.directory.cleanup)
        self.output = Path(self.directory.name) / "evidence.json"
        self.before = self.output.with_name("before.json")
        self.restore = fixture("restore")
        self.primary = fixture("main")
        self.commands = []
        self.returncode = 0
        self.post = copy.deepcopy(self.restore)
        for value in self.post.values():
            value["State"]["Running"] = False
            value["Mounts"].reverse()
        self.post_by_id = {value["Id"]: value for value in self.post.values()}
        replacements = [patch.object(stop, "OUTPUT", self.output), patch.object(stop, "BEFORE", self.before),
                        patch.object(stop.os, "geteuid", return_value=0),
                        patch.dict(stop.os.environ, {"ALLOW_STOP_RESTORE20": "yes"}),
                        patch.object(stop, "containers", side_effect=self.inventory),
                        patch.object(stop.probe, "inspect", side_effect=lambda ident: self.post_by_id[ident]),
                        patch.object(stop.probe, "run", return_value=b"[]"),
                        patch.object(stop.subprocess, "run", side_effect=self.execute),
                        patch("sys.stdout", new=io.StringIO())]
        for replacement in replacements:
            replacement.start()
            self.addCleanup(replacement.stop)

    def inventory(self, project):
        return self.restore if project == stop.RESTORE_PROJECT else self.primary

    def execute(self, command, **kwargs):
        self.commands.append(command)
        return SimpleNamespace(returncode=self.returncode)

    def test_only_exact_restore_ids_stopped_and_reordered_mounts_accepted(self):
        stop.main()
        self.assertTrue(self.before.is_file() and self.output.is_file())
        self.assertEqual(len(self.commands), 6)
        self.assertTrue(all(command[:4] == ["docker", "stop", "--time", "20"] for command in self.commands))
        self.assertEqual({ident for command in self.commands for ident in command[4:]},
                         {value["Id"] for value in self.restore.values()})
        self.assertEqual(self.before.stat().st_mode & 0o777, 0o600)
        self.assertEqual(self.output.stat().st_mode & 0o777, 0o600)

    def test_existing_pre_snapshot_refuses_without_stop(self):
        self.before.touch()
        with self.assertRaises(AssertionError):
            stop.main()
        self.assertFalse(self.commands)

    def test_existing_final_evidence_refuses_without_stop(self):
        self.output.touch()
        with self.assertRaises(AssertionError):
            stop.main()
        self.assertFalse(self.commands)

    def test_missing_service_refuses_without_stop(self):
        self.restore.pop("nginx")
        with self.assertRaises(AssertionError):
            stop.main()
        self.assertFalse(self.commands)

    def test_unhealthy_primary_refuses_without_stop(self):
        self.primary["postgres"]["State"]["Health"]["Status"] = "unhealthy"
        with self.assertRaises(AssertionError):
            stop.main()
        self.assertFalse(self.commands)

    def test_shared_id_refuses_without_stop(self):
        self.restore["nginx"]["Id"] = self.primary["nginx"]["Id"]
        with self.assertRaises(AssertionError):
            stop.main()
        self.assertFalse(self.commands)

    def test_failed_stop_keeps_snapshot_and_refuses_retry(self):
        self.returncode = 1
        with self.assertRaises(AssertionError):
            stop.main()
        self.assertTrue(self.before.is_file())
        self.assertFalse(self.output.exists())
        self.assertEqual(len(self.commands), 1)
        with self.assertRaises(AssertionError):
            stop.main()
        self.assertEqual(len(self.commands), 1)

    def test_changed_mount_not_misclassified_as_order_only(self):
        self.post["postgres"]["Mounts"][0]["Source"] = "/changed/config"
        with self.assertRaises(AssertionError):
            stop.main()
        self.assertTrue(self.before.is_file())
        self.assertFalse(self.output.exists())


if __name__ == "__main__":
    unittest.main()
