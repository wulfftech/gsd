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

export const useRewards = () => {
  return useQuery({
    queryKey: ['rewards'],
    queryFn: () => GetRewards().then(r => r.json()).then(d => d ?? []),
  })
}

export const useRedemptions = () => {
  return useQuery({
    queryKey: ['rewardRedemptions'],
    queryFn: () => GetRewardRedemptions().then(r => r.json()).then(d => d ?? []),
  })
}

export const useCreateReward = () => {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: reward => CreateReward(reward).then(r => r.json()),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['rewards'] }),
  })
}

export const useUpdateReward = () => {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: ({ id, ...reward }) =>
      UpdateReward(id, reward).then(r => r.json()),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['rewards'] }),
  })
}

export const useDeleteReward = () => {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: id => DeleteReward(id),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['rewards'] }),
  })
}

export const useRedeemReward = () => {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: id => RedeemReward(id),
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
    mutationFn: id => FulfillRedemption(id),
    onSuccess: () =>
      queryClient.invalidateQueries({ queryKey: ['rewardRedemptions'] }),
  })
}
