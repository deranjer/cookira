import { Center, Loader } from '@mantine/core'
import type { ReactNode } from 'react'
import { Navigate, Outlet, Route, Routes, useLocation } from 'react-router-dom'
import { useMe, useSetupStatus } from './lib/api/hooks'
import { AppShell } from './components/AppShell'
import { Login } from './pages/Login'
import { Setup } from './pages/Setup'
import { RecipesView } from './pages/RecipesView'
import { RecipeDetail } from './pages/RecipeDetail'
import { RecipeEditor } from './pages/RecipeEditor'
import { SettingsView } from './pages/SettingsView'
import { OAuthAuthorize } from './pages/OAuthAuthorize'
import { SharedRecipe } from './pages/SharedRecipe'

function Spinner() {
  return (
    <Center h="100vh">
      <Loader color="orange" />
    </Center>
  )
}

function RequireAuth() {
  const location = useLocation()
  const { data: user, isLoading, isError } = useMe()
  if (isLoading) return <Spinner />
  if (isError || !user) {
    // Remember where we were headed (e.g. the OAuth consent screen) so login can return there.
    const next = location.pathname + location.search
    return <Navigate to={next === '/' ? '/login' : `/login?next=${encodeURIComponent(next)}`} replace />
  }
  return <Outlet />
}

// Before any user exists, /setup is the only reachable route; afterwards the
// backend permanently rejects it, so redirect away from /setup too.
function RequireSetup({ children }: { children: ReactNode }) {
  const location = useLocation()
  const { data, isLoading, isError } = useSetupStatus()
  if (isLoading) return <Spinner />
  const setupRequired = !isError && data?.required === true
  if (setupRequired && location.pathname !== '/setup') return <Navigate to="/setup" replace />
  if (!setupRequired && location.pathname === '/setup') return <Navigate to="/login" replace />
  return <>{children}</>
}

function App() {
  return (
    <Routes>
      {/* Share links work without login or setup state. */}
      <Route path="/s/:token" element={<SharedRecipe />} />
      <Route
        path="*"
        element={
          <RequireSetup>
            <Routes>
              <Route path="/setup" element={<Setup />} />
              <Route path="/login" element={<Login />} />
              <Route element={<RequireAuth />}>
                <Route path="/oauth/authorize" element={<OAuthAuthorize />} />
                <Route element={<AppShell />}>
                  <Route index element={<RecipesView />} />
                  <Route path="/recipes/new" element={<RecipeEditor />} />
                  <Route path="/recipes/:ref" element={<RecipeDetail />} />
                  <Route path="/recipes/:ref/edit" element={<RecipeEditor />} />
                  <Route path="/settings" element={<SettingsView />} />
                </Route>
              </Route>
              <Route path="*" element={<Navigate to="/" replace />} />
            </Routes>
          </RequireSetup>
        }
      />
    </Routes>
  )
}

export default App
