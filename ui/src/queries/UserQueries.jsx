import { useQuery, useQueryClient } from '@tanstack/react-query'
import {
  GetAllCircleMembers,
  GetAllUsers,
  GetDeviceTokens,
  GetManagedUsers,
  GetUserProfile,
} from '../utils/Fetcher'

// Helper to check if we have a valid token
const isTokenValid = () => {
  const token = localStorage.getItem('token')
  if (!token) return false

  const expiry = localStorage.getItem('token_expiry')
  if (!expiry) return true // No expiry set, assume valid

  return new Date() < new Date(expiry)
}

export const useAllUsers = () => {
  return useQuery({
    queryKey: ['allUsers'],
    queryFn: GetAllUsers,
  })
}

export const useCircleMembers = () => {
  const queryClient = useQueryClient()

  const { data, error, isLoading } = useQuery({
    queryKey: ['allCircleMembers'],
    queryFn: GetAllCircleMembers,
  })

  const handleRefetch = () => {
    queryClient.invalidateQueries(['allCircleMembers'])
  }

  return { data, error, isLoading, handleRefetch }
}

export const useUserProfile = () => {
  const queryClient = useQueryClient()

  const { data, error, isLoading } = useQuery({
    queryKey: ['userProfile'],
    queryFn: async () => {
      const resp = await GetUserProfile()
      const result = await resp.json()
      // if we got 403 then user probably deleted their account and token is still valid. navigate to login

      return result.res || null
    },
    staleTime: 30 * 60 * 1000, // 30 minutes in milliseconds
    gcTime: 30 * 60 * 1000, // 30 minutes in milliseconds
    enabled: isTokenValid(), // Only run query when we have a valid token
  })
  return {
    data,
    error,
    isLoading,
    refetch: () => queryClient.invalidateQueries(['userProfile']),
  }
}

export const useDeviceTokens = () => {
  const queryClient = useQueryClient()

  const { data, error, isLoading } = useQuery({
    queryKey: ['deviceTokens'],
    queryFn: async () => {
      if (!isTokenValid()) {
        return null
      }
      const resp = await GetDeviceTokens(true) // Only get active devices
      const result = await resp.json()
      return result.res || []
    },
    staleTime: 0, // Always fetch fresh data
    gcTime: 10 * 60 * 1000, // 10 minutes
  })

  return {
    data,
    error,
    isLoading,
    refetch: () => queryClient.invalidateQueries(['deviceTokens']),
  }
}

export const useManagedUsers = () => {
  const queryClient = useQueryClient()

  const { data, error, isLoading } = useQuery({
    queryKey: ['managedUsers'],
    queryFn: async () => {
      if (!isTokenValid()) return []
      const resp = await GetManagedUsers()
      if (!resp.ok) return []
      const result = await resp.json()
      return result.res || []
    },
    staleTime: 5 * 60 * 1000,
    gcTime: 10 * 60 * 1000,
  })

  return {
    data,
    error,
    isLoading,
    refetch: () => queryClient.invalidateQueries(['managedUsers']),
  }
}

// Legacy alias
export const useChildUsers = useManagedUsers
