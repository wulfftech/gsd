import { CardGiftcard, Person, Toll } from '@mui/icons-material'
import {
  Avatar,
  Box,
  Button,
  Card,
  Chip,
  CircularProgress,
  Divider,
  Stack,
  Typography,
} from '@mui/joy'
import { useState } from 'react'

import { useResponsiveModal } from '../../hooks/useResponsiveModal.js'
import { useRewards, useRedeemReward } from '../Rewards/RewardQueries.jsx'
import { resolvePhotoURL } from '../../utils/Helpers.jsx'

function RewardsPickerModal({ config }) {
  const { ResponsiveModal } = useResponsiveModal()
  const { data: rewards = [], isLoading } = useRewards()
  const redeemReward = useRedeemReward()
  const [redeemedId, setRedeemedId] = useState(null)

  const available = config?.available ?? 0

  const handleRedeem = reward => {
    setRedeemedId(reward.id)
    redeemReward.mutate(reward.id, {
      onSuccess: () => {
        config?.onRedeemed?.()
        config?.onClose?.()
      },
      onError: () => {
        setRedeemedId(null)
      },
    })
  }

  return (
    <ResponsiveModal open={config?.isOpen} onClose={config?.onClose} size='md'>
      <Stack spacing={2}>
        {/* Header */}
        <Box sx={{ display: 'flex', alignItems: 'center', gap: 1.5 }}>
          <CardGiftcard sx={{ fontSize: '1.5rem' }} />
          <Typography level='h4' sx={{ fontWeight: 600 }}>
            Redeem a Reward
          </Typography>
        </Box>

        <Divider />

        {/* User info + balance */}
        <Card variant='soft' sx={{ p: 2 }}>
          <Stack direction='row' spacing={2} alignItems='center'>
            <Avatar
              size='md'
              src={resolvePhotoURL(config?.user?.image)}
              sx={{ border: '2px solid', borderColor: 'warning.200' }}
            >
              <Person />
            </Avatar>
            <Box sx={{ flex: 1 }}>
              <Typography level='title-sm' sx={{ fontWeight: 600 }}>
                {config?.user?.displayName || 'You'}
              </Typography>
              <Chip
                size='sm'
                variant='soft'
                color='success'
                startDecorator={<Toll />}
                sx={{ mt: 0.5 }}
              >
                {available} points available
              </Chip>
            </Box>
          </Stack>
        </Card>

        {/* Rewards list */}
        {isLoading ? (
          <Box sx={{ display: 'flex', justifyContent: 'center', py: 4 }}>
            <CircularProgress size='md' />
          </Box>
        ) : rewards.length === 0 ? (
          <Box sx={{ textAlign: 'center', py: 4 }}>
            <CardGiftcard sx={{ fontSize: '3rem', color: 'neutral.300', mb: 1 }} />
            <Typography level='body-md' sx={{ color: 'neutral.500' }}>
              No rewards available yet
            </Typography>
          </Box>
        ) : (
          <Stack spacing={1.5}>
            {rewards.map(reward => {
              const canAfford = available >= reward.points
              const isPending =
                redeemReward.isPending && redeemedId === reward.id

              return (
                <Card
                  key={reward.id}
                  variant='outlined'
                  sx={{
                    opacity: canAfford ? 1 : 0.45,
                    transition: 'box-shadow 0.15s',
                    '&:hover': canAfford ? { boxShadow: 'sm' } : undefined,
                  }}
                >
                  <Stack
                    direction='row'
                    alignItems='center'
                    spacing={2}
                  >
                    <Box sx={{ flex: 1, minWidth: 0 }}>
                      <Typography level='title-sm' sx={{ fontWeight: 600 }}>
                        {reward.name}
                      </Typography>
                      {reward.description && (
                        <Typography
                          level='body-xs'
                          sx={{
                            color: 'text.secondary',
                            overflow: 'hidden',
                            textOverflow: 'ellipsis',
                            whiteSpace: 'nowrap',
                          }}
                        >
                          {reward.description}
                        </Typography>
                      )}
                    </Box>
                    <Stack direction='row' alignItems='center' spacing={1} sx={{ flexShrink: 0 }}>
                      <Chip
                        size='sm'
                        variant='soft'
                        color={canAfford ? 'success' : 'neutral'}
                        startDecorator={<Toll />}
                      >
                        {reward.points}
                      </Chip>
                      <Button
                        size='sm'
                        disabled={!canAfford || redeemReward.isPending}
                        loading={isPending}
                        onClick={() => handleRedeem(reward)}
                        sx={{ minWidth: 75 }}
                      >
                        Redeem
                      </Button>
                    </Stack>
                  </Stack>
                </Card>
              )
            })}
          </Stack>
        )}

        <Divider />

        <Button
          variant='outlined'
          color='neutral'
          onClick={config?.onClose}
          fullWidth
        >
          Close
        </Button>
      </Stack>
    </ResponsiveModal>
  )
}

export default RewardsPickerModal
