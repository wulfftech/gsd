import NavBar from '@/views/components/NavBar'
import { Button, Typography, useColorScheme } from '@mui/joy'
import Tracker from '@openreplay/tracker'
import { useCallback, useEffect } from 'react'
import { Outlet } from 'react-router-dom'
import { useRegisterSW } from 'virtual:pwa-register/react'
import { registerCapacitorListeners } from './CapacitorListener'
import PageTransition from './components/animations/PageTransition'
import { PINNED_DRAWER_WIDTH } from './constants/layout'
import { ImpersonateUserProvider } from './contexts/ImpersonateUserContext'
import { NavLayoutProvider, useNavLayout } from './contexts/NavLayoutContext'
import SSEProvider from './contexts/SSEContext'
import { AuthProvider } from './hooks/useAuth.jsx'
import { useNotification } from './service/NotificationProvider'

import NetworkBanner from './views/components/NetworkBanner'

const add = className => {
  document.getElementById('root').classList.add(className)
}

const remove = className => {
  document.getElementById('root').classList.remove(className)
}

// TODO: Update the interval to at 60 minutes
const intervalMS = 5 * 60 * 1000 // 5 minutes

// TODO: dead code -- never called, so OpenReplay session recording is never initialised.
// Kept deliberately rather than deleted; decide whether to wire it up or drop it.
// eslint-disable-next-line no-unused-vars
const startOpenReplay = () => {
  if (!import.meta.env.VITE_OPENREPLAY_PROJECT_KEY) return
  const tracker = new Tracker({
    projectKey: import.meta.env.VITE_OPENREPLAY_PROJECT_KEY,
  })
  tracker.start()
}

// Reserves horizontal space for NavBar's persistent pinned sidebar, which is
// a position:fixed Sheet and so takes no space in normal flow on its own
// (see NavBar.jsx). The spacer is a plain flex item placed first in DOM
// order — flex-direction "row" runs along the inline axis, so with
// document.dir kept in sync with the current language by
// LocalizationContext, it lands on the left in LTR and the right in RTL
// without any RTL branching here, matching where NavBar itself pins the
// Sheet's left/right edge.
const AppLayout = () => {
  const { isPersistent } = useNavLayout()

  return (
    <div style={{ display: 'flex' }}>
      {isPersistent && (
        <div
          aria-hidden='true'
          style={{
            flex: `0 0 ${PINNED_DRAWER_WIDTH}px`,
            width: PINNED_DRAWER_WIDTH,
          }}
        />
      )}
      <div style={{ flex: '1 1 auto', minWidth: 0 }}>
        <NavBar />
        <PageTransition>
          <Outlet />
        </PageTransition>
      </div>
    </div>
  )
}

const AppContent = () => {
  const { showNotification } = useNotification()

  const {
    needRefresh: [needRefresh, setNeedRefresh],
    updateServiceWorker,
  } = useRegisterSW({
    onRegistered(r) {
      console.log('SW Registered: ' + r)
      r &&
        setInterval(() => {
          r.update()
        }, intervalMS)
    },
    onRegisterError(error) {
      console.log('SW registration error', error)
    },
  })

  useEffect(() => {
    if (needRefresh) {
      showNotification({
        type: 'custom',
        component: (
          <div>
            <Typography level='body-md'>
              A new version is now available. Click on reload button to update.
            </Typography>
            <Button
              color='secondary'
              size='small'
              onClick={() => {
                updateServiceWorker(true)
                setNeedRefresh(false)
              }}
              sx={{ ml: 2 }}
            >
              Refresh
            </Button>
          </div>
        ),
        snackbarProps: {
          autoHideDuration: null, // Persistent until user action
        },
      })
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [needRefresh])

  return (
    <NavLayoutProvider>
      <ImpersonateUserProvider>
        <AppLayout />
      </ImpersonateUserProvider>
    </NavLayoutProvider>
  )
}

function App() {
  // startOpenReplay()

  const { mode, systemMode } = useColorScheme()

  const setThemeClass = useCallback(() => {
    const value = JSON.parse(localStorage.getItem('themeMode')) || mode

    if (value === 'system') {
      if (systemMode === 'dark') {
        return add('dark')
      }
      return remove('dark')
    }

    if (value === 'dark') {
      return add('dark')
    }

    return remove('dark')
  }, [mode, systemMode])

  useEffect(() => {
    setThemeClass()
  }, [setThemeClass])

  useEffect(() => {
    registerCapacitorListeners()
  }, [])

  return (
    <>
      <NetworkBanner />

      <AuthProvider>
        <SSEProvider>
          <AppContent />
        </SSEProvider>
      </AuthProvider>
    </>
  )
}

export default App
