import { ContentCopy, Delete } from '@mui/icons-material'
import {
  Alert,
  Box,
  Button,
  Card,
  Chip,
  Divider,
  FormControl,
  FormHelperText,
  FormLabel,
  IconButton,
  Input,
  List,
  ListItem,
  ListItemContent,
  Typography,
} from '@mui/joy'
import moment from 'moment'
import { useState } from 'react'
import { useCircleRole } from '../../hooks/useCircleRole'
import useConfirmationModal from '../../hooks/useConfirmationModal'
import {
  useCreateHoliday,
  useCreateKioskToken,
  useDeleteHoliday,
  useHolidays,
  useKioskStatus,
  useSetKioskPin,
} from '../../queries/HouseholdQueries.jsx'
import { useNotification } from '../../service/NotificationProvider'
import LoadingComponent from '../components/Loading.jsx'
import ConfirmationModal from '../Modals/Inputs/ConfirmationModal'
import SettingsLayout from './SettingsLayout'

const formatDate = date => moment(date, 'YYYY-MM-DD').format('D MMM YYYY')

const HolidaysSection = ({ showConfirmation }) => {
  const { data: holidays = [], isLoading, isError, error } = useHolidays()
  const createHoliday = useCreateHoliday()
  const deleteHoliday = useDeleteHoliday()
  const { showError, showSuccess } = useNotification()

  const [name, setName] = useState('')
  const [startDate, setStartDate] = useState('')
  const [endDate, setEndDate] = useState('')

  const formError =
    startDate && endDate && endDate < startDate
      ? 'End date must be on or after the start date'
      : null
  const canAdd = name.trim() && startDate && endDate && !formError

  const handleAdd = () => {
    if (!canAdd) return
    createHoliday.mutate(
      { endDate, name: name.trim(), startDate },
      {
        onError: err =>
          showError({ message: err.message, title: 'Could not add holiday' }),
        onSuccess: () => {
          showSuccess({ message: `Added ${name.trim()}`, title: 'Saved' })
          setName('')
          setStartDate('')
          setEndDate('')
        },
      },
    )
  }

  const handleDelete = holiday => {
    showConfirmation(
      `Delete ${holiday.name} (${formatDate(holiday.startDate)} to ${formatDate(holiday.endDate)})?`,
      'Delete holiday',
      () =>
        deleteHoliday.mutate(holiday.id, {
          onError: err =>
            showError({
              message: err.message,
              title: 'Could not delete holiday',
            }),
        }),
      'Delete',
      'Cancel',
      'danger',
    )
  }

  const sorted = [...holidays].sort((a, b) =>
    a.startDate.localeCompare(b.startDate),
  )

  return (
    <Box>
      <Typography level='title-lg'>School holidays</Typography>
      <Typography level='body-sm' sx={{ mb: 1 }}>
        Tasks set to &ldquo;Skip school holidays&rdquo; won&apos;t fall due on
        these dates. Start and end dates are included.
      </Typography>

      {isError && (
        <Alert color='danger' sx={{ mb: 1 }}>
          {error?.message}
        </Alert>
      )}
      {isLoading ? (
        <LoadingComponent />
      ) : (
        <Card variant='outlined' sx={{ mb: 2 }}>
          {sorted.length === 0 ? (
            <Typography level='body-sm' sx={{ color: 'text.tertiary' }}>
              No holidays added yet.
            </Typography>
          ) : (
            <List sx={{ '--ListItem-paddingX': 0 }}>
              {sorted.map(holiday => (
                <ListItem
                  key={holiday.id}
                  endAction={
                    <IconButton
                      aria-label={`Delete ${holiday.name}`}
                      color='danger'
                      variant='plain'
                      disabled={deleteHoliday.isPending}
                      onClick={() => handleDelete(holiday)}
                    >
                      <Delete />
                    </IconButton>
                  }
                >
                  <ListItemContent>
                    <Typography level='title-sm'>{holiday.name}</Typography>
                    <Typography level='body-xs'>
                      {formatDate(holiday.startDate)} to{' '}
                      {formatDate(holiday.endDate)}
                    </Typography>
                  </ListItemContent>
                </ListItem>
              ))}
            </List>
          )}
        </Card>
      )}

      <Box
        sx={{
          alignItems: 'flex-end',
          display: 'flex',
          flexWrap: 'wrap',
          gap: 1,
        }}
      >
        <FormControl sx={{ minWidth: 180 }}>
          <FormLabel>Name</FormLabel>
          <Input
            placeholder='e.g. Term 4 break'
            value={name}
            onChange={e => setName(e.target.value)}
          />
        </FormControl>
        <FormControl>
          <FormLabel>Start date</FormLabel>
          <Input
            type='date'
            value={startDate}
            onChange={e => setStartDate(e.target.value)}
          />
        </FormControl>
        <FormControl error={Boolean(formError)}>
          <FormLabel>End date</FormLabel>
          <Input
            type='date'
            value={endDate}
            slotProps={{ input: { min: startDate || undefined } }}
            onChange={e => setEndDate(e.target.value)}
          />
        </FormControl>
        <Button
          disabled={!canAdd}
          loading={createHoliday.isPending}
          onClick={handleAdd}
        >
          Add
        </Button>
      </Box>
      {formError && (
        <Typography level='body-xs' color='danger' sx={{ mt: 0.5 }}>
          {formError}
        </Typography>
      )}
    </Box>
  )
}

