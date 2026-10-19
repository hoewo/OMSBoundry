import re
import unittest
from pathlib import Path


ROOT = Path(__file__).resolve().parents[2]
SOURCE = ROOT / "apps" / "unified-workbench" / "internal" / "server" / "oms_phase1.js"


class OMSPhase1DependencyBoundaryTests(unittest.TestCase):
    def test_workbench_calls_only_the_six_phase1_fact_owners(self) -> None:
        source = SOURCE.read_text(encoding="utf-8")
        called = set(re.findall(r"(?:api|authorize|app)\('([^']+)'", source))
        self.assertEqual(
            called,
            {
                "party-directory",
                "project-workspace",
                "contract-ledger",
                "billing-invoice",
                "money-flow-ledger",
                "evidence-service",
            },
        )

    def test_non_phase1_business_apps_are_not_runtime_dependencies(self) -> None:
        source = SOURCE.read_text(encoding="utf-8")
        forbidden = {
            "people-groups",
            "business-engagement",
            "production-operations",
            "approval-audit-results",
            "cost-outsourcing",
            "bonus-distribution",
            "design-operations",
        }
        called = set(re.findall(r"(?:api|authorize|app)\('([^']+)'", source))
        self.assertTrue(called.isdisjoint(forbidden))


if __name__ == "__main__":
    unittest.main()
