import type {CostScope,CreateProductionCostRequest,ProductionCost} from '../types'
import {costForProject} from './registry'
export const productionService={
 listCosts:async(id:string|number,scope:CostScope):Promise<ProductionCost[]>=>costForProject(String(id),scope).list(),
 getCostStatistics:async(id:string|number,scope:CostScope)=>costForProject(String(id),scope).statistics(),
 createCost:async(id:string|number,data:CreateProductionCostRequest,scope:CostScope)=>costForProject(String(id),scope).create(data),
 updateCost:async(id:string|number,costID:string,data:CreateProductionCostRequest,scope:CostScope)=>costForProject(String(id),scope).correct(costID,data),
 withdrawCost:async(id:string|number,costID:string,scope:CostScope,reason:string)=>costForProject(String(id),scope).withdraw(costID,reason),
 history:async(id:string|number,costID:string,scope:CostScope)=>costForProject(String(id),scope).history(costID),
 canCreate:(id:string|number,scope:CostScope):boolean=>costForProject(String(id),scope).canCreate(),
 canEvidence:(id:string|number,scope:CostScope,costID?:string):boolean=>costForProject(String(id),scope).canEvidence(costID),
 stagedFiles:(id:string|number,scope:CostScope)=>costForProject(String(id),scope).stagedFiles(),
}
