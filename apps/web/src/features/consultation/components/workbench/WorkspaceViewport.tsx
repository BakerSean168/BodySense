import type { ReactNode } from "react";
import type { BodyStateSnapshot } from "../../types/consultation";
import type { WorkspaceView } from "../../model/workbenchView";
import { BodyOverview } from "./BodyOverview";
import { cn } from "@/lib/utils";

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
  const showBodyOverview = view === "state";

  return (
    <div className="relative h-full min-h-0 overflow-y-auto bg-background custom-scrollbar">
      <div
        key={view}
        className={cn(
          "workbench-view-enter min-h-full px-6 py-5 sm:px-7 sm:py-6 lg:px-8 lg:py-7",
          showBodyOverview
            ? "grid items-start gap-7 xl:gap-9 lg:grid-cols-[minmax(300px,0.82fr)_minmax(0,1.48fr)]"
            : "mx-auto w-full max-w-[1120px]",
        )}
      >
        {showBodyOverview ? (
          <BodyOverview
            snapshot={bodyState}
            className="lg:sticky lg:top-6 lg:self-start"
          />
        ) : null}
        <section
          id={`workspace-panel-${view}`}
          role="tabpanel"
          aria-labelledby={`workspace-tab-${view}`}
          className={cn("min-w-0", !showBodyOverview && "py-1")}
        >
          {content}
        </section>
      </div>
      {overlay}
    </div>
  );
}
