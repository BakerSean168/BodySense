import {
  expect,
  test,
  type APIRequestContext,
  type APIResponse,
  type Page,
} from "@playwright/test";
import { refreshBrowserAccessToken } from "./support/auth";
import { profileReplacement } from "./support/profile";
import { fillRegisterPassword } from "./support/register";

const apiBase = process.env.E2E_API_BASE_URL || "http://127.0.0.1:8080";

type Headers = Record<string, string>;

interface Exercise {
  planExerciseId: string;
  exerciseId: string;
  nameSnapshot: string;
  sets: number;
  target:
    | { kind: "repetitions"; min: number }
    | { kind: "duration"; seconds: number };
  restSeconds?: number;
  side?: "alternating" | "both";
}

interface PlanRecord {
  planId: string;
  currentAcceptedRevisionId: string;
}

interface ProposalRecord {
  proposalId: string;
  state: string;
}

async function json<T>(response: APIResponse, label: string): Promise<T> {
  const body = await response.text();
  expect(
    response.ok(),
    `${label} failed with ${response.status()}: ${body}`,
  ).toBeTruthy();
  return JSON.parse(body) as T;
}

function draft(main: Exercise[]) {
  return {
    title: "日常活动计划",
    goal: "恢复日常活动能力",
    relatedConditionIds: [],
    relatedBodyRefs: [],
    content: {
      sessions: [
        {
          templateId: "a",
          title: "A · 基础课",
          stages: [
            {
              phase: "warmup",
              exercises: [
                {
                  planExerciseId: "cat-cow",
                  exerciseId: "cat-cow",
                  nameSnapshot: "猫牛式",
                  sets: 1,
                  target: { kind: "repetitions", min: 10 },
                },
              ],
            },
            { phase: "main", exercises: main },
            {
              phase: "cooldown",
              exercises: [
                {
                  planExerciseId: "quad",
                  exerciseId: "quad-stretch",
                  nameSnapshot: "股四头肌拉伸",
                  sets: 1,
                  target: { kind: "duration", seconds: 30 },
                  side: "both",
                },
              ],
            },
          ],
        },
      ],
      frequency: { sessionsPerWeek: 3 },
      schedule: { mode: "frequency_only" },
      constraints: [],
    },
  };
}

const BRIDGE: Exercise = {
  planExerciseId: "bridge",
  exerciseId: "glute-bridge",
  nameSnapshot: "臀桥",
  sets: 3,
  target: { kind: "repetitions", min: 12 },
  restSeconds: 45,
};
const WALL_SIT: Exercise = {
  planExerciseId: "wall-sit",
  exerciseId: "wall-sit",
  nameSnapshot: "靠墙静蹲",
  sets: 3,
  target: { kind: "duration", seconds: 30 },
  restSeconds: 60,
};
const DEAD_BUG: Exercise = {
  planExerciseId: "dead-bug",
  exerciseId: "dead-bug",
  nameSnapshot: "死虫式",
  sets: 2,
  target: { kind: "repetitions", min: 10 },
  restSeconds: 30,
};

async function register(
  page: Page,
  request: APIRequestContext,
): Promise<Headers> {
  const email = `e2e-tp-${Date.now()}-${Math.random().toString(16).slice(2)}@example.com`;
  const password = "BodySenseE2E!123";
  await page.goto("/register");
  await page.getByLabel("邮箱地址").fill(email);
  await fillRegisterPassword(page, password);
  await page.getByRole("button", { name: "创建账号" }).click();
  await page.waitForURL(/\/(dashboard|onboarding)/);
  const accessToken = await refreshBrowserAccessToken(page, apiBase);
  const headers = {
    Authorization: `Bearer ${accessToken}`,
    "Content-Type": "application/json",
  };
  const profile = await request.put(`${apiBase}/api/v1/profile`, {
    headers,
    data: profileReplacement({ gender: "female", birth_date: "1994-03-12" }),
  });
  expect(profile.ok(), await profile.text()).toBeTruthy();
  return headers;
}

/** The user's own complete draft, saved as a pending proposal (never accepted here). */
async function propose(
  request: APIRequestContext,
  headers: Headers,
  main: Exercise[],
  base?: PlanRecord,
): Promise<ProposalRecord> {
  return json<ProposalRecord>(
    await request.post(`${apiBase}/api/v1/plan-proposals`, {
      headers,
      data: {
        draft: draft(main),
        ...(base
          ? {
              planId: base.planId,
              expectedRevisionId: base.currentAcceptedRevisionId,
            }
          : {}),
        idempotencyKey: crypto.randomUUID(),
      },
    }),
    "save user PlanProposal",
  );
}

async function onlyPlan(
  request: APIRequestContext,
  headers: Headers,
): Promise<PlanRecord> {
  const { plans } = await json<{ plans: PlanRecord[] }>(
    await request.get(`${apiBase}/api/v1/training-plans`, { headers }),
    "list training plans",
  );
  expect(plans).toHaveLength(1);
  return plans[0]!;
}

async function openTraining(page: Page) {
  await page.goto("/consultation?view=training");
  const window = page.getByTestId("training-window");
  await expect(window).toBeVisible();
  return window;
}

