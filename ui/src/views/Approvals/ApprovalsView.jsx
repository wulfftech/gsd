import {
  CheckCircleOutline,
  DoneAll,
  HighlightOff,
  HourglassEmpty,
} from '@mui/icons-material'
import {
  Alert,
  Box,
  Button,
  Card,
  Chip,
  Container,
  Divider,
  IconButton,
  List,
  ListItem,
  ListItemContent,
  Stack,
  Typography,
} from '@mui/joy'
import moment from 'moment'
import { useMemo, useState } from 'react'
import { useCircleRole } from '../../hooks/useCircleRole'
import useConfirmationModal from '../../hooks/useConfirmationModal'
import {
  useBulkApproveChores,
  useBulkRejectChores,
  usePendingApprovals,
} from '../../queries/HouseholdQueries.jsx'
import { useNotification } from '../../service/NotificationProvider'
import LoadingComponent from '../components/Loading.jsx'
import ConfirmationModal from '../Modals/Inputs/ConfirmationModal'
import TextModal from '../Modals/Inputs/TextModal'

const CHECKPOINT_LABELS = {
  afternoon: 'After school',
  evening: 'Bedtime',
  extra: 'Paid extra',
  morning: 'Morning',
}

// Group pending items by the person who ticked them, keeping the server's
// order (completedBy, then performedAt).
const groupByCompleter = items => {
  const groups = []
  const byId = new Map()
  items.forEach(item => {
    let group = byId.get(item.completedBy)
    if (!group) {
      group = {
        items: [],
        name: item.completedByName || 'Someone',
        userId: item.completedBy,
      }
      byId.set(item.completedBy, group)
      groups.push(group)
    }
    group.items.push(item)
  })
  return groups
}

