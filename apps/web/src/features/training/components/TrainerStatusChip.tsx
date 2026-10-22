import { useQuery } from "@tanstack/react-query";
import { ChevronRight } from "lucide-react";
import { TrainingIcon } from "@/features/body-canvas/components/TrainingIcon";
import { trainerKeys } from "../hooks/trainerKeys";
import {
  trainingApi,
  type TrainingSessionRecord,
} from "../services/trainingService";

/**
 * The session the Home chip points at: the one holding the in-progress
 * slot, otherwise the most recently started paused one. Ended sessions
 * never show.
 */
export function pickLiveSession(
  sessions: readonly TrainingSessionRecord[] | undefined,
): TrainingSessionRecord | null {
  const open = (sessions ?? []).filter((session) => session.state !== "ended");
  return (
    open.find((session) => session.state === "in_progress") ??
    open.toSorted(
      (a, b) => Date.parse(b.startedAt) - Date.parse(a.startedAt),
    )[0] ??
    null
  );
}

export interface TrainerStatusChipProps {
  onOpen: (session: TrainingSessionRecord) => void;
}

/**
 * Penpot Home / trainer-active and trainer-paused: a live session stays one
 * tap away from the body canvas. It only reads session receipts; opening it
 * reopens that session and never starts or resumes anything by itself.
 */
export function TrainerStatusChip({ onOpen }: TrainerStatusChipProps) {
  const sessions = useQuery({
    queryKey: trainerKeys.sessions(),
    queryFn: trainingApi.listSessions,
    retry: false,
    staleTime: 30_000,
  });
  const live = pickLiveSession(sessions.data);
  if (!live) return null;
  const paused = live.state === "paused";
  const label = paused ? "训练已暂停" : "训练进行中";
  return (
    <button
      type="button"
      className="bc-trainer-chip"
      data-state={live.state}
      data-testid="home-trainer-chip"
      aria-label={`回到训练 · ${label}`}
      onClick={() => onOpen(live)}
    >
      <TrainingIcon size={18} aria-hidden="true" />
      <span className="bc-trainer-chip-status">
        <span className="bc-trainer-chip-dot" aria-hidden="true" />
        {label}
      </span>
      <ChevronRight size={16} aria-hidden="true" />
    </button>
  );
}
