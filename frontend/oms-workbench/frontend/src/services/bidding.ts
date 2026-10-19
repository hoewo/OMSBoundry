import { adapterFor, type BiddingInfo } from './registry'
export const biddingService = {
  getBiddingInfo: async (id: string): Promise<BiddingInfo> => adapterFor(id).info(),
  appendUploadedFile: (id: string, kind: string, viewID: string) => adapterFor(id).append(id, kind, viewID),
  removeMaterialReference: (id: string, kind: string, referenceID: string) => adapterFor(id).remove(id, kind, referenceID),
  createOrUpdateBiddingInfo: async (_id: string, _payload: unknown): Promise<never> => { throw new Error('材料要求由承接方式决定，不使用旧 OA 数组覆盖接口') },
}
