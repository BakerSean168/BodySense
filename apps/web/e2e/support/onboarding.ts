import { expect, type Page } from "@playwright/test";

/**
 * Walks the Penpot "02 Onboarding" flow (16 one-question steps) in the browser.
 * Shared by specs that onboard through the UI, so an onboarding change lands here.
 * Single-choice rows advance on click; date / height / weight and the multi-select
 * chips use 继续; the last step submits with 完成. Optional `onStep` runs on each
 * step after its question renders (used for screenshots).
 */
export async function completeOnboarding(
  page: Page,
  onStep?: (step: number) => Promise<void>,
): Promise<void> {
  const root = page.getByTestId("onboarding-page");
  const atStep = async (step: number) => {
    await expect(root).toHaveAttribute("data-step", String(step));
    if (onStep) await onStep(step);
  };
  const choose = (name: string) =>
    page.getByRole("button", { name, exact: true }).click();
  const next = () => page.getByRole("button", { name: "继续" }).click();

  await atStep(1);
  await choose("男");
  await atStep(2);
  await page.getByLabel("出生日期").fill("1995-06-15");
  await next();
  await atStep(3);
  await page.getByLabel("身高（cm）").fill("170");
  await next();
  await atStep(4);
  await page.getByLabel("体重（kg）").fill("65");
  await next();
  await atStep(5);
  await choose("颈部");
  await choose("腰部");
  await next();
  await atStep(6);
  await choose("无");
  await atStep(7);
  await choose("久坐办公");
  await atStep(8);
  await choose("4–6 小时");
  await atStep(9);
  await choose("半小时–1 小时");
  await atStep(10);
  await choose("作息比较规律");
  await next();
  await atStep(11);
  await choose("有规律地锻炼");
  await atStep(12);
  await choose("跑步 / 快走");
  await atStep(13);
  await choose("偶尔");
  await atStep(14);
  await choose("规律三餐");
  await atStep(15);
  await choose("都不碰");
  await atStep(16);
  await page.getByRole("button", { name: "完成", exact: true }).click();
}
