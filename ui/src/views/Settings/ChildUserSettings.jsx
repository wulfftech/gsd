import DeleteIcon from '@mui/icons-material/Delete'
import EditIcon from '@mui/icons-material/Edit'
import PersonAddIcon from '@mui/icons-material/PersonAdd'
import {
  Avatar,
  Box,
  Button,
  Card,
  CardContent,
  Divider,
  IconButton,
  Typography,
} from '@mui/joy'
import { useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'
import useConfirmationModal from '../../hooks/useConfirmationModal'
import { useManagedUsers } from '../../queries/UserQueries'
import { useCircleMembers, useUserProfile } from '../../queries/UserQueries'
import { useNotification } from '../../service/NotificationProvider'
import {
  CreateManagedUser,
  DeleteManagedUser,
  UpdateManagedUser,
} from '../../utils/Fetcher'
import ConfirmationModal from '../Modals/Inputs/ConfirmationModal'
import ManagedUserModal from '../Modals/Inputs/ManagedUserModal'
import SettingsLayout from './SettingsLayout'

const ChildUserSettings = () => {
  const { data: userProfile } = useUserProfile()
  const { data: circleMembersData } = useCircleMembers()
  const { data: managedUsers = [], isLoading, refetch } = useManagedUsers()
  const { showNotification } = useNotification()
  const queryClient = useQueryClient()
  const { confirmModalConfig, showConfirmation } = useConfirmationModal()

  const [createModalOpen, setCreateModalOpen] = useState(false)
  const [editModalUser, setEditModalUser] = useState(null) // the user being edited
  const [deletingId, setDeletingId] = useState(null)

  // Only circle admins can manage these accounts
  const members = circleMembersData?.res || []
  const isAdmin =
    members.find(m => m.userId === userProfile?.id)?.role === 'admin'

  const handleCreate = async ({ username, displayName, password }) => {
    const response = await CreateManagedUser(username, displayName, password)
    if (response.ok) {
      const result = await response.json()
      showNotification({
        type: 'success',
        message: `Account "${result.res?.displayName || username}" created`,
      })
      refetch()
      queryClient.invalidateQueries(['managedUsers'])
      queryClient.invalidateQueries(['allCircleMembers'])
    } else {
      const err = await response.json()
      throw new Error(err.error || 'Failed to create account')
    }
  }

  const handleEdit = async ({ username, displayName, password }) => {
    if (!editModalUser) return
    const response = await UpdateManagedUser(editModalUser.id, {
      username,
      displayName,
      password,
    })
    if (response.ok) {
      showNotification({ type: 'success', message: 'Account updated' })
      refetch()
      queryClient.invalidateQueries(['managedUsers'])
      queryClient.invalidateQueries(['allCircleMembers'])
    } else {
      const err = await response.json()
      throw new Error(err.error || 'Failed to update account')
    }
  }

  const handleDelete = user => {
    showConfirmation(
      `Delete the managed account "${user.displayName || user.username}"? This cannot be undone.`,
      'Delete Managed Account',
      async () => {
        setDeletingId(user.id)
        try {
          const response = await DeleteManagedUser(user.id)
          if (response.ok) {
            showNotification({
              type: 'success',
              message: `Account "${user.displayName || user.username}" deleted`,
            })
            refetch()
            queryClient.invalidateQueries(['managedUsers'])
            queryClient.invalidateQueries(['allCircleMembers'])
          } else {
            const err = await response.json()
            throw new Error(err.error || 'Failed to delete account')
          }
        } catch (err) {
          showNotification({ type: 'error', message: err.message })
        } finally {
          setDeletingId(null)
        }
      },
      'Delete',
      'Cancel',
      'danger',
    )
  }

  if (!isAdmin) {
    return (
      <SettingsLayout title='Managed Accounts'>
        <Typography level='body-md' color='neutral'>
          Only circle admins can manage these accounts.
        </Typography>
      </SettingsLayout>
    )
  }

  return (
    <SettingsLayout title='Managed Accounts'>
      <div className='grid gap-4'>
        <Typography level='body-md'>
          Managed accounts can log in and complete tasks. Any circle admin can
          create, edit or delete them.
        </Typography>

        <Box
          sx={{
            display: 'flex',
            justifyContent: 'space-between',
            alignItems: 'center',
          }}
        >
          <Typography level='title-lg'>
            Accounts ({managedUsers.length})
          </Typography>
          <Button
            startDecorator={<PersonAddIcon />}
            onClick={() => setCreateModalOpen(true)}
          >
            Add Account
          </Button>
        </Box>

        {isLoading ? (
          <Typography>Loading…</Typography>
        ) : managedUsers.length === 0 ? (
          <Card variant='soft' sx={{ textAlign: 'center', py: 4 }}>
            <CardContent>
              <Typography level='title-md' mb={1}>
                No Managed Accounts
              </Typography>
              <Typography level='body-sm' mb={3}>
                Create accounts so household or team members can log in and
                complete their assigned tasks.
              </Typography>
              <Button
                startDecorator={<PersonAddIcon />}
                onClick={() => setCreateModalOpen(true)}
              >
                Add First Account
              </Button>
            </CardContent>
          </Card>
        ) : (
          <div className='grid gap-3'>
            {managedUsers.map(user => (
              <Card key={user.id} variant='outlined'>
                <CardContent>
                  <Box sx={{ display: 'flex', alignItems: 'center', gap: 2 }}>
                    <Avatar size='lg'>
                      {(user.displayName || user.username)?.[0]?.toUpperCase()}
                    </Avatar>

                    <Box sx={{ flex: 1 }}>
                      <Typography level='title-md'>
                        {user.displayName || user.username}
                      </Typography>
                      <Typography level='body-sm' color='neutral'>
                        @{user.username}
                      </Typography>
                      <Typography level='body-xs' color='neutral'>
                        Created {new Date(user.createdAt).toLocaleDateString()}
                      </Typography>
                    </Box>

                    <Box sx={{ display: 'flex', gap: 1 }}>
                      <IconButton
                        size='sm'
                        variant='soft'
                        title='Edit account'
                        onClick={() => setEditModalUser(user)}
                      >
                        <EditIcon />
                      </IconButton>
                      <IconButton
                        size='sm'
                        variant='soft'
                        color='danger'
                        title='Delete account'
                        loading={deletingId === user.id}
                        onClick={() => handleDelete(user)}
                      >
                        <DeleteIcon />
                      </IconButton>
                    </Box>
                  </Box>
                </CardContent>
              </Card>
            ))}
          </div>
        )}

        <Divider sx={{ my: 2 }} />

        <Box>
          <Typography level='title-md' mb={2}>
            How Managed Accounts Work
          </Typography>
          <Typography level='body-sm' mb={1}>
            · Any circle admin can create, rename, change the password of, or
            delete a managed account.
          </Typography>
          <Typography level='body-sm' mb={1}>
            · Managed accounts log in with their username and password — no
            email required.
          </Typography>
          <Typography level='body-sm'>
            · Managed accounts are added to the circle as members and can
            complete tasks, but have no admin permissions.
          </Typography>
        </Box>
      </div>

      {/* Create modal */}
      <ManagedUserModal
        isOpen={createModalOpen}
        onClose={() => setCreateModalOpen(false)}
        onSuccess={async data => {
          await handleCreate(data)
          setCreateModalOpen(false)
        }}
      />

      {/* Edit modal */}
      <ManagedUserModal
        isOpen={editModalUser !== null}
        managedUser={editModalUser}
        onClose={() => setEditModalUser(null)}
        onSuccess={async data => {
          await handleEdit(data)
          setEditModalUser(null)
        }}
      />

      <ConfirmationModal config={confirmModalConfig} />
    </SettingsLayout>
  )
}

export default ChildUserSettings
