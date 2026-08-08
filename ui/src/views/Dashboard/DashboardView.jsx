import AccessTimeIcon from '@mui/icons-material/AccessTime'
import CheckCircleIcon from '@mui/icons-material/CheckCircle'
import EmojiEventsIcon from '@mui/icons-material/EmojiEvents'
import FolderOpenIcon from '@mui/icons-material/FolderOpen'
import TaskAltIcon from '@mui/icons-material/TaskAlt'
import WarningAmberIcon from '@mui/icons-material/WarningAmber'
import {
  Avatar,
  Box,
  CircularProgress,
  Divider,
  Sheet,
  Typography,
} from '@mui/joy'
import { useNavigate } from 'react-router-dom'
import { useChores, useChoresHistory } from '../../queries/ChoreQueries'
import { useCircleMembers, useUserProfile } from '../../queries/UserQueries'
import { resolvePhotoURL } from '../../utils/Helpers'
import { useProjects } from '../Projects/ProjectQueries'

// ── Constants ─────────────────────────────────────────────────────────────────
const TASK_PENDING = 0
const PROJECT_ACTIVE = 0

// ── Helpers ───────────────────────────────────────────────────────────────────
const formatDue = dateStr => {
  const now = new Date()
  const due = new Date(dateStr)
  const diffMs = due - now
  const absH = Math.abs(diffMs) / 3600000
  const absD = Math.floor(absH / 24)

  if (diffMs < 0) {
    if (absH < 1) return `${Math.round(absH * 60)}m overdue`
    if (absH < 24) return `${Math.round(absH)}h overdue`
    return `${absD}d overdue`
  }
  if (absH < 1) return `in ${Math.round(absH * 60)}m`
  if (absH < 24) return `in ${Math.round(absH)}h`
  return `in ${absD}d`
}

const isOverdue = dateStr => new Date(dateStr) < new Date()

// ── Chore row (left panel) ────────────────────────────────────────────────────
const ChoreRow = ({ chore, members, navigate }) => {
  const overdue = isOverdue(chore.nextDueDate)
  const assignee = members.find(m => m.userId === chore.assignedTo)

  return (
    <Box
      onClick={() => navigate(`/chores/${chore.id}`)}
      sx={{
        display: 'flex',
        alignItems: 'center',
        gap: 1,
        px: 1.5,
        py: 0.875,
        mb: 0.75,
        borderRadius: 'sm',
        border: '1px solid',
        borderColor: overdue ? 'danger.300' : 'divider',
        bgcolor: overdue ? 'danger.softBg' : 'background.surface',
        borderLeft: '3px solid',
        borderLeftColor: overdue ? 'var(--joy-palette-danger-400)' : 'var(--joy-palette-primary-400)',
        cursor: 'pointer',
        '&:hover': { filter: 'brightness(0.97)' },
      }}
    >
      {overdue ? (
        <WarningAmberIcon sx={{ fontSize: 15, color: 'danger.500', flexShrink: 0 }} />
      ) : (
        <AccessTimeIcon sx={{ fontSize: 15, color: 'primary.400', flexShrink: 0 }} />
      )}

      <Box sx={{ flex: 1, minWidth: 0 }}>
        <Typography
          level='body-sm'
          sx={{
            fontWeight: 500,
            overflow: 'hidden',
            textOverflow: 'ellipsis',
            whiteSpace: 'nowrap',
            lineHeight: 1.3,
          }}
        >
          {chore.name}
        </Typography>
        <Typography
          level='body-xs'
          color={overdue ? 'danger' : 'neutral'}
          sx={{ lineHeight: 1.2 }}
        >
          {formatDue(chore.nextDueDate)}
        </Typography>
      </Box>

      {assignee && (
        <Avatar
          src={resolvePhotoURL(assignee.image)}
          onClick={e => {
            e.stopPropagation()
            navigate(`/points?userId=${assignee.userId}`)
          }}
          sx={{ width: 22, height: 22, fontSize: 10, flexShrink: 0, cursor: 'pointer' }}
        >
          {assignee.displayName?.charAt(0)}
        </Avatar>
      )}
    </Box>
  )
}