const KioskSection = ({ showConfirmation }) => {
  const { data: kiosk, isLoading, isError, error } = useKioskStatus()
  const createToken = useCreateKioskToken()
  const setPin = useSetKioskPin()
  const { showError, showSuccess } = useNotification()

  // Full kiosk URL, shown once right after it's created.
  const [kioskUrl, setKioskUrl] = useState(null)
  const [pin, setPinValue] = useState('')

  const pinValid = /^\d{4,8}$/.test(pin)

  const generateToken = () => {
    createToken.mutate(undefined, {
      onError: err =>
        showError({
          message: err.message,
          title: 'Could not create kiosk link',
        }),
      onSuccess: res => {
        if (res?.path) {
          setKioskUrl(window.location.origin + res.path)
        }
      },
    })
  }

  const handleCreateLink = () => {
    if (kiosk?.tokenSet) {
      showConfirmation(
        'The old kiosk link will stop working straight away. Any screen using it will need the new link.',
        'Replace kiosk link?',
        generateToken,
        'Replace',
        'Cancel',
        'warning',
      )
    } else {
      generateToken()
    }
  }

  const handleCopy = async () => {
    try {
      await navigator.clipboard.writeText(kioskUrl)
      showSuccess({ message: 'Kiosk link copied', title: 'Copied' })
    } catch {
      showError({
        message: 'Copy the link by hand instead.',
        title: 'Could not copy',
      })
    }
  }

  const handleSetPin = () => {
    if (!pinValid) return
    setPin.mutate(pin, {
      onError: err =>
        showError({ message: err.message, title: 'Could not set PIN' }),
      onSuccess: () => {
        setPinValue('')
        showSuccess({ message: 'Parent PIN saved', title: 'Saved' })
      },
    })
  }

  return (
    <Box>
      <Typography level='title-lg'>Kiosk</Typography>
      <Typography level='body-sm' sx={{ mb: 1 }}>
        A link for a shared screen (like a tablet on the fridge) where the kids
        can tick off jobs. Parents use the PIN to approve on that screen.
      </Typography>

      {isError && (
        <Alert color='danger' sx={{ mb: 1 }}>
          {error?.message}
        </Alert>
      )}
      {isLoading ? (
        <LoadingComponent />
      ) : (
        <>
          <Box sx={{ display: 'flex', flexWrap: 'wrap', gap: 1, mb: 2 }}>
            <Chip
              color={kiosk?.tokenSet ? 'success' : 'neutral'}
              variant='soft'
            >
              {kiosk?.tokenSet ? 'Kiosk link set' : 'No kiosk link'}
            </Chip>
            <Chip color={kiosk?.pinSet ? 'success' : 'neutral'} variant='soft'>
              {kiosk?.pinSet ? 'Parent PIN set' : 'No parent PIN'}
            </Chip>
          </Box>

          <Button
            variant={kiosk?.tokenSet ? 'outlined' : 'solid'}
            color={kiosk?.tokenSet ? 'warning' : 'primary'}
            loading={createToken.isPending}
            onClick={handleCreateLink}
          >
            {kiosk?.tokenSet ? 'Replace kiosk link' : 'Create kiosk link'}
          </Button>

          {kioskUrl && (
            <Alert color='success' variant='soft' sx={{ mt: 2 }}>
              <Box sx={{ width: '100%' }}>
                <Typography level='title-sm'>Your kiosk link</Typography>
                <Box
                  sx={{
                    alignItems: 'center',
                    display: 'flex',
                    gap: 1,
                    mt: 0.5,
                  }}
                >
                  <Input
                    readOnly
                    value={kioskUrl}
                    sx={{ flex: 1, fontFamily: 'monospace' }}
                    onFocus={e => e.target.select()}
                  />
                  <IconButton
                    aria-label='Copy kiosk link'
                    variant='soft'
                    onClick={handleCopy}
                  >
                    <ContentCopy />
                  </IconButton>
                </Box>
                <Typography level='body-xs' sx={{ mt: 0.5 }}>
                  This link is only shown once. Copy it now. If you lose it,
                  replace it with a new one.
                </Typography>
              </Box>
            </Alert>
          )}

          <FormControl
            sx={{ maxWidth: 320, mt: 3 }}
            error={Boolean(pin) && !pinValid}
          >
            <FormLabel>Set parent PIN</FormLabel>
            <Box sx={{ display: 'flex', gap: 1 }}>
              <Input
                type='password'
                placeholder={kiosk?.pinSet ? 'New PIN' : 'PIN'}
                value={pin}
                slotProps={{
                  input: {
                    autoComplete: 'new-password',
                    inputMode: 'numeric',
                    maxLength: 8,
                    pattern: '[0-9]*',
                  },
                }}
                onChange={e => setPinValue(e.target.value.replace(/\D/g, ''))}
                sx={{ flex: 1 }}
              />
              <Button
                disabled={!pinValid}
                loading={setPin.isPending}
                onClick={handleSetPin}
              >
                Save PIN
              </Button>
            </Box>
            <FormHelperText>4 to 8 digits.</FormHelperText>
          </FormControl>
        </>
      )}
    </Box>
  )
}

const HouseholdSettings = () => {
  const { isAdmin, isLoading } = useCircleRole()
  const { confirmModalConfig, showConfirmation } = useConfirmationModal()

  if (isLoading) {
    return (
      <SettingsLayout title='Household'>
        <LoadingComponent />
      </SettingsLayout>
    )
  }

  if (!isAdmin) {
    return (
      <SettingsLayout title='Household'>
        <Typography level='body-md'>
          Only a circle admin can change household settings.
        </Typography>
      </SettingsLayout>
    )
  }

  return (
    <SettingsLayout title='Household'>
      <div className='grid gap-4'>
        <HolidaysSection showConfirmation={showConfirmation} />
        <Divider />
        <KioskSection showConfirmation={showConfirmation} />
      </div>
      {confirmModalConfig?.isOpen && (
        <ConfirmationModal config={confirmModalConfig} />
      )}
    </SettingsLayout>
  )
}

export default HouseholdSettings
