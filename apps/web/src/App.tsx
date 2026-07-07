import { Suspense, lazy } from 'react';
import { BrowserRouter, Routes, Route, Navigate } from 'react-router';
import { QueryClientProvider } from '@tanstack/react-query';
import { AuthBootstrap } from './components/AuthBootstrap';
import { LoginPage } from './pages/LoginPage';
import { RegisterPage } from './pages/RegisterPage';
import { ProtectedRoute } from './components/ProtectedRoute';
import { AppShellSkeleton } from './components/layout/AppShellSkeleton';
import { Toaster } from './components/ui/sonner';
import { queryClient } from './lib/queryClient';

const DashboardPage = lazy(() =>
  import('./pages/DashboardPage').then((module) => ({
    default: module.DashboardPage,
  })),
);
const OnboardingPage = lazy(() =>
  import('./features/profile/pages/OnboardingPage').then((module) => ({
    default: module.OnboardingPage,
  })),
);
const ProfilePage = lazy(() =>
  import('./features/profile/pages/ProfilePage').then((module) => ({
    default: module.ProfilePage,
  })),
);
const ConsultationPage = lazy(() =>
  import('./features/consultation/pages/ConsultationPage').then((module) => ({
    default: module.ConsultationPage,
  })),
);
const SharePage = lazy(() =>
  import('./features/consultation/components/SharePage').then((module) => ({
    default: module.SharePage,
  })),
);
const AssessmentListPage = lazy(() =>
  import('./features/assessment/pages/AssessmentListPage').then((module) => ({
    default: module.AssessmentListPage,
  })),
);
const AssessmentDetailPage = lazy(() =>
  import('./features/assessment/pages/AssessmentDetailPage').then((module) => ({
    default: module.AssessmentDetailPage,
  })),
);
const HistoryPage = lazy(() =>
  import('./features/history/pages/HistoryPage').then((module) => ({
    default: module.HistoryPage,
  })),
);
const TrainingPage = lazy(() =>
  import('./features/training/pages/TrainingPage').then((module) => ({
    default: module.TrainingPage,
  })),
);

function RouteFallback({ variant = 'default' }: { variant?: 'default' | 'consultation' }) {
  return <AppShellSkeleton variant={variant} />;
}

function ProtectedRouteElement({
  children,
  variant = 'default',
}: {
  children: React.ReactNode;
  variant?: 'default' | 'consultation';
}) {
  return (
    <ProtectedRoute>
      <Suspense fallback={<RouteFallback variant={variant} />}>{children}</Suspense>
    </ProtectedRoute>
  );
}

export function App() {
  return (
    <QueryClientProvider client={queryClient}>
      <BrowserRouter>
        <AuthBootstrap />
        <Toaster />
        <Routes>
          {/* Public routes */}
          <Route path="/login" element={<LoginPage />} />
          <Route path="/register" element={<RegisterPage />} />
          <Route
            path="/consultation/share/:token"
            element={
              <Suspense fallback={<RouteFallback variant="consultation" />}>
                <SharePage />
              </Suspense>
            }
          />

          {/* Protected routes */}
          <Route
            path="/dashboard"
            element={
              <ProtectedRouteElement>
                <DashboardPage />
              </ProtectedRouteElement>
            }
          />
          <Route
            path="/onboarding"
            element={
              <ProtectedRouteElement>
                <OnboardingPage />
              </ProtectedRouteElement>
            }
          />
          <Route
            path="/profile"
            element={
              <ProtectedRouteElement>
                <ProfilePage />
              </ProtectedRouteElement>
            }
          />
          <Route
            path="/consultation"
            element={
              <ProtectedRouteElement variant="consultation">
                <ConsultationPage />
              </ProtectedRouteElement>
            }
          />
          <Route
            path="/consultation/:id"
            element={
              <ProtectedRouteElement variant="consultation">
                <ConsultationPage />
              </ProtectedRouteElement>
            }
          />
          <Route
            path="/assessment"
            element={
              <ProtectedRouteElement>
                <AssessmentListPage />
              </ProtectedRouteElement>
            }
          />
          <Route
            path="/assessment/:id"
            element={
              <ProtectedRouteElement>
                <AssessmentDetailPage />
              </ProtectedRouteElement>
            }
          />
          <Route
            path="/history"
            element={
              <ProtectedRouteElement>
                <HistoryPage />
              </ProtectedRouteElement>
            }
          />
          <Route
            path="/training/:id"
            element={
              <ProtectedRouteElement>
                <TrainingPage />
              </ProtectedRouteElement>
            }
          />

          {/* Redirect root to dashboard */}
          <Route path="/" element={<Navigate to="/dashboard" replace />} />
        </Routes>
      </BrowserRouter>
    </QueryClientProvider>
  );
}
