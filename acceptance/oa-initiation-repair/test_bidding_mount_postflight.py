"""Pure recovery checks; never reconstruct an unavailable native before snapshot."""
import ast
import copy
from pathlib import Path
import unittest

source = ast.parse(Path(__file__).with_name('finish-bidding-reference-rollout-ecs.py').read_text())
namespace = {}
exec(compile(ast.Module(body=[node for node in source.body if isinstance(node, ast.FunctionDef) and node.name == 'validate_mounts'], type_ignores=[]), '<pure mount postflight>', 'exec'), namespace)
validate = namespace['validate_mounts']


class MountPostflightTest(unittest.TestCase):
    def setUp(self):
        self.expected = {'/public': {'Type': 'volume', 'Name': 'original-key-volume', 'Source': '/original', 'RW': False}}
        self.actual = [{'Destination': '/public', **self.expected['/public']}, {'Destination': '/var/lib/postgresql', 'Type': 'volume', 'Name': 'old-unused-image-volume', 'RW': True}]
        self.image_volumes = {'/var/lib/postgresql': {}}
        self.created = {'original-key-volume': 1, 'old-unused-image-volume': 2}

    def test_original_declarative_mounts_and_preexisting_image_volume_pass_in_either_order(self):
        validate(self.expected, self.actual, self.image_volumes, self.created, 3)
        validate(self.expected, list(reversed(self.actual)), self.image_volumes, self.created, 3)

    def test_changed_source_name_permissions_missing_extra_or_duplicate_mount_fail(self):
        for kind in ('source', 'name', 'permissions', 'missing', 'extra', 'duplicate'):
            actual = copy.deepcopy(self.actual)
            if kind == 'source': actual[0]['Source'] = '/another'
            elif kind == 'name': actual[0]['Name'] = 'another-volume'
            elif kind == 'permissions': actual[0]['RW'] = True
            elif kind == 'missing': actual.pop(0)
            elif kind == 'extra': actual.append({'Destination': '/unexpected', 'Type': 'bind'})
            else: actual.append(copy.deepcopy(actual[0]))
            with self.subTest(kind=kind), self.assertRaises(AssertionError):
                validate(self.expected, actual, self.image_volumes, self.created, 3)

    def test_newly_created_or_substituted_image_volume_fails(self):
        created = dict(self.created, **{'old-unused-image-volume': 4})
        with self.assertRaises(AssertionError): validate(self.expected, self.actual, self.image_volumes, created, 3)


if __name__ == '__main__': unittest.main()
