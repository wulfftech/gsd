import AddIcon from '@mui/icons-material/Add'
import DeleteOutlineIcon from '@mui/icons-material/DeleteOutline'
import {
  Avatar,
  Box,
  Button,
  Chip,
  FormControl,
  FormLabel,
  IconButton,
  Input,
  Option,
  Select,
  Stack,
  Textarea,
  Typography,
} from '@mui/joy'
import { useEffect, useState } from 'react'
import { useResponsiveModal } from '../../../hooks/useResponsiveModal'
import PROJECT_COLORS, {
  getTextColorFromBackgroundColor,
} from '../../../utils/Colors'
import PROJECT_ICONS, { getIconComponent } from '../../../utils/ProjectIcons'
import { useCircleMembers } from '../../../queries/UserQueries'
import {
  useCreateProject,
  useUpdateProject,
} from '../../Projects/ProjectQueries'
import IconPickerModal from './IconPickerModal'

// Task status constants
const TASK_PENDING = 0
const TASK_PENDING_APPROVAL = 1
const TASK_COMPLETED = 2

const ProjectModal = ({ isOpen, onClose, onSave, project }) => {
  const { ResponsiveModal } = useResponsiveModal()

  const [name, setName] = useState('')
  const [description, setDescription] = useState('')
  const [color, setColor] = useState(PROJECT_COLORS[0].value)
  const [icon, setIcon] = useState(PROJECT_ICONS[0].value)
  const [dueDate, setDueDate] = useState('')
  const [points, setPoints] = useState(0)
  const [assigneeIDs, setAssigneeIDs] = useState([])
  const [tasks, setTasks] = useState([])
  const [newTaskName, setNewTaskName] = useState('')
  const [error, setError] = useState('')
  const [isIconPickerOpen, setIsIconPickerOpen] = useState(false)

  const { data: circleMembersData } = useCircleMembers()
  const members = circleMembersData?.res || []

  const createMutation = useCreateProject()
  const updateMutation = useUpdateProject()

  useEffect(() => {
    if (!isOpen) return
    if (project) {
      setName(project.name || '')
      setDescription(project.description || '')
      setColor(project.color || PROJECT_COLORS[0].value)
      setIcon(project.icon || PROJECT_ICONS[0].value)
      setDueDate(
        project.dueDate
          ? new Date(project.dueDate).toISOString().slice(0, 10)
          : '',
      )
      setPoints(project.points || 0)
      setAssigneeIDs((project.assignees || []).map(a => a.userId))
      setTasks(
        (project.tasks || []).map(t => ({
          id: t.id,
          name: t.name,
          description: t.description || '',
          order: t.order,
          status: t.status ?? TASK_PENDING,
          completedBy: t.completedBy || null,
        })),
      )
    } else {
      setName('')
      setDescription('')
      setColor(PROJECT_COLORS[0].value)
      setIcon(PROJECT_ICONS[0].value)
      setDueDate('')
      setPoints(0)
      setAssigneeIDs([])
      setTasks([])
    }
    setNewTaskName('')
    setError('')
  }, [isOpen, project])

  const handleAddTask = () => {
    const trimmed = newTaskName.trim()
    if (!trimmed) return
    setTasks(prev => [
      ...prev,
      { id: undefined, name: trimmed, description: '', order: prev.length },
    ])
    setNewTaskName('')
  }

  const handleRemoveTask = idx => {
    setTasks(prev => prev.filter((_, i) => i !== idx))
  }

  const handleSubmit = e => {
    e.preventDefault()
    if (!name.trim()) {
      setError('Project name is required')
      return
    }
    setError('')

    const payload = {
      name: name.trim(),
      description: description.trim() || null,
      color,
      icon,
      dueDate: dueDate ? new Date(dueDate).toISOString() : null,
      points: Number(points) || 0,
      assigneeIds: assigneeIDs,
      tasks: tasks.map((t, i) => ({
        id: t.id ?? undefined,
        name: t.name,
        description: t.description || null,
        order: i,
        status: t.status ?? TASK_PENDING,
        completedBy: t.completedBy || null,
      })),
    }

    const mutation = project ? updateMutation : createMutation
    const mutArg = project ? { id: project.id, ...payload } : payload

    mutation.mutate(mutArg, {
      onSuccess: result => {
        onSave(result)
        onClose()
      },
      onError: err => {
        setError(err.message || 'Something went wrong')
      },
    })
  }

  const isSubmitting = createMutation.isPending || updateMutation.isPending

  const handleToggleAssignee = userId => {
    setAssigneeIDs(prev =>
      prev.includes(userId)
        ? prev.filter(id => id !== userId)
        : [...prev, userId],
    )
  }

  return (
    <ResponsiveModal
      open={isOpen}
      onClose={() => !isSubmitting && onClose()}
      size='md'
      unmountDelay={250}
      fullWidth
      title={project ? 'Edit Project' : 'New Project'}
      footer={
        <Box display='flex' justifyContent='space-around' gap={1}>
          <Button
            type='submit'
            form='project-form'
            loading={isSubmitting}
            disabled={!name.trim() || isSubmitting}
            fullWidth
            size='lg'
          >
            {project ? 'Update' : 'Create'}
          </Button>
          <Button
            variant='outlined'
            onClick={onClose}
            disabled={isSubmitting}
            fullWidth
            size='lg'
          >
            Cancel
          </Button>
        </Box>
      }
    >
      <form onSubmit={handleSubmit} id='project-form'>
        <Stack spacing={2.5}>
          {/* Name */}
          <FormControl required>
            <FormLabel>Project Name</FormLabel>
            <Input
              value={name}
              onChange={e => setName(e.target.value)}
              placeholder='Enter project name…'
              autoFocus
              disabled={isSubmitting}
            />
          </FormControl>

          {/* Description */}
          <FormControl>
            <FormLabel>Description</FormLabel>
            <Textarea
              value={description}
              onChange={e => setDescription(e.target.value)}
              placeholder='Optional description…'
              minRows={2}
              maxRows={4}
              disabled={isSubmitting}
            />
          </FormControl>

          {/* Icon + Color row */}
          <Box sx={{ display: 'flex', gap: 2 }}>
            <FormControl sx={{ flex: 1 }}>
              <FormLabel>Icon</FormLabel>
              <Button
                variant='outlined'
                onClick={() => setIsIconPickerOpen(true)}
                startDecorator={
                  <Avatar
                    size='sm'
                    sx={{
                      width: 24,
                      height: 24,
                      bgcolor: color,
                    }}
                  >
                    {(() => {
                      const IconComponent = getIconComponent(icon)
                      return (
                        <IconComponent
                          sx={{
                            fontSize: 14,
                            color: getTextColorFromBackgroundColor(color),
                          }}
                        />
                      )
                    })()}
                  </Avatar>
                }
                sx={{ justifyContent: 'flex-start' }}
              >
                {PROJECT_ICONS.find(i => i.value === icon)?.name || 'Select'}
              </Button>
            </FormControl>

            <FormControl sx={{ flex: 1 }}>
              <FormLabel>Color</FormLabel>
              <Select
                value={color}
                onChange={(_, v) => v && setColor(v)}
                renderValue={sel => (
                  <Typography
                    startDecorator={
                      <Box
                        sx={{
                          width: 16,
                          height: 16,
                          borderRadius: '50%',
                          background: sel.value,
                        }}
                      />
                    }
                  >
                    {sel.label}
                  </Typography>
                )}
              >
                {PROJECT_COLORS.map(c => (
                  <Option key={c.value} value={c.value}>
                    <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
                      <Box
                        sx={{
                          width: 16,
                          height: 16,
                          borderRadius: '50%',
                          background: c.value,
                        }}
                      />
                      {c.name}
                    </Box>
                  </Option>
                ))}
              </Select>
            </FormControl>
          </Box>

          {/* Due date + Points row */}
          <Box sx={{ display: 'flex', gap: 2 }}>
            <FormControl sx={{ flex: 1 }}>
              <FormLabel>Due Date</FormLabel>
              <Input
                type='date'
                value={dueDate}
                onChange={e => setDueDate(e.target.value)}
                disabled={isSubmitting}
                slotProps={{
                  input: { min: new Date().toISOString().slice(0, 10) },
                }}
              />
            </FormControl>

            <FormControl sx={{ flex: 1 }}>
              <FormLabel>Points</FormLabel>
              <Input
                type='number'
                value={points}
                onChange={e => setPoints(Math.max(0, Number(e.target.value)))}
                slotProps={{ input: { min: 0 } }}
                disabled={isSubmitting}
              />
            </FormControl>
          </Box>

          {/* Assignees */}
          <FormControl>
            <FormLabel>Assignees</FormLabel>
            <Box sx={{ display: 'flex', flexWrap: 'wrap', gap: 1, mt: 0.5 }}>
              {members.map(m => {
                const selected = assigneeIDs.includes(m.userId)
                return (
                  <Chip
                    key={m.userId}
                    variant={selected ? 'solid' : 'outlined'}
                    color={selected ? 'primary' : 'neutral'}
                    onClick={() => handleToggleAssignee(m.userId)}
                    sx={{ cursor: 'pointer' }}
                  >
                    {m.displayName}
                  </Chip>
                )
              })}
              {members.length === 0 && (
                <Typography level='body-sm' color='neutral'>
                  No circle members found
                </Typography>
              )}
            </Box>
          </FormControl>

          {/* Tasks */}
          <FormControl>
            <FormLabel>Tasks</FormLabel>
            <Stack spacing={1}>
              {tasks.map((t, idx) => (
                <Box
                  key={idx}
                  sx={{
                    display: 'flex',
                    alignItems: 'center',
                    gap: 1,
                    px: 1.5,
                    py: 0.75,
                    borderRadius: 'sm',
                    border: '1px solid',
                    borderColor: 'divider',
                    bgcolor: 'background.surface',
                  }}
                >
                  <Typography level='body-sm' sx={{ flex: 1 }}>
                    {t.name}
                  </Typography>
                  {/* Task status chip */}
                  <Chip
                    size='sm'
                    variant='soft'
                    color={
                      t.status === TASK_COMPLETED
                        ? 'success'
                        : t.status === TASK_PENDING_APPROVAL
                          ? 'warning'
                          : 'neutral'
                    }
                  >
                    {t.status === TASK_COMPLETED
                      ? 'Completed'
                      : t.status === TASK_PENDING_APPROVAL
                        ? 'Pending Approval'
                        : 'Pending'}
                  </Chip>
                  {/* Only show delete for pending tasks */}
                  <IconButton
                    size='sm'
                    variant='plain'
                    color='danger'
                    onClick={() => handleRemoveTask(idx)}
                    disabled={t.status !== TASK_PENDING}
                  >
                    <DeleteOutlineIcon sx={{ fontSize: 18 }} />
                  </IconButton>
                </Box>
              ))}

              {/* Add task row */}
              <Box sx={{ display: 'flex', gap: 1 }}>
                <Input
                  size='sm'
                  placeholder='Add a task…'
                  value={newTaskName}
                  onChange={e => setNewTaskName(e.target.value)}
                  onKeyDown={e => {
                    if (e.key === 'Enter') {
                      e.preventDefault()
                      handleAddTask()
                    }
                  }}
                  sx={{ flex: 1 }}
                />
                <IconButton
                  size='sm'
                  variant='soft'
                  color='primary'
                  onClick={handleAddTask}
                  disabled={!newTaskName.trim()}
                >
                  <AddIcon />
                </IconButton>
              </Box>
            </Stack>
          </FormControl>

          {error && (
            <Typography color='danger' level='body-sm'>
              {error}
            </Typography>
          )}
        </Stack>
      </form>

      <IconPickerModal
        isOpen={isIconPickerOpen}
        onClose={() => setIsIconPickerOpen(false)}
        onSelect={v => {
          setIcon(v)
          setIsIconPickerOpen(false)
        }}
        currentIcon={icon}
        projectColor={color}
      />
    </ResponsiveModal>
  )
}

export default ProjectModal
