import { Navigate, useLocation } from 'react-router';
import { useAuthStore } from '@/stores/authStore';
import { AppShellSkeleton } from '@/components/layout/AppShellSkeleton';

interface ProtectedRouteProps {
  children: React.ReactNode;
}

export function ProtectedRoute({ children }: ProtectedRouteProps) {
  const hasHydrated = useAuthStore((state) => state.hasHydrated);
  const isAuthenticated = useAuthStore((state) => state.isAuthenticated);
  const isAuthResolved = useAuthStore((state) => state.isAuthResolved);
  const location = useLocation();
  const shellVariant = location.pathname.startsWith('/consultation')
    ? 'consultation'
    : 'default';

  if (!hasHydrated) {
    return <AppShellSkeleton variant={shellVariant} />;
  }

  // Not authenticated at all — redirect immediately
  if (!isAuthenticated) {
    return <Navigate to="/login" state={{ from: location }} replace />;
  }

  if (!isAuthResolved) {
    return <AppShellSkeleton variant={shellVariant} />;
  }

  return <>{children}</>;
}
