import { describe, expect, it } from "vitest";
import { activeNavigationItem, appNavigation } from "./appNavigation";

describe("appNavigation", () => {
  it("keeps primary destinations unique", () => {
    const hrefs = appNavigation.map((item) => item.href);
    expect(new Set(hrefs).size).toBe(hrefs.length);
  });

  it("maps nested routes to their owning section", () => {
    expect(activeNavigationItem("/assessment/report-1").label).toBe("健康评估");
    expect(activeNavigationItem("/consultation/conversation-1").label).toBe(
      "智能问诊",
    );
    expect(activeNavigationItem("/training/plan-1").label).toBe("训练执行");
  });
});
