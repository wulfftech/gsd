// Helpers for surfacing the server's `{"error": "..."}` message to the user
// instead of failing silently.

// Read the `error` field from a failed fetch Response. Clones the response so
// callers can still read the body themselves. Returns `fallback` when the
// body is missing or isn't JSON.
export const readApiError = async (
  response,
  fallback = 'Something went wrong',
) => {
  if (!response) return fallback
  try {
    const data = await response.clone().json()
    if (data && typeof data.error === 'string' && data.error) {
      return data.error
    }
  } catch {
    // Not JSON - fall through to the fallback.
  }
  return fallback
}

// Error carrying the server's message and HTTP status, for mutationFns that
// throw on a non-ok response.
export class ApiError extends Error {
  constructor(message, status) {
    super(message)
    this.name = 'ApiError'
    this.status = status
  }
}

// Throw an ApiError with the server's message when `response` isn't ok.
// Returns the parsed JSON body on success.
export const parseApiResponse = async (response, fallback) => {
  if (!response) {
    throw new ApiError(fallback || 'No response from server', 0)
  }
  if (!response.ok) {
    throw new ApiError(await readApiError(response, fallback), response.status)
  }
  try {
    return await response.json()
  } catch {
    return null
  }
}
