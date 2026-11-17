import { useMemo } from 'react'
import { useCircleMembers, useUserProfile } from '../queries/UserQueries'

// The signed-in user's circle role: 'admin', 'manager' or 'member'.
// Members are the kids: the server rejects most write actions for them
// (create/edit/delete/archive/skip/reschedule/reassign, history edits,
// approvals, payouts, household settings), so the UI hides those controls.
//
// While the profile or member list is still loading, `role` is null and every
// flag is false, so restricted controls stay hidden rather than flashing up.
export const useCircleRole = () => {
  const { data: userProfile, isLoading: isProfileLoading } = useUserProfile()
  const { data: circleMembersData, isLoading: isMembersLoading } =
    useCircleMembers()

  return useMemo(() => {
    const members = circleMembersData?.res || []
    const role = members.find(m => m.userId === userProfile?.id)?.role || null
    return {
      role,
      isLoading: isProfileLoading || isMembersLoading,
      isAdmin: role === 'admin',
      // Admins and managers can approve, reject and manage chores.
      canManage: role === 'admin' || role === 'manager',
      isMember: role === 'member',
    }
  }, [circleMembersData, userProfile?.id, isProfileLoading, isMembersLoading])
}

export default useCircleRole
