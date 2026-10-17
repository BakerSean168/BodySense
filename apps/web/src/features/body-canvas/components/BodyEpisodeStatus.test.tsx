import { render, screen } from "@testing-library/react";
import { expect, it } from "vitest";
import type { BodyEpisode } from "@/features/profile/services/bodyEpisodeService";
import { BodyEpisodeStatus } from "./BodyEpisodeStatus";

const episode: BodyEpisode = {
  id: "episode",
  user_id: "owner",
  symptom_type: "肩膀不适",
  body_region_ids: [],
  laterality: "unknown",
  identity_status: "current",
  created_revision: 1,
  created_at: "2026-10-17T00:00:00Z",
  links: [],
  projection: {
    state: "ongoing",
    reason: "explicit_current_report",
    state_as_of: "2026-10-15",
    basis_fact_ids: ["fact"],
    based_on_revision: 1,
  },
};
it("qualifies ongoing with the reported event time and statement basis, never today's date", () => {
  render(<BodyEpisodeStatus episode={episode} />);
  expect(screen.getByText("截至明确报告时仍在持续")).toBeVisible();
  expect(screen.getByText(/2026-10-15/)).toBeVisible();
  expect(screen.getByText(/依据已关联陈述推导/)).toBeVisible();
  expect(
    screen.queryByText(/2026-10-17|确诊|治愈|已解决/),
  ).not.toBeInTheDocument();
});
it.each(["insufficient_evidence", "conflicting_evidence"] as const)(
  "retains uncertainty for %s even with an inconsistent ongoing field",
  (reason) => {
    render(
      <BodyEpisodeStatus
        episode={{ ...episode, projection: { ...episode.projection, reason } }}
      />,
    );
    expect(screen.getByText("持续状态尚未确认")).toBeVisible();
    expect(
      screen.queryByText("截至明确报告时仍在持续"),
    ).not.toBeInTheDocument();
  },
);
it("does not equate ended with medical recovery and does not invent a missing event time", () => {
  render(
    <BodyEpisodeStatus
      episode={{
        ...episode,
        projection: {
          ...episode.projection,
          state: "ended",
          reason: "explicit_resolution",
          state_as_of: undefined,
        },
      }}
    />,
  );
  expect(screen.getByText("有明确的结束报告")).toBeVisible();
  expect(screen.getByText(/报告发生时间未提供/)).toBeVisible();
  expect(screen.queryByText(/治愈|康复|已解决/)).not.toBeInTheDocument();
});
it("does not display ongoing without any basis or for a superseded identity", () => {
  const { rerender } = render(
    <BodyEpisodeStatus
      episode={{
        ...episode,
        projection: { ...episode.projection, basis_fact_ids: [] },
      }}
    />,
  );
  expect(screen.getByText("持续状态尚未确认")).toBeVisible();
  rerender(
    <BodyEpisodeStatus
      episode={{ ...episode, identity_status: "superseded" }}
    />,
  );
  expect(screen.getByText("经历身份已被替代")).toBeVisible();
  expect(screen.queryByText("截至明确报告时仍在持续")).not.toBeInTheDocument();
});