const ApprovalsView = () => {
  const { canManage, isLoading: isRoleLoading } = useCircleRole()
  const {
    data: pending = [],
    isLoading,
    isError,
    error,
  } = usePendingApprovals({ enabled: canManage })
  const bulkApprove = useBulkApproveChores()
  const bulkReject = useBulkRejectChores()
  const { showError, showSuccess } = useNotification()
  const { confirmModalConfig, showConfirmation } = useConfirmationModal()

  // Chore the reject-note modal is open for, or null.
  const [rejecting, setRejecting] = useState(null)
  // Failures from the last bulk action: [{ choreId, name, error }].
  const [failures, setFailures] = useState([])

  const groups = useMemo(() => groupByCompleter(pending), [pending])
  const busy = bulkApprove.isPending || bulkReject.isPending

  const handleResult = (result, verb, snapshot) => {
    const done = result?.done?.length || 0
    const failed = (result?.failed || []).map(f => ({
      ...f,
      name:
        snapshot.find(p => p.choreId === f.choreId)?.name ||
        `Task ${f.choreId}`,
    }))
    setFailures(failed)
    if (done > 0) {
      showSuccess({
        message: `${verb} ${done} task${done === 1 ? '' : 's'}`,
        title: verb,
      })
    }
    if (failed.length > 0) {
      showError({
        message: `${failed.length} task${failed.length === 1 ? '' : 's'} couldn't be ${verb.toLowerCase()}. See the list below.`,
        title: 'Some tasks failed',
      })
    }
  }

  const approve = choreIds => {
    const snapshot = pending
    bulkApprove.mutate(choreIds, {
      onError: err =>
        showError({ message: err.message, title: 'Could not approve' }),
      onSuccess: result => handleResult(result, 'Approved', snapshot),
    })
  }

  const reject = (choreIds, note) => {
    const snapshot = pending
    bulkReject.mutate(
      { choreIds, note },
      {
        onError: err =>
          showError({ message: err.message, title: 'Could not reject' }),
        onSuccess: result => handleResult(result, 'Rejected', snapshot),
      },
    )
  }

  if (isRoleLoading || (canManage && isLoading)) {
    return <LoadingComponent />
  }

  if (!canManage) {
    return (
      <Container maxWidth='md'>
        <Card sx={{ mt: 2, p: 3, textAlign: 'center' }}>
          <Typography level='title-lg'>Parents only</Typography>
          <Typography level='body-md' sx={{ mt: 1 }}>
            Only a parent can approve tasks.
          </Typography>
        </Card>
      </Container>
    )
  }

  return (
    <Container maxWidth='md' sx={{ pb: 10 }}>
      <Box
        sx={{
          alignItems: 'center',
          display: 'flex',
          flexWrap: 'wrap',
          gap: 1,
          justifyContent: 'space-between',
          mb: 2,
          mt: 1,
        }}
      >
        <Box>
          <Typography level='h3'>Approvals</Typography>
          <Typography level='body-sm' sx={{ color: 'text.tertiary' }}>
            Ticked-off tasks waiting for a parent to check.
          </Typography>
        </Box>
        {pending.length > 0 && (
          <Button
            color='success'
            startDecorator={<DoneAll />}
            disabled={busy}
            onClick={() =>
              showConfirmation(
                `Approve all ${pending.length} task${pending.length === 1 ? '' : 's'}?`,
                'Approve all',
                () => approve('all'),
                'Approve all',
                'Cancel',
                'success',
              )
            }
          >
            Approve all
          </Button>
        )}
      </Box>

      {isError && (
        <Alert color='danger' sx={{ mb: 2 }}>
          {error?.message || 'Could not load approvals'}
        </Alert>
      )}

      {failures.length > 0 && (
        <Alert
          color='warning'
          sx={{ alignItems: 'flex-start', mb: 2 }}
          endDecorator={
            <Button
              size='sm'
              variant='plain'
              color='warning'
              onClick={() => setFailures([])}
            >
              Dismiss
            </Button>
          }
        >
          <Box>
            <Typography level='title-sm'>
              These tasks weren&apos;t updated:
            </Typography>
            <List size='sm' sx={{ '--ListItem-paddingX': 0 }}>
              {failures.map(f => (
                <ListItem key={f.choreId}>
                  {f.name}: {f.error}
                </ListItem>
              ))}
            </List>
          </Box>
        </Alert>
      )}

      {!isError && pending.length === 0 && (
        <Card variant='soft' sx={{ alignItems: 'center', p: 4 }}>
          <CheckCircleOutline sx={{ color: 'success.500', fontSize: 48 }} />
          <Typography level='title-md'>All caught up</Typography>
          <Typography level='body-sm' sx={{ color: 'text.tertiary' }}>
            Nothing is waiting for approval.
          </Typography>
        </Card>
      )}

      <Stack spacing={2}>
        {groups.map(group => (
          <Card key={group.userId} variant='outlined'>
            <Box
              sx={{
                alignItems: 'center',
                display: 'flex',
                gap: 1,
                justifyContent: 'space-between',
              }}
            >
              <Typography level='title-lg'>
                {group.name}{' '}
                <Chip size='sm' variant='soft' color='warning'>
                  {group.items.length}
                </Chip>
              </Typography>
              <Button
                size='sm'
                variant='soft'
                color='success'
                startDecorator={<DoneAll />}
                disabled={busy}
                onClick={() => approve(group.items.map(i => i.choreId))}
              >
                Approve all
              </Button>
            </Box>
            <Divider />
            <List sx={{ '--ListItem-paddingX': 0 }}>
              {group.items.map(item => (
                <ListItem
                  key={item.historyId || item.choreId}
                  endAction={
                    <Box sx={{ display: 'flex', gap: 0.5 }}>
                      <IconButton
                        aria-label={`Approve ${item.name}`}
                        color='success'
                        variant='soft'
                        disabled={busy}
                        onClick={() => approve([item.choreId])}
                      >
                        <CheckCircleOutline />
                      </IconButton>
                      <IconButton
                        aria-label={`Reject ${item.name}`}
                        color='danger'
                        variant='soft'
                        disabled={busy}
                        onClick={() => setRejecting(item)}
                      >
                        <HighlightOff />
                      </IconButton>
                    </Box>
                  }
                >
                  <ListItemContent sx={{ pr: 12 }}>
                    <Typography level='title-sm'>{item.name}</Typography>
                    <Box
                      sx={{
                        alignItems: 'center',
                        display: 'flex',
                        flexWrap: 'wrap',
                        gap: 1,
                        mt: 0.5,
                      }}
                    >
                      {item.checkpoint && (
                        <Chip size='sm' variant='outlined'>
                          {CHECKPOINT_LABELS[item.checkpoint] ||
                            item.checkpoint}
                        </Chip>
                      )}
                      <Typography
                        level='body-xs'
                        startDecorator={<HourglassEmpty fontSize='inherit' />}
                        sx={{ color: 'text.tertiary' }}
                        title={
                          item.performedAt
                            ? moment(item.performedAt).format('llll')
                            : undefined
                        }
                      >
                        {item.performedAt
                          ? `Ticked ${moment(item.performedAt).fromNow()}`
                          : 'Ticked'}
                      </Typography>
                      {item.points > 0 && (
                        <Typography
                          level='body-xs'
                          sx={{ color: 'text.tertiary' }}
                        >
                          {item.points} pts
                        </Typography>
                      )}
                    </Box>
                    {item.note && (
                      <Typography level='body-sm' sx={{ mt: 0.5 }}>
                        &ldquo;{item.note}&rdquo;
                      </Typography>
                    )}
                  </ListItemContent>
                </ListItem>
              ))}
            </List>
          </Card>
        ))}
      </Stack>

      {rejecting && (
        <TextModal
          key={rejecting.choreId}
          isOpen={Boolean(rejecting)}
          title={`Reject "${rejecting.name}"? Add a note if you like.`}
          current=''
          okText='Reject'
          onClose={() => setRejecting(null)}
          onSave={note => reject([rejecting.choreId], note?.trim() || '')}
        />
      )}
      {confirmModalConfig?.isOpen && (
        <ConfirmationModal config={confirmModalConfig} />
      )}
    </Container>
  )
}

export default ApprovalsView
