import type { ReactNode } from "react";
import { LoaderCircle } from "lucide-react";
import { episodeStateLabel } from "../../../../model/bodyEpisodeAssociations";
import { episodeContinuityChip } from "../../../../model/personRecordsTimeline";
import type { BodyRecords } from "@/features/profile/services/bodyRecordsService";

type Episode = BodyRecords["episodes"][number];

export function PersonRecordsLoading({ label }: { label: string }) {
  return (
    <p role="status" className="bc-person-loading">
      <LoaderCircle size={16} className="animate-spin" />
      {label}
    </p>
  );
}

export function ContinuityChip({ episode }: { episode: Episode }) {
  return (
    <span
      className="bc-person-chip bc-person-continuity"
      data-state={episode.projection.state}
      title={episodeStateLabel(episode)}
    >
      {episodeContinuityChip(episode)}
    </span>
  );
}

/** Penpot Body / Episode chrome: title, chips, sections, footer back link. */
export function PersonSubview({
  title,
  chips,
  testId,
  onBack,
  backLabel = "返回记录",
  action,
  children,
}: {
  title: string;
  chips?: ReactNode;
  testId: string;
  onBack: () => void;
  backLabel?: string;
  action?: ReactNode;
  children: ReactNode;
}) {
  return (
    <div className="bc-person-category bc-person-detail" data-testid={testId}>
      <div className="bc-person-detail-body">
        <h3 className="bc-person-detail-title">{title}</h3>
        {chips ? <div className="bc-person-chips">{chips}</div> : null}
        {children}
      </div>
      <footer className="bc-person-footer">
        <button
          type="button"
          className="bc-person-footer-link"
          onClick={onBack}
        >
          {backLabel}
        </button>
        {action}
      </footer>
    </div>
  );
}
