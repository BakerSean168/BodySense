import type { ReactNode } from "react";
import type { BodyStateSnapshot } from "../../types/consultation";
import type { WorkspaceView } from "../../model/workbenchView";
import { BodyOverview } from "./BodyOverview";

interface WorkspaceViewportProps {
  view: WorkspaceView;
  bodyState: BodyStateSnapshot | null;
  state: ReactNode;
  diagnosis: ReactNode;
  treatment: ReactNode;
  progress: ReactNode;
  overlay?: ReactNode;
}

export function WorkspaceViewport({
  view,
  bodyState,
  state,
  diagnosis,
  treatment,
  progress,
  overlay,
}: WorkspaceViewportProps) {
  const content = {
    state,
    diagnosis,
    treatment,
    progress,
  }[view];

  return (
    <div className="relative h-full min-h-0 overflow-y-auto bg-muted/20 custom-scrollbar">
      <div className="grid min-h-full gap-4 p-3 sm:p-4 lg:grid-cols-[minmax(280px,0.72fr)_minmax(0,1.35fr)] lg:gap-5 lg:p-5">
        <BodyOverview snapshot={bodyState} className="lg:sticky lg:top-5" />
        <section
          id={`workspace-panel-${view}`}
          role="tabpanel"
          aria-labelledby={`workspace-tab-${view}`}
          className="min-w-0 rounded-2xl border border-border bg-background p-3 shadow-sm sm:p-4 lg:p-5"
        >
          {content}
        </section>
      </div>
      {overlay}
    </div>
  );
}
