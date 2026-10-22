import { ArrowRight, CircleHelp, Clock3, Sparkle, X } from "lucide-react";
import type {
  ProposalChangeView,
  ProposalView,
  TrainingTone,
} from "../../model/trainingPlans";
import type { TrainingPlanControls } from "./types";
import { Banner, Chip } from "./TrainingPrimitives";

const CHANGE_LABELS: Record<
  ProposalChangeView["kind"],
  { label: string; tone: TrainingTone }
> = {
  modified: { label: "修改", tone: "warn" },
  added: { label: "新增", tone: "ok" },
  removed: { label: "移除", tone: "bad" },
};

function ChangeCard({
  change,
  muted,
}: {
  change: ProposalChangeView;
  muted: boolean;
}) {
  const meta = CHANGE_LABELS[change.kind];
  if (muted)
    return (
      <li className="bc-tp-change-muted" data-change={change.kind}>
        <strong>{change.name}</strong>
        <span>
          {change.kind === "modified" ? (
            <>
              {change.before} <ArrowRight size={12} aria-label="改为" />{" "}
              {change.after}
            </>
          ) : change.kind === "added" ? (
            `${change.after} · 新增`
          ) : (
            `${change.before} · 移除`
          )}
        </span>
      </li>
    );
  return (
    <li className="bc-tp-change" data-change={change.kind}>
      <span className="bc-tp-change-head">
        <Chip tone={meta.tone} dot>
          {meta.label}
        </Chip>
        <strong>{change.name}</strong>
      </span>
      <span className="bc-tp-change-dose">
        {change.kind === "modified" ? (
          <>
            <s className="bc-tp-change-before">{change.before}</s>
            <ArrowRight size={12} aria-label="改为" />
            <span className="bc-tp-change-after">{change.after}</span>
          </>
        ) : change.kind === "added" ? (
          <span className="bc-tp-change-after">{change.after}</span>
        ) : (
          <s className="bc-tp-change-before">{change.before}</s>
        )}
      </span>
    </li>
  );
}

const EXPIRED_REASON_TEXT: Record<
  NonNullable<ProposalView["expiredReason"]>,
  string
> = {
  timeout: "未确认，计划保持原样",
  base_revision_changed: "计划已有新的修订，这条建议未生效，计划保持原样",
  relevant_context_changed:
    "身体、诊断或安全情况已变化，这条建议未生效，计划保持原样",
};

function formatDay(iso: string): string {
  const date = new Date(iso);
  return Number.isNaN(date.getTime())
    ? ""
    : new Intl.DateTimeFormat("zh-CN", {
        month: "long",
        day: "numeric",
      }).format(date);
}

export function ProposalScene({
  proposal,
  controls,
}: {
  proposal: ProposalView;
  controls: TrainingPlanControls;
}) {
  const expired = proposal.phase !== "pending";
  return (
    <div
      className="bc-tp-scene"
      data-testid={expired ? "training-proposal-expired" : "training-proposal"}
      data-proposal-phase={proposal.phase}
    >
      <div className="bc-dx-status">
        {expired ? (
          <>
            <Chip tone="idle" dot>
              {proposal.phase === "accepted"
                ? "已采纳"
                : proposal.phase === "rejected"
                  ? "已拒绝"
                  : "已过期"}
            </Chip>
            <Chip tone="neutral" icon={<Clock3 size={12} aria-hidden="true" />}>
              {formatDay(proposal.expiresAt)}
            </Chip>
          </>
        ) : (
          <>
            <Chip tone="warn" dot>
              待确认
            </Chip>
            {proposal.expiresInDays !== null ? (
              <Chip
                tone="neutral"
                icon={<Clock3 size={12} aria-hidden="true" />}
              >
                {proposal.expiresInDays} 天后过期
              </Chip>
            ) : null}
            {proposal.byAgent ? (
              <Chip tone="ai" icon={<Sparkle size={12} aria-hidden="true" />}>
                AI 建议
              </Chip>
            ) : null}
          </>
        )}
      </div>
      {expired && proposal.phase === "expired" && proposal.expiredReason ? (
        <Banner
          tone="warn"
          icon={<CircleHelp size={16} aria-hidden="true" />}
          role="status"
          testId="training-proposal-stale"
        >
          {EXPIRED_REASON_TEXT[proposal.expiredReason]}
        </Banner>
      ) : null}
      {controls.error ? (
        <Banner
          tone="bad"
          icon={<X size={16} aria-hidden="true" />}
          role="alert"
        >
          {controls.error}
        </Banner>
      ) : null}
      <section
        className="bc-dx-section"
        aria-label={expired ? "当时的调整" : "调整"}
      >
        <h4 className="bc-dx-label">
          {proposal.kind === "create"
            ? `新计划 · ${proposal.title}`
            : expired
              ? "当时的调整"
              : "调整"}
        </h4>
        <ul className={expired ? "bc-tp-changes-muted" : "bc-tp-changes"}>
          {proposal.changes.map((change) => (
            <ChangeCard
              key={`${change.kind}:${change.planExerciseId}`}
              change={change}
              muted={expired}
            />
          ))}
        </ul>
        {proposal.changes.length === 0 ? (
          <p className="bc-dx-note">内容与当前计划一致。</p>
        ) : null}
      </section>
    </div>
  );
}
