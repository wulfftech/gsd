import { CheckCircle, Error, Info, Undo, Warning } from '@mui/icons-material'
import { Box, Button, Snackbar, Typography } from '@mui/joy'
import React, {
  createContext,
  useCallback,
  useContext,
  useMemo,
  useState,
} from 'react'

const NotificationContext = createContext()

export const useNotification = () => useContext(NotificationContext)

// For backward compatibility
export const useError = () => {
  const { showError } = useNotification()
  return { showError }
}

// Normalize notification input. Pure, so it lives outside the component and
// never needs memoizing.
const normalizeNotification = (input, type) => {
  if (typeof input === 'string') {
    return {
      type,
      message: input,
    }
  }

  if (typeof input === 'object' && input !== null) {
    // If it's already a properly structured notification
    if (input.title || input.message) {
      return {
        type,
        ...input,
      }
    }

    // If it's a simple object with just message content
    return {
      type,
      message: input.message || input.toString(),
      title: input.title,
      ...input,
    }
  }

  return {
    type,
    message: input?.toString() || 'Unknown notification',
  }
}

// Notification types configuration with default titles
const NOTIFICATION_TYPES = {
  error: {
    color: 'danger',
    icon: <Error color='danger' />,
    autoHideDuration: 6000,
    showDismissButton: true,
    defaultTitle: 'Error',
  },
  success: {
    color: 'success',
    icon: <CheckCircle color='success' />,
    autoHideDuration: 5000,
    showDismissButton: false,
    defaultTitle: 'Success',
  },
  undo: {
    color: 'success',
    icon: <Undo color='success' />,
    autoHideDuration: null,
    showDismissButton: false,
    defaultTitle: 'Undone Successfully',
  },
  warning: {
    color: 'warning',
    icon: <Warning color='warning' />,
    autoHideDuration: 4000,
    showDismissButton: false,
    defaultTitle: 'Warning',
  },
  info: {
    color: 'primary',
    icon: <Info color='primary' />,
    autoHideDuration: 4000,
    showDismissButton: false,
    defaultTitle: 'Information',
  },
  custom: {
    color: 'neutral',
    icon: null,
    autoHideDuration: null,
    showDismissButton: false,
    defaultTitle: 'Notification',
  },
}

export const NotificationProvider = ({ children }) => {
  const [notifications, setNotifications] = useState([])

  const removeNotification = useCallback(id => {
    setNotifications(prev => prev.filter(n => n.id !== id))
  }, [])

  const addNotification = useCallback(
    notification => {
      const id = Date.now() + Math.random()
      const newNotification = {
        id,
        ...notification,
        timestamp: Date.now(),
      }

      setNotifications(prev => [...prev, newNotification])

      // Auto-remove notification if it has a duration
      const config =
        NOTIFICATION_TYPES[notification.type] || NOTIFICATION_TYPES.info
      if (config.autoHideDuration) {
        setTimeout(() => {
          removeNotification(id)
        }, config.autoHideDuration)
      }

      return id
    },
    [removeNotification],
  )

  const clearAllNotifications = useCallback(() => {
    setNotifications([])
  }, [])

  // Unified notification method
  const showNotification = useCallback(
    notification => {
      // Handle different input formats
      if (typeof notification === 'string') {
        return addNotification(normalizeNotification(notification, 'info'))
      }

      return addNotification(
        normalizeNotification(notification, notification.type || 'info'),
      )
    },
    [addNotification],
  )

  // Specific notification methods with enhanced language
  const showError = useCallback(
    error => {
      return addNotification(normalizeNotification(error, 'error'))
    },
    [addNotification],
  )

  const showUndo = useCallback(
    message => {
      return addNotification(normalizeNotification(message, 'undo'))
    },
    [addNotification],
  )

  const showSuccess = useCallback(
    message => {
      return addNotification(normalizeNotification(message, 'success'))
    },
    [addNotification],
  )

  const showWarning = useCallback(
    message => {
      return addNotification(normalizeNotification(message, 'warning'))
    },
    [addNotification],
  )

  const showInfo = useCallback(
    message => {
      return addNotification(normalizeNotification(message, 'info'))
    },
    [addNotification],
  )

  const contextValue = useMemo(
    () => ({
      showNotification,
      showError,
      showSuccess,
      showUndo,
      showWarning,
      showInfo,
      removeNotification,
      clearAllNotifications,
      notifications,
    }),
    [
      showNotification,
      showError,
      showSuccess,
      showUndo,
      showWarning,
      showInfo,
      removeNotification,
      clearAllNotifications,
      notifications,
    ],
  )

  const renderNotification = notification => {
    const config =
      NOTIFICATION_TYPES[notification.type] || NOTIFICATION_TYPES.info

    // Handle custom notifications with components
    if (notification.type === 'custom' && notification.component) {
      return (
        <Snackbar
          key={notification.id}
          open={true}
          onClose={() => removeNotification(notification.id)}
          anchorOrigin={
            notification.anchorOrigin || {
              vertical: 'bottom',
              horizontal: 'right',
            }
          }
          {...(notification.snackbarProps || {})}
        >
          {React.cloneElement(notification.component, {
            onClose: () => removeNotification(notification.id),
            ...notification.componentProps,
          })}
        </Snackbar>
      )
    }

    // Handle standard notifications
    // Determine the icon to use
    const notificationIcon = notification.icon || config.icon

    // Determine title and message
    const title = notification.title || config.defaultTitle
    const message = notification.message

    return (
      <Snackbar
        key={notification.id}
        open={true}
        autoHideDuration={config.autoHideDuration}
        onClose={() => removeNotification(notification.id)}
        startDecorator={notificationIcon}
        endDecorator={
          notification.undoAction ? (
            <Button
              variant='outlined'
              color={config.color}
              onClick={() => {
                notification.undoAction()
                removeNotification(notification.id)
              }}
            >
              Undo
            </Button>
          ) : config.showDismissButton ? (
            <Button
              variant='outlined'
              color={config.color}
              onClick={() => removeNotification(notification.id)}
            >
              Dismiss
            </Button>
          ) : null
        }
        anchorOrigin={
          notification.anchorOrigin || {
            vertical: 'bottom',
            horizontal: 'right',
          }
        }
        {...(notification.snackbarProps || {})}
      >
        {/* Enhanced structure like ErrorProvider - always show title and message for consistency */}
        {title && message ? (
          <Box>
            <Typography color={config.color} level='title-sm'>
              {title}
            </Typography>
            <Typography color={config.color} level='body-sm'>
              {message}
            </Typography>
          </Box>
        ) : (
          <Typography color={config.color} level='body-md'>
            {message || title || 'Notification'}
          </Typography>
        )}
      </Snackbar>
    )
  }

  return (
    <NotificationContext.Provider value={contextValue}>
      {children}
      {notifications.map(renderNotification)}
    </NotificationContext.Provider>
  )
}
