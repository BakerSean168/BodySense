import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { expect, it, vi } from "vitest";
import { DiagnosisWindow } from "./DiagnosisWindow";
import { projectNativeDiagnosisWindow } from "../../model/diagnosisDecisionViewModel";

it("offers a real read retry on Diagnosis failure without presenting an empty diagnosis", async () => {
  const retry = vi.fn();
  render(
    <DiagnosisWindow
      view={projectNativeDiagnosisWindow({ diagnosis: undefined, run: null })}
      error="诊断读取失败"
      onRetry={retry}
    />,
  );
  expect(screen.getByRole("alert")).toHaveTextContent("诊断读取失败");
  expect(screen.queryByTestId("diagnosis-empty")).not.toBeInTheDocument();
  await userEvent.click(screen.getByRole("button", { name: "重新读取" }));
  expect(retry).toHaveBeenCalledTimes(1);
});

import {
  makeCondition,
  makeDiagnosis,
} from "../../model/__fixtures__/nativeDiagnosisFixtures";
import { DiagnosisHistoryList } from "../../components/DiagnosisHistoryPanel";

it("keeps confirmed diagnoses visible during a partial read failure", async () => {
  const retry = vi.fn();
  const condition = makeCondition();
  render(
    <DiagnosisWindow
      view={projectNativeDiagnosisWindow({
        diagnosis: makeDiagnosis({ conditionEntries: [condition] }),
        run: null,
      })}
      partialError="分析进度读取失败"
      onRetry={retry}
    />,
  );
  expect(screen.getByRole("alert")).toHaveTextContent("分析进度读取失败");
  expect(screen.getByText(condition.medicalName)).toBeVisible();
  await userEvent.click(screen.getByRole("button", { name: "重新读取" }));
  expect(retry).toHaveBeenCalledTimes(1);
});

it("retries failed history reads instead of claiming there are no decisions", async () => {
  const retry = vi.fn();
  render(
    <DiagnosisHistoryList
      entries={[]}
      inactive={[]}
      error="历史读取失败"
      onRetry={retry}
      renderCondition={() => null}
    />,
  );
  expect(
    screen.queryByTestId("diagnosis-history-empty"),
  ).not.toBeInTheDocument();
  await userEvent.click(screen.getByRole("button", { name: "重新读取" }));
  expect(retry).toHaveBeenCalledTimes(1);
});
