import { describe, expect, it, afterEach } from 'vitest';
import { act, render, screen } from '@testing-library/react';
import { MemoryRouter, Route, Routes } from 'react-router';
import { ProtectedRoute } from '../ProtectedRoute';
import { useAuthStore } from '@/stores/authStore';

function renderProtectedRoute(path = '/consultation') {
  return render(
    <MemoryRouter initialEntries={[path]}>
      <Routes>
        <Route path="/login" element={<div>登录页</div>} />
        <Route
          path="/consultation"
          element={
            <ProtectedRoute>
              <div>受保护内容</div>
            </ProtectedRoute>
          }
        />
        <Route
          path="/dashboard"
          element={
            <ProtectedRoute>
              <div>控制台内容</div>
            </ProtectedRoute>
          }
        />
      </Routes>
    </MemoryRouter>,
  );
}

describe('ProtectedRoute', () => {
  afterEach(() => {
    act(() => {
      useAuthStore.setState({
        user: null,
        accessToken: null,
        refreshToken: null,
        isAuthenticated: false,
        hasHydrated: false,
        isAuthResolved: false,
        isVerifyingSession: false,
        isLoading: false,
        error: null,
      });
    });
  });

  it('renders shell skeleton before auth store hydration completes', () => {
    act(() => {
      useAuthStore.setState({
        hasHydrated: false,
        isAuthenticated: false,
        isAuthResolved: false,
      });
    });

    renderProtectedRoute('/dashboard');

    expect(screen.getByTestId('app-shell-skeleton')).toHaveAttribute(
      'data-variant',
      'default',
    );
  });

  it('renders consultation shell skeleton while persisted session is verifying', () => {
    act(() => {
      useAuthStore.setState({
        hasHydrated: true,
        isAuthenticated: true,
        isAuthResolved: false,
      });
    });

    renderProtectedRoute('/consultation');

    expect(screen.getByTestId('app-shell-skeleton')).toHaveAttribute(
      'data-variant',
      'consultation',
    );
  });

  it('redirects to login after hydration when unauthenticated', () => {
    act(() => {
      useAuthStore.setState({
        hasHydrated: true,
        isAuthenticated: false,
        isAuthResolved: true,
      });
    });

    renderProtectedRoute('/dashboard');

    expect(screen.getByText('登录页')).toBeInTheDocument();
  });

  it('renders children when auth is hydrated and resolved', () => {
    act(() => {
      useAuthStore.setState({
        hasHydrated: true,
        isAuthenticated: true,
        isAuthResolved: true,
      });
    });

    renderProtectedRoute('/dashboard');

    expect(screen.getByText('控制台内容')).toBeInTheDocument();
  });
});
