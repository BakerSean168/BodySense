function SkeletonBlock({ className }: { className: string }) {
  return <div className={`animate-pulse rounded-xl bg-muted ${className}`} />;
}

export function InfoPanelSkeleton() {
  return (
    <div
      className="flex min-h-[360px] flex-col bg-transparent"
      data-testid="info-panel-skeleton"
    >
      <div className="space-y-3 pb-5">
        <SkeletonBlock className="h-6 w-36" />
        <SkeletonBlock className="h-4 w-72 max-w-[70%]" />
      </div>

      <div className="flex-1 space-y-4">
        <SkeletonBlock className="h-32 w-full" />
        <SkeletonBlock className="h-24 w-full" />
        <SkeletonBlock className="h-40 w-full" />
      </div>
    </div>
  );
}
