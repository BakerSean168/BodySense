import { Fragment } from 'react';
import { useNavigate } from 'react-router';
import type { HealthJourneyState } from '@bodysense/contracts';
import { Card } from '@/components/ui/Card';
import { Button } from '@/components/ui/Button';
import { resolveJourneyActions } from '../lib/journeyActions';

export interface JourneyNextStepCardProps {
  journey: HealthJourneyState | null;
  isLoading: boolean;
  error: string | null;
  onRetry: () => void;
}

/**
 * Render the backend-derived next steps for the user's health journey.
 *
 * The card shows whatever `available_actions` the backend reports, in the order
 * it reports them: the first action is the primary call to action. When the
 * journey state cannot be loaded the card degrades to a retry prompt rather
 * than inventing a next step locally.
 */
export function JourneyNextStepCard({
  journey,
  isLoading,
  error,
  onRetry,
}: JourneyNextStepCardProps) {
  const navigate = useNavigate();

  if (isLoading) {
    return (
      <Card className="p-6">
        <div className="animate-pulse space-y-4">
          <div className="h-4 w-24 rounded bg-[#E5E3DF]" />
          <div className="h-6 w-2/3 rounded bg-[#E5E3DF]" />
          <div className="h-10 w-40 rounded-lg bg-[#E5E3DF]" />
        </div>
      </Card>
    );
  }

  if (error || !journey) {
    return (
      <Card className="p-6">
        <h2 className="text-lg font-display font-semibold text-[#2E3C36]">下一步</h2>
        <p className="mt-2 text-sm text-[#5D6B63]">暂时无法获取你的健康旅程状态。</p>
        <Button variant="outline" size="sm" className="mt-4" onClick={onRetry}>
          重试
        </Button>
      </Card>
    );
  }

  const actions = resolveJourneyActions(journey.available_actions, journey.artifacts);

  if (actions.length === 0) {
    return (
      <Card className="p-6">
        <h2 className="text-lg font-display font-semibold text-[#2E3C36]">下一步</h2>
        <p className="mt-2 text-sm text-[#5D6B63]">{journey.stage_reason}</p>
      </Card>
    );
  }

  const [primary, ...secondary] = actions;

  return (
    <Card className="p-6 sm:p-8">
      <div className="flex flex-wrap items-baseline justify-between gap-2">
        <h2 className="text-lg font-display font-semibold text-[#2E3C36]">下一步</h2>
        <span className="text-xs font-semibold uppercase tracking-wider text-[#5E7D6F]">
          {journey.stage_reason}
        </span>
      </div>

      <p className="mt-4 text-base font-medium leading-relaxed text-[#2E3C36]">
        {primary.description}
      </p>

      <div className="mt-6 flex flex-wrap gap-3">
        <Button
          className="bg-[#CD7B67] text-white shadow-sm shadow-[#CD7B67]/15 hover:bg-[#B65E49]"
          onClick={() => navigate(primary.href)}
        >
          {primary.label}
        </Button>

        {secondary.map((action) => (
          <Fragment key={action.action}>
            <Button
              variant="outline"
              className="border-[#CD7B67] text-[#CD7B67] hover:bg-[#CD7B67]/5"
              onClick={() => navigate(action.href)}
            >
              {action.label}
            </Button>
          </Fragment>
        ))}
      </div>
    </Card>
  );
}
