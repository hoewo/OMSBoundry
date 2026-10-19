import { adapterFor } from '../services/registry'
export const useProjectPermissions = (id: string): {canUploadBusinessFile: boolean; canDeleteBusinessFile: boolean} => adapterFor(id).permissions()
export const PROJECT_PERMISSION_KEYS = {businessMemberAssign:'project.business.member.assign'}
