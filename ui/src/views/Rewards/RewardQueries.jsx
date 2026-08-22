import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import {
  CreateReward,
  DeleteReward,
  FulfillRedemption,
  GetRewardRedemptions,
  GetRewards,
  RedeemReward,
  UpdateReward,
} from '../../utils/Fetcher'

const parseError = async resp => {
  try {
    const data = await resp.json()
    return new Error(data.error || 'Request failed')
  } catch {
    return new Error(`Request failed (${resp.status})`)
  }
}

export const useRewards = () => {
  return useQuery({
    queryKey: ['rewards'],
    queryFn: () =>
      GetRewards()
        .then(r => r.json())
        .then(d => d ?? []),
  })
}

export const useRedemptions = () => {
  return useQuery({
    queryKey: ['rewardRedemptions'],
    queryFn: () =>
      GetRewardRedemptions()
        .then(r => r.json())
        .then(d => d ?? []),
  })
}

export const useCreateReward = () => {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async reward => {
      const resp = await CreateReward(reward)
      if (!resp.ok) throw await parseError(resp)
      return resp.json()
    },
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['rewards'] }),
  })
}

export const useUpdateReward = () => {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async ({ id, ...reward }) => {
      const resp = await UpdateReward(id, reward)
      if (!resp.ok) throw await parseError(resp)
      return resp.json()
    },
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['rewards'] }),
  })
}

export const useDeleteReward = () => {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async id => {
      const resp = await DeleteReward(id)
      if (!resp.ok) throw await parseError(resp)
      return id
    },
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['rewards'] }),
  })
}

export const useRedeemReward = () => {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async id => {
      const resp = await RedeemReward(id)
      if (!resp.ok) throw await parseError(resp)
      return id
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['rewards'] })
      queryClient.invalidateQueries({ queryKey: ['allCircleMembers'] })
      queryClient.invalidateQueries({ queryKey: ['userProfile'] })
    },
  })
}

export const useFulfillRedemption = () => {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async id => {
      const resp = await FulfillRedemption(id)
      if (!resp.ok) throw await parseError(resp)
      return id
    },
    onSuccess: () =>
      queryClient.invalidateQueries({ queryKey: ['rewardRedemptions'] }),
  })
}
