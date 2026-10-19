"""Test only the pure readback gate; never import remote deployment helpers."""
import ast
from pathlib import Path
import unittest

source = ast.parse(Path(__file__).with_name('verify-public-continuous-browser-ecs.py').read_text())
selected = [node for node in source.body
            if isinstance(node, ast.FunctionDef) and node.name == 'phase_state_matches'
            or isinstance(node, ast.Assign) and any(isinstance(target, ast.Name) and target.id == 'EXPECTED' for target in node.targets)]
namespace = {}
exec(compile(ast.Module(body=selected, type_ignores=[]), '<pure phase gate>', 'exec'), namespace)
matches = namespace['phase_state_matches']


class PhaseGateTest(unittest.TestCase):
    def test_observed_failed_clear_cannot_pass(self):
        owners = [{'role_key': 'business_owner'}, {'role_key': 'production_owner'}]
        self.assertFalse(matches('production-clear', {'status': 'active', 'revision': 4}, owners))

    def test_revision_alone_does_not_prove_clear(self):
        owners = [{'role_key': 'business_owner'}, {'role_key': 'production_owner'}]
        self.assertFalse(matches('production-clear', {'status': 'active', 'revision': 5}, owners))

    def test_clear_preserves_ended_owner_history(self):
        owners = [{'role_key': 'business_owner'}, {'role_key': 'production_owner', 'ended_at': '2026-10-02T00:00:00Z'}]
        self.assertTrue(matches('production-clear', {'status': 'active', 'revision': 5}, owners))
        owners[0]['ended_at'] = '2026-10-02T00:00:01Z'
        self.assertTrue(matches('business-clear', {'status': 'active', 'revision': 6}, owners))

    def test_restoration_requires_both_current_roles(self):
        project = {'status': 'active', 'revision': 7}
        self.assertFalse(matches('owners-restored', project, [{'role_key': 'business_owner'}]))
        self.assertTrue(matches('owners-restored', project, [{'role_key': 'business_owner'}, {'role_key': 'production_owner'}]))

    def test_completion_and_noop_require_exact_state(self):
        self.assertTrue(matches('completed', {'status': 'active', 'revision': 3}, [{'role_key': 'business_owner'}]))
        self.assertFalse(matches('completed', {'status': 'draft', 'revision': 3}, [{'role_key': 'business_owner'}]))
        self.assertFalse(matches('no-op', {'status': 'active', 'revision': 5}, [{'role_key': 'business_owner'}, {'role_key': 'production_owner'}]))

    def test_unknown_phase_cannot_be_accepted(self):
        with self.assertRaises(KeyError):
            matches('unknown', {'status': 'active', 'revision': 7}, [])


if __name__ == '__main__':
    unittest.main()
