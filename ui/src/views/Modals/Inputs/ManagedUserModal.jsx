/**
 * ManagedUserModal — used for both creating and editing admin-managed accounts.
 *
 * Create mode  (managedUser === null):  username + displayName + password required
 * Edit mode    (managedUser !== null):  all fields optional — leave blank to keep current
 */
import {
  Box,
  Button,
  FormControl,
  FormHelperText,
  FormLabel,
  Input,
  Typography,
} from '@mui/joy'
import { useEffect, useState } from 'react'
import { useResponsiveModal } from '../../../hooks/useResponsiveModal'

const USERNAME_RE = /^[a-zA-Z0-9._-]+$/

function ManagedUserModal({ isOpen, onClose, onSuccess, managedUser = null }) {
  const { ResponsiveModal } = useResponsiveModal()
  const isEdit = managedUser !== null

  const [username, setUsername] = useState('')
  const [displayName, setDisplayName] = useState('')
  const [password, setPassword] = useState('')
  const [confirmPassword, setConfirmPassword] = useState('')
  const [errors, setErrors] = useState({})
  const [touched, setTouched] = useState({})
  const [isSubmitting, setIsSubmitting] = useState(false)

  // Reset form when modal opens
  useEffect(() => {
    if (!isOpen) return
    setUsername(isEdit ? managedUser.username : '')
    setDisplayName(isEdit ? managedUser.displayName || '' : '')
    setPassword('')
    setConfirmPassword('')
    setErrors({})
    setTouched({})
  }, [isOpen, isEdit, managedUser])

  // Live validation
  useEffect(() => {
    const errs = {}

    if (touched.username) {
      if (!isEdit && !username.trim()) {
        errs.username = 'Username is required'
      } else if (username && username.length < 2) {
        errs.username = 'Username must be at least 2 characters'
      } else if (username && username.length > 30) {
        errs.username = 'Username must be 30 characters or fewer'
      } else if (username && !USERNAME_RE.test(username)) {
        errs.username = 'Only letters, numbers, dots, dashes and underscores'
      }
    }

    if (touched.displayName && displayName.length > 50) {
      errs.displayName = 'Display name must be 50 characters or fewer'
    }

    if (touched.password && password) {
      if (password.length < 8 || password.length > 64) {
        errs.password = 'Password must be 8–64 characters'
      }
    }
    if (!isEdit && touched.password && !password) {
      errs.password = 'Password is required'
    }

    if (touched.confirmPassword && password !== confirmPassword) {
      errs.confirmPassword = 'Passwords do not match'
    }

    setErrors(errs)
  }, [username, displayName, password, confirmPassword, touched, isEdit])

  const touch = field => setTouched(prev => ({ ...prev, [field]: true }))

  const handleSubmit = async () => {
    // Touch all fields to surface errors
    setTouched({
      username: true,
      displayName: true,
      password: true,
      confirmPassword: true,
    })

    const errs = {}
    if (!isEdit && !username.trim()) errs.username = 'Username is required'
    if (!isEdit && !password) errs.password = 'Password is required'
    if (password && (password.length < 8 || password.length > 64))
      errs.password = 'Password must be 8–64 characters'
    if (password && password !== confirmPassword)
      errs.confirmPassword = 'Passwords do not match'

    if (Object.keys(errs).length > 0) {
      setErrors(errs)
      return
    }

    setIsSubmitting(true)
    try {
      await onSuccess({
        username: username.trim() || undefined,
        displayName: displayName.trim() || undefined,
        password: password || undefined,
      })
      onClose()
    } catch (err) {
      console.error('ManagedUserModal submit error:', err)
    } finally {
      setIsSubmitting(false)
    }
  }

  const canSubmit =
    !isSubmitting &&
    Object.keys(errors).length === 0 &&
    (isEdit || (username.trim() && password && password === confirmPassword))

  return (
    <ResponsiveModal open={isOpen} onClose={onClose}>
      <Typography level='h4' mb={1}>
        {isEdit ? 'Edit Managed Account' : 'Create Managed Account'}
      </Typography>
      <Typography level='body-sm' color='neutral' mb={3}>
        {isEdit
          ? 'Leave fields blank to keep current values.'
          : 'Managed accounts can log in and complete tasks. Any circle admin can manage them.'}
      </Typography>

      {/* Username */}
      <FormControl error={!!errors.username} sx={{ mb: 2 }}>
        <FormLabel>{isEdit ? 'Username' : 'Username *'}</FormLabel>
        <Input
          placeholder={
            isEdit
              ? `Keep current (${managedUser?.username})`
              : 'e.g. alex or kids.alex'
          }
          value={username}
          onChange={e => {
            setUsername(e.target.value)
            touch('username')
          }}
          autoFocus={!isEdit}
        />
        {errors.username && <FormHelperText>{errors.username}</FormHelperText>}
      </FormControl>

      {/* Display name */}
      <FormControl error={!!errors.displayName} sx={{ mb: 2 }}>
        <FormLabel>Display Name</FormLabel>
        <Input
          placeholder={
            isEdit
              ? `Keep current (${managedUser?.displayName || managedUser?.username})`
              : 'Optional — defaults to username'
          }
          value={displayName}
          onChange={e => {
            setDisplayName(e.target.value)
            touch('displayName')
          }}
        />
        {errors.displayName && (
          <FormHelperText>{errors.displayName}</FormHelperText>
        )}
      </FormControl>

      {/* Password */}
      <FormControl error={!!errors.password} sx={{ mb: 2 }}>
        <FormLabel>{isEdit ? 'New Password' : 'Password *'}</FormLabel>
        <Input
          type='password'
          placeholder={
            isEdit ? 'Leave blank to keep current password' : '8–64 characters'
          }
          value={password}
          onChange={e => {
            setPassword(e.target.value)
            touch('password')
          }}
        />
        {errors.password && <FormHelperText>{errors.password}</FormHelperText>}
      </FormControl>

      {/* Confirm password — only shown when a password is entered */}
      {(!isEdit || password) && (
        <FormControl error={!!errors.confirmPassword} sx={{ mb: 3 }}>
          <FormLabel>Confirm Password {!isEdit ? '*' : ''}</FormLabel>
          <Input
            type='password'
            placeholder='Confirm password'
            value={confirmPassword}
            onChange={e => {
              setConfirmPassword(e.target.value)
              touch('confirmPassword')
            }}
          />
          {errors.confirmPassword && (
            <FormHelperText>{errors.confirmPassword}</FormHelperText>
          )}
        </FormControl>
      )}

      <Box display='flex' gap={2}>
        <Button
          size='lg'
          variant='outlined'
          onClick={onClose}
          disabled={isSubmitting}
          sx={{ flex: 1 }}
        >
          Cancel
        </Button>
        <Button
          size='lg'
          onClick={handleSubmit}
          disabled={!canSubmit}
          loading={isSubmitting}
          sx={{ flex: 1 }}
        >
          {isEdit ? 'Save Changes' : 'Create Account'}
        </Button>
      </Box>
    </ResponsiveModal>
  )
}

export default ManagedUserModal
