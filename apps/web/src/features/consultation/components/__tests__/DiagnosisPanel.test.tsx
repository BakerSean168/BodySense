import { describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { DiagnosisPanel } from "../DiagnosisPanel";
import type { Citation, DiagnosisCandidate } from "../../types/consultation";

const sampleCandidates: DiagnosisCandidate[] = [
  {
    candidate_id: "11111111-1111-1111-1111-111111111111",
    concern_key: "head-neck",
    name: "上交叉综合征倾向",
    confidence: "高",
    severity: "轻度",
    evidence_strength: "中",
    basis: "肩颈酸胀、久坐后明显",
    typical_symptoms: "圆肩、头前伸",
    differential: "需与肩袖损伤区分",
    reasoning_summary:
      "当前姿态观察与长期久坐模式相符，但仍需结合后续变化验证。",
    counterevidence_ids: ["fact-negative-1"],
  },
  {
    candidate_id: "22222222-2222-2222-2222-222222222222",
    concern_key: "head-neck",
    name: "颈肩功能性紧张倾向",
    confidence: "中",
    basis: "颈部不适和活动后缓解",
    typical_symptoms: "颈部僵硬",
  },
  {
    candidate_id: "33333333-3333-3333-3333-333333333333",
    concern_key: "hip-leg",
    name: "久坐相关臀髋功能性不适倾向",
    confidence: "中",
    basis: "左臀久坐后酸胀并伴随活动模式变化",
  },
];

const sampleCitations: Citation[] = [
  {
    title: "上交叉综合征的自测与改善",
    summary: "常见于久坐低头人群",
    body_markdown: "# 上交叉综合征\n\n详细内容...",
    category: "definition",
    source_title: "体态健康手册",
    source_author: "张医生",
  },
];

describe("DiagnosisPanel", () => {
  it("renders an insufficient-information state with zero candidates", () => {
    render(
      <DiagnosisPanel
        status="insufficient_information"
        summary="还需要补充右膝活动时的具体表现。"
        candidates={[]}
      />,
    );

    expect(screen.getByText("当前信息不足以形成可靠候选")).toBeInTheDocument();
    expect(
      screen.getByText(/还需要补充右膝活动时的具体表现/),
    ).toBeInTheDocument();
  });

  it("renders a safety-blocked state with zero candidates", () => {
    render(
      <DiagnosisPanel
        status="safety_blocked"
        summary="当前存在需要优先处理的安全信号。"
        candidates={[]}
      />,
    );

    expect(screen.getByText("当前分析受安全状态限制")).toBeInTheDocument();
    expect(screen.queryByText("保存我的判断")).not.toBeInTheDocument();
  });

  it("renders all candidates without a top-3 restriction and groups by concern", () => {
    render(
      <DiagnosisPanel
        analysisId="aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
        bodyStateRevision={143}
        candidates={sampleCandidates}
        onSaveAssessments={vi.fn()}
      />,
    );

    expect(screen.getByText("BodyState R143")).toBeInTheDocument();
    expect(screen.getByText("head-neck")).toBeInTheDocument();
    expect(screen.getByText("hip-leg")).toBeInTheDocument();
    expect(screen.getByText("上交叉综合征倾向")).toBeInTheDocument();
    expect(screen.getByText("颈肩功能性紧张倾向")).toBeInTheDocument();
    expect(screen.getByText("久坐相关臀髋功能性不适倾向")).toBeInTheDocument();
  });

  it("displays confidence, severity, evidence strength, reasoning and counterevidence", () => {
    render(<DiagnosisPanel candidates={[sampleCandidates[0]]} />);

    expect(screen.getByText(/置信度：高/)).toBeInTheDocument();
    expect(screen.getByText("轻度")).toBeInTheDocument();
    expect(screen.getByText("证据：中")).toBeInTheDocument();
    expect(
      screen.getByText(/当前姿态观察与长期久坐模式相符/),
    ).toBeInTheDocument();
    expect(screen.getByText(/存在 1 项反向\/削弱证据/)).toBeInTheDocument();
  });

  it("lets the user assess candidates independently and saves only assessed candidates", async () => {
    const onSave = vi.fn().mockResolvedValue(undefined);
    const user = userEvent.setup();

    render(
      <DiagnosisPanel
        analysisId="aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
        candidates={sampleCandidates}
        onSaveAssessments={onSave}
      />,
    );

    const saveButton = screen.getByRole("button", { name: "保存我的判断" });
    expect(saveButton).toBeDisabled();

    const confirmedButtons = screen.getAllByRole("button", {
      name: "✓ 符合我的情况",
    });
    const unsureButtons = screen.getAllByRole("button", { name: "? 不确定" });

    await user.click(confirmedButtons[0]);
    await user.click(unsureButtons[2]);

    expect(saveButton).not.toBeDisabled();
    await user.click(saveButton);

    expect(onSave).toHaveBeenCalledWith([
      {
        candidate_id: "11111111-1111-1111-1111-111111111111",
        state: "confirmed",
      },
      {
        candidate_id: "33333333-3333-3333-3333-333333333333",
        state: "unsure",
      },
    ]);
  });

  it("supports changing a candidate assessment before saving", async () => {
    const onSave = vi.fn().mockResolvedValue(undefined);
    const user = userEvent.setup();

    render(
      <DiagnosisPanel
        analysisId="aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
        candidates={[sampleCandidates[0]]}
        onSaveAssessments={onSave}
      />,
    );

    await user.click(screen.getByRole("button", { name: "? 不确定" }));
    await user.click(screen.getByRole("button", { name: "○ 暂不符合" }));
    await user.click(screen.getByRole("button", { name: "保存我的判断" }));

    expect(onSave).toHaveBeenCalledWith([
      {
        candidate_id: "11111111-1111-1111-1111-111111111111",
        state: "not_applicable",
      },
    ]);
  });

  it("shows saving state without coupling diagnosis assessment to treatment generation", () => {
    render(
      <DiagnosisPanel
        analysisId="aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
        candidates={[sampleCandidates[0]]}
        onSaveAssessments={vi.fn()}
        isSavingAssessments
      />,
    );

    expect(screen.getByRole("button", { name: "保存中..." })).toBeDisabled();
    expect(screen.queryByText(/生成改善方案/)).not.toBeInTheDocument();
  });

  it("renders and expands diagnosis citations", async () => {
    const user = userEvent.setup();
    render(
      <DiagnosisPanel
        candidates={[sampleCandidates[0]]}
        citations={sampleCitations}
      />,
    );

    expect(screen.getByText("📚 参考来源")).toBeInTheDocument();
    expect(screen.getByText("上交叉综合征的自测与改善")).toBeInTheDocument();

    await user.click(
      screen.getByRole("button", { name: /上交叉综合征的自测与改善/ }),
    );
    expect(screen.getByText(/常见于久坐低头人群/)).toBeInTheDocument();
  });

  it("renders the non-medical-diagnosis disclaimer", () => {
    render(<DiagnosisPanel candidates={[sampleCandidates[0]]} />);
    expect(screen.getByText(/不构成医疗诊断/)).toBeInTheDocument();
  });
});
