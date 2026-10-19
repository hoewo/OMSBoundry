// View model only: identity/version remain Evidence-owned, reference BE-owned.
export type FileCategory = 'tender' | 'bid' | 'award_notice' | 'contract_main' | 'contract_amendment' | 'finalization_sheet'
// UI compatibility shapes, not replicated IAM or People domain entities.
export type EmploymentStatus = 'active' | 'resigned'
// Original Bonus UI compatibility only. Bonus retains native facts and history.
export interface BonusPersonnelOption {userId:string;roleLabel:string;name:string;uniqueKey:string;user?:User}
export interface CreateBonusAllocationBatchRequest {bonus_category:'business'|'production';base_amount:string;occurred_at:string;description?:string;items:{recipient_id:string;assignment_id:string;allocation_role:string;allocation_method:'ratio'|'fixed';allocation_ratio:number;amount:string}[]}
export interface ManualBonusView {id:string;project_id:string;bonus_category:'business'|'production';recipient:User;assignment_id:string;allocation_role:string;allocation_method:'ratio'|'fixed';allocation_ratio:number|null;allocation_base_amount:string|null;amount:string|null;occurred_at:string;description:string;state:'recorded'|'withdrawn';revision:number;batch_id:string;can_withdraw:boolean;can_history:boolean}
export interface User {id:string; username?:string; real_name?:string; phone?:string; department?:string; employment_status?:EmploymentStatus; roles?:string[]; role?:string}
export interface ProjectMember {id:string; project_id:string; user_id:string; role:string; is_active:boolean; user?:User}
export interface File {
  id: string
  file_id: string
  version: number
  material_reference_id?: string
  original_name: string
  file_name: string
  file_size: number
  mime_type: string
  created_at: string
}
// Original stage presentation shapes only. Production owns receipts/state;
// Evidence owns each file identity/version and bytes. Not a shared ORM model.
export type ProductionStage='scheme'|'preliminary'|'construction'|'change'|'completion'|'test'|'consultation'
export interface ProductionFile {id:string;project_id:string;file_id:string;stage:ProductionStage;file?:File;created_at:string;score?:string|null;review_sheet_file_id?:string}
export interface StageFileInfo {stage:ProductionStage;main_files:ProductionFile[];review_sheet?:ProductionFile|null;review_sheets:ProductionFile[];score?:string|null}
export type BusinessChainType='client_to_qualification'|'qualification_to_our_company'
// Display-only Money cash + Project relation, never a shared FinancialRecord domain.
export interface CashPaymentView {
  id:string;project_id:string;chain_type:BusinessChainType;fee_type:'design_fee'|'survey_fee'|'consultation_fee'
  amount:string|null;amount_visible:boolean;occurred_at:string;description:string
  payer:{id:string;revision:number;legal_name:string};payee:{id:string;revision:number;legal_name:string}
  ledger_owner_party_id:string;counterparty_party_id:string;account_reference:string;manual_source_reference:string
  status:'recorded'|'reversed';effective_amount:string|null;allocated_amount:string|null;unallocated_amount:string|null
  reversal:null|{id:string;reason:string;reversed_at:string};can_reverse:boolean
}
export interface CreateCashPaymentRequest {
  chain_type:BusinessChainType;fee_type:CashPaymentView['fee_type'];amount:string;occurred_at:string
  ledger_owner_party_id:string;counterparty_party_id:string;account_reference:string;manual_source_reference:string;description?:string
}
export type ContractAmendmentChangeType='adjustment'|'revision'|'no_amount_change'
export type CostScope='business'|'production'
export type ProductionCostType='vehicle'|'accommodation'|'transport'|'expert_fee'|'bid_signup_fee'|'bid_service_fee'|'other'
// Cost-owned overlay views. Amounts remain exact decimal strings, never floats.
export interface ProductionCost {
 id:string;project_id:string;cost_scope:CostScope;cost_type:ProductionCostType;revision:number;state:'recorded'|'withdrawn'
 amount:string|null;description?:string;incurred_at?:string;invoice_files:File[];invoice_file?:File;invoice_file_id?:string
 can_correct:boolean;can_withdraw:boolean;can_history:boolean;can_evidence:boolean
}
export interface CreateProductionCostRequest {
 cost_scope:CostScope;cost_type:ProductionCostType;amount:string;description?:string;incurred_at?:string
 invoice_file_ids:string[];invoice_file_id?:string;reason?:string
}
export interface InvoiceView {
 id:string;project_id:string;chain_type:BusinessChainType;fee_type:CashPaymentView['fee_type'];revision:number;link_source_revision:number
 status:'recorded'|'registration_invalidated';amount_visible:boolean;amount:string|null;occurred_at:string;description:string
 issuer:{id:string;revision:number;legal_name:string};recipient:{id:string;revision:number;legal_name:string}
 issuer_party_id:string;recipient_party_id:string;invoice_files:File[];can_correct:boolean;can_invalidate:boolean
}
export interface CreateInvoiceRequest {chain_type:BusinessChainType;fee_type:CashPaymentView['fee_type'];amount:string;occurred_at:string;issuer_party_id:string;recipient_party_id:string;description?:string}
export interface ContractAmendment {
  id:string;contract_id:string;amendment_number:string;sign_date:string;change_type:ContractAmendmentChangeType
  design_fee:string;survey_fee:string;consultation_fee:string;amendment_amount:string;effective_amount?:string
  contract_rate?:string;description?:string;cancelled?:boolean;created_at:string;updated_at:string
  amendment_files?:File[];amendment_file?:File
  can_edit?:boolean;native_confirmation?:{instrument_id:string;contract_revision:number;effective_amount:string;effective_on:string}
}
export interface CreateContractAmendmentRequest {
  amendment_number:string;sign_date:string;change_type:ContractAmendmentChangeType;design_fee:string;survey_fee:string;consultation_fee:string
  contract_rate?:string;amendment_file_id?:string;amendment_file_ids?:string[];description?:string
}
export interface Contract {
  id:string;project_id:string;chain_type:BusinessChainType;contract_number:string;sign_date:string
  contract_rate?:string;design_fee:string;survey_fee:string;consultation_fee:string;contract_amount:string
  settlement_follows_contract_amount:boolean;settlement_amount?:string;current_effective_amount?:string;supplementary_amount?:string
  native_settlement?:{id:string;sequence:number;source_revision:number;kind:'progress'|'final';amount:string;confirmed_on:string}
  current_design_fee?:string;current_survey_fee?:string;current_consultation_fee?:string;current_unallocated_amount?:string
  amendments:ContractAmendment[];contract_files?:File[];contract_file?:File;finalization_files?:File[];finalization_file?:File
  created_at:string;updated_at:string;native_status:string;can_edit:boolean
}
export interface CreateContractRequest {
  contract_number:string;sign_date:string;chain_type:BusinessChainType;contract_rate?:string
  design_fee:string;survey_fee:string;consultation_fee:string;contract_amount:string
  settlement_amount?:string;settlement_follows_contract_amount:boolean
  contract_file_ids?:string[];contract_file_id?:string;finalization_file_ids?:string[];finalization_file_id?:string
}
