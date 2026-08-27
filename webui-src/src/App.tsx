import { useEffect } from 'react';
import { HashRouter, Redirect, useRouter } from '@/lib/router';
import { useAuthStore } from '@/stores/authStore';
import { MainLayout } from '@/components/layout/MainLayout';
import { Login } from '@/app/pages/Login';
import { Setup } from '@/app/pages/Setup';
import { Dashboard } from '@/app/pages/Dashboard';
import { AccountList } from '@/app/pages/AccountList';
import { AccountDetail } from '@/app/pages/AccountDetail';
import { SettingsPage } from '@/app/pages/Settings';
import { Loader2 } from 'lucide-react';
import { createContext, useContext } from 'react';

function AuthGate({ children, requireAuth }: { children: React.ReactNode; requireAuth: boolean }) {
  const { isAuthenticated, isLoading, initialized } = useAuthStore();

  if (isLoading) {
    return (
      <div className="min-h-screen flex items-center justify-center">
        <Loader2 className="w-8 h-8 text-blue-400 animate-spin" />
      </div>
    );
  }

  if (!initialized) {
    return <Redirect to="/setup" />;
  }

  if (requireAuth && !isAuthenticated) {
    return <Redirect to="/login" />;
  }

  if (!requireAuth && isAuthenticated) {
    return <Redirect to="/" />;
  }

  return <>{children}</>;
}

// Params context for passing route params to child components
const ParamsContext = createContext<Record<string, string>>({});
export function useParams() {
  return useContext(ParamsContext);
}

function matchAndExtract(pattern: string, path: string): Record<string, string> | null {
  const patternParts = pattern.split('/').filter(Boolean);
  const pathParts = path.split('/').filter(Boolean);
  if (patternParts.length !== pathParts.length) return null;
  const params: Record<string, string> = {};
  for (let i = 0; i < patternParts.length; i++) {
    if (patternParts[i].startsWith(':')) {
      params[patternParts[i].slice(1)] = decodeURIComponent(pathParts[i]);
    } else if (patternParts[i] !== pathParts[i]) {
      return null;
    }
  }
  return params;
}

function AppRoutes() {
  const { path } = useRouter();

  let params = matchAndExtract('/accounts/:id', path);
  if (params) {
    return (
      <ParamsContext.Provider value={params}>
        <AuthGate requireAuth>
          <MainLayout>
            <AccountDetail />
          </MainLayout>
        </AuthGate>
      </ParamsContext.Provider>
    );
  }

  if (matchAndExtract('/login', path)) {
    return (
      <AuthGate requireAuth={false}>
        <Login />
      </AuthGate>
    );
  }

  if (matchAndExtract('/setup', path)) {
    return (
      <AuthGate requireAuth={false}>
        <Setup />
      </AuthGate>
    );
  }

  if (matchAndExtract('/accounts', path)) {
    return (
      <AuthGate requireAuth>
        <MainLayout>
          <AccountList />
        </MainLayout>
      </AuthGate>
    );
  }

  if (matchAndExtract('/settings', path)) {
    return (
      <AuthGate requireAuth>
        <MainLayout>
          <SettingsPage />
        </MainLayout>
      </AuthGate>
    );
  }

  return (
    <AuthGate requireAuth>
      <MainLayout>
        <Dashboard />
      </MainLayout>
    </AuthGate>
  );
}

export default function App() {
  const { checkAuth } = useAuthStore();

  useEffect(() => {
    checkAuth();
    const handleAuthExpired = () => {
      window.location.hash = '#/login';
    };
    window.addEventListener('auth-expired', handleAuthExpired);
    return () => window.removeEventListener('auth-expired', handleAuthExpired);
  }, [checkAuth]);

  return (
    <HashRouter>
      <AppRoutes />
    </HashRouter>
  );
}
