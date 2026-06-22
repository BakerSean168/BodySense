import { BrowserRouter, Routes, Route, Navigate } from 'react-router';
import { LoginPage } from './pages/LoginPage';
import { RegisterPage } from './pages/RegisterPage';
import { DashboardPage } from './pages/DashboardPage';
import { ProtectedRoute } from './components/ProtectedRoute';
import { OnboardingPage } from './features/profile/pages/OnboardingPage';
import { ProfilePage } from './features/profile/pages/ProfilePage';
import { ConsultationPage } from './features/consultation/pages/ConsultationPage';
import { AssessmentListPage } from './features/assessment/pages/AssessmentListPage';
import { AssessmentDetailPage } from './features/assessment/pages/AssessmentDetailPage';

export function App() {
  return (
    <BrowserRouter>
      <Routes>
        {/* Public routes */}
        <Route path="/login" element={<LoginPage />} />
        <Route path="/register" element={<RegisterPage />} />

        {/* Protected routes */}
        <Route
          path="/dashboard"
          element={
            <ProtectedRoute>
              <DashboardPage />
            </ProtectedRoute>
          }
        />
        <Route
          path="/onboarding"
          element={
            <ProtectedRoute>
              <OnboardingPage />
            </ProtectedRoute>
          }
        />
        <Route
          path="/profile"
          element={
            <ProtectedRoute>
              <ProfilePage />
            </ProtectedRoute>
          }
        />
        <Route
          path="/consultation"
          element={
            <ProtectedRoute>
              <ConsultationPage />
            </ProtectedRoute>
          }
        />
        <Route
          path="/consultation/:id"
          element={
            <ProtectedRoute>
              <ConsultationPage />
            </ProtectedRoute>
          }
        />
        <Route
          path="/assessment"
          element={
            <ProtectedRoute>
              <AssessmentListPage />
            </ProtectedRoute>
          }
        />
        <Route
          path="/assessment/:id"
          element={
            <ProtectedRoute>
              <AssessmentDetailPage />
            </ProtectedRoute>
          }
        />

        {/* Redirect root to dashboard */}
        <Route path="/" element={<Navigate to="/dashboard" replace />} />
      </Routes>
    </BrowserRouter>
  );
}
