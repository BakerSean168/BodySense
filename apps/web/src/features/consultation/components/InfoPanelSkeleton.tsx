function SkeletonLine({ className }: { className: string }) {
  return (
    <div
      className={`animate-pulse rounded-full bg-white/[0.065] motion-reduce:animate-none ${className}`}
    />
  );
}

export function InfoPanelSkeleton() {
  return (
    <div
      className="min-h-[360px] bg-transparent"
      data-testid="info-panel-skeleton"
    >
      <div className="space-y-3 pb-7">
        <SkeletonLine className="h-5 w-28 rounded-md" />
        <SkeletonLine className="h-3 w-64 max-w-[68%] rounded-md" />
      </div>

      <div className="space-y-0">
        {[0, 1, 2].map((row) => (
          <div
            key={row}
            className="space-y-3 border-b border-white/[0.055] py-5 first:pt-0 last:border-b-0"
          >
            <div className="flex items-center justify-between gap-6">
              <SkeletonLine className="h-3 w-[38%] rounded-md" />
              <SkeletonLine className="h-3 w-14 rounded-md" />
            </div>
            <SkeletonLine className="h-3 w-[78%] rounded-md" />
            <SkeletonLine className="h-3 w-[55%] rounded-md" />
          </div>
        ))}
      </div>
    </div>
  );
}
