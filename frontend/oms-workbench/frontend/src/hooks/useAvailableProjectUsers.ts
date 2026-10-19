import { useQuery } from '@tanstack/react-query'
import { contributorFor } from '../services/registry'
export const useAvailableProjectUsers = (projectID: string) => useQuery({queryKey:['availableProjectUsers',projectID],queryFn:()=>contributorFor(projectID).users(),enabled:!!projectID})
