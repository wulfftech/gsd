import { useQuery } from '@tanstack/react-query'
import { GetResource } from '../utils/Fetcher'

// Helper to check if we have a valid token
// TODO: dead code -- never called.
// Kept deliberately rather than deleted; decide whether to wire it up or drop it.
// eslint-disable-next-line no-unused-vars
const isTokenValid = () => {
  const token = localStorage.getItem('token')
  if (!token) return false

  const expiry = localStorage.getItem('token_expiry')
  if (!expiry) return true // No expiry set, assume valid

  return new Date() < new Date(expiry)
}

export const useResource = () => {
  const { data, isLoading, error, refetch } = useQuery({
    queryKey: ['resource'],
    queryFn: async () => {
      const response = await GetResource()
      return response
    },
    staleTime: 6 * 60 * 60 * 1000, // 6 hours in milliseconds
    refetchOnWindowFocus: false,
    refetchOnReconnect: false,
  })
  return { data, isLoading, error, refetch }
}
