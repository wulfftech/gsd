import {
  Box,
  Button,
  Divider,
  FormControl,
  FormHelperText,
  FormLabel,
  Input,
  Stack,
  Textarea,
  Typography,
} from '@mui/joy'
import { useState } from 'react'
import { Toll } from '@mui/icons-material'
import { useResponsiveModal } from '../../../hooks/useResponsiveModal.js'

function RewardModal({ config }) {
  const { ResponsiveModal } = useResponsiveModal()
  const isEdit = !!config?.reward

  const [name, setName] = useState(config?.reward?.name || '')
  const [description, setDescription] = useState(
    config?.reward?.description || '',
  )
  const [points, setPoints] = useState(config?.reward?.points || '')
  const [errors, setErrors] = useState({})

  const validate = () => {
    const e = {}
    if (!name.trim()) e.name = 'Name is required'
    const pts = Number(points)
    if (!points || isNaN(pts) || pts < 1)
      e.points = 'Points must be a positive number'
    setErrors(e)
    return Object.keys(e).length === 0
  }

  const handleSave = () => {
    if (!validate()) return
    config?.onSave({
      id: config?.reward?.id,
      name: name.trim(),
      description: description.trim(),
      points: Number(points),
    })
  }

  return (
    <ResponsiveModal open={config?.isOpen} onClose={config?.onClose} size='sm'>
      <Stack spacing={2}>
        <Typography level='h4' sx={{ fontWeight: 600 }}>
          {isEdit ? 'Edit Reward' : 'New Reward'}
        </Typography>

        <Divider />

        <FormControl error={!!errors.name}>
          <FormLabel>Name</FormLabel>
          <Input
            value={name}
            placeholder='e.g. Extra screen time'
            onChange={e => setName(e.target.value)}
          />
          {errors.name && <FormHelperText>{errors.name}</FormHelperText>}
        </FormControl>

        <FormControl>
          <FormLabel>Description (optional)</FormLabel>
          <Textarea
            value={description}
            placeholder='Describe what this reward entails…'
            minRows={2}
            onChange={e => setDescription(e.target.value)}
          />
        </FormControl>

        <FormControl error={!!errors.points}>
          <FormLabel>Points Cost</FormLabel>
          <Input
            type='number'
            value={points}
            placeholder='e.g. 50'
            startDecorator={<Toll />}
            slotProps={{ input: { min: 1 } }}
            onChange={e => setPoints(e.target.value)}
          />
          {errors.points && <FormHelperText>{errors.points}</FormHelperText>}
        </FormControl>

        {config?.error && (
          <Typography color='danger' level='body-sm'>
            {config.error}
          </Typography>
        )}

        <Divider />

        <Box sx={{ display: 'flex', gap: 1, justifyContent: 'flex-end' }}>
          <Button variant='outlined' color='neutral' onClick={config?.onClose}>
            Cancel
          </Button>
          <Button
            onClick={handleSave}
            disabled={config?.isPending}
            loading={config?.isPending}
          >
            {isEdit ? 'Save Changes' : 'Create Reward'}
          </Button>
        </Box>
      </Stack>
    </ResponsiveModal>
  )
}

export default RewardModal
