import App from '@/App'
import ChoreEdit from '@/views/ChoreEdit/ChoreEdit'
import Error from '@/views/Error'
import AccountSettings from '@/views/Settings/AccountSettings'
import AdvancedSettings from '@/views/Settings/AdvancedSettings'
import ChildUserSettings from '@/views/Settings/ChildUserSettings'
import CircleSettings from '@/views/Settings/CircleSettings'
import DeveloperSettings from '@/views/Settings/DeveloperSettings'
import Settings from '@/views/Settings/Settings'
import SettingsOverview from '@/views/Settings/SettingsOverview'
import SettingsRoutes from '@/views/Settings/SettingsRoutes'
import ThemeSettings from '@/views/Settings/ThemeSettings'
import {
  Navigate,
  RouterProvider,
  createBrowserRouter,
  useParams,
} from 'react-router-dom'
import AuthenticationLoading from '../views/Authorization/Authenticating'
import ForgotPasswordView from '../views/Authorization/ForgotPasswordView'
import LoginSettings from '../views/Authorization/LoginSettings'
import LoginView from '../views/Authorization/LoginView'
import SignupView from '../views/Authorization/Signup'
import UpdatePasswordView from '../views/Authorization/UpdatePasswordView'
import ArchivedTasks from '../views/Chores/ArchivedTasks'
import MyChores from '../views/Chores/MyChores'
import JoinCircleView from '../views/Circles/JoinCircle'
import NotFound from '../views/components/NotFound'
import FilterView from '../views/Filters/FilterView'
import ChoreHistory from '../views/History/ChoreHistory'
import LabelView from '../views/Labels/LabelView'
import Landing from '../views/Landing/Landing'
import PaymentCancelledView from '../views/Payments/PaymentFailView'
import PaymentSuccessView from '../views/Payments/PaymentSuccessView'
import PrivacyPolicyView from '../views/PrivacyPolicy/PrivacyPolicyView'
import DashboardView from '../views/Dashboard/DashboardView'
import ProjectView from '../views/Projects/ProjectView'
import APITokenSettings from '../views/Settings/APITokenSettings'
import LocalizationSettings from '../views/Settings/LocalizationSettings'
import MFASettings from '../views/Settings/MFASettings'
import NotificationSetting from '../views/Settings/NotificationSetting'
import ProfileSettings from '../views/Settings/ProfileSettings'
import SidepanelSettings from '../views/Settings/SidepanelSettings'
import StorageSettings from '../views/Settings/StorageSettings'
import TermsView from '../views/Terms/TermsView'
import ThingsHistory from '../views/Things/ThingsHistory'
import ThingsView from '../views/Things/ThingsView'
import TimerDetails from '../views/Timer/TimerDetails'
import UserActivities from '../views/User/UserActivities'
import UserPoints from '../views/User/UserPoints'
import RewardsView from '../views/Rewards/RewardsView'
const getMainRoute = () => {
  if (
    // if domain is www.donetick.com or donetick.com  then show landing page:
    window.location.hostname === 'www.donetick.com' ||
    window.location.hostname === 'donetick.com'
  ) {
    return <Landing />
  }
  return <MyChores />
}

// The chore detail page is now a modal rendered from /chores. This redirects
// old/deep-linked /chores/:choreId URLs (e.g. from push notifications - see
// CapacitorListener.js) to /chores with an openChore search param that
// MyChores reads on mount to open the modal for that chore.
const ChoreDeepLinkRedirect = () => {
  const { choreId } = useParams()
  return <Navigate to={`/chores?openChore=${choreId}`} replace />
}

const Router = createBrowserRouter([
  {
    path: '/',
    element: <App />,
    errorElement: <Error />,
    children: [
      {
        path: '/',
        element: getMainRoute(),
      },
      {
        path: '/settings',
        element: <SettingsRoutes />,
        children: [
          {
            index: true,
            element: <SettingsOverview />,
          },
          {
            path: 'detailed',
            element: <Settings />,
          },
          {
            path: 'profile',
            element: <ProfileSettings />,
          },
          {
            path: 'circle',
            element: <CircleSettings />,
          },
          {
            path: 'account',
            element: <AccountSettings />,
          },
          {
            path: 'subaccounts',
            element: <ChildUserSettings />,
          },
          {
            path: 'notifications',
            element: <NotificationSetting />,
          },
          {
            path: 'mfa',
            element: <MFASettings />,
          },
          {
            path: 'apitokens',
            element: <APITokenSettings />,
          },
          {
            path: 'storage',
            element: <StorageSettings />,
          },
          {
            path: 'sidepanel',
            element: <SidepanelSettings />,
          },
          {
            path: 'theme',
            element: <ThemeSettings />,
          },
          {
            path: 'localization',
            element: <LocalizationSettings />,
          },
          {
            path: 'advanced',
            element: <AdvancedSettings />,
          },
          {
            path: 'developer',
            element: <DeveloperSettings />,
          },
        ],
      },
      {
        path: '/chores',
        element: <MyChores />,
      },
      {
        path: '/archived',
        element: <ArchivedTasks />,
      },
      {
        path: '/chores/:choreId/edit',
        element: <ChoreEdit />,
      },
      {
        path: '/chores/:choreId',
        element: <ChoreDeepLinkRedirect />,
      },
      {
        path: '/chores/create',
        element: <ChoreEdit />,
      },
      {
        path: '/chores/:choreId/history',
        element: <ChoreHistory />,
      },
      {
        path: '/chores/:choreId/timer',
        element: <TimerDetails />,
      },
      {
        path: '/my/chores',
        element: <MyChores />,
      },
      {
        path: '/activities',
        element: <UserActivities />,
      },
      {
        path: '/points',
        element: <UserPoints />,
      },
      {
        path: '/login',
        element: <LoginView />,
      },
      {
        path: '/login/settings',
        element: <LoginSettings />,
      },
      {
        path: '/signup',
        element: <SignupView />,
      },

      {
        path: '/auth/:provider',
        element: <AuthenticationLoading />,
      },
      {
        path: '/welcome',
        element: <Landing />,
      },
      {
        path: '/forgot-password',
        element: <ForgotPasswordView />,
      },
      {
        path: '/password/update',
        element: <UpdatePasswordView />,
      },
      {
        path: '/privacy',
        element: <PrivacyPolicyView />,
      },
      {
        path: '/terms',
        element: <TermsView />,
      },
      {
        path: 'circle/join',
        element: <JoinCircleView />,
      },
      {
        path: 'payments/success',
        element: <PaymentSuccessView />,
      },
      {
        path: 'payments/cancel',
        element: <PaymentCancelledView />,
      },
      {
        path: 'things',
        element: <ThingsView />,
      },
      {
        path: 'things/:id',
        element: <ThingsHistory />,
      },
      {
        path: 'labels/',
        element: <LabelView />,
      },
      {
        path: 'rewards/',
        element: <RewardsView />,
      },
      {
        path: 'dashboard',
        element: <DashboardView />,
      },
      {
        path: 'projects/',
        element: <ProjectView />,
      },
      {
        path: 'filters/',
        element: <FilterView />,
      },
      {
        path: '*',
        element: <NotFound />,
      },
    ],
  },
])

const RouterContext = () => {
  return <RouterProvider router={Router} />
}

export default RouterContext
