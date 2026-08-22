import { useMediaQuery } from '@mui/material'
import { createContext, useContext, useMemo } from 'react'
import useStickyState from '../hooks/useStickyState'

const NavLayoutContext = createContext()

// Whether NavBar's sidebar is rendered as a persistent, space-reserving
// Sheet (true) or an overlay Drawer (false): pinned AND wide-viewport only,
// narrow viewports always fall back to the overlay regardless of the pin
// setting. NavBar needs this to decide what to render, and App.jsx needs it
// to reserve layout space for the sidebar, so it's lifted here instead of
// each computing it independently -- two separate
// useStickyState('navDrawerPinned') calls would each own their own React
// state, so toggling the pin from inside NavBar wouldn't re-render App.jsx's
// layout until the next reload.
export const NavLayoutProvider = ({ children }) => {
  const [navPinned, setNavPinned] = useStickyState(true, 'navDrawerPinned')
  const isWideViewport = useMediaQuery(theme => theme.breakpoints.up('md'))
  const isPersistent = navPinned && isWideViewport

  const value = useMemo(
    () => ({ isPersistent, isWideViewport, navPinned, setNavPinned }),
    [isPersistent, isWideViewport, navPinned, setNavPinned],
  )

  return (
    <NavLayoutContext.Provider value={value}>
      {children}
    </NavLayoutContext.Provider>
  )
}

export const useNavLayout = () => {
  const context = useContext(NavLayoutContext)
  if (!context) {
    throw new Error('useNavLayout must be used within NavLayoutProvider')
  }
  return context
}
