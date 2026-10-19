"""Pure assembly safety gates, without Docker or business API calls."""
import ast
import copy
from pathlib import Path
import unittest

tree = ast.parse(Path(__file__).with_name('deploy-public-slice-ecs.py').read_text())
selected = [node for node in tree.body if
            isinstance(node, ast.FunctionDef) and node.name in ('with_bidding_reference_endpoint', 'validate_project_reference_repair') or
            isinstance(node, ast.Assign) and any(isinstance(target, ast.Name) and target.id == 'BIDDING_REFERENCE_ENDPOINT' for target in node.targets)]
namespace = {'copy': copy}
exec(compile(ast.Module(body=selected, type_ignores=[]), '<pure reference configuration>', 'exec'), namespace)
repair = namespace['with_bidding_reference_endpoint']
validate = namespace['validate_project_reference_repair']
endpoint = namespace['BIDDING_REFERENCE_ENDPOINT']


class ReferenceConfigurationTest(unittest.TestCase):
    def test_target_agrees_with_current_declared_d1_edge(self):
        compose = Path(__file__).resolve().parents[2] / 'deployments/d1/compose.yaml'
        self.assertIn('BUSINESS_ENGAGEMENT_BASE_URL: ' + endpoint, compose.read_text())

    def test_only_one_endpoint_added_and_input_and_iam_preserved(self):
        original = {'services': {'identity-access': {'environment': {'policy': 'unchanged'}}, 'project-workspace': {'environment': {'IDENTITY_PROVIDER': 'iam'}, 'volumes': ['unchanged']}}}
        frozen = copy.deepcopy(original)
        result = repair(original)
        self.assertEqual(original, frozen)
        self.assertEqual(result['services']['identity-access'], original['services']['identity-access'])
        expected = copy.deepcopy(original)
        expected['services']['project-workspace']['environment']['BUSINESS_ENGAGEMENT_BASE_URL'] = endpoint
        self.assertEqual(result, expected)

    def test_conflicting_target_or_static_token_rejected(self):
        for environment in ({'BUSINESS_ENGAGEMENT_BASE_URL': 'http://another-service:8089'}, {'BUSINESS_ENGAGEMENT_ACCESS_TOKEN': 'unsafe-static-token'}):
            with self.subTest(environment=environment), self.assertRaises(AssertionError):
                repair({'services': {'project-workspace': {'environment': environment}}})

    def pair(self):
        before = {'Image': 'unchanged-project-image', 'Config': {'Env': ['IDENTITY_PROVIDER=iam', 'IAM_SERVICE_CREDENTIAL_URL=http://identity-access:8080/api/v1/service-credentials', 'DATABASE_URL=unchanged', 'WORKLOAD_ID=project-workspace']}, 'Mounts': [{'Destination': '/public', 'Source': 'same', 'RW': False}], 'HostConfig': {'PortBindings': {}}}
        after = copy.deepcopy(before)
        after['Config']['Env'].append('BUSINESS_ENGAGEMENT_BASE_URL=' + endpoint)
        return before, after

    def test_runtime_one_endpoint_change_passes(self):
        validate(*self.pair())

    def test_identical_mounts_in_another_docker_inspect_order_pass(self):
        before, after = self.pair()
        before['Mounts'].append({'Destination': '/workload-private', 'Source': 'same-key-volume', 'RW': False})
        after['Mounts'] = list(reversed(copy.deepcopy(before['Mounts'])))
        validate(before, after)

    def test_runtime_image_database_key_mount_port_or_extra_env_change_fails(self):
        for key in ('image', 'database', 'credential', 'mount', 'port', 'extra'):
            before, after = self.pair()
            if key == 'image': after['Image'] = 'different-image'
            elif key == 'database': after['Config']['Env'][2] = 'DATABASE_URL=different'
            elif key == 'credential': after['Config']['Env'].append('BUSINESS_ENGAGEMENT_ACCESS_TOKEN=static-token')
            elif key == 'mount': after['Mounts'][0]['RW'] = True
            elif key == 'port': after['HostConfig']['PortBindings'] = {'8086/tcp': [{'HostPort': '8086'}]}
            else: after['Config']['Env'].append('NEW_ENV=unexpected')
            with self.subTest(key=key), self.assertRaises(AssertionError): validate(before, after)


if __name__ == '__main__': unittest.main()
