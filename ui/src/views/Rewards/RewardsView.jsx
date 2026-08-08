import {
  AddCircleOutline,
  CardGiftcard,
  CheckCircleOutline,
  Delete,
  Edit,
  HourglassEmpty,
  Toll,
} from '@mui/icons-material'
import {
  Box,
  Button,
  Card,
  CardContent,
  Chip,
  Container,
  Grid,
  IconButton,
  Stack,
  Tab,
  TabList,
  Tabs,
  Typography,
} from '@mui/joy'
import { useState } from 'react'

import { useUserProfile } from '../../queries/UserQueries.jsx'
import { useCircleMembers } from '../../queries/UserQueries.jsx'
import { useNotification } from '../../service/NotificationProvider'
import ConfirmationModal from '../Modals/Inputs/ConfirmationModal.jsx'
import RewardModal from '../Modals/Inputs/RewardModal.jsx'
import {
  useCreateReward,
  useDeleteReward,
  useFulfillRedemption,
  useRedeemReward,
  useRedemptions,
  useRewards,
  useUpdateReward,
} from './RewardQueries.jsx'

const RewardsView = () => {
  const [tab, setTab] = useState(0)
  const [rewardModalConfig, setRewardModalConfig] = useState(null)
  const [rewardModalError, setRewardModalError] = useState('')
  const [deleteConfirmConfig, setDeleteConfirmConfig] = useState(null)
  const [redeemConfirmConfig, setRedeemConfirmConfig] = useState(null)

  const { showError } = useNotification()
  const { data: userProfile } = useUserProfile()
  const { data: circleMembersData } = useCircleMembers()
  const { data: rewards = [], isLoading: rewardsLoading } = useRewards()
  const { data: redemptions = [], isLoading: redemptionsLoading } =
    useRedemptions()

  const createReward = useCreateReward()
  const updateReward = useUpdateReward()
  const deleteReward = useDeleteReward()
  const redeemReward = useRedeemReward()
  const fulfillRedemption = useFulfillRedemption()

  const circleUsers = circleMembersData?.res || []
  const isAdmin =
    circleUsers.find(u => u.userId === userProfile?.id)?.role === 'admin'

  const currentMember = circleUsers.find(u => u.userId === userProfile?.id)
  const availablePoints = currentMember ? currentMember.points || 0 : 0

  const handleSaveReward = data => {
    setRewardModalError('')
    if (data.id) {
      updateReward.mutate(data, {
        onSuccess: () => {
          setRewardModalConfig(null)
          setRewardModalError('')
        },
        onError: err => {
          const errMsg = err.message || 'Failed to update reward'
          setRewardModalError(errMsg)
          showError(errMsg)
        },
      })
    } else {
      createReward.mutate(data, {
        onSuccess: () => {
          setRewardModalConfig(null)
          setRewardModalError('')
        },
        onError: err => {
          const errMsg = err.message || 'Failed to create reward'
          setRewardModalError(errMsg)
          showError(errMsg)
        },
      })
    }
  }

  const handleDeleteReward = reward => {
    setDeleteConfirmConfig({
      isOpen: true,
      title: 'Delete Reward',
      message: `Are you sure you want to delete "${reward.name}"? This cannot be undone.`,
      confirmText: 'Delete',
      cancelText: 'Cancel',
      color: 'danger',
      onClose: confirmed => {
        if (confirmed) {
          deleteReward.mutate(reward.id, {
            onError: err => showError(err.message || 'Failed to delete reward'),
          })
        }
        setDeleteConfirmConfig(null)
      },
    })
  }

  const handleRedeemReward = reward => {
    if (availablePoints < reward.points) return
    setRedeemConfirmConfig({
      isOpen: true,
      title: 'Redeem Reward',
      message: `Redeem "${reward.name}" for ${reward.points} points? You have ${availablePoints} points available.`,
      confirmText: 'Redeem',
      cancelText: 'Cancel',
      color: 'primary',
      onClose: confirmed => {
        if (confirmed) {
          redeemReward.mutate(reward.id, {
            onError: err => showError(err.message || 'Failed to redeem reward'),
          })
        }
        setRedeemConfirmConfig(null)
      },
    })
  }

  return (
    <Container maxWidth='md' sx={{ py: 2 }}>
      {/* Header */}
      <Stack
        direction='row'
        alignItems='center'
        justifyContent='space-between'
        sx={{ mb: 3 }}
      >
        <Stack direction='row' alignItems='center' spacing={1.5}>
          <CardGiftcard sx={{ fontSize: '2rem', color: 'primary.500' }} />
          <Typography level='h3' sx={{ fontWeight: 700 }}>
            Rewards
          </Typography>
        </Stack>
        <Chip
          size='lg'
          variant='soft'
          color='success'
          startDecorator={<Toll />}
        >
          {availablePoints} pts available
        </Chip>
      </Stack>

      {/* Tabs — admin sees all three, members only see Marketplace */}
      <Tabs
        value={tab}
        onChange={(_, v) => setTab(v)}
        sx={{ mb: 3, borderRadius: 'md' }}
      >
        <TabList>
          <Tab>Marketplace</Tab>
          {isAdmin && <Tab>Manage</Tab>}
          {isAdmin && <Tab>Redemptions</Tab>}
        </TabList>
      </Tabs>

      {/* ── MARKETPLACE TAB ── */}
      {tab === 0 && (
        <>
          {rewardsLoading ? (
            <Typography>Loading rewards…</Typography>
          ) : rewards.length === 0 ? (
            <Card variant='soft' sx={{ textAlign: 'center', py: 6 }}>
              <CardGiftcard
                sx={{ fontSize: '3rem', color: 'neutral.400', mb: 1 }}
              />
              <Typography level='title-md' sx={{ color: 'neutral.500' }}>
                No rewards yet
              </Typography>
              {isAdmin && (
                <Typography level='body-sm' sx={{ color: 'neutral.400' }}>
                  Add some in the Manage tab
                </Typography>
              )}
            </Card>
          ) : (
            <Grid container spacing={2}>
              {rewards.map(reward => {
                const canAfford = availablePoints >= reward.points
                return (
                  <Grid key={reward.id} xs={12} sm={6}>
                    <Card
                      variant='outlined'
                      sx={{
                        height: '100%',
                        opacity: canAfford ? 1 : 0.6,
                        transition: 'box-shadow 0.2s',
                        '&:hover': canAfford
                          ? { boxShadow: 'md' }
                          : undefined,
                      }}
                    >
                      <CardContent>
                        <Stack spacing={1} sx={{ height: '100%' }}>
                          <Typography level='title-md' sx={{ fontWeight: 600 }}>
                            {reward.name}
                          </Typography>
                          {reward.description && (
                            <Typography
                              level='body-sm'
                              sx={{ color: 'text.secondary', flex: 1 }}
                            >
                              {reward.description}
                            </Typography>
                          )}
                          <Stack
                            direction='row'
                            justifyContent='space-between'
                            alignItems='center'
                            sx={{ mt: 'auto', pt: 1 }}
                          >
                            <Chip
                              variant='soft'
                              color={canAfford ? 'success' : 'neutral'}
                              startDecorator={<Toll />}
                            >
                              {reward.points} pts
                            </Chip>
                            <Button
                              size='sm'
                              disabled={!canAfford || redeemReward.isPending}
                              onClick={() => handleRedeemReward(reward)}
                            >
                              Redeem
                            </Button>
                          </Stack>
                        </Stack>
                      </CardContent>
                    </Card>
                  </Grid>
                )
              })}
            </Grid>
          )}
        </>
      )}

      {/* ── MANAGE TAB (admin only) ── */}
      {tab === 1 && isAdmin && (
        <Stack spacing={2}>
          <Box sx={{ display: 'flex', justifyContent: 'flex-end' }}>
            <Button
              startDecorator={<AddCircleOutline />}
              onClick={() =>
                setRewardModalConfig({
                  isOpen: true,
                  reward: null,
                  onClose: () => setRewardModalConfig(null),
                  onSave: handleSaveReward,
                  isPending: createReward.isPending,
                })
              }
            >
              Add Reward
            </Button>
          </Box>

          {rewardsLoading ? (
            <Typography>Loading…</Typography>
          ) : rewards.length === 0 ? (
            <Typography sx={{ color: 'neutral.500', textAlign: 'center', py: 4 }}>
              No rewards defined yet
            </Typography>
          ) : (
            rewards.map(reward => (
              <Card key={reward.id} variant='outlined'>
                <CardContent>
                  <Stack
                    direction='row'
                    alignItems='center'
                    justifyContent='space-between'
                  >
                    <Box sx={{ flex: 1, minWidth: 0 }}>
                      <Typography level='title-md' sx={{ fontWeight: 600 }}>
                        {reward.name}
                      </Typography>
                      {reward.description && (
                        <Typography
                          level='body-sm'
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
                    <Stack direction='row' alignItems='center' spacing={1}>
                      <Chip variant='soft' color='success' startDecorator={<Toll />}>
                        {reward.points} pts
                      </Chip>
                      <IconButton
                        size='sm'
                        variant='plain'
                        aria-label='Edit reward'
                        onClick={() =>
                          setRewardModalConfig({
                            isOpen: true,
                            reward,
                            onClose: () => setRewardModalConfig(null),
                            onSave: handleSaveReward,
                            isPending: updateReward.isPending,
                          })
                        }
                      >
                        <Edit />
                      </IconButton>
                      <IconButton
                        size='sm'
                        variant='plain'
                        color='danger'
                        aria-label='Delete reward'
                        onClick={() => handleDeleteReward(reward)}
                      >
                        <Delete />
                      </IconButton>
                    </Stack>
                  </Stack>
                </CardContent>
              </Card>
            ))
          )}
        </Stack>
      )}

      {/* ── REDEMPTIONS TAB (admin only) ── */}
      {tab === 2 && isAdmin && (
        <Stack spacing={2}>
          {redemptionsLoading ? (
            <Typography>Loading…</Typography>
          ) : redemptions.length === 0 ? (
            <Typography sx={{ color: 'neutral.500', textAlign: 'center', py: 4 }}>
              No redemptions yet
            </Typography>
          ) : (
            redemptions.map(r => (
              <Card key={r.id} variant='outlined'>
                <CardContent>
                  <Stack
                    direction='row'
                    alignItems='center'
                    justifyContent='space-between'
                  >
                    <Box>
                      <Stack direction='row' spacing={1} alignItems='center'>
                        <Typography level='title-sm' sx={{ fontWeight: 600 }}>
                          {r.rewardName}
                        </Typography>
                        <Chip
                          size='sm'
                          variant='soft'
                          color={r.status === 'fulfilled' ? 'success' : 'warning'}
                          startDecorator={
                            r.status === 'fulfilled' ? (
                              <CheckCircleOutline />
                            ) : (
                              <HourglassEmpty />
                            )
                          }
                        >
                          {r.status}
                        </Chip>
                      </Stack>
                      <Typography level='body-sm' sx={{ color: 'text.secondary' }}>
                        {r.displayName || r.username} ·{' '}
                        <Chip size='sm' variant='plain' startDecorator={<Toll />}>
                          {r.points} pts
                        </Chip>{' '}
                        · {new Date(r.redeemedAt).toLocaleDateString()}
                      </Typography>
                    </Box>
                    {r.status === 'pending' && (
                      <Button
                        size='sm'
                        color='success'
                        variant='soft'
                        disabled={fulfillRedemption.isPending}
                        onClick={() =>
                          fulfillRedemption.mutate(r.id, {
                            onError: err =>
                              showError(err.message || 'Failed to fulfill redemption'),
                          })
                        }
                      >
                        Mark Fulfilled
                      </Button>
                    )}
                  </Stack>
                </CardContent>
              </Card>
            ))
          )}
        </Stack>
      )}

      {/* Modals */}
      {rewardModalConfig && (
        <RewardModal config={{ ...rewardModalConfig, error: rewardModalError }} />
      )}

      {deleteConfirmConfig && (
        <ConfirmationModal config={deleteConfirmConfig} />
      )}

      {redeemConfirmConfig && (
        <ConfirmationModal config={redeemConfirmConfig} />
      )}
    </Container>
  )
}

export default RewardsView
