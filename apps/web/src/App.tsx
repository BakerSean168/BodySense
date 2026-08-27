import { Suspense, lazy, useEffect } from "react";
import { BrowserRouter, Routes, Route, Navigate } from "react-router";
import { QueryClientProvider } from "@tanstack/react-query";
import { AuthBootstrap } from "./components/AuthBootstrap";
import { LoginPage } from "./pages/LoginPage";
import { RegisterPage } from "./pages/RegisterPage";
import { ProtectedRoute } from "./components/ProtectedRoute";
import { WorkbenchProfileGate } from "./components/WorkbenchProfileGate";
import { AppShellSkeleton } from "./components/layout/AppShellSkeleton";
import { RouteErrorBoundary } from "./components/errors/RouteErrorBoundary";
import { Toaster } from "./components/ui/sonner";
import { queryClient } from "./lib/queryClient";
import { ThemeProvider } from "./components/theme/ThemeProvider";

const OnboardingPage = lazy(() =>
  import("./features/profile/pages/OnboardingPage").then((module) => ({
    default: module.OnboardingPage,
  })),
);
const loadConsultationPage = () =>
  import("./features/consultation/pages/ConsultationPage").then((module) => ({
    default: module.ConsultationPage,
  }));
const ConsultationPage = lazy(loadConsultationPage);
const SharePage = lazy(() =>
  import("./features/consultation/components/SharePage").then((module) => ({
    default: module.SharePage,
  })),
);

function RouteFallback({
  variant = "default",
}: {
  variant?: "default" | "consultation";
}) {
  return <AppShellSkeleton variant={variant} />;
}

function ProtectedRouteElement({
  children,
  variant = "default",
}: {
  children: React.ReactNode;
  variant?: "default" | "consultation";
}) {
  return (
    <ProtectedRoute>
      <RouteErrorBoundary variant={variant}>
        <Suspense fallback={<RouteFallback variant={variant} />}>
          {children}
        </Suspense>
      </RouteErrorBoundary>
    </ProtectedRoute>
  );
}

function WorkbenchRouteElement() {
  // The workbench is the primary product surface. Start its route chunk while
  // auth/profile bootstrap is still running so high-latency tailnet access does
  // not turn auth -> profile -> route into a serial download waterfall.
  useEffect(() => {
    void loadConsultationPage();
  }, []);

  return (
    <ProtectedRouteElement variant="consultation">
      <WorkbenchProfileGate>
        <ConsultationPage />
      </WorkbenchProfileGate>
    </ProtectedRouteElement>
  );
}

export function App() {
  return (
    <ThemeProvider
      attribute="class"
      defaultTheme="light"
      forcedTheme="light"
      enableSystem={false}
      disableTransitionOnChange
    >
      <QueryClientProvider client={queryClient}>
        <BrowserRouter>
          <AuthBootstrap />
          <Toaster />
          <Routes>
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

            <Route
              path="/onboarding"
              element={
                <ProtectedRouteElement>
                  <OnboardingPage />
                </ProtectedRouteElement>
              }
            />
            <Route path="/consultation" element={<WorkbenchRouteElement />} />
            <Route path="/consultation/:id" element={<WorkbenchRouteElement />} />

            {/* Legacy product pages were retired by the single-workbench IA. */}
            <Route path="/dashboard" element={<Navigate to="/consultation" replace />} />
            <Route path="/profile" element={<Navigate to="/consultation?view=state" replace />} />
            <Route path="/assessment" element={<Navigate to="/consultation?view=state" replace />} />
            <Route path="/assessment/:id" element={<Navigate to="/consultation?view=state" replace />} />
            <Route path="/history" element={<Navigate to="/consultation" replace />} />
            <Route path="/training/:id" element={<Navigate to="/consultation?view=treatment" replace />} />

            <Route path="/" element={<Navigate to="/consultation" replace />} />
            <Route path="*" element={<Navigate to="/consultation" replace />} />
          </Routes>
        </BrowserRouter>
      </QueryClientProvider>
    </ThemeProvider>
  );
}
