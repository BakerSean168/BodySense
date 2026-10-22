import { lazy, Suspense, type ReactNode } from "react";
import { retryDynamicImport } from "@/lib/retryDynamicImport";

// The 身体档案 drawer (files, source removal, erasure) opens on demand, so it
// is fetched on first open and stays out of the consultation chunk.
const ProfileDrawer = lazy(() =>
  retryDynamicImport(
    () => import("@/features/profile/components/profile/ProfileDrawer"),
  ).then((module) => ({ default: module.ProfileDrawer })),
);

export function ConsultationPageLayout({
  children,
  profileDrawerMounted,
  isProfileOpen,
  onProfileOpenChange,
}: {
  children: ReactNode;
  profileDrawerMounted: boolean;
  isProfileOpen: boolean;
  onProfileOpenChange: (open: boolean) => void;
}) {
  return (
    <div className="h-dvh overflow-hidden text-foreground">
      {children}
      {profileDrawerMounted ? (
        <Suspense fallback={null}>
          <ProfileDrawer
            open={isProfileOpen}
            onOpenChange={onProfileOpenChange}
          />
        </Suspense>
      ) : null}
    </div>
  );
}
