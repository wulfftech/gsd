import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { parseApiResponse } from '../utils/ApiErrors'
import {
  BulkApproveChores,
  BulkRejectChores,
  CreateHoliday,
  CreateKioskToken,
  DeleteHoliday,
  GetHolidays,
  GetKioskStatus,
  GetPendingApprovals,
  PayoutPoints,
  SetKioskPin,
} from '../utils/Fetcher'

const PENDING_REFETCH_MS = 30 * 1000

// Chores waiting for a parent's approval. Only admins and managers may read
// this, so callers pass `enabled` from the user's circle role.
export const usePendingApprovals = ({ enabled = true } = {}) => {
  return useQuery({
    queryKey: ['pendingApprovals'],
    queryFn: async () => {
      const data = await parseApiResponse(
        await GetPendingApprovals(),
        'Could not load approvals',
      )
      return data?.res || []
    },
    enabled,
    refetchInterval: PENDING_REFETCH_MS,
    refetchOnWindowFocus: true,
  })
}

// After approving or rejecting, the chore list, chore details/history and
// pending list are all stale.
const invalidateAfterApproval = queryClient => {
  queryClient.invalidateQueries({ queryKey: ['pendingApprovals'] })
  queryClient.invalidateQueries({ queryKey: ['chores'] })
  queryClient.invalidateQueries({ queryKey: ['choreDetails'] })
  queryClient.invalidateQueries({ queryKey: ['choreHistory'] })
  queryClient.invalidateQueries({ queryKey: ['choresHistory'] })
  queryClient.invalidateQueries({ queryKey: ['allCircleMembers'] })
}

// Resolves to { done: number[], failed: [{ choreId, error }] }.
// `choreIds` is an array of ids, or 'all'.
export const useBulkApproveChores = () => {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async choreIds => {
      const data = await parseApiResponse(
        await BulkApproveChores(choreIds),
        'Could not approve',
      )
      return data?.res || { done: [], failed: [] }
    },
    onSettled: () => invalidateAfterApproval(queryClient),
  })
}

export const useBulkRejectChores = () => {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async ({ choreIds, note }) => {
      const data = await parseApiResponse(
        await BulkRejectChores(choreIds, note),
        'Could not reject',
      )
      return data?.res || { done: [], failed: [] }
    },
    onSettled: () => invalidateAfterApproval(queryClient),
  })
}

export const useHolidays = ({ enabled = true } = {}) => {
  return useQuery({
    queryKey: ['holidays'],
    queryFn: async () => {
      const data = await parseApiResponse(
        await GetHolidays(),
        'Could not load school holidays',
      )
      return data?.res || []
    },
    enabled,
  })
}

export const useCreateHoliday = () => {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async holiday =>
      parseApiResponse(
        await CreateHoliday(holiday),
        'Could not add the holiday',
      ),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['holidays'] }),
  })
}

export const useDeleteHoliday = () => {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async id =>
      parseApiResponse(await DeleteHoliday(id), 'Could not delete the holiday'),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['holidays'] }),
  })
}

export const useKioskStatus = ({ enabled = true } = {}) => {
  return useQuery({
    queryKey: ['kioskStatus'],
    queryFn: async () => {
      const data = await parseApiResponse(
        await GetKioskStatus(),
        'Could not load kiosk settings',
      )
      return data?.res || { pinSet: false, tokenSet: false }
    },
    enabled,
  })
}

// Resolves to { token, path }. The token is only ever returned here.
export const useCreateKioskToken = () => {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async () => {
      const data = await parseApiResponse(
        await CreateKioskToken(),
        'Could not create the kiosk link',
      )
      return data?.res
    },
    onSuccess: () =>
      queryClient.invalidateQueries({ queryKey: ['kioskStatus'] }),
  })
}

export const useSetKioskPin = () => {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async pin =>
      parseApiResponse(await SetKioskPin(pin), 'Could not set the PIN'),
    onSuccess: () =>
      queryClient.invalidateQueries({ queryKey: ['kioskStatus'] }),
  })
}

// Resolves to { userId, paid }.
export const usePayoutPoints = () => {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async ({ userId, circleId }) => {
      const data = await parseApiResponse(
        await PayoutPoints(userId, circleId),
        'Could not pay out points',
      )
      return data?.res
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['allCircleMembers'] })
    },
  })
}
