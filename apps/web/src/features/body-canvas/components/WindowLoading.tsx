/**
 * Centered ink-pulse loader for business windows. No visible text; the label
 * stays available to assistive tech through role="status".
 */
export function WindowLoading({ label }: { label: string }) {
  return (
    <div role="status" aria-label={label} className="bc-window-spinner">
      <span className="sr-only">{label}</span>
      <span className="bc-window-spinner-ink" aria-hidden="true">
        <span />
        <span />
        <span />
      </span>
    </div>
  );
}
