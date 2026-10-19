// Presentation ports only. No production endpoint/schema/role is invented here.
// Normal Workbench selection is explicit; native owner, enrollment and IAM
// capabilities remain independent gates, not permissions inferred from this UI.
import {createContext,useContext} from 'react'
import {useQuery} from '@tanstack/react-query'
import type {User} from './types'
export type MemberRole='designer'|'participant'|'reviewer'|'auditor'|'approver'|'estimator'
export interface Discipline {id:string;name:string;description?:string;is_active?:boolean}
export interface ProjectMember {id:string;project_id:string;user_id:string;role:MemberRole;discipline_id?:string;discipline?:Discipline;user?:User}
export interface CreateDisciplineRequest {name:string;description?:string}
export type UpdateDisciplineRequest=CreateDisciplineRequest
export interface DisciplinePersonnelInput {
 original_discipline_id?:string;discipline_id:string;designer_ids?:string[];participant_ids?:string[];participant_operation:'preserve'|'replace';reviewer_id?:string;auditor_id?:string
}
export interface ProductionPersonnelUIPort {
 projectID:string
 listMembers(projectID:string):Promise<ProjectMember[]>
 candidates(projectID:string):Promise<User[]>
 permissions(projectID:string):Promise<{can_manage:boolean;can_manage_disciplines:boolean;participant_not_applicable:boolean;not_enrolled?:boolean}>
 initialize():Promise<void>
 saveDiscipline(projectID:string,input:DisciplinePersonnelInput):Promise<void>
 saveReviewers(projectID:string,input:{approver_id?:string;estimator_id?:string}):Promise<void>
 endDiscipline(projectID:string,disciplineID:string,input:{participant_operation:'preserve'|'end'}):Promise<void>
 listDisciplines(includeInactive:boolean):Promise<Discipline[]>
 createDiscipline(input:CreateDisciplineRequest):Promise<Discipline>
 updateDiscipline(id:string,input:UpdateDisciplineRequest):Promise<Discipline>
 deleteDiscipline(id:string):Promise<void>
}
export const ProductionPersonnelContext=createContext<ProductionPersonnelUIPort|null>(null)
export function useProductionPersonnelPort(){
 const port=useContext(ProductionPersonnelContext)
 if(!port)throw new Error('原生产人员界面的显式端口未配置')
 return port
}
export function useProductionPersonnelPermissions(projectID:string){
 const port=useProductionPersonnelPort()
 if(projectID!==port.projectID)throw new Error('生产人员界面项目上下文不一致')
 const query=useQuery({queryKey:['productionPersonnelPermissions',projectID],queryFn:()=>port.permissions(projectID)})
 return {...query,initialize:()=>port.initialize(),canAssignProductionMembers:query.data?.can_manage===true,canManageDisciplines:query.data?.can_manage_disciplines===true,participantNotApplicable:query.data?.participant_not_applicable===true,notEnrolled:query.data?.not_enrolled===true}
}
export const useProductionPersonnelCandidates=(projectID:string,visible:boolean)=>{
 const port=useProductionPersonnelPort()
 if(projectID!==port.projectID)throw new Error('生产候选界面项目上下文不一致')
 return useQuery({queryKey:['productionPersonnelCandidates',projectID],queryFn:()=>port.candidates(projectID),enabled:visible})
}
