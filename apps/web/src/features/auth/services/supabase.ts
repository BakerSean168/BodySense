import type { SupabaseClient } from "@supabase/supabase-js";

let clientPromise: Promise<SupabaseClient> | null = null;

/**
 * Supabase owns credentials, session rotation and password handling.
 * Split the full Supabase SDK into a lazy authentication chunk rather than
 * making every unrelated page eagerly load Storage, Realtime and Auth.
 * Concurrent callers share one client initialization.
 */
export function getSupabaseClient(): Promise<SupabaseClient> {
  if (clientPromise) return clientPromise;

  const url = import.meta.env.VITE_SUPABASE_URL?.trim();
  const key = import.meta.env.VITE_SUPABASE_PUBLISHABLE_KEY?.trim();
  if (!url || !key) {
    return Promise.reject(
      new Error(
        "Supabase Auth 未配置：需要 VITE_SUPABASE_URL 和 VITE_SUPABASE_PUBLISHABLE_KEY",
      ),
    );
  }

  clientPromise = import("@supabase/supabase-js")
    .then(({ createClient }) =>
      createClient(url, key, {
        auth: {
          persistSession: true,
          autoRefreshToken: true,
          detectSessionInUrl: true,
        },
      }),
    )
    .catch((error: unknown) => {
      clientPromise = null;
      throw error;
    });
  return clientPromise;
}