// ── Project task row (left panel) ─────────────────────────────────────────────
const ProjectTaskRow = ({ task, navigate }) => {
  const projectColor = task.project.color || '#9c27b0'

  return (
    <Box
      onClick={() => navigate(`/projects?projectId=${task.project.id}&taskId=${task.id}`)}
      sx={{
        display: 'flex',
        alignItems: 'center',
        gap: 1,
        px: 1.5,
        py: 0.875,
        mb: 0.75,
        borderRadius: 'sm',
        border: '1px solid',
        borderColor: 'warning.200',
        bgcolor: 'warning.softBg',
        borderLeft: `3px solid ${projectColor}`,
        cursor: 'pointer',
        '&:hover': { filter: 'brightness(0.97)' },
      }}
    >
      <FolderOpenIcon sx={{ fontSize: 15, color: projectColor, flexShrink: 0 }} />

      <Box sx={{ flex: 1, minWidth: 0 }}>
        <Typography
          level='body-sm'
          sx={{
            fontWeight: 500,
            overflow: 'hidden',
            textOverflow: 'ellipsis',
            whiteSpace: 'nowrap',
            lineHeight: 1.3,
          }}
        >
          {task.name}
        </Typography>
        <Typography
          level='body-xs'
          sx={{ color: projectColor, lineHeight: 1.2 }}
        >
          {task.project.name}
        </Typography>
      </Box>
    </Box>
  )
}

// ── User card (right panel) ───────────────────────────────────────────────────
const UserCard = ({ member, assignedChores, projectTaskCount, completedToday, points, navigate }) => {
  // Show up to 3 task names, then "+N more"
  const MAX_SHOWN = 3
  const shown = assignedChores.slice(0, MAX_SHOWN)
  const overflow = assignedChores.length - MAX_SHOWN

  return (
    <Sheet
      variant='outlined'
      sx={{
        display: 'flex',
        alignItems: 'stretch',
        gap: 0,
        mb: 1,
        borderRadius: 'md',
        overflow: 'hidden',
        height: 'calc((100vh - 160px) / 6)',
        minHeight: 72,
        maxHeight: 130,
      }}
    >
      {/* Avatar + name column */}
      <Box
        onClick={() => navigate(`/points?userId=${member.userId}`)}
        sx={{
          display: 'flex',
          flexDirection: 'column',
          alignItems: 'center',
          justifyContent: 'center',
          gap: 0.5,
          px: 1.5,
          minWidth: 64,
          bgcolor: 'background.level1',
          cursor: 'pointer',
          '&:hover': { filter: 'brightness(0.97)' },
        }}
      >
        <Avatar
          src={resolvePhotoURL(member.image)}
          sx={{ width: 34, height: 34, fontSize: 14 }}
        >
          {member.displayName?.charAt(0)}
        </Avatar>
        <Typography
          level='body-xs'
          sx={{
            fontWeight: 600,
            textAlign: 'center',
            maxWidth: 58,
            overflow: 'hidden',
            textOverflow: 'ellipsis',
            whiteSpace: 'nowrap',
            fontSize: 10,
          }}
        >
          {member.displayName?.split(' ')[0]}
        </Typography>
      </Box>

      <Divider orientation='vertical' />

      {/* Assigned tasks column */}
      <Box
        sx={{
          flex: 1,
          px: 1.25,
          py: 0.75,
          display: 'flex',
          flexDirection: 'column',
          justifyContent: 'center',
          overflow: 'hidden',
          minWidth: 0,
        }}
      >
        {assignedChores.length === 0 && projectTaskCount === 0 ? (
          <Typography level='body-xs' color='success' sx={{ fontSize: 11 }}>
            ✓ All clear
          </Typography>
        ) : (
          <>
            {shown.map(c => (
              <Typography
                key={c.id}
                level='body-xs'
                onClick={() => navigate(`/chores/${c.id}`)}
                sx={{
                  overflow: 'hidden',
                  textOverflow: 'ellipsis',
                  whiteSpace: 'nowrap',
                  lineHeight: 1.4,
                  fontSize: 11,
                  cursor: 'pointer',
                  color: isOverdue(c.nextDueDate)
                    ? 'var(--joy-palette-danger-500)'
                    : 'text.primary',
                  '&:hover': { textDecoration: 'underline' },
                }}
              >
                · {c.name}
              </Typography>
            ))}
            {overflow > 0 && (
              <Typography level='body-xs' color='neutral' sx={{ fontSize: 10, fontStyle: 'italic' }}>
                +{overflow} more
              </Typography>
            )}
            {projectTaskCount > 0 && (
              <Typography
                level='body-xs'
                color='warning'
                onClick={() => navigate('/projects')}
                sx={{ fontSize: 10, cursor: 'pointer', '&:hover': { textDecoration: 'underline' } }}
              >
                {projectTaskCount} project {projectTaskCount === 1 ? 'task' : 'tasks'}
              </Typography>
            )}
          </>
        )}
      </Box>

      <Divider orientation='vertical' />

      {/* Stats column */}
      <Box
        sx={{
          display: 'flex',
          flexDirection: 'column',
          justifyContent: 'center',
          gap: 0.625,
          px: 1.25,
          minWidth: 72,
        }}
      >
        <Box
          onClick={() => navigate(`/points?userId=${member.userId}`)}
          sx={{ display: 'flex', alignItems: 'center', gap: 0.5, cursor: 'pointer', '&:hover': { textDecoration: 'underline' } }}
        >
          <EmojiEventsIcon sx={{ fontSize: 13, color: 'warning.500' }} />
          <Typography level='body-xs' sx={{ fontSize: 11, color: 'warning.600', fontWeight: 600 }}>
            {points}
          </Typography>
          <Typography level='body-xs' sx={{ fontSize: 10, color: 'text.tertiary' }}>pts</Typography>
        </Box>
        <Box sx={{ display: 'flex', alignItems: 'center', gap: 0.5 }}>
          <CheckCircleIcon sx={{ fontSize: 13, color: 'success.500' }} />
          <Typography level='body-xs' sx={{ fontSize: 11, color: 'success.600', fontWeight: 600 }}>
            {completedToday}
          </Typography>
          <Typography level='body-xs' sx={{ fontSize: 10, color: 'text.tertiary' }}>today</Typography>
        </Box>
        <Box sx={{ display: 'flex', alignItems: 'center', gap: 0.5 }}>
          <TaskAltIcon sx={{ fontSize: 13, color: 'neutral.500' }} />
          <Typography level='body-xs' sx={{ fontSize: 11, fontWeight: 600 }}>
            {assignedChores.length + projectTaskCount}
          </Typography>
          <Typography level='body-xs' sx={{ fontSize: 10, color: 'text.tertiary' }}>tasks</Typography>
        </Box>
      </Box>
    </Sheet>
  )
}

