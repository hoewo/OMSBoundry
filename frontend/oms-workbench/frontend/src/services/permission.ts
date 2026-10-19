import { contributorFor,contractForProject,cashForProject,invoiceForProject,bonusForProject } from './registry'
export const permissionService = {getEffectivePermissions: async(id: string):Promise<{permissions:Record<string,boolean>}>=>({permissions:{'project.business.member.assign':contributorFor(id).canManage()}}),canManageBusinessInfo:async(id:string):Promise<boolean>=>contractForProject(id).canManage(),canManageContract:async(project:string,id?:string):Promise<boolean>=>id?contractForProject(project).canEdit(id):contractForProject(project).canManage(),canManageContractAmendments:async(project:string,id:string):Promise<boolean>=>contractForProject(project).canManageAmendments(id)}
export const canManageCash=async(id:string):Promise<boolean>=>cashForProject(id).canManage()
export const canManageInvoice=async(id:string):Promise<boolean>=>invoiceForProject(id).canManage()
export const canManageBonus=async(id:string,scope:string):Promise<boolean>=>bonusForProject(id,scope).canManage()
