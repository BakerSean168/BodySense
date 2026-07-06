import { useEffect, useState } from 'react';
import { Navigate, useLocation } from 'react-router';
import { useAuthStore } from '@/stores/authStore';
import { authFetch } from '@/features/auth/services/authService';

interface ProtectedRouteProps {
  children: React.ReactNode;
}

/**
 * ProtectedRoute validates authentication on mount by calling GET /api/v1/me.
 *
 * This catches stale tokens (e.g. DB cleared while browser still has a valid JWT).
 * Without this check, a stale token would pass the boolean check and the user
 * would enter the app, only to fail on the first write operation.
 *
 * Verification states:
 *   null  — still checking
 *   true  — token valid, user exists
 *   false — token invalid or network error
 */
export function ProtectedRoute({ children }: ProtectedRouteProps) {
  const { isAuthenticated, logout } = useAuthStore();
  const location = useLocation();
  // null = still verifying, true = valid, false = invalid or unreachable
  const [verified, setVerified] = useState<boolean | null>(null);

  useEffect(() => {
    if (!isAuthenticated) {
      setVerified(false);
      return;
    }

    let cancelled = false;

    authFetch('/api/v1/me')
      .then((res) => {
        if (cancelled) return;

        if (res.ok) {
          setVerified(true);
        } else {
          // Token invalid or user doesn't exist — clean up
          logout();
          setVerified(false);
        }
      })
      .catch(() => {
        if (cancelled) return;
        // Network error — can't verify, show error state
        setVerified(false);
      });

    return () => {
      cancelled = true;
    };
  }, [isAuthenticated, logout]);

  // Not authenticated at all — redirect immediately
  if (!isAuthenticated) {
    return <Navigate to="/login" state={{ from: location }} replace />;
  }

  // Still verifying — blank screen (brief, <100ms typically)
  if (verified === null) {
    return null;
  }

  // Verification failed (token invalid, user deleted, or network error)
  if (!verified) {
    return <Navigate to="/login" state={{ from: location }} replace />;
  }

  return <>{children}</>;
}
