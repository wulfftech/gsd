import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import {
  ClearChoreTimer,
  DeleteTimeSession,
  GetChoreTimer,
  PauseChore,
  ResetChoreTimer,
  StartChore,
  UpdateTimeSession,
} from '../utils/Fetcher'

export const useChoreTimer = choreId => {
  return useQuery({
    queryKey: ['choreTimer', choreId],
    queryFn: async () => {
      if (!choreId) {
        throw new Error('Chore ID is required to fetch timer')
      }
      const response = await GetChoreTimer(choreId)
      if (response && response.ok) {
        return await response.json()
      }
      throw new Error('Failed to fetch chore timer')
    },
    enabled: !!choreId,
  })
}

export const useStartChore = () => {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: StartChore,
    onSuccess: (data, choreId) => {
      // choreTimer/choreHistory/choreDetails are cached under the string
      // chore id (URL param - see ChoreView.jsx/TimerDetails.jsx), but this
      // mutation is also triggered from the MyChores list with a numeric
      // chore.id (useChoreActions.js), so coerce or the invalidation
      // silently misses (same reasoning as useSSE.js's choreDetailsKey).
      const normalizedChoreId = String(choreId)
      queryClient.invalidateQueries({
        queryKey: ['choreTimer', normalizedChoreId],
      })
      queryClient.invalidateQueries({ queryKey: ['chores'] })
      queryClient.invalidateQueries({
        queryKey: ['choreHistory', normalizedChoreId],
      })
      // Starting/pausing a chore changes its status, which choreDetails
      // reflects (see useSSE.js's chore.status handling) - previously this
      // was covered only by the v4/v5 blanket-invalidation bug.
      queryClient.invalidateQueries({
        queryKey: ['choreDetails', normalizedChoreId],
      })
    },
  })
}

export const usePauseChore = () => {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: PauseChore,
    onSuccess: (data, choreId) => {
      // Same reasoning as useStartChore above.
      const normalizedChoreId = String(choreId)
      queryClient.invalidateQueries({
        queryKey: ['choreTimer', normalizedChoreId],
      })
      queryClient.invalidateQueries({ queryKey: ['chores'] })
      queryClient.invalidateQueries({
        queryKey: ['choreHistory', normalizedChoreId],
      })
      queryClient.invalidateQueries({
        queryKey: ['choreDetails', normalizedChoreId],
      })
    },
  })
}

export const useUpdateTimeSession = () => {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: ({ choreId, sessionId, sessionData }) =>
      UpdateTimeSession(choreId, sessionId, sessionData),
    onSuccess: (data, { choreId }) => {
      // choreId here comes from TimerDetails.jsx's useParams() (its only
      // live caller), already a string - but normalize for consistency with
      // the other timer mutations in this file, which do receive numbers.
      const normalizedChoreId = String(choreId)
      queryClient.invalidateQueries({
        queryKey: ['choreTimer', normalizedChoreId],
      })
      queryClient.invalidateQueries({ queryKey: ['chores'] })
      queryClient.invalidateQueries({
        queryKey: ['choreHistory', normalizedChoreId],
      })
      queryClient.invalidateQueries({
        queryKey: ['choreDetails', normalizedChoreId],
      })
    },
  })
}

export const useDeleteTimeSession = () => {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: ({ choreId, sessionId }) =>
      DeleteTimeSession(choreId, sessionId),
    onSuccess: (data, { choreId }) => {
      // choreId here comes from ChoreView.jsx's normalized string choreId
      // (its only live caller) - normalize anyway for consistency, see
      // useStartChore above.
      const normalizedChoreId = String(choreId)
      queryClient.invalidateQueries({
        queryKey: ['choreTimer', normalizedChoreId],
      })
      queryClient.invalidateQueries({ queryKey: ['chores'] })
      queryClient.invalidateQueries({
        queryKey: ['choreHistory', normalizedChoreId],
      })
      queryClient.invalidateQueries({
        queryKey: ['choreDetails', normalizedChoreId],
      })
    },
  })
}

export const useResetChoreTimer = () => {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: ResetChoreTimer,
    onSuccess: (data, choreId) => {
      // choreId here comes from ChoreView.jsx's normalized string choreId
      // (its only live caller) - normalize anyway for consistency, see
      // useStartChore above.
      const normalizedChoreId = String(choreId)
      queryClient.invalidateQueries({
        queryKey: ['choreTimer', normalizedChoreId],
      })
      queryClient.invalidateQueries({ queryKey: ['chores'] })
      queryClient.invalidateQueries({
        queryKey: ['choreHistory', normalizedChoreId],
      })
      queryClient.invalidateQueries({
        queryKey: ['choreDetails', normalizedChoreId],
      })
    },
  })
}

export const useClearChoreTimer = () => {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: ClearChoreTimer,
    onSuccess: (data, choreId) => {
      // Not currently called anywhere in the app; normalized for
      // consistency with the other timer mutations above (same reasoning
      // as useStartChore).
      const normalizedChoreId = String(choreId)
      queryClient.invalidateQueries({
        queryKey: ['choreTimer', normalizedChoreId],
      })
      queryClient.invalidateQueries({ queryKey: ['chores'] })
      queryClient.invalidateQueries({
        queryKey: ['choreHistory', normalizedChoreId],
      })
      queryClient.invalidateQueries({
        queryKey: ['choreDetails', normalizedChoreId],
      })
    },
  })
}
