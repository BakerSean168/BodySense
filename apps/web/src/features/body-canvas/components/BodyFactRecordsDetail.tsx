import type { ReactNode } from "react";
import { formatInstantInZone } from "@/features/profile/model/profilePreferences";
import type { BodyFactRecordsPage } from "@/features/profile/services/bodyFactRecordsService";
import type {
  BodyFactRecordViewModel,
  BodyFactSurfaceState,
} from "../model/bodyFactViewModel";
import {
  BodyFactActionsMenu,
  BodyFactAmendment,
  type BodyFactAmendMode,
} from "./BodyFactAmendment";
import type { StatementErasureTarget } from "./StatementErasure";

/** Episode chips a Records detail can link to; supplied by the Person window. */
export interface BodyFactRelatedLink {
  id: string;
  label: string;
}

/** Recorded instant on the preferred display clock; the instant is unchanged. */
function recordedTime(value: string, zone: string): string {
  return formatInstantInZone(value, zone) ?? "记录时间未知";
}

/**
 * Penpot Records detail: one statement, its organising links and the only
 * places it can be amended. Leaving the list never discards the ledger read.
 */
export function BodyFactRecordsDetail({
  selected,
  habitsOnly,
  canEdit,
  page,
  pagesDisagree,
  amendMode,
  displayZone,
  contract,
  state,
  relatedLinks,
  onOpenRelated,
  onCorrect,
  onWithdraw,
  onErase,
  onAmendModeChange,
  onRefresh,
  onCommitted,
  onBack,
  feedback,
}: {
  selected: BodyFactRecordViewModel;
  habitsOnly: boolean;
  canEdit: boolean;
  page: BodyFactRecordsPage | undefined;
  pagesDisagree: boolean;
  amendMode: BodyFactAmendMode;
  displayZone: string;
  /** Bounded transport projection labels, kept stable for the ledger contract. */
  contract: string;
  state: BodyFactSurfaceState;
  relatedLinks?: (factId: string) => readonly BodyFactRelatedLink[];
  onOpenRelated?: (link: BodyFactRelatedLink) => void;
  onCorrect: () => void;
  onWithdraw: () => void;
  onErase: (target: StatementErasureTarget) => void;
  onAmendModeChange: (mode: BodyFactAmendMode) => void;
  onRefresh: () => void;
  onCommitted: (message: string, committedRevision: number) => void;
  onBack: () => void;
  feedback: ReactNode;
}) {
  const links = relatedLinks?.(selected.id) ?? [];
  const sourceId = selected.record.source_evidence_id;
  return (
    <div
      className="bc-person-category bc-person-detail"
      data-testid="body-fact-records"
      data-vm-contract={contract}
      data-vm-state={state}
    >
      <div className="bc-person-detail-body" data-testid="body-fact-detail">
        <div className="bc-person-detail-head">
          <h3 className="bc-person-detail-title">{selected.label}</h3>
          {canEdit && page && !pagesDisagree && !amendMode ? (
            <BodyFactActionsMenu
              onCorrect={onCorrect}
              onWithdraw={onWithdraw}
              onErase={
                sourceId
                  ? () =>
                      onErase({
                        recordId: selected.record.id,
                        statementId: sourceId,
                        ownerId: selected.record.user_id,
                      })
                  : undefined
              }
            />
          ) : null}
        </div>
        {selected.isCorrection || links.length ? (
          <div className="bc-person-chips">
            {selected.isCorrection ? (
              <span className="bc-person-chip">更正后的陈述</span>
            ) : null}
            {links.map((link) => (
              <button
                key={link.id}
                type="button"
                className="bc-person-chip bc-person-chip-link"
                onClick={() => onOpenRelated?.(link)}
              >
                {link.label}
              </button>
            ))}
          </div>
        ) : null}
        {feedback}
        <p className="bc-person-quote">“{selected.statement}”</p>
        <p className="bc-person-row-meta">
          发生：{selected.occurrence} · 记录{" "}
          <time dateTime={selected.recordedAt}>
            {recordedTime(selected.recordedAt, displayZone)}
          </time>
        </p>
        {canEdit && page && !pagesDisagree ? (
          <BodyFactAmendment
            record={selected.record}
            revision={page.current_revision}
            mode={amendMode}
            onModeChange={onAmendModeChange}
            onRefresh={onRefresh}
            onCommitted={onCommitted}
          />
        ) : null}
      </div>
      <footer className="bc-person-footer">
        <button
          type="button"
          className="bc-person-footer-link"
          onClick={onBack}
        >
          {habitsOnly ? "返回生活习惯" : "返回记录"}
        </button>
      </footer>
    </div>
  );
}
