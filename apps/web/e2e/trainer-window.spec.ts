import {
  expect,
  test,
  type APIRequestContext,
  type APIResponse,
  type Locator,
  type Page,
} from "@playwright/test";
import { refreshBrowserAccessToken } from "./support/auth";
import { profileReplacement } from "./support/profile";
import { completeStructuredSafetyIntake } from "./support/safety";

const apiBase = process.env.E2E_API_BASE_URL || "http://127.0.0.1:8080";

type Headers = Record<string, string>;

interface SessionAction {
  actionId: string;
  planExerciseId: string;
  scope: "set" | "exercise";
  setNumber?: number;
  kind: "performed" | "skipped" | "corrected";
  correctsActionId?: string;
  actualReps?: number;
}

interface SessionRecord {
  sessionId: string;
  state: string;
  endedReason?: string;
  actions: SessionAction[];
  feedback?: { text: string };
}

async function json<T>(response: APIResponse, label: string): Promise<T> {
  const body = await response.text();
  expect(
    response.ok(),
    `${label} failed with ${response.status()}: ${body}`,
  ).toBeTruthy();
  return JSON.parse(body) as T;
}

// No rest periods, so the journey never waits on a countdown.
const DRAFT = {
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
          {
            phase: "main",
            exercises: [
              {
                planExerciseId: "bridge",
                exerciseId: "glute-bridge",
                nameSnapshot: "臀桥",
                sets: 2,
                target: { kind: "repetitions", min: 12 },
              },
              {
                planExerciseId: "wall-sit",
                exerciseId: "wall-sit",
                nameSnapshot: "靠墙静蹲",
                sets: 2,
                target: { kind: "duration", seconds: 30 },
              },
            ],
          },
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

async function register(
  page: Page,
  request: APIRequestContext,
): Promise<Headers> {
  const email = `e2e-tr-${Date.now()}-${Math.random().toString(16).slice(2)}@example.com`;
  const password = "BodySenseE2E!123";
  await page.goto("/register");
  await page.getByLabel("邮箱地址").fill(email);
  await page.getByLabel("密码", { exact: true }).fill(password);
  await page.getByRole("checkbox", { name: /同意/ }).check();
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

/** The structured Safety screening, so the Go TrainerGate permits training. */
async function screenSafety(request: APIRequestContext, headers: Headers) {
  const opening = await request.post(`${apiBase}/api/v1/consultation-runs`, {
    headers,
    data: {
      conversationId: null,
      clientMessageId: `o-${crypto.randomUUID()}`,
      requestId: crypto.randomUUID(),
      message: { role: "user", parts: [{ type: "text", text: "你好" }] },
    },
    timeout: 60_000,
  });
  expect(opening.ok(), await opening.text()).toBeTruthy();
  const conversation = await json<{ conversation: { id: string } }>(
    await request.get(`${apiBase}/api/v1/conversation`, { headers }),
    "read conversation",
  );
  await completeStructuredSafetyIntake(
    request,
    apiBase,
    headers,
    conversation.conversation.id,
  );
}

async function onlySession(
  request: APIRequestContext,
  headers: Headers,
): Promise<SessionRecord> {
  const { sessions } = await json<{ sessions: SessionRecord[] }>(
    await request.get(`${apiBase}/api/v1/training-sessions`, { headers }),
    "list training sessions",
  );
  expect(sessions).toHaveLength(1);
  return sessions[0]!;
}

/** Close windows until the bare body canvas (Home) shows. */
async function goHome(page: Page) {
  for (let attempt = 0; attempt < 4; attempt += 1) {
    if (
      (await page
        .locator(".bc-business-window:visible, .bc-business-sheet:visible")
        .count()) === 0
    )
      return;
    await page.keyboard.press("Escape");
  }
}

async function expectTouchTarget(target: Locator, width: number) {
  const box = await target.boundingBox();
  expect(box).not.toBeNull();
  expect(box!.height).toBeGreaterThanOrEqual(44);
  expect(box!.x).toBeGreaterThanOrEqual(0);
  expect(box!.x + box!.width).toBeLessThanOrEqual(width);
}

test("Trainer: corrections, exercise skip, Home chip, feedback and 390px", async ({
  page,
  request,
}) => {
  test.setTimeout(150_000);
  await page.setViewportSize({ width: 1280, height: 800 });
  const headers = await register(page, request);
  await screenSafety(request, headers);
  const proposal = await json<{ proposalId: string }>(
    await request.post(`${apiBase}/api/v1/plan-proposals`, {
      headers,
      data: { draft: DRAFT, idempotencyKey: crypto.randomUUID() },
    }),
    "save user PlanProposal",
  );
  expect(proposal.proposalId).toBeTruthy();

  await page.goto("/consultation?view=training");
  const plan = page.getByTestId("training-window");
  await plan.getByRole("button", { name: "接受" }).click();
  await expect(plan.getByTestId("training-receipt")).toBeVisible();
  await plan.getByRole("button", { name: "开始训练" }).click();
  const trainer = page.getByTestId("trainer-session-panel");
  await expect(trainer.getByTestId("trainer-in-progress")).toBeVisible();

  // A double tap is one receipt.
  await trainer.getByRole("button", { name: "完成本组" }).dblclick();
  await expect(trainer.getByRole("heading", { name: "臀桥" })).toBeVisible();
  expect((await onlySession(request, headers)).actions).toHaveLength(1);

  // Record 13, then correct it to 11: retract (after confirming) and re-record.
  await trainer.getByRole("button", { name: "增加" }).click();
  await trainer.getByRole("button", { name: "完成本组" }).click();
  const receipt = trainer.getByTestId("trainer-last-receipt");
  await expect(receipt).toContainText("第 1 组已记录 · 13 次");
  await receipt.getByRole("button", { name: "更正" }).click();
  await expect(trainer.getByTestId("trainer-correction")).toContainText(
    "撤回「臀桥 · 第 1 组」后重新记录？",
  );
  await trainer.getByRole("button", { name: "撤回重记" }).click();
  await expect(receipt).toContainText("第 1 组已撤回 · 请重新记录");
  await expect(trainer.getByLabel("实际次数")).toHaveText("13");
  await trainer.getByRole("button", { name: "减少" }).click();
  await trainer.getByRole("button", { name: "减少" }).click();
  await trainer.getByRole("button", { name: "完成本组" }).click();
  await expect(receipt).toContainText("第 1 组已记录 · 11 次");
  let record = await onlySession(request, headers);
  const bridge = record.actions.filter((a) => a.planExerciseId === "bridge");
  expect(bridge.map((a) => a.kind)).toEqual([
    "performed",
    "corrected",
    "performed",
  ]);
  expect(bridge[1]!.correctsActionId).toBe(bridge[0]!.actionId);
  expect(bridge[0]!.actualReps).toBe(13);
  expect(bridge[2]!.actualReps).toBe(11);

  // One set skipped, then the whole next exercise as one receipt.
  await trainer.getByRole("button", { name: "跳过本组" }).click();
  await expect(
    trainer.getByRole("heading", { name: "靠墙静蹲" }),
  ).toBeVisible();
  await trainer.getByRole("button", { name: "跳过此动作：靠墙静蹲" }).click();
  await expect(
    trainer.getByRole("heading", { name: "股四头肌拉伸" }),
  ).toBeVisible();
  await expect(receipt).toContainText("靠墙静蹲已跳过");
  record = await onlySession(request, headers);
  expect(
    record.actions.filter((a) => a.planExerciseId === "wall-sit"),
  ).toMatchObject([{ scope: "exercise", kind: "skipped" }]);

  // Pause, go Home: the chip says paused and reopens the same session.
  await trainer.getByRole("button", { name: "暂停训练" }).click();
  await expect(trainer.getByTestId("trainer-recovery")).toBeVisible();
  await goHome(page);
  const chip = page.getByTestId("home-trainer-chip");
  await expect(chip).toHaveAttribute("data-state", "paused");
  await expect(chip).toContainText("训练已暂停");
  await chip.click();
  await expect(trainer.getByTestId("trainer-recovery")).toBeVisible();
  await trainer.getByRole("button", { name: "继续" }).click();
  await expect(trainer.getByTestId("trainer-in-progress")).toBeVisible();
  await goHome(page);
  await expect(chip).toHaveAttribute("data-state", "in_progress");
  await expect(chip).toContainText("训练进行中");

  // 390px: the chip and the Trainer actions stay on screen with 44px targets.
  await page.setViewportSize({ width: 390, height: 844 });
  await expectTouchTarget(chip, 390);
  await chip.click();
  await expect(trainer.getByTestId("trainer-in-progress")).toBeVisible();
  for (const name of ["完成本组", "暂停训练", "跳过此动作：股四头肌拉伸"]) {
    await expectTouchTarget(trainer.getByRole("button", { name }), 390);
  }
  await page.setViewportSize({ width: 1280, height: 800 });
  expect((await onlySession(request, headers)).state).toBe("in_progress");

  // Finish: the Ended board, then 「说说感受」 posts the user's words once.
  await trainer.getByRole("button", { name: "完成本组" }).click();
  await trainer.getByRole("button", { name: "结束训练" }).click();
  await expect(trainer.getByTestId("trainer-ended")).toContainText(
    "按计划完成",
  );
  await expect(trainer.getByTestId("trainer-recorded")).toContainText(
    "臀桥1 / 2 组",
  );
  await trainer.getByRole("button", { name: "说说感受" }).click();
  const words = "臀桥第二组膝盖有点酸，静蹲没做。";
  await trainer.getByLabel("这次训练感觉怎么样？").fill(words);
  await trainer.getByRole("button", { name: "发送" }).click();
  await expect(trainer.getByTestId("trainer-feedback")).toContainText(words);
  await expect(trainer.getByRole("button", { name: "说说感受" })).toHaveCount(
    0,
  );
  record = await onlySession(request, headers);
  expect(record).toMatchObject({
    state: "ended",
    endedReason: "finished",
    feedback: { text: words },
  });
  await page.setViewportSize({ width: 390, height: 844 });
  await expectTouchTarget(trainer.getByRole("button", { name: "完成" }), 390);
  await goHome(page);
  await expect(page.getByTestId("home-trainer-chip")).toHaveCount(0);
});
