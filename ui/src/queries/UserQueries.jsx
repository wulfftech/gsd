import { useQuery, useQueryClient } from '@tanstack/react-query'
import {
  GetAllCircleMembers,
  GetAllUsers,
  GetDeviceTokens,
  GetManagedUsers,
  GetUserProfile,
} from '../utils/Fetcher'
import { apiClient } from '../utils/ApiClient'

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

      // Guard against null response from ApiClient (can occur during refresh cooldown
      // or refresh failure, in which case handleLogout() was already called internally)
      if (!resp) {
        return null
      }

      // Check response status before parsing
      if (!resp.ok) {
        // if we got 401 or 403 then logout (403 means account deleted, 401 means token invalid)
        if (resp.status === 401 || resp.status === 403) {
          apiClient.handleLogout()
        }
        return null
      }

      const result = await resp.json()
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
