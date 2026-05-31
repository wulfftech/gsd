import { AddCard, Person, Toll } from '@mui/icons-material'
import {
  Avatar,
  Box,
  Button,
  Card,
  Chip,
  Divider,
  FormControl,
  FormLabel,
  IconButton,
  Input,
  Stack,
  Typography,
} from '@mui/joy'
import { useEffect, useState } from 'react'

import { useResponsiveModal } from '../../hooks/useResponsiveModal.js'
import { resolvePhotoURL } from '../../utils/Helpers.jsx'

function DepositPointsModal({ config }) {
  const { ResponsiveModal } = useResponsiveModal()

  const [points, setPoints] = useState(0)
  const predefinedPoints = [1, 5, 10, 25, 50]

  useEffect(() => {
    setPoints(0)
  }, [config])

  const handlePointsChange = value => {
    const numValue = Number(value)
    if (numValue < 0) {
      setPoints(0)
      return
    }
    setPoints(numValue)
  }

  const addPredefinedPoints = point => {
    setPoints(points + point)
  }

  const canDeposit = points > 0

  return (
    <ResponsiveModal open={config?.isOpen} onClose={config?.onClose} size='md'>
      <Stack spacing={2}>
        <Box sx={{ display: 'flex', alignItems: 'center', gap: 1.5 }}>
          <AddCard
            sx={{
              fontSize: '1.5rem',
            }}
          />
          <Typography level='h4' sx={{ fontWeight: 600 }}>
            Deposit Points
          </Typography>
        </Box>

        <Divider />

        {/* User Info Card */}
        <Card
          variant='soft'
          sx={{
            p: 2,
          }}
        >
          <Stack direction='row' spacing={2} alignItems='center'>
            <Avatar
              size='md'
              src={resolvePhotoURL(config?.user?.image)}
              sx={{
                border: '2px solid',
                borderColor: 'success.200',
              }}
            >
              <Person />
            </Avatar>
            <Box sx={{ flex: 1 }}>
              <Typography level='title-sm' sx={{ fontWeight: 600 }}>
                {config?.user?.displayName || 'User'}
              </Typography>
              <Chip
                size='sm'
                variant='soft'
                color='success'
                startDecorator={<Toll />}
                sx={{ mt: 0.5 }}
              >
                {config?.available || 0} points available
              </Chip>
            </Box>
          </Stack>
        </Card>

        {/* Points Input Section */}
        <FormControl>
          <FormLabel sx={{ fontWeight: 600, mb: 1 }}>
            Points to Deposit
          </FormLabel>
          <Input
            type='number'
            value={points}
            size='lg'
            variant='outlined'
            startDecorator={<Toll />}
            slotProps={{
              input: {
                min: 0,
                placeholder: 'Enter points...',
              },
            }}
            onChange={e => handlePointsChange(e.target.value)}
            sx={{
              '--Input-decoratorChildHeight': '45px',
              fontSize: 'lg',
              fontWeight: 500,
              '&:focus-within': {
                borderColor: 'success.500',
                boxShadow: '0 0 0 2px rgba(76, 175, 80, 0.2)',
              },
            }}
          />
        </FormControl>

        {/* Quick Selection Buttons */}
        <Box>
          <Typography level='body-sm' sx={{ fontWeight: 600, mb: 1.5 }}>
            Quick Add:
          </Typography>
          <Stack
            direction='row'
            spacing={1}
            justifyContent='center'
            flexWrap='wrap'
            useFlexGap
          >
            {predefinedPoints.map(point => (
              <IconButton
                key={point}
                variant='outlined'
                onClick={() => addPredefinedPoints(point)}
                sx={{
                  borderRadius: '50%',
                  minWidth: 45,
                  minHeight: 45,
                  fontWeight: 600,
                  fontSize: 'sm',
                  '&:hover': {
                    transform: 'scale(1.05)',
                    boxShadow: 'sm',
                  },
                  transition: 'all 0.2s ease',
                }}
              >
                +{point}
              </IconButton>
            ))}
          </Stack>
        </Box>

        {/* Summary Section */}
        {points > 0 && (
          <Card
            variant='soft'
            color='success'
            sx={{
              p: 2,
              textAlign: 'center',
              background:
                'linear-gradient(135deg, rgba(76,175,80,0.1) 0%, rgba(76,175,80,0.05) 100%)',
            }}
          >
            <Typography level='body-sm' sx={{ color: 'text.secondary' }}>
              You are about to deposit
            </Typography>
            <Typography
              level='h4'
              sx={{ color: 'success.600', fontWeight: 700 }}
            >
              {points} points
            </Typography>
            <Typography
              level='body-xs'
              sx={{ color: 'text.secondary', mt: 0.5 }}
            >
              New balance: {(config?.available || 0) + points} points
            </Typography>
          </Card>
        )}

        <Divider />

        {/* Action Buttons */}
        <Stack direction='row' spacing={2}>
          <Button
            size='lg'
            onClick={config?.onClose}
            variant='outlined'
            color='neutral'
            fullWidth
            sx={{
              '&:hover': {
                backgroundColor: 'neutral.50',
              },
            }}
          >
            Cancel
          </Button>
          <Button
            size='lg'
            color='success'
            onClick={() =>
              config?.onSave({
                points: Number(points),
                userId: config?.user?.userId,
              })
            }
            disabled={!canDeposit}
            fullWidth
            startDecorator={<AddCard />}
            sx={{
              transition: 'all 0.2s ease',
            }}
          >
            Deposit
          </Button>
        </Stack>
      </Stack>
    </ResponsiveModal>
  )
}

export default DepositPointsModal
