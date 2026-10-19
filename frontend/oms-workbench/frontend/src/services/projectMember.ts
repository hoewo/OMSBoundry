import { contributorFor, contributorForMember } from './registry'
export const projectMemberService = {
  list: (id: string) => contributorFor(id).load({renewCredential:false}),
  create: (id: string, input: any) => contributorFor(id).create(id,input),
  remove: (id: string) => contributorForMember(id).remove(id),
}
