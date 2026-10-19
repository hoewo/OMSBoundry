import { adapterFor, type BiddingInfo } from './registry'
export const oa2026Service = {
  getBidding: async (id: string): Promise<BiddingInfo> => adapterFor(id).info(),
  updateBidding: async (id: string, method: string): Promise<BiddingInfo> => adapterFor(id).updateMethod(id, method),
}
