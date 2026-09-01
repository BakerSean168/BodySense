import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { OCRResultView } from "./OCRResultView";
import type { OCRResult } from "../../types/upload.types";

function resultWithIndicator(
  overrides: Partial<OCRResult["indicators"][number]> = {},
): OCRResult {
  return {
    raw_text: "Vitamin D 25.3 ng/mL",
    confidence: "high",
    indicators: [
      {
        name: "Vitamin D",
        value: "25.3",
        unit: "ng/mL",
        reference_range: "30-100",
        confidence: "high",
        ...overrides,
      },
    ],
  };
}

describe("OCRResultView", () => {
  it("fails historical indicators without admissibility metadata closed to review", () => {
    render(<OCRResultView result={resultWithIndicator()} />);

    expect(screen.getByText("OCR 识别候选")).toBeInTheDocument();
    expect(screen.getByText(/0 项可用于评估，1 项待复核/)).toBeInTheDocument();
    expect(screen.getByText("待复核")).toBeInTheDocument();
  });

  it("shows only explicitly admissible indicators as usable by Assessment", () => {
    render(
      <OCRResultView
        result={resultWithIndicator({
          evidence_admissibility: {
            status: "admissible",
            policy_revision: "ocr-indicator-admissibility-v1",
            reason_codes: ["high_confidence_ocr_and_indicator"],
          },
        })}
      />,
    );

    expect(screen.getByText(/1 项可用于评估/)).toBeInTheDocument();
    expect(screen.getByText("可用于评估")).toBeInTheDocument();
    expect(
      screen.getByText(/不代表医学正常\/异常或已经由你确认/),
    ).toBeInTheDocument();
  });
});