// ── Section header ────────────────────────────────────────────────────────────
const SectionHeader = ({ children, count }) => (
  <Box sx={{ display: 'flex', alignItems: 'baseline', gap: 1, mb: 1.5 }}>
    <Typography level='title-sm' sx={{ fontWeight: 700 }}>
      {children}
    </Typography>
    {count !== undefined && (
      <Typography level='body-xs' color='neutral'>
        ({count})
      </Typography>
    )}
  </Box>
)

// ── Main view ─────────────────────────────────────────────────────────────────
const DashboardView = () => {
  const navigate = useNavigate()
  const { data: choresData, isLoading: choresLoading } = useChores(false)
  const { data: projects = [], isLoading: projectsLoading } = useProjects()
  const { data: circleMembersData } = useCircleMembers()
  const { data: userProfile } = useUserProfile()
  const { data: historyRaw } = useChoresHistory(100, false)
  // Normalise: useChoresHistory may return an array or {res:[]} depending on cache state
  const history = Array.isArray(historyRaw)
    ? historyRaw
    : historyRaw?.res || []

  const chores = choresData?.res || []
  const members = circleMembersData?.res || []

  const now = new Date()
  const in24h = new Date(now.getTime() + 24 * 60 * 60 * 1000)

  const todayStart = new Date(now)
  todayStart.setHours(0, 0, 0, 0)

  // ── Left panel data ──────────────────────────────────────────────────────
  // Chores due within the next 24 hours (including overdue), sorted soonest first
  const dueSoonChores = chores
    .filter(c => {
      if (!c.nextDueDate) return false
      if (c.status === 3) return false // already pending approval
      return new Date(c.nextDueDate) <= in24h
    })
    .sort((a, b) => new Date(a.nextDueDate) - new Date(b.nextDueDate))

  // Pending project tasks from active projects
  const pendingProjectTasks = projects
    .filter(p => p.status === PROJECT_ACTIVE)
    .flatMap(p =>
      (p.tasks || [])
        .filter(t => t.status === TASK_PENDING)
        .map(t => ({ ...t, project: p })),
    )

  // ── Right panel data ─────────────────────────────────────────────────────
  // Completed chores today, keyed by userId
  const completedTodayByUser = {}
  ;history.forEach(h => {
    const performedAt = h.performedAt || h.completedAt || h.updatedAt
    if (!performedAt) return
    if (new Date(performedAt) < todayStart) return
    if (h.status !== 1) return // only "completed" entries
    const uid = h.completedBy
    if (uid) completedTodayByUser[uid] = (completedTodayByUser[uid] || 0) + 1
  })

  // Chores assigned to each user
  const choresByUser = {}
  chores.forEach(c => {
    if (c.assignedTo) {
      if (!choresByUser[c.assignedTo]) choresByUser[c.assignedTo] = []
      choresByUser[c.assignedTo].push(c)
    }
  })

  // Pending project tasks assigned to each user (via project assignees)
  const projectTaskCountByUser = {}
  pendingProjectTasks.forEach(t => {
    ;(t.project.assignees || []).forEach(a => {
      projectTaskCountByUser[a.userId] = (projectTaskCountByUser[a.userId] || 0) + 1
    })
  })

  const availablePoints = m => Math.max(0, m.points || 0)

  const isLoading = choresLoading || projectsLoading

  if (isLoading) {
    return (
      <Box display='flex' justifyContent='center' alignItems='center' height='60vh'>
        <CircularProgress />
      </Box>
    )
  }

  const leftCount = dueSoonChores.length + pendingProjectTasks.length

  return (
    <Box
      sx={{
        display: 'flex',
        flexDirection: { xs: 'column', md: 'row' },
        gap: 2,
        px: 2,
        py: 2,
        height: { md: 'calc(100vh - 80px)' },
        overflow: 'hidden',
      }}
    >
      {/* ── LEFT: Due in 24h ────────────────────────────────────────────── */}
      <Box
        sx={{
          width: { xs: '100%', md: '36%' },
          minWidth: { md: 260 },
          display: 'flex',
          flexDirection: 'column',
          overflow: 'hidden',
        }}
      >
        <SectionHeader count={leftCount}>Due in 24 Hours</SectionHeader>

        <Box sx={{ flex: 1, overflowY: 'auto', pr: 0.5 }}>
          {leftCount === 0 ? (
            <Box sx={{ textAlign: 'center', py: 4 }}>
              <Typography level='body-sm' color='success'>
                ✓ Nothing due in the next 24 hours
              </Typography>
            </Box>
          ) : (
            <>
              {dueSoonChores.map(c => (
                <ChoreRow key={`c-${c.id}`} chore={c} members={members} navigate={navigate} />
              ))}

              {pendingProjectTasks.length > 0 && (
                <>
                  {dueSoonChores.length > 0 && (
                    <Typography
                      level='body-xs'
                      color='neutral'
                      sx={{
                        textTransform: 'uppercase',
                        letterSpacing: 0.8,
                        mt: 1.5,
                        mb: 0.75,
                        fontSize: 10,
                      }}
                    >
                      Project Tasks
                    </Typography>
                  )}
                  {pendingProjectTasks.map(t => (
                    <ProjectTaskRow key={`pt-${t.id}`} task={t} navigate={navigate} />
                  ))}
                </>
              )}
            </>
          )}
        </Box>
      </Box>

      {/* ── RIGHT: Team cards ───────────────────────────────────────────── */}
      <Box
        sx={{
          flex: 1,
          display: 'flex',
          flexDirection: 'column',
          overflow: 'hidden',
          minWidth: 0,
        }}
      >
        <SectionHeader count={members.length}>Team</SectionHeader>

        <Box sx={{ flex: 1, overflowY: 'auto', pr: 0.5 }}>
          {members.length === 0 ? (
            <Typography level='body-sm' color='neutral'>
              No circle members found.
            </Typography>
          ) : (
            members.map(m => (
              <UserCard
                key={m.userId}
                member={m}
                assignedChores={choresByUser[m.userId] || []}
                projectTaskCount={projectTaskCountByUser[m.userId] || 0}
                completedToday={completedTodayByUser[m.userId] || 0}
                points={availablePoints(m)}
                navigate={navigate}
              />
            ))
          )}
        </Box>
      </Box>
    </Box>
  )
}

export default DashboardView
