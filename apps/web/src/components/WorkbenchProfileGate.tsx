import { useEffect, useState } from "react";
import { Navigate } from "react-router";
import { useProfileStore } from "@/stores/profileStore";
import { AppShellSkeleton } from "@/components/layout/AppShellSkeleton";
import { Button } from "@/components/ui/Button";

interface WorkbenchProfileGateProps {
  children: React.ReactNode;
}

export function WorkbenchProfileGate({ children }: WorkbenchProfileGateProps) {
  const profile = useProfileStore((state) => state.profile);
  const error = useProfileStore((state) => state.error);
  const fetchProfile = useProfileStore((state) => state.fetchProfile);
  const [checked, setChecked] = useState(false);

  useEffect(() => {
    let active = true;
    void fetchProfile().finally(() => {
      if (active) setChecked(true);
    });
    return () => {
      active = false;
    };
  }, [fetchProfile]);

  if (!checked) {
    return <AppShellSkeleton variant="consultation" />;
  }

  if (error && !profile) {
    return (
      <div className="flex min-h-dvh items-center justify-center bg-background p-6 text-foreground">
        <div className="w-full max-w-md rounded-2xl border border-border bg-card p-6 text-center shadow-sm">
          <h1 className="text-lg font-semibold">暂时无法读取身体档案</h1>
          <p className="mt-2 text-sm text-muted-foreground">
            工作台需要先确认长期身体档案状态。请重试，不会覆盖服务器中的已有数据。
          </p>
          <Button
            className="mt-5"
            onClick={() => {
              setChecked(false);
              void fetchProfile().finally(() => setChecked(true));
            }}
          >
            重新加载
          </Button>
        </div>
      </div>
    );
  }

  if (!profile) {
    return <Navigate to="/onboarding" replace />;
  }

  return <>{children}</>;
}