test("TrainingPlan window: proposals change the plan only through receipts, stale 409 and gate", async ({
  page,
  request,
}) => {
  test.setTimeout(90_000);
  await page.setViewportSize({ width: 1280, height: 800 });
  const headers = await register(page, request);

  // No plan yet: the Empty board, never a demo plan.
  let window = await openTraining(page);
  await expect(window.getByTestId("training-empty")).toBeVisible();

  // A pending create proposal is the window content; accept → receipt → plan.
  await propose(request, headers, [BRIDGE, WALL_SIT]);
  window = await openTraining(page);
  await expect(window.getByTestId("training-proposal")).toBeVisible();
  await expect(window.getByText("新计划 · 日常活动计划")).toBeVisible();
  await window.getByRole("button", { name: "接受" }).click();
  await expect(window.getByTestId("training-receipt")).toHaveText(
    "已采纳新计划 · 现在生效",
  );
  await expect(
    page.getByRole("combobox", { name: "选择训练计划" }),
  ).toHaveAttribute("data-value", (await onlyPlan(request, headers)).planId);

  // A fresh account has no Safety screening: the Go TrainerGate blocks Start.
  await expect(window.getByTestId("training-gate")).toHaveText("暂不能开始");
  await expect(window.getByRole("button", { name: "开始训练" })).toBeDisabled();
  await expect(window.getByTestId("training-gate-reason")).toContainText(
    "暂不能开始",
  );

  // An update proposal: the plan shows it as pending until the user decides.
  const before = await onlyPlan(request, headers);
  await propose(
    request,
    headers,
    [
      { ...BRIDGE, target: { kind: "repetitions", min: 15 } },
      DEAD_BUG,
      WALL_SIT,
    ],
    before,
  );
  window = await openTraining(page);
  await expect(window.getByTestId("training-pending-proposal")).toContainText(
    "2 项调整建议待确认",
  );
  await expect(window.getByText("3 组 × 12 次")).toBeVisible();
  await window.getByRole("button", { name: "查看调整建议" }).click();
  const proposal = window.getByTestId("training-proposal");
  await expect(proposal.getByText("3 组 × 12 次 · 休息 45 秒")).toBeVisible();
  await expect(proposal.getByText("3 组 × 15 次 · 休息 45 秒")).toBeVisible();
  await expect(proposal.getByText("死虫式")).toBeVisible();
  // A double click is one decision.
  await window.getByRole("button", { name: "接受" }).dblclick();
  await expect(window.getByTestId("training-receipt")).toHaveText(
    "已采纳调整 · 新修订已生效",
  );
  await expect(window.getByTestId("training-exercise-change")).toHaveText([
    "已调整",
    "新增",
  ]);
  await expect(window.getByText("3 组 × 15 次")).toBeVisible();
  const after = await onlyPlan(request, headers);
  expect(after.currentAcceptedRevisionId).not.toBe(
    before.currentAcceptedRevisionId,
  );
  const { proposals } = await json<{ proposals: ProposalRecord[] }>(
    await request.get(`${apiBase}/api/v1/plan-proposals?state=accepted`, {
      headers,
    }),
    "list accepted proposals",
  );
  expect(proposals).toHaveLength(2);

  // Stale: the user is looking at a proposal when another one changes the plan.
  await propose(
    request,
    headers,
    [{ ...BRIDGE, sets: 4 }, DEAD_BUG, WALL_SIT],
    after,
  );
  window = await openTraining(page);
  await window.getByRole("button", { name: "查看调整建议" }).click();
  await expect(window.getByTestId("training-proposal")).toBeVisible();
  const elsewhere = await propose(
    request,
    headers,
    [BRIDGE, DEAD_BUG, WALL_SIT],
    after,
  );
  const fresh = await json<{ proposalId: string; contentFingerprint: string }>(
    await request.get(
      `${apiBase}/api/v1/plan-proposals/${elsewhere.proposalId}`,
      { headers },
    ),
    "read the competing proposal",
  );
  await json(
    await request.post(
      `${apiBase}/api/v1/plan-proposals/${elsewhere.proposalId}/accept`,
      {
        headers,
        data: {
          decisionId: crypto.randomUUID(),
          presentedFingerprint: fresh.contentFingerprint,
          targetPlanId: after.planId,
          expectedRevisionId: after.currentAcceptedRevisionId,
          idempotencyKey: crypto.randomUUID(),
        },
      },
    ),
    "accept the competing proposal elsewhere",
  );
  const moved = await onlyPlan(request, headers);
  await window.getByRole("button", { name: "接受" }).click();
  await expect(window.getByTestId("training-proposal-expired")).toBeVisible();
  await expect(window.getByRole("button", { name: "接受" })).toHaveCount(0);
  expect((await onlyPlan(request, headers)).currentAcceptedRevisionId).toBe(
    moved.currentAcceptedRevisionId,
  );
  await window.getByRole("button", { name: "返回计划" }).click();
  await expect(window.getByTestId("training-plan")).toBeVisible();

  // Reject writes a receipt and leaves the plan as it is.
  await propose(request, headers, [WALL_SIT], moved);
  window = await openTraining(page);
  await window.getByRole("button", { name: "查看调整建议" }).click();
  await window.getByRole("button", { name: "拒绝" }).click();
  await expect(window.getByTestId("training-rejected")).toHaveText(
    "已拒绝调整 · 计划保持原样",
  );
  expect((await onlyPlan(request, headers)).currentAcceptedRevisionId).toBe(
    moved.currentAcceptedRevisionId,
  );

  // 390px: the same boards in the mobile sheet with 44px touch targets.
  await page.setViewportSize({ width: 390, height: 844 });
  await propose(request, headers, [WALL_SIT], moved);
  window = await openTraining(page);
  await window.getByRole("button", { name: "查看调整建议" }).click();
  for (const name of ["拒绝", "接受"]) {
    const box = await window.getByRole("button", { name }).boundingBox();
    expect(box).not.toBeNull();
    expect(box!.height).toBeGreaterThanOrEqual(44);
    expect(box!.x + box!.width).toBeLessThanOrEqual(390);
  }
});
