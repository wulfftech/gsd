import CheckCircleIcon from '@mui/icons-material/CheckCircle'
import CheckCircleOutlineIcon from '@mui/icons-material/CheckCircleOutline'
import DeleteOutlineIcon from '@mui/icons-material/DeleteOutline'
import EditIcon from '@mui/icons-material/Edit'
import ExpandLessIcon from '@mui/icons-material/ExpandLess'
import ExpandMoreIcon from '@mui/icons-material/ExpandMore'
import HourglassEmptyIcon from '@mui/icons-material/HourglassEmpty'
import ReplayIcon from '@mui/icons-material/Replay'
import TaskAltIcon from '@mui/icons-material/TaskAlt'
import {
  Add,
  EmojiEvents,
  CalendarToday,
  ThumbDown,
  ThumbUp,
} from '@mui/icons-material'
import {
  Avatar,
  Box,
  Button,
  Chip,
  CircularProgress,
  Container,
  IconButton,
  LinearProgress,
  Stack,
  Typography,
} from '@mui/joy'
import { useState } from 'react'
import { useCircleMembers, useUserProfile } from '../../queries/UserQueries'
import { getTextColorFromBackgroundColor } from '../../utils/Colors'
import { getIconComponent } from '../../utils/ProjectIcons'
import { getSafeBottomStyles } from '../../utils/SafeAreaUtils'
import ConfirmationModal from '../Modals/Inputs/ConfirmationModal'
import ProjectModal from '../Modals/Inputs/ProjectModal'
import {
  useApproveTask,
  useDeleteProject,
  useMarkTaskDone,
  useProjects,
  useRejectTask,
  useReopenProject,
} from './ProjectQueries'

// Status constants matching backend
const TASK_PENDING = 0
const TASK_PENDING_APPROVAL = 1
const TASK_COMPLETED = 2

const PROJECT_ACTIVE = 0
const PROJECT_COMPLETED = 1

// ── Task row ──────────────────────────────────────────────────────────────────
const TaskRow = ({ task, isAdmin, isAssignee, projectId, projectStatus, members }) => {
  const markDone = useMarkTaskDone()
  const approveTask = useApproveTask()
  const rejectTask = useRejectTask()

  const canMarkDone =
    projectStatus === PROJECT_ACTIVE &&
    task.status === TASK_PENDING &&
    (isAdmin || isAssignee)

  const canApproveReject =
    projectStatus === PROJECT_ACTIVE &&
    task.status === TASK_PENDING_APPROVAL &&
    isAdmin

  const claimedBy = task.status === TASK_PENDING_APPROVAL && task.completedBy
    ? (members.find(m => m.userId === task.completedBy)?.displayName || `User ${task.completedBy}`)
    : null

  const completedBy = task.status === TASK_COMPLETED && task.completedBy
    ? (members.find(m => m.userId === task.completedBy)?.displayName || null)
    : null

  const statusIcon = () => {
    if (task.status === TASK_COMPLETED)
      return <TaskAltIcon sx={{ fontSize: 20, color: 'success.500' }} />
    if (task.status === TASK_PENDING_APPROVAL)
      return <HourglassEmptyIcon sx={{ fontSize: 20, color: 'warning.500' }} />
    return (
      <CheckCircleOutlineIcon sx={{ fontSize: 20, color: 'neutral.400' }} />
    )
  }

  return (
    <Box
      sx={{
        display: 'flex',
        alignItems: 'center',
        gap: 1,
        px: 2,
        py: 1,
        borderBottom: '1px solid',
        borderColor: 'divider',
        bgcolor: task.status === TASK_PENDING_APPROVAL ? 'warning.softBg' : 'transparent',
        opacity: task.status === TASK_COMPLETED ? 0.65 : 1,
      }}
    >
      {statusIcon()}

      <Box sx={{ flex: 1, minWidth: 0 }}>
        <Typography
          level='body-sm'
          sx={{
            fontWeight: task.status === TASK_COMPLETED ? 400 : 500,
            textDecoration:
              task.status === TASK_COMPLETED ? 'line-through' : 'none',
          }}
        >
          {task.name}
        </Typography>
        {claimedBy && (
          <Typography level='body-xs' color='warning' sx={{ fontStyle: 'italic' }}>
            Claimed by {claimedBy} · awaiting approval
          </Typography>
        )}
        {completedBy && (
          <Typography level='body-xs' color='success'>
            Approved · completed by {completedBy}
          </Typography>
        )}
        {task.status === TASK_COMPLETED && !completedBy && (
          <Typography level='body-xs' color='success'>
            Approved
          </Typography>
        )}
      </Box>

      {canMarkDone && (
        <Button
          size='sm'
          variant='soft'
          color='success'
          loading={markDone.isPending}
          onClick={() => markDone.mutate({ projectId, taskId: task.id })}
          startDecorator={<CheckCircleIcon sx={{ fontSize: 16 }} />}
        >
          Done
        </Button>
      )}

      {canApproveReject && (
        <Box sx={{ display: 'flex', gap: 0.5 }}>
          <IconButton
            size='sm'
            variant='soft'
            color='success'
            loading={approveTask.isPending}
            onClick={() => approveTask.mutate({ projectId, taskId: task.id })}
            title='Approve'
          >
            <ThumbUp sx={{ fontSize: 16 }} />
          </IconButton>
          <IconButton
            size='sm'
            variant='soft'
            color='danger'
            loading={rejectTask.isPending}
            onClick={() => rejectTask.mutate({ projectId, taskId: task.id })}
            title='Reject — returns task to Pending'
          >
            <ThumbDown sx={{ fontSize: 16 }} />
          </IconButton>
        </Box>
      )}
    </Box>
  )
}

