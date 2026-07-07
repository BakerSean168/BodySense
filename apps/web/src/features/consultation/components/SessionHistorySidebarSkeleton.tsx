function SkeletonBlock({ className }: { className: string }) {
  return <div className={`animate-pulse rounded-2xl bg-[#E9E6DF] ${className}`} />;
}

export function SessionHistorySidebarSkeleton() {
  return (
    <div className="flex h-full flex-col" data-testid="session-history-sidebar-skeleton">
      <div className="space-y-2 p-3">
        <SkeletonBlock className="h-10 w-full rounded-full" />
        <SkeletonBlock className="h-4 w-24 rounded-full" />
      </div>

      <div className="border-t" />

      <div className="space-y-2 px-3 py-3">
        <SkeletonBlock className="h-4 w-14 rounded-full" />
        {Array.from({ length: 7 }).map((_, index) => (
          <SkeletonBlock key={index} className="h-14 w-full" />
        ))}
      </div>
    </div>
  );
}
