import { useAuthStore } from "@/stores/authStore";
import { apiUrl } from "@/lib/api-url";

interface FetchOptions extends RequestInit {
  skipAuth?: boolean;
}

// Custom fetch wrapper with automatic token refresh
export async function authFetch(
  url: string,
  options: FetchOptions = {},
): Promise<Response> {
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
  fetchOptions.credentials ??= "same-origin";
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
