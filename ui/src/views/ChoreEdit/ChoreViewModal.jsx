import { ArrowBack, Refresh } from '@mui/icons-material'
import { Box, CircularProgress, IconButton, Typography } from '@mui/joy'
import { useQueryClient } from '@tanstack/react-query'
import { useCallback, useEffect, useState, useSyncExternalStore } from 'react'
import { useResponsiveModal } from '../../hooks/useResponsiveModal'
import ChoreView from './ChoreView'

/**
 * Modal wrapper around ChoreView (the chore detail/action page), so it can be opened
 * on top of the task list instead of navigating to /chores/:choreId.
 *
 * Cheap to mount unconditionally: returns null whenever there is nothing to show, so a
 * parent can render `<ChoreViewModal choreId={selectedId} open={isOpen} onClose={...} />`
 * once and just flip `open`/`choreId` rather than conditionally mounting/unmounting it.
 */
const ChoreViewModal = ({ choreId, open, onClose, choreName }) => {
  const { ResponsiveModal, isMobile } = useResponsiveModal()

  // ChoreView sets document.title to the chore name. On the old standalone
  // page the next route overwrote it; as a modal nothing does, so the tab
  // would keep a closed task's name. Restore it when the modal goes away.
  useEffect(() => {
    if (!open) {
      return undefined
    }
    const previousTitle = document.title
    return () => {
      document.title = previousTitle
    }
  }, [open])
  const queryClient = useQueryClient()
  const [isRefreshing, setIsRefreshing] = useState(false)

  // useChoreDetails (called by ChoreView, our body) keys its cache entry off a string —
  // see the matching normalization in ChoreView.jsx. Match that here so cache reads and
  // invalidations below actually hit the same entry regardless of whether choreId comes
  // in as a number (e.g. chore.id from a list row) or a string.
  const normalizedChoreId = choreId != null ? String(choreId) : null

  // Read the chore's name out of the same React Query cache entry ChoreView populates
  // via useChoreDetails, instead of fetching it again here. Subscribing to the query
  // cache directly (rather than mounting a second useQuery observer) means this
  // component never triggers a fetch of its own - it only reads what ChoreView (or an
  // SSE cache write) has already put there - while still updating the header live.
  //
  // This goes through useSyncExternalStore rather than useState + an effect: the cache
  // emits synchronously while ChoreView's own observer is being set up, i.e. during a
  // child's render, and calling setState from there warns "Cannot update a component
  // while rendering a different component". useSyncExternalStore is built for exactly
  // this and schedules the update safely.
  const subscribeToChoreDetails = useCallback(
    onStoreChange => {
      if (!normalizedChoreId) {
        return () => {}
      }
      return queryClient.getQueryCache().subscribe(event => {
        const eventKey = event?.query?.queryKey
        if (
          eventKey?.[0] === 'choreDetails' &&
          eventKey?.[1] === normalizedChoreId
        ) {
          onStoreChange()
        }
      })
    },
    [queryClient, normalizedChoreId],
  )

  // getQueryData returns the same object reference until the entry actually changes,
  // so this is a stable snapshot and will not loop.
  const getChoreDetailsSnapshot = useCallback(
    () =>
      normalizedChoreId
        ? queryClient.getQueryData(['choreDetails', normalizedChoreId])
        : undefined,
    [queryClient, normalizedChoreId],
  )

  const cachedChore = useSyncExternalStore(
    subscribeToChoreDetails,
    getChoreDetailsSnapshot,
    getChoreDetailsSnapshot,
  )

  if (!open || choreId == null) {
    return null
  }

  const title = cachedChore?.res?.name || choreName || 'Task'

  const handleRefresh = async () => {
    setIsRefreshing(true)
    try {
      // Invalidate both keys the app uses for a single chore: 'choreDetails' is what
      // this modal's body (ChoreView) reads via useChoreDetails, 'chore' is what other
      // parts of the app (e.g. useChore) read - a caller opening this modal from a list
      // built on useChore should also see fresh data once it refetches.
      await queryClient.invalidateQueries({
        queryKey: ['choreDetails', normalizedChoreId],
      })
      await queryClient.invalidateQueries({
        queryKey: ['chore', normalizedChoreId],
      })
      // Belt-and-suspenders explicit refetch, mirroring the pattern useSSE.js uses for
      // chore.updated/chore.status: invalidateQueries alone should already refetch this
      // active query, but force it so the click visibly does something every time.
      await queryClient.refetchQueries({
        queryKey: ['choreDetails', normalizedChoreId],
      })
    } finally {
      setIsRefreshing(false)
    }
  }

  return (
    <ResponsiveModal
      open={open}
      onClose={onClose}
      size='sm'
      // Full-bleed only as a mobile bottom sheet. On desktop FadeModal's
      // fullWidth default (minWidth 90%) turns this into a full-width band
      // rather than a dialog - the detail view was a maxWidth='sm' page.
      fullWidth={isMobile}
      unmountDelay={250}
    >
      <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, mb: 2 }}>
        <IconButton
          aria-label='Back'
          variant='plain'
          color='neutral'
          size='sm'
          onClick={onClose}
        >
          <ArrowBack />
        </IconButton>
        <Typography level='title-lg' sx={{ flex: 1, fontWeight: 600 }} noWrap>
          {title}
        </Typography>
        <IconButton
          aria-label='Refresh task details'
          variant='plain'
          color='neutral'
          size='sm'
          onClick={handleRefresh}
          disabled={isRefreshing}
        >
          {isRefreshing ? <CircularProgress size='sm' /> : <Refresh />}
        </IconButton>
      </Box>
      <ChoreView choreId={choreId} isModal onClose={onClose} />
    </ResponsiveModal>
  )
}

export default ChoreViewModal
