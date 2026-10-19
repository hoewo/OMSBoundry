import importlib.util, json, unittest
from pathlib import Path
from unittest.mock import patch
spec=importlib.util.spec_from_file_location('material_deploy',Path(__file__).with_name('deploy-bidding-materials-ecs.py'))
deploy=importlib.util.module_from_spec(spec);spec.loader.exec_module(deploy)

class NativeOpsEntrypoint(unittest.TestCase):
    def invoke(self,kind,original=False):
        calls=[]
        def inspect(name):
            if name.startswith('omsbizfoundry-'):
                return {'Image':'original-image','Config':{'Env':['DATABASE_URL=fixture-only']},'Mounts':[]}
            return {'State':{'ExitCode':0,'OOMKilled':False}}
        def run(args,*_,**__):calls.append(args);return b'current-schema'
        published={'images':{'business-engagement':{'reference':'candidate-image'}}}
        with patch.object(deploy,'inspect',side_effect=inspect),patch.object(deploy,'run',side_effect=run),patch.object(Path,'read_text',return_value=json.dumps(published)):
            if kind=='business':deploy.business_ops(False,'migrate',original=original)
            else:deploy.project_ops(False,'check')
        create=calls[0]
        self.assertEqual(create[create.index('--entrypoint')+1],'/app/ops')
        self.assertNotIn('/app/server',create)
        self.assertEqual(create[-1],'migrate' if kind=='business' else 'check')
        self.assertEqual(create.count('/app/ops'),1)
        self.assertIn('--read-only',create)
        self.assertIn('ALL',create)
        return create

    def test_new_be_ops_does_not_start_image_server(self):
        self.assertEqual(self.invoke('business')[-2],'candidate-image')

    def test_original_be_backup_schema_is_checked_with_ops(self):
        self.assertEqual(self.invoke('business',True)[-2],'original-image')

    def test_project_ops_explicit_entrypoint_is_compatible(self):
        self.assertEqual(self.invoke('project')[-2],'original-image')

if __name__=='__main__':unittest.main()
