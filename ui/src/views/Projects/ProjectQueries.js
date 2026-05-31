import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import {
  ApproveProjectTask,
  CreateProject,
  DeleteProject,
  GetProjects,
  MarkProjectTaskDone,
  RejectProjectTask,
  ReopenProject,
  UpdateProject,
} from '../../utils/Fetcher'

const PROJECTS_KEY = ['projects']

const parseError = async resp => {
  try {
    const data = await resp.json()
    return new Error(data.error || 'Request failed')
  } catch {
    return new Error(`Request failed (${resp.status})`)
  }
}

// ── Queries ──────────────────────────────────────────────────────────────────

export const useProjects = () => {
  return useQuery({
    queryKey: PROJECTS_KEY,
    queryFn: async () => {
      const resp = await GetProjects()
      if (!resp.ok) throw await parseError(resp)
      return resp.json()
    },
  })
}

// ── Mutations ─────────────────────────────────────────────────────────────────

export const useCreateProject = () => {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async projectData => {
      const resp = await CreateProject(projectData)
      if (!resp.ok) throw await parseError(resp)
      const data = await resp.json()
      return data.res
    },
    onSuccess: () => queryClient.invalidateQueries({ queryKey: PROJECTS_KEY }),
  })
}

export const useUpdateProject = () => {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async ({ id, ...projectData }) => {
      const resp = await UpdateProject(id, projectData)
      if (!resp.ok) throw await parseError(resp)
      const data = await resp.json()
      return data.res
    },
    onSuccess: () => queryClient.invalidateQueries({ queryKey: PROJECTS_KEY }),
  })
}

export const useDeleteProject = () => {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async id => {
      const resp = await DeleteProject(id)
      if (!resp.ok) throw await parseError(resp)
      return id
    },
    onSuccess: () => queryClient.invalidateQueries({ queryKey: PROJECTS_KEY }),
  })
}

export const useMarkTaskDone = () => {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async ({ projectId, taskId }) => {
      const resp = await MarkProjectTaskDone(projectId, taskId)
      if (!resp.ok) throw await parseError(resp)
      return { projectId, taskId }
    },
    onSuccess: () => queryClient.invalidateQueries({ queryKey: PROJECTS_KEY }),
  })
}

export const useApproveTask = () => {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async ({ projectId, taskId }) => {
      const resp = await ApproveProjectTask(projectId, taskId)
      if (!resp.ok) throw await parseError(resp)
      return resp.json()
    },
    onSuccess: () => queryClient.invalidateQueries({ queryKey: PROJECTS_KEY }),
  })
}

export const useRejectTask = () => {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async ({ projectId, taskId }) => {
      const resp = await RejectProjectTask(projectId, taskId)
      if (!resp.ok) throw await parseError(resp)
      return { projectId, taskId }
    },
    onSuccess: () => queryClient.invalidateQueries({ queryKey: PROJECTS_KEY }),
  })
}

export const useReopenProject = () => {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async projectId => {
      const resp = await ReopenProject(projectId)
      if (!resp.ok) throw await parseError(resp)
      return projectId
    },
    onSuccess: () => queryClient.invalidateQueries({ queryKey: PROJECTS_KEY }),
  })
}
