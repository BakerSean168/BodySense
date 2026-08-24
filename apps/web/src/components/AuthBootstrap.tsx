import { useEffect } from "react";
import { useAuthStore } from "@/stores/authStore";

export function AuthBootstrap() {
  const hasHydrated = useAuthStore((state) => state.hasHydrated);
  const bootstrapSession = useAuthStore((state) => state.bootstrapSession);

  useEffect(() => {
    if (!hasHydrated) {
      void bootstrapSession();
    }
  }, [hasHydrated, bootstrapSession]);

  return null;
}