// ── Project card ──────────────────────────────────────────────────────────────
const ProjectCard = ({ project, isAdmin, currentUserId, onEdit, onDelete, members }) => {
  // Active projects with tasks start expanded; completed projects start collapsed
  const [expanded, setExpanded] = useState(
    project.status === PROJECT_ACTIVE && (project.tasks || []).length > 0,
  )
  const reopenProject = useReopenProject()

  const tasks = project.tasks || []
  const assignees = project.assignees || []
  const completedCount = tasks.filter(t => t.status === TASK_COMPLETED).length
  const progress = tasks.length > 0 ? (completedCount / tasks.length) * 100 : 0
  const isCompleted = project.status === PROJECT_COMPLETED

  const isAssignee = assignees.some(a => a.userId === currentUserId)

  const IconComponent = getIconComponent(project.icon)
  const iconColor = project.color || '#1976d2'

  const dueDateStr = project.dueDate
    ? new Date(project.dueDate).toLocaleDateString(undefined, {
        month: 'short',
        day: 'numeric',
        year: 'numeric',
      })
    : null

  const isOverdue =
    !isCompleted &&
    project.dueDate &&
    (() => {
      const dueDate = new Date(project.dueDate)
      // Compare local calendar dates: overdue if today's local date is after the due date
      // Create a Date for local midnight of the day after the due date
      const endOfDueDay = new Date(
        dueDate.getUTCFullYear(),
        dueDate.getUTCMonth(),
        dueDate.getUTCDate() + 1,
      )
      return new Date() >= endOfDueDay
    })()

  return (
    <Box
      sx={{
        mb: 1.5,
        borderRadius: 'md',
        border: '1px solid',
        borderColor: 'divider',
        bgcolor: 'background.surface',
        overflow: 'hidden',
      }}
    >
      {/* Card header */}
      <Box
        sx={{
          display: 'flex',
          alignItems: 'center',
          gap: 1.5,
          px: 2,
          py: 1.5,
          cursor: 'pointer',
          '&:hover': { bgcolor: 'background.level1' },
        }}
        onClick={() => setExpanded(e => !e)}
      >
        {/* Icon avatar */}
        <Avatar
          size='sm'
          sx={{ bgcolor: iconColor, flexShrink: 0, width: 36, height: 36 }}
        >
          <IconComponent
            sx={{
              fontSize: 18,
              color: getTextColorFromBackgroundColor(iconColor),
            }}
          />
        </Avatar>

        {/* Main info */}
        <Box sx={{ flex: 1, minWidth: 0 }}>
          <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, mb: 0.25 }}>
            <Typography
              level='title-sm'
              sx={{
                fontWeight: 600,
                overflow: 'hidden',
                textOverflow: 'ellipsis',
                whiteSpace: 'nowrap',
              }}
            >
              {project.name}
            </Typography>
            {isCompleted && (
              <Chip size='sm' color='success' variant='soft'>
                Completed
              </Chip>
            )}
          </Box>

          {/* Meta row */}
          <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, flexWrap: 'wrap' }}>
            <Typography level='body-xs' color='neutral'>
              {completedCount}/{tasks.length} tasks
            </Typography>

            {dueDateStr && (
              <Box sx={{ display: 'flex', alignItems: 'center', gap: 0.25 }}>
                <CalendarToday sx={{ fontSize: 11, color: isOverdue ? 'danger.500' : 'neutral.500' }} />
                <Typography
                  level='body-xs'
                  color={isOverdue ? 'danger' : 'neutral'}
                >
                  {dueDateStr}
                </Typography>
              </Box>
            )}

            {project.points > 0 && (
              <Box sx={{ display: 'flex', alignItems: 'center', gap: 0.25 }}>
                <EmojiEvents sx={{ fontSize: 12, color: 'warning.500' }} />
                <Typography level='body-xs' color='warning'>
                  {project.points} pts
                </Typography>
              </Box>
            )}
          </Box>

          {/* Progress bar */}
          {tasks.length > 0 && (
            <LinearProgress
              determinate
              value={progress}
              color={isCompleted ? 'success' : 'primary'}
              sx={{ mt: 0.75, height: 4, borderRadius: 'sm' }}
            />
          )}
        </Box>

        {/* Right actions */}
        <Box sx={{ display: 'flex', alignItems: 'center', gap: 0.5, flexShrink: 0 }}>
          {isAdmin && !isCompleted && (
            <>
              <IconButton
                size='sm'
                variant='plain'
                color='neutral'
                aria-label={`Edit project ${project.name}`}
                onClick={e => {
                  e.stopPropagation()
                  onEdit(project)
                }}
              >
                <EditIcon sx={{ fontSize: 18 }} />
              </IconButton>
              <IconButton
                size='sm'
                variant='plain'
                color='danger'
                aria-label={`Delete project ${project.name}`}
                onClick={e => {
                  e.stopPropagation()
                  onDelete(project)
                }}
              >
                <DeleteOutlineIcon sx={{ fontSize: 18 }} />
              </IconButton>
            </>
          )}
          {expanded ? (
            <ExpandLessIcon sx={{ fontSize: 20, color: 'neutral.500' }} />
          ) : (
            <ExpandMoreIcon sx={{ fontSize: 20, color: 'neutral.500' }} />
          )}
        </Box>
      </Box>

      {/* Expanded detail */}
      {expanded && (
        <Box>
          {/* Assignees */}
          {assignees.length > 0 && (
            <Box
              sx={{
                px: 2,
                py: 1,
                display: 'flex',
                alignItems: 'center',
                gap: 1,
                borderTop: '1px solid',
                borderColor: 'divider',
                bgcolor: 'background.level1',
              }}
            >
              <Typography level='body-xs' color='neutral' sx={{ mr: 0.5 }}>
                Assignees:
              </Typography>
              {assignees.map(a => {
                const member = members.find(m => m.userId === a.userId)
                return (
                  <Chip key={a.userId} size='sm' variant='soft' color='neutral'>
                    {member?.displayName || `User ${a.userId}`}
                  </Chip>
                )
              })}
            </Box>
          )}

          {/* Tasks */}
          {tasks.length === 0 ? (
            <Box sx={{ px: 2, py: 1.5, borderTop: '1px solid', borderColor: 'divider' }}>
              <Typography level='body-sm' color='neutral'>
                No tasks yet
              </Typography>
            </Box>
          ) : (
            tasks.map(task => (
              <TaskRow
                key={task.id}
                task={task}
                isAdmin={isAdmin}
                isAssignee={isAssignee}
                projectId={project.id}
                projectStatus={project.status}
                members={members}
              />
            ))
          )}

          {/* Reopen button for completed projects */}
          {isCompleted && isAdmin && (
            <Box
              sx={{
                px: 2,
                py: 1,
                borderTop: '1px solid',
                borderColor: 'divider',
              }}
            >
              <Button
                size='sm'
                variant='soft'
                color='neutral'
                startDecorator={<ReplayIcon sx={{ fontSize: 16 }} />}
                loading={reopenProject.isPending}
                onClick={() => reopenProject.mutate(project.id)}
              >
                Reopen Project
              </Button>
            </Box>
          )}
        </Box>
      )}
    </Box>
  )
}

