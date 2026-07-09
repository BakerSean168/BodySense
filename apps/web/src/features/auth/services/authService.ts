import { useAuthStore } from '@/stores/authStore';
import { apiUrl, safeJson, extractErrorMessage } from '@/lib/api-url';

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
  let response = await fetch(apiUrl(url), fetchOptions);

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
      response = await fetch(apiUrl(url), fetchOptions);
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
      throw new Error(await extractErrorMessage(response));
    }

    return safeJson(response);
  },

  login: async (email: string, password: string) => {
    const response = await authFetch('/api/v1/auth/login', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ email, password }),
      skipAuth: true,
    });

    if (!response.ok) {
      throw new Error(await extractErrorMessage(response));
    }

    return safeJson(response);
  },

  logout: async (refreshToken: string) => {
    const response = await authFetch('/api/v1/auth/logout', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ refresh_token: refreshToken }),
    });

    return safeJson(response);
  },

  getMe: async () => {
    const response = await authFetch('/api/v1/me');

    if (!response.ok) {
      throw new Error(await extractErrorMessage(response));
    }

    return safeJson(response);
  },
};
