import { Person } from '@mui/icons-material'
import { Avatar, Box, Button, Card, Divider, Stack, Typography } from '@mui/joy'
import { useState } from 'react'

import { useResponsiveModal } from '../../hooks/useResponsiveModal.js'
import { resolvePhotoURL } from '../../utils/Helpers.jsx'

function PerformerPickerModal({ config }) {
  const { ResponsiveModal } = useResponsiveModal()
  const [selectedId, setSelectedId] = useState(config?.currentUserId ?? null)

  // Reset selection when modal opens for a new chore
  const performers = config?.performers ?? []

  const handleConfirm = () => {
    config?.onConfirm?.(selectedId)
  }

  return (
    <ResponsiveModal open={config?.isOpen} onClose={config?.onClose} size='sm'>
      <Stack spacing={2}>
        <Typography level='h4' sx={{ fontWeight: 600 }}>
          Who completed this?
        </Typography>
        <Typography level='body-sm' sx={{ color: 'text.secondary' }}>
          Select the person who completed this task.
        </Typography>

        <Divider />

        <Stack spacing={1}>
          {performers.map(p => {
            const isSelected = selectedId === p.userId
            return (
              <Card
                key={p.userId}
                variant={isSelected ? 'solid' : 'outlined'}
                color={isSelected ? 'primary' : 'neutral'}
                onClick={() => setSelectedId(p.userId)}
                sx={{
                  p: 1.5,
                  cursor: 'pointer',
                  transition: 'all 0.15s',
                  '&:hover': { boxShadow: 'sm' },
                }}
              >
                <Stack direction='row' spacing={1.5} alignItems='center'>
                  <Avatar
                    size='sm'
                    src={resolvePhotoURL(p.image)}
                    sx={{
                      border: '2px solid',
                      borderColor: isSelected ? 'primary.200' : 'neutral.200',
                    }}
                  >
                    <Person />
                  </Avatar>
                  <Typography
                    level='title-sm'
                    sx={{ fontWeight: isSelected ? 700 : 500 }}
                  >
                    {p.displayName || p.username}
                    {p.userId === config?.currentUserId && (
                      <Typography
                        component='span'
                        level='body-xs'
                        sx={{
                          ml: 1,
                          color: isSelected ? 'primary.100' : 'text.tertiary',
                        }}
                      >
                        (me)
                      </Typography>
                    )}
                  </Typography>
                </Stack>
              </Card>
            )
          })}
        </Stack>

        <Divider />

        <Box sx={{ display: 'flex', gap: 1 }}>
          <Button
            variant='outlined'
            color='neutral'
            fullWidth
            onClick={config?.onClose}
          >
            Cancel
          </Button>
          <Button
            fullWidth
            disabled={selectedId === null}
            onClick={handleConfirm}
          >
            Mark as Done
          </Button>
        </Box>
      </Stack>
    </ResponsiveModal>
  )
}

export default PerformerPickerModal
