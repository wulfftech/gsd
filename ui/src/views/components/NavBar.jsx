import { Capacitor } from '@capacitor/core'
import {
  Archive,
  ArrowBack,
  CardGiftcard,
  Dashboard,
  FilterAlt,
  FolderOpen,
  History,
  Inbox,
  ListAlt,
  Logout,
  MenuRounded,
  PushPin,
  PushPinOutlined,
  SettingsOutlined,
  Toll,
  Widgets,
} from '@mui/icons-material'
import {
  Badge,
  Box,
  Chip,
  Drawer,
  IconButton,
  List,
  ListItemButton,
  ListItemContent,
  ListItemDecorator,
  Sheet,
  Typography,
} from '@mui/joy'
import { useMediaQuery } from '@mui/material'

import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { useLocation, useNavigate, useSearchParams } from 'react-router-dom'
import { useProjects } from '../Projects/ProjectQueries'
import { useCircleMembers, useUserProfile } from '../../queries/UserQueries'
import { version } from '../../../package.json'
import UserProfileAvatar from '../../components/UserProfileAvatar'
import { useLocalization } from '../../contexts/LocalizationContext'
import useStickyState from '../../hooks/useStickyState'
import NavBarLink from './NavBarLink'

import { SafeArea } from 'capacitor-plugin-safe-area'
import Z_INDEX from '../../constants/zIndex'
import { useResource } from '../../queries/ResourceQueries'
import { apiClient } from '../../utils/ApiClient'

const publicPages = ['/landing', '/privacy', '/terms']

// Matches Joy Drawer's 'sm' horizontal size so the pinned sidebar lines up
// visually with the overlay drawer it replaces.
const PINNED_DRAWER_WIDTH = 256

// Nav links + logout/version footer, shared between the overlay Drawer
// (mobile / unpinned) and the persistent sidebar (pinned + md-and-up), so the
// list markup isn't duplicated between the two render paths.
const NavContent = ({
  links,
  t,
  version,
  resource,
  showPinControl,
  navPinned,
  onTogglePin,
  onItemClick,
  onLogout,
}) => (
  <>
    {showPinControl && (
      <Box sx={{ display: 'flex', justifyContent: 'flex-end', px: 1, pt: 1 }}>
        <IconButton
          size='sm'
          variant='plain'
          aria-label={
            navPinned
              ? t('navigation.unpinMenu', 'Unpin menu')
              : t('navigation.pinMenu', 'Pin menu')
          }
          onClick={onTogglePin}
        >
          {navPinned ? (
            <PushPin fontSize='small' />
          ) : (
            <PushPinOutlined fontSize='small' />
          )}
        </IconButton>
      </Box>
    )}
    <div>
      <List
        size='md'
        onClick={onItemClick}
        sx={{
          borderRadius: 4,
          width: '100%',
          padding: 1,
          paddingTop:
            Capacitor.getPlatform() === 'android'
              ? `calc(var(--safe-area-inset-top, 0px))`
              : '',
        }}
      >
        {links.map((link, index) => (
          <NavBarLink key={index} link={link} />
        ))}
      </List>
    </div>
    <div>
      <List
        sx={{
          p: 2,
          height: 'min-content',
          position: 'absolute',
          bottom: 0,
          borderRadius: 4,
          width: '100%',
          padding: 2,
        }}
        size='md'
        onClick={onItemClick}
      >
        <ListItemButton
          onClick={onLogout}
          sx={{
            py: 1.2,
          }}
        >
          <ListItemDecorator>
            <Logout />
          </ListItemDecorator>
          <ListItemContent>{t('logout')}</ListItemContent>
        </ListItemButton>
        <Typography
          onClick={
            // force service worker to update:
            () => window.location.reload(true)
          }
          level='body-xs'
          sx={{
            p: 1,
            color: 'text.tertiary',
            textAlign: 'center',
            mb: 'calc(var(--safe-area-inset-bottom, 0px) )',
          }}
        >
          V{version} (API: {resource?.api_version || 'unavailable'})
        </Typography>
      </List>
    </div>
  </>
)

