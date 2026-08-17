import { useEffect } from "react";
import { useAuthStore } from "@/stores/authStore";

export function AuthBootstrap() {
  const hasHydrated = useAuthStore((state) => state.hasHydrated);
  const isAuthenticated = useAuthStore((state) => state.isAuthenticated);
  const isAuthResolved = useAuthStore((state) => state.isAuthResolved);
  const verifySession = useAuthStore((state) => state.verifySession);

  useEffect(() => {
    if (!hasHydrated || !isAuthenticated || isAuthResolved) {
      return;
    }

    void verifySession();
  }, [hasHydrated, isAuthenticated, isAuthResolved, verifySession]);

  return null;
}
