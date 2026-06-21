import { useAuthStore } from '@/stores/authStore';

const API_BASE_URL = import.meta.env.VITE_API_BASE_URL || 'http://localhost:8080';

interface FetchOptions extends RequestInit {
  skipAuth?: boolean;
}

// Custom fetch wrapper with automatic token refresh
export async function authFetch(url: string, options: FetchOptions = {}): Promise<Response> {
  const { skipAuth = false, ...fetchOptions } = options;
  const { accessToken, refreshAccessToken } = useAuthStore.getState();

  // Add authorization header if not skipped
  if (!skipAuth && accessToken) {
    fetchOptions.headers = {
      ...fetchOptions.headers,
      Authorization: `Bearer ${accessToken}`,
    };
  }

  // Make the request
  let response = await fetch(`${API_BASE_URL}${url}`, fetchOptions);

  // If 401 and not skipped, try to refresh token
  if (response.status === 401 && !skipAuth) {
    const refreshed = await refreshAccessToken();

    if (refreshed) {
      // Retry with new token
      const newAccessToken = useAuthStore.getState().accessToken;
      fetchOptions.headers = {
        ...fetchOptions.headers,
        Authorization: `Bearer ${newAccessToken}`,
      };
      response = await fetch(`${API_BASE_URL}${url}`, fetchOptions);
    }
  }

  return response;
}

// Auth API functions
export const authApi = {
  register: async (email: string, password: string) => {
    const response = await authFetch('/api/v1/auth/register', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ email, password }),
      skipAuth: true,
    });

    if (!response.ok) {
      const error = await response.json();
      throw new Error(error.message || 'Registration failed');
    }

    return response.json();
  },

  login: async (email: string, password: string) => {
    const response = await authFetch('/api/v1/auth/login', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ email, password }),
      skipAuth: true,
    });

    if (!response.ok) {
      const error = await response.json();
      throw new Error(error.message || 'Login failed');
    }

    return response.json();
  },

  logout: async (refreshToken: string) => {
    const response = await authFetch('/api/v1/auth/logout', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ refresh_token: refreshToken }),
    });

    return response.json();
  },

  getMe: async () => {
    const response = await authFetch('/api/v1/me');

    if (!response.ok) {
      throw new Error('Failed to fetch user');
    }

    return response.json();
  },
};
