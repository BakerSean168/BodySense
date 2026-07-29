import { useNavigate } from 'react-router';
import { Card } from '@/components/ui/Card';
import { Button } from '@/components/ui/Button';
import { useJourneyState } from '../hooks/useJourneyState';
import { resolveJourneyActions } from '../lib/journeyActions';
import {
  getJourneyRouteReadiness,
  type JourneyGatedRoute,
} from '../lib/journeyGuards';

export interface JourneySoftGuardProps {
  /** Which gated surface the user is trying to open. */
  route: JourneyGatedRoute;
  /** The real page content, rendered when the stage is ready (or still loading). */
  children: React.ReactNode;
}

/**
 * Soft journey guard: never hard-blocks a route.
 *
 * When the backend-derived stage is not ready for `route`, we keep the user on
 * the page but replace the main content with a guidance card driven by
 * `available_actions`. Once the stage advances, the same component re-renders
 * the real children without a hard redirect loop.
 */
export function JourneySoftGuard({ route, children }: JourneySoftGuardProps) {
  const navigate = useNavigate();
  const { journey, isLoading, error, refresh } = useJourneyState();

  // While loading, or if journey cannot be fetched, do not invent a gate —
  // show the page and let its own empty/error states handle the rest.
  if (isLoading || error || !journey) {
    return <>{children}</>;
  }

  const readiness = getJourneyRouteReadiness(route, journey.stage);
  if (readiness.ready) {
    return <>{children}</>;
  }

  const actions = resolveJourneyActions(
    journey.available_actions,
    journey.artifacts,
  );
  const [primary, ...secondary] = actions;

  return (
    <Card className="mx-auto max-w-xl p-8 text-center shadow-sm">
      <div className="mx-auto mb-5 flex h-14 w-14 items-center justify-center rounded-full bg-[#f1f5f2] border border-[#c5d7cc]/40">
        <svg
          className="h-7 w-7 text-primary-700"
          fill="none"
          viewBox="0 0 24 24"
          stroke="currentColor"
          aria-hidden
        >
          <path
            strokeLinecap="round"
            strokeLinejoin="round"
            strokeWidth={1.8}
            d="M13 16h-1v-4h-1m1-4h.01M12 2a10 10 0 100 20 10 10 0 000-20z"
          />
        </svg>
      </div>

      <p className="text-xs font-semibold uppercase tracking-wider text-[#5E7D6F]">
        {journey.stage_reason}
      </p>
      <h2 className="mt-2 text-xl font-display font-semibold text-[#2E3C36]">
        还不能进入{readiness.title}
      </h2>
      <p className="mt-3 text-sm font-medium leading-relaxed text-[#5D6B63]">
        {readiness.hint}
      </p>

      <div className="mt-6 flex flex-wrap items-center justify-center gap-3">
        {primary ? (
          <Button
            className="bg-[#CD7B67] text-white shadow-sm shadow-[#CD7B67]/15 hover:bg-[#B65E49]"
            onClick={() => navigate(primary.href)}
          >
            {primary.label}
          </Button>
        ) : (
          <Button
            className="bg-[#CD7B67] text-white shadow-sm shadow-[#CD7B67]/15 hover:bg-[#B65E49]"
            onClick={() => navigate('/dashboard')}
          >
            返回首页查看下一步
          </Button>
        )}

        {secondary.slice(0, 2).map((action) => (
          <Button
            key={action.action}
            variant="outline"
            className="border-[#CD7B67] text-[#CD7B67] hover:bg-[#CD7B67]/5"
            onClick={() => navigate(action.href)}
          >
            {action.label}
          </Button>
        ))}

        <Button variant="ghost" size="sm" onClick={() => void refresh()}>
          刷新状态
        </Button>
      </div>
    </Card>
  );
}
