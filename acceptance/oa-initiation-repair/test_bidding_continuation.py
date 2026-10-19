"""Pure preservation gate tests; no remote deployment imports or business writes."""
import ast
import copy
from pathlib import Path
import unittest

source = ast.parse(Path(__file__).with_name('verify-public-bidding-ecs.py').read_text())
selected = [node for node in source.body if
            isinstance(node, ast.FunctionDef) and node.name in ('validate_continuation','validate_receipt_repair') or
            isinstance(node, ast.Assign) and any(isinstance(target, ast.Name) and target.id in ('CONTINUED_FIXTURE','CONTINUED_SOURCE') for target in node.targets)]
namespace = {}
exec(compile(ast.Module(body=selected, type_ignores=[]), '<pure bidding continuation>', 'exec'), namespace)
validate = namespace['validate_continuation']
fixture = namespace['CONTINUED_FIXTURE']


class ContinuationTest(unittest.TestCase):
    def setUp(self):
        self.baseline = dict(passed=True, fixture_id=None, existing_rows={'project': 'old'}, grants_digest='grant', user_facts_sha256='user', assets={'reference-oa-initiation-runtime.js':'init', 'reference-oa-business-runtime.js':'business'}, other_business_databases={'money':'unchanged'})
        self.completed = dict(passed=True, fixture_id=fixture, project_revision=3, no_get_creation=True)
        self.result = copy.deepcopy(self.baseline)
        self.result['fixture_id'] = fixture

    def test_same_failed_fixture_and_unchanged_owners_pass(self):
        validate(self.result, self.baseline, self.completed)

    def test_new_fixture_cannot_hide_failure(self):
        self.result['fixture_id'] = 'another-project'
        with self.assertRaises(AssertionError): validate(self.result, self.baseline, self.completed)

    def test_existing_business_or_grants_or_user_mutation_fails(self):
        for key in ('existing_rows', 'grants_digest', 'user_facts_sha256', 'other_business_databases'):
            with self.subTest(key=key):
                result=copy.deepcopy(self.result); result[key]='changed'
                with self.assertRaises(AssertionError): validate(result, self.baseline, self.completed)

    def test_failed_or_already_written_old_observation_fails(self):
        for key, value in (('passed',False), ('no_get_creation',False), ('project_revision',4)):
            with self.subTest(key=key):
                completed=copy.deepcopy(self.completed); completed[key]=value
                with self.assertRaises(AssertionError): validate(self.result, self.baseline, completed)

    def test_unrelated_frontend_assets_must_stay_equal(self):
        self.result['assets']['reference-oa-business-runtime.js']='changed'
        with self.assertRaises(AssertionError): validate(self.result, self.baseline, self.completed)

    def test_receipt_repair_reuses_failed_source(self):
        baseline=copy.deepcopy(self.result)
        failed=dict(passed=False,project=dict(id=fixture,revision=3),engagement=[dict(id=namespace['CONTINUED_SOURCE'],project_id=fixture,revision=1,applicability='undetermined',engagement_method=None)])
        result=copy.deepcopy(self.result);result['engagement_id']=namespace['CONTINUED_SOURCE']
        namespace['validate_receipt_repair'](result,baseline,failed)
        result['engagement_id']='new-source'
        with self.assertRaises(AssertionError): namespace['validate_receipt_repair'](result,baseline,failed)

    def test_source_identity_matches_actual_failed_command_not_transcribed_id(self):
        # Literal from the immutable i command-response-mismatch receipt, not
        # derived from the verifier constant under test.
        actual='231f2977-79d4-4b36-a178-f19a0d8d3119'
        self.assertEqual(namespace['CONTINUED_SOURCE'],actual)
        baseline=copy.deepcopy(self.result)
        failed=dict(passed=False,project=dict(id=fixture,revision=3),engagement=[dict(id=actual,project_id=fixture,revision=1,applicability='undetermined',engagement_method=None)])
        result=copy.deepcopy(self.result);result['engagement_id']=actual
        namespace['validate_receipt_repair'](result,baseline,failed)
        result['engagement_id']='1231f297-79d4-4b36-a178-f19a0d8d3119'
        with self.assertRaises(AssertionError): namespace['validate_receipt_repair'](result,baseline,failed)

    def test_receipt_repair_cannot_hide_an_already_changed_failed_source(self):
        baseline=copy.deepcopy(self.result)
        failed=dict(passed=False,project=dict(id=fixture,revision=3),engagement=[dict(id=namespace['CONTINUED_SOURCE'],project_id=fixture,revision=2,applicability='applicable',engagement_method=None)])
        result=copy.deepcopy(self.result);result['engagement_id']=namespace['CONTINUED_SOURCE']
        with self.assertRaises(AssertionError): namespace['validate_receipt_repair'](result,baseline,failed)


if __name__ == '__main__': unittest.main()