const NavBar = () => {
  const { t } = useTranslation('common')
  const { isRTL } = useLocalization()
  const { data: resource } = useResource()

  const navigate = useNavigate()
  const location = useLocation()
  const [drawerOpen, setDrawerOpen] = useState(false)
  const isWideViewport = useMediaQuery(theme => theme.breakpoints.up('md'))
  const [navPinned, setNavPinned] = useStickyState(true, 'navDrawerPinned')
  // Persistent sidebar only makes sense once pinned AND there's room for it;
  // narrow viewports always fall back to the overlay drawer regardless of
  // the pin setting.
  const isPersistent = navPinned && isWideViewport

  // Pending-approval alert for admins — only query when authenticated
  const { data: userProfile } = useUserProfile()
  const { data: circleMembersData } = useCircleMembers()
  const isPublicPage =
    publicPages.includes(location.pathname) ||
    window.location.hostname === 'www.donetick.com' ||
    window.location.hostname === 'donetick.com'
  const { data: projects = [] } = useProjects({ enabled: !isPublicPage && !!userProfile })
  const circleMembers = circleMembersData?.res || []
  const isAdmin = circleMembers.find(m => m.userId === userProfile?.id)?.role === 'admin'
  const pendingApprovalCount = isAdmin
    ? projects.reduce(
        (sum, p) =>
          sum + (p.tasks || []).filter(t => t.status === 1).length,
        0,
      )
    : 0
  
  const links = [
    {
      to: '/dashboard',
      label: 'Dashboard',
      icon: <Dashboard />,
    },
    {
      to: '/chores',
      label: t('navigation.allTasks'),
      icon: <Inbox />,
    },
    {
      to: '/archived',
      label: t('navigation.archived'),
      icon: <Archive />,
    },
    {
      to: '/things',
      label: t('navigation.things'),
      icon: <Widgets />,
    },
    {
      to: 'labels',
      label: t('navigation.labels'),
      icon: <ListAlt />,
    },
    {
      to: 'projects',
      label: t('navigation.projects'),
      icon: <FolderOpen />,
    },
    {
      to: 'filters',
      label: t('navigation.filters'),
      icon: <FilterAlt />,
    },
    {
      to: 'activities',
      label: t('navigation.activities'),
      icon: <History />,
    },
    {
      to: 'points',
      label: t('navigation.points'),
      icon: <Toll />,
    },
    {
      to: 'rewards',
      label: t('navigation.rewards', 'Rewards'),
      icon: <CardGiftcard />,
    },
    {
      to: '/settings',
      label: t('navigation.settings'),
      icon: <SettingsOutlined />,
    },
  ]
  const [openDrawer, closeDrawer] = [
    () => setDrawerOpen(true),
    () => setDrawerOpen(false),
  ]
  const handleTogglePin = e => {
    // Stop this from bubbling to the Drawer's own onClick={closeDrawer} in
    // overlay mode, which would close the menu right after toggling pin.
    e.stopPropagation()
    setNavPinned(prev => !prev)
  }
  const [searchParams] = useSearchParams()
  useEffect(() => {
    SafeArea.getSafeAreaInsets().then(data => {
      const { insets } = data
      const drawerContent = document.querySelector('.drawer-content')
      if (drawerContent) {
        drawerContent.style.paddingTop = `${insets.top}px`
        drawerContent.style.paddingRight = `${insets.right}px`
        drawerContent.style.paddingBottom = `${insets.bottom}px`
        drawerContent.style.paddingLeft = `${insets.left}px`
      }
    })
  }, [])

  // When the sidebar is pinned+persistent, inset the app content so the
  // sidebar doesn't sit on top of it. NavBar doesn't own the layout
  // wrapper around <Outlet /> (see App.jsx), so this nudges the app root
  // directly rather than reformatting that file's flow layout — the same
  // imperative-style-on-a-shared-node approach the SafeArea effect above
  // already uses in this component.
  useEffect(() => {
    const root = document.getElementById('root')
    if (!root) {
      return undefined
    }
    if (isPersistent) {
      root.style[isRTL ? 'paddingRight' : 'paddingLeft'] =
        `${PINNED_DRAWER_WIDTH}px`
      root.style[isRTL ? 'paddingLeft' : 'paddingRight'] = ''
    } else {
      root.style.paddingLeft = ''
      root.style.paddingRight = ''
    }
    return () => {
      root.style.paddingLeft = ''
      root.style.paddingRight = ''
    }
  }, [isPersistent, isRTL])

  // Avoid a stray overlay drawer appearing (e.g. from an earlier hamburger
  // click) once we switch into persistent sidebar mode.
  useEffect(() => {
    if (isPersistent) {
      setDrawerOpen(false)
    }
  }, [isPersistent])

  const getMenuIcon = () => {
    // With the sidebar pinned open there is nothing for the hamburger to do,
    // so drop it rather than leaving a visible control that no-ops. Unpinning
    // is done from the pin button inside the sidebar itself.
    const menuRounded = isPersistent ? null : (
      <IconButton
        size='md'
        variant='plain'
        aria-label={t('navigation.openMenu', 'Open menu')}
        onClick={() => setDrawerOpen(true)}
      >
        <MenuRounded />
      </IconButton>
    )
    if (!Capacitor.isNativePlatform()) {
      return menuRounded
    }
    if (
      ['/chores', '/'].includes(location.pathname) &&
      !searchParams.get('filterId')
    ) {
      return menuRounded
    }
    return (
      <IconButton
        size='md'
        variant='plain'
        onClick={() => {
          if (location.pathname === '/chores') {
            // Navigate back to calendar view
            navigate('/')
          } else {
            // Default back navigation
            navigate(-1)
          }
        }}
        title={
          searchParams.get('from') === 'calendar' ? t('backToCalendar') : t('back')
        }
      >
        <ArrowBack />
      </IconButton>
    )
  }

  if (
    [
      '/signup',
      '/login',
      '/auth/oauth2',
      '/forgot-password',
      '/password/update',
      '/login/settings',
      '/welcome',
    ].includes(location.pathname)
  ) {
    return (
      // no navbar but show the safe area padding
      <div
        style={{
          paddingTop: `calc(var(--safe-area-inset-top, 0px))`,
          top: 0,
        }}
      />
    )
  }
  // if url has /landing then remove the navbar:
  if (publicPages.includes(location.pathname)) {
    return null
  }
  if (
    window.location.hostname === 'www.donetick.com' ||
    window.location.hostname === 'donetick.com'
  ) {
    return null
  }

  return (
    <nav
      className='flex gap-2 p-3'
      style={{
        paddingTop:
          Capacitor.getPlatform() === 'android'
            ? `calc(var(--safe-area-inset-top, 0px))`
            : '',
        position: 'sticky',
        zIndex: Z_INDEX.NAVBAR,
        top: 0,
        minHeight: '35px',
        backgroundColor: 'var(--joy-palette-background-body)',
      }}
    >
      {getMenuIcon()}
      <Box className='flex-1' />
      <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
        {pendingApprovalCount > 0 && (
          <Chip
            size='sm'
            color='warning'
            variant='solid'
            onClick={() => navigate('/projects')}
            sx={{ cursor: 'pointer', fontWeight: 600, fontSize: 11 }}
          >
            {pendingApprovalCount} to approve
          </Chip>
        )}
        <UserProfileAvatar />
        {/* <ThemeToggleButton /> */}
      </Box>
      {isPersistent ? (
        // Pinned + wide viewport: render the nav list as a persistent
        // sidebar instead of the overlay Drawer. Joy's Drawer has no
        // "permanent" variant, so this is a plain fixed-position Sheet with
        // no backdrop and no onClick-to-close — links navigate without
        // dismissing it.
        <Sheet
          variant='plain'
          sx={{
            position: 'fixed',
            top: 0,
            ...(isRTL ? { right: 0 } : { left: 0 }),
            height: '100%',
            width: PINNED_DRAWER_WIDTH,
            display: 'flex',
            flexDirection: 'column',
            overflow: 'auto',
            zIndex: Z_INDEX.DRAWER,
            backgroundColor: 'background.surface',
            borderRight: isRTL ? 'none' : '1px solid',
            borderLeft: isRTL ? '1px solid' : 'none',
            borderColor: 'divider',
          }}
        >
          <NavContent
            links={links}
            t={t}
            version={version}
            resource={resource}
            showPinControl={isWideViewport}
            navPinned={navPinned}
            onTogglePin={handleTogglePin}
            onLogout={() => apiClient.handleLogout()}
          />
        </Sheet>
      ) : (
        <Drawer
          open={drawerOpen}
          onClose={closeDrawer}
          anchor={isRTL ? 'right' : 'left'}
          size='sm'
          onClick={closeDrawer}
          sx={{
            '& .MuiDrawer-content': {
              position: 'fixed',
              // pt: 'calc(var(--safe-area-inset-top, 0px))',
              ...(isRTL ? { right: 0 } : { left: 0 }),
              // pb: 'calc(var(--safe-area-inset-bottom, 0px))',
              // height:
              //   'calc(100vh - var(--safe-area-inset-top, 0px) - var(--safe-area-inset-bottom, 0px))',
              overflow: 'auto',
              zIndex: Z_INDEX.DRAWER,
            },
          }}
        >
          <NavContent
            links={links}
            t={t}
            version={version}
            resource={resource}
            showPinControl={isWideViewport}
            navPinned={navPinned}
            onTogglePin={handleTogglePin}
            onItemClick={openDrawer}
            onLogout={() => apiClient.handleLogout()}
          />
        </Drawer>
      )}
    </nav>
  )
}

export default NavBar