// ── Main view ─────────────────────────────────────────────────────────────────
const ProjectView = () => {
  const { data: projects = [], isLoading, isError } = useProjects()
  const { data: userProfile } = useUserProfile()
  const { data: circleMembersData } = useCircleMembers()
  const deleteProject = useDeleteProject()

  const [modalConfig, setModalConfig] = useState(null) // null | { project? }
  const [confirmConfig, setConfirmConfig] = useState({})

  const members = circleMembersData?.res || []
  const isAdmin =
    members.find(m => m.userId === userProfile?.id)?.role === 'admin'

  const handleEdit = project => setModalConfig({ project })
  const handleAdd = () => setModalConfig({ project: null })

  const handleDelete = project => {
    setConfirmConfig({
      isOpen: true,
      title: 'Delete Project',
      message: `Delete "${project.name}"? This cannot be undone. Chores linked to this project will be unlinked.`,
      confirmText: 'Delete',
      color: 'danger',
      cancelText: 'Cancel',
      onClose: confirmed => {
        if (confirmed === true) {
          deleteProject.mutate(project.id)
        }
        setConfirmConfig({})
      },
    })
  }

  if (isLoading) {
    return (
      <Box display='flex' justifyContent='center' alignItems='center' height='60vh'>
        <CircularProgress />
      </Box>
    )
  }

  if (isError) {
    return (
      <Box sx={{ p: 3, textAlign: 'center' }}>
        <Typography color='danger'>Failed to load projects. Please try again.</Typography>
      </Box>
    )
  }

  // Separate active from completed for display order
  const active = projects.filter(p => p.status === PROJECT_ACTIVE)
  const completed = projects.filter(p => p.status === PROJECT_COMPLETED)

  return (
    <Container maxWidth='md' sx={{ px: 2 }}>
      <Box sx={{ display: 'flex', alignItems: 'center', gap: 2, mb: 2, pt: 2 }}>
        <Stack sx={{ flex: 1 }}>
          <Typography level='h3' sx={{ fontWeight: 'lg' }}>
            Projects
          </Typography>
          <Typography level='body-sm' color='neutral'>
            Task collections with assignees and point rewards
          </Typography>
        </Stack>
      </Box>

      {projects.length === 0 && (
        <Box
          sx={{
            textAlign: 'center',
            py: 6,
            color: 'text.tertiary',
          }}
        >
          <Typography level='body-md'>No projects yet.</Typography>
          {isAdmin && (
            <Typography level='body-sm'>
              Tap the + button to create one.
            </Typography>
          )}
        </Box>
      )}

      {/* Active projects */}
      {active.map(project => (
        <ProjectCard
          key={project.id}
          project={project}
          isAdmin={isAdmin}
          currentUserId={userProfile?.id}
          members={members}
          onEdit={handleEdit}
          onDelete={handleDelete}
        />
      ))}

      {/* Completed projects */}
      {completed.length > 0 && (
        <>
          <Typography
            level='body-xs'
            color='neutral'
            sx={{ mt: 3, mb: 1, textTransform: 'uppercase', letterSpacing: 1 }}
          >
            Completed
          </Typography>
          {completed.map(project => (
            <ProjectCard
              key={project.id}
              project={project}
              isAdmin={isAdmin}
              currentUserId={userProfile?.id}
              members={members}
              onEdit={handleEdit}
              onDelete={handleDelete}
            />
          ))}
        </>
      )}

      {/* FAB — admin only */}
      {isAdmin && (
        <Box
          sx={{
            ...getSafeBottomStyles({ bottom: 0, padding: 16 }),
            left: 10,
            display: 'flex',
            justifyContent: 'flex-end',
            gap: 2,
            zIndex: 1000,
          }}
        >
          <IconButton
            color='primary'
            variant='solid'
            sx={{ borderRadius: '50%', width: 50, height: 50 }}
            onClick={handleAdd}
          >
            <Add />
          </IconButton>
        </Box>
      )}

      {/* Create / Edit modal */}
      {modalConfig !== null && (
        <ProjectModal
          isOpen
          project={modalConfig.project}
          onClose={() => setModalConfig(null)}
          onSave={() => setModalConfig(null)}
        />
      )}

      <ConfirmationModal config={confirmConfig} />
    </Container>
  )
}

export default ProjectView
