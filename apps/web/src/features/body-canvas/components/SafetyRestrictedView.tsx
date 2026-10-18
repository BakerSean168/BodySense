import { useMemo } from "react";
import { useQuery } from "@tanstack/react-query";
import { CircleHelp, Plus, ShieldCheck, X } from "lucide-react";
import {
  safetyRestrictionKeys,
  safetyRestrictionService,
} from "@/features/consultation/services/safetyRestrictionService";
import { errorMessage } from "@/lib/api-client";
import {
  projectSafetyRestrictions,
  type SafetyRestrictionView,
} from "../model/safetyRestrictions";

export interface SafetyRestrictedViewProps {
  restrictions: readonly SafetyRestrictionView[];
  loading?: boolean;
  error?: string | null;
  onBack: () => void;
  /** "补充复核": continue in Chat; nothing is lifted from the web. */
  onAddReview: () => void;
}

function RestrictionBlock({ item }: { item: SafetyRestrictionView }) {
  return (
    <article
      className="bc-sr-item"
      data-restriction-id={item.id}
      data-urgent={item.urgent || undefined}
    >
      <div className="bc-dx-status">
        <span className="bc-dx-chip" data-tone="bad">
          <span className="bc-dx-chip-dot" aria-hidden="true" />
          {item.statusLabel}
        </span>
        <span className="bc-dx-chip" data-tone="neutral">
          {item.scopeLabel}
        </span>
      </div>
      <section className="bc-dx-section" aria-label="来源">
        <h4 className="bc-dx-label">来源</h4>
        <ul className="bc-dx-rows">
          <li className="bc-dx-row" data-mark="counter">
            <span className="bc-dx-mark" data-mark="counter" aria-hidden="true">
              <X size={12} strokeWidth={2.6} />
            </span>
            <span className="bc-dx-row-text">
              <span className="bc-dx-row-statement">{item.reason}</span>
              <span className="bc-dx-row-meta">{item.meta}</span>
            </span>
          </li>
        </ul>
      </section>
      <section className="bc-dx-section" aria-label="解除条件">
        <h4 className="bc-dx-label">解除条件</h4>
        <ul className="bc-dx-rows">
          {(item.liftConditions.length > 0
            ? item.liftConditions
            : [item.liftPolicyLabel]
          ).map((line) => (
            <li key={line} className="bc-dx-row" data-mark="gap">
              <span className="bc-dx-mark" data-mark="gap" aria-hidden="true">
                <CircleHelp size={14} strokeWidth={2.2} />
              </span>
              <span className="bc-dx-row-text">
                <span className="bc-dx-row-statement">{line}</span>
                {item.liftConditions.length > 0 ? (
                  <span className="bc-dx-row-meta">{item.liftPolicyLabel}</span>
                ) : null}
              </span>
            </li>
          ))}
        </ul>
      </section>
      <div className="bc-dx-banner bc-sr-scope" data-tone="info">
        <ShieldCheck size={15} aria-hidden="true" />
        <span className="bc-dx-banner-text">{item.scopeNote}</span>
      </div>
    </article>
  );
}

/** Penpot Safety / Restricted inside the Training window. */
export function SafetyRestrictedPanel({
  restrictions,
  loading = false,
  error = null,
  onBack,
  onAddReview,
}: SafetyRestrictedViewProps) {
  return (
    <section
      className="bc-business-window-body bc-dx bc-sr"
      aria-label="安全限制"
      data-testid="safety-restricted"
    >
      <div className="bc-window-body" role="region" aria-label="安全限制内容">
        {error ? (
          <div className="bc-dx-banner" data-tone="bad" role="alert">
            <span className="bc-dx-banner-text">{error}</span>
          </div>
        ) : loading ? (
          <p role="status" className="bc-dx-note">
            正在读取安全限制
          </p>
        ) : restrictions.length === 0 ? (
          <p className="bc-dx-note" data-testid="safety-restricted-none">
            现在没有生效的安全限制。如果对话里刚刚提到新的不适，安全核对完成后会显示在这里。
          </p>
        ) : (
          restrictions.map((item) => <RestrictionBlock key={item.id} item={item} />)
        )}
        <p className="bc-dx-note">
          安全限制由系统按你的原话和规则确定，不能在这里直接解除；补充复核材料后会重新核对。
        </p>
      </div>
      <div className="bc-window-footer bc-dx-footer">
        <button type="button" className="bc-dx-link bc-dx-early" onClick={onBack}>
          返回计划
        </button>
        <button type="button" className="bc-dx-primary" onClick={onAddReview}>
          <Plus size={16} strokeWidth={2.6} aria-hidden="true" />
          补充复核
        </button>
      </div>
    </section>
  );
}

export default function SafetyRestrictedView(
  props: Pick<SafetyRestrictedViewProps, "onBack" | "onAddReview">,
) {
  const query = useQuery({
    queryKey: safetyRestrictionKeys.all,
    queryFn: safetyRestrictionService.list,
    retry: false,
    staleTime: 0,
  });
  const restrictions = useMemo(
    () => projectSafetyRestrictions(query.data?.restrictions ?? []),
    [query.data],
  );
  return (
    <SafetyRestrictedPanel
      {...props}
      restrictions={restrictions}
      loading={query.isPending}
      error={
        query.error ? errorMessage(query.error, "安全限制暂时无法读取。") : null
      }
    />
  );
}
