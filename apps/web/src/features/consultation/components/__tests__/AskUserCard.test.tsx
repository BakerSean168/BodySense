import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { AskUserCard } from "../AskUserCard";
import type { AskUserQuestion } from "../../types/consultation";

const textQuestion: AskUserQuestion = {
  question: "你的年龄是多少？",
  answer_type: "text",
  required: true,
  context: "用于评估体态问题",
};

const singleChoiceQuestion: AskUserQuestion = {
  question: "你的主要不适部位？",
  answer_type: "single_choice",
  options: ["肩颈", "腰背", "膝踝"],
  allow_custom_input: true,
  required: true,
};

const multiChoiceQuestion: AskUserQuestion = {
  question: "你有哪些症状？",
  answer_type: "multi_choice",
  options: ["疼痛", "僵硬", "麻木", "无力"],
  allow_custom_input: true,
  required: true,
};

const numberQuestion: AskUserQuestion = {
  question: "你的身高(cm)？",
  answer_type: "number",
  required: true,
};

describe("AskUserCard", () => {
  it("does not offer unadvertised custom choices", () => {
    render(
      <AskUserCard
        question={{
          question: "Which side?",
          answer_type: "single_choice",
          options: ["left", "right"],
        }}
        onSubmit={vi.fn()}
      />,
    );
    expect(screen.queryByText("自定义输入")).toBeNull();
  });
  it("does not silently omit later required fields from the question", async () => {
    const onSubmit = vi.fn();
    const user = userEvent.setup();
    render(
      <AskUserCard
        question={{
          question: "Details",
          answer_type: "text",
          fields: [1, 2, 3, 4].map((i) => ({
            key: `f${i}`,
            label: `Field ${i}`,
            answer_type: "text",
            required: true,
          })),
        }}
        onSubmit={onSubmit}
      />,
    );
    for (const i of [1, 2, 3, 4])
      await user.type(screen.getByPlaceholderText(`Field ${i}`), `Answer ${i}`);
    await user.click(screen.getByText("提交"));
    expect(onSubmit).toHaveBeenCalledWith({
      text: "Field 1: Answer 1；Field 2: Answer 2；Field 3: Answer 3；Field 4: Answer 4",
      fields: {
        f1: "Answer 1",
        f2: "Answer 2",
        f3: "Answer 3",
        f4: "Answer 4",
      },
    });
  });

  it("labels the response control and scopes choice groups to each question", () => {
    const { container } = render(
      <>
        <AskUserCard question={textQuestion} onSubmit={vi.fn()} />
        <AskUserCard question={singleChoiceQuestion} onSubmit={vi.fn()} />
        <AskUserCard question={singleChoiceQuestion} onSubmit={vi.fn()} />
      </>,
    );
    expect(
      screen.getByRole("textbox", { name: textQuestion.question }),
    ).toBeDefined();
    const cards = container.querySelectorAll(".ask-user-card");
    expect(cards).toHaveLength(3);
    const firstName = cards[1]
      .querySelector('input[type="radio"]')
      ?.getAttribute("name");
    const secondName = cards[2]
      .querySelector('input[type="radio"]')
      ?.getAttribute("name");
    expect(firstName).toBeTruthy();
    expect(secondName).not.toBe(firstName);
  });

  it("renders question text", () => {
    render(<AskUserCard question={textQuestion} onSubmit={vi.fn()} />);
    expect(screen.getByText("你的年龄是多少？")).toBeDefined();
  });

  it("renders context when provided", () => {
    render(<AskUserCard question={textQuestion} onSubmit={vi.fn()} />);
    expect(screen.getByText("用于评估体态问题")).toBeDefined();
  });

  it("renders text input for text answer type", () => {
    render(<AskUserCard question={textQuestion} onSubmit={vi.fn()} />);
    expect(screen.getByPlaceholderText("输入你的回答...")).toBeDefined();
  });

  it("renders radio buttons for single_choice", () => {
    render(<AskUserCard question={singleChoiceQuestion} onSubmit={vi.fn()} />);
    expect(screen.getByText("肩颈")).toBeDefined();
    expect(screen.getByText("腰背")).toBeDefined();
    expect(screen.getByText("膝踝")).toBeDefined();
  });

  it("renders checkboxes for multi_choice", () => {
    render(<AskUserCard question={multiChoiceQuestion} onSubmit={vi.fn()} />);
    expect(screen.getByText("疼痛")).toBeDefined();
    expect(screen.getByText("僵硬")).toBeDefined();
    expect(screen.getByText("麻木")).toBeDefined();
    expect(screen.getByText("无力")).toBeDefined();
  });

  it("renders number input for number answer type", () => {
    render(<AskUserCard question={numberQuestion} onSubmit={vi.fn()} />);
    const input = screen.getByPlaceholderText("输入数字...");
    expect(input).toBeDefined();
    expect(input.getAttribute("type")).toBe("number");
  });

  it("calls onSubmit with text answer", async () => {
    const onSubmit = vi.fn();
    const user = userEvent.setup();
    render(<AskUserCard question={textQuestion} onSubmit={onSubmit} />);

    const input = screen.getByPlaceholderText("输入你的回答...");
    await user.type(input, "25");
    await user.click(screen.getByText("提交"));

    expect(onSubmit).toHaveBeenCalledWith({ text: "25" });
  });

  it("calls onSubmit with single choice answer", async () => {
    const onSubmit = vi.fn();
    const user = userEvent.setup();
    render(<AskUserCard question={singleChoiceQuestion} onSubmit={onSubmit} />);

    await user.click(screen.getByLabelText("肩颈"));
    await user.click(screen.getByText("提交"));

    expect(onSubmit).toHaveBeenCalledWith({ text: "肩颈", selected: ["肩颈"] });
  });

  it("calls onSubmit with multi choice answer", async () => {
    const onSubmit = vi.fn();
    const user = userEvent.setup();
    render(<AskUserCard question={multiChoiceQuestion} onSubmit={onSubmit} />);

    await user.click(screen.getByLabelText("疼痛"));
    await user.click(screen.getByLabelText("麻木"));
    await user.click(screen.getByText("提交"));

    expect(onSubmit).toHaveBeenCalledWith({
      text: "疼痛，麻木",
      selected: ["疼痛", "麻木"],
    });
  });

  it("supports custom input for single choice questions", async () => {
    const onSubmit = vi.fn();
    const user = userEvent.setup();
    render(<AskUserCard question={singleChoiceQuestion} onSubmit={onSubmit} />);

    await user.click(screen.getByLabelText("自定义输入"));
    await user.type(
      screen.getByPlaceholderText("请输入你的补充回答..."),
      "偶尔胸闷",
    );
    await user.click(screen.getByText("提交"));

    expect(onSubmit).toHaveBeenCalledWith({
      text: "偶尔胸闷",
      selected: ["偶尔胸闷"],
      is_custom: true,
    });
  });

  it("supports mixed preset and custom answers for multi choice questions", async () => {
    const onSubmit = vi.fn();
    const user = userEvent.setup();
    render(<AskUserCard question={multiChoiceQuestion} onSubmit={onSubmit} />);

    await user.click(screen.getByLabelText("疼痛"));
    await user.click(screen.getByLabelText("自定义输入"));
    await user.type(
      screen.getByPlaceholderText("请输入你的补充回答..."),
      "头晕",
    );
    await user.click(screen.getByText("提交"));

    expect(onSubmit).toHaveBeenCalledWith({
      text: "疼痛，头晕",
      selected: ["疼痛", "头晕"],
      is_custom: true,
    });
  });

  it("submits structured symptom intake fields without using the ordinary composer", async () => {
    const onSubmit = vi.fn();
    const user = userEvent.setup();
    render(
      <AskUserCard
        question={{
          question: "请补全症状信息",
          answer_type: "text",
          required: true,
          fields: [
            {
              key: "duration",
              label: "大约持续多久了？",
              answer_type: "single_choice",
              options: ["2–7天", "1–4周"],
              required: true,
            },
            {
              key: "severity",
              label: "最明显时有多严重？",
              answer_type: "single_choice",
              options: ["轻微", "中度"],
              required: true,
            },
          ],
        }}
        onSubmit={onSubmit}
      />,
    );

    await user.click(screen.getByLabelText("1–4周"));
    await user.click(screen.getByLabelText("中度"));
    await user.click(screen.getByText("提交"));

    expect(onSubmit).toHaveBeenCalledWith({
      text: "大约持续多久了？: 1–4周；最明显时有多严重？: 中度",
      fields: { duration: "1–4周", severity: "中度" },
    });
  });

  it("submits safety checklist selections as a list in the bound form", async () => {
    const onSubmit = vi.fn();
    const user = userEvent.setup();
    render(
      <AskUserCard
        question={{
          question: "请补全症状信息",
          answer_type: "text",
          required: true,
          fields: [
            {
              key: "safety_signals",
              label: "请选择所有安全信号",
              answer_type: "multi_choice",
              options: [
                "外伤或创伤",
                "放射痛",
                "麻木",
                "无力",
                "头晕",
                "以上均无",
              ],
              exclusive_options: ["以上均无"],
              required: true,
            },
          ],
        }}
        onSubmit={onSubmit}
      />,
    );
    await user.click(screen.getByLabelText("放射痛"));
    await user.click(screen.getByLabelText("麻木"));
    await user.click(screen.getByText("提交"));
    expect(onSubmit).toHaveBeenCalledWith({
      text: "请选择所有安全信号: 放射痛，麻木",
      fields: { safety_signals: ["放射痛", "麻木"] },
    });
  });

  it.each([
    { order: ["麻木", "以上均无"], expected: ["以上均无"] },
    { order: ["以上均无", "麻木"], expected: ["麻木"] },
  ])(
    "keeps exclusive field options mutually exclusive: $order",
    async ({ order, expected }) => {
      const onSubmit = vi.fn();
      const user = userEvent.setup();
      render(
        <AskUserCard
          question={{
            question: "请选择",
            answer_type: "text",
            fields: [
              {
                key: "signals",
                label: "信号",
                answer_type: "multi_choice",
                options: ["麻木", "以上均无"],
                exclusive_options: ["以上均无"],
              },
            ],
          }}
          onSubmit={onSubmit}
        />,
      );
      for (const option of order) {
        await user.click(screen.getByLabelText(option));
      }
      expect((screen.getByLabelText("麻木") as HTMLInputElement).checked).toBe(
        expected.includes("麻木"),
      );
      expect(
        (screen.getByLabelText("以上均无") as HTMLInputElement).checked,
      ).toBe(expected.includes("以上均无"));
      await user.click(screen.getByText("提交"));
      expect(onSubmit).toHaveBeenCalledWith({
        text: `信号: ${expected.join("，")}`,
        fields: { signals: expected },
      });
    },
  );

  it("disables submit button when isSubmitting", () => {
    render(
      <AskUserCard
        question={textQuestion}
        onSubmit={vi.fn()}
        isSubmitting={true}
      />,
    );
    const button = screen.getByText("提交中...");
    expect(button).toBeDefined();
    expect(button.hasAttribute("disabled")).toBe(true);
  });

  it("shows error message when error prop is provided", () => {
    render(
      <AskUserCard
        question={textQuestion}
        onSubmit={vi.fn()}
        error="网络错误"
      />,
    );
    expect(screen.getByText("提交失败：网络错误")).toBeDefined();
  });

  it("shows retry button when error and onRetry provided", async () => {
    const onRetry = vi.fn();
    const user = userEvent.setup();
    render(
      <AskUserCard
        question={textQuestion}
        onSubmit={vi.fn()}
        error="失败"
        onRetry={onRetry}
      />,
    );

    await user.click(screen.getByText("重试"));
    expect(onRetry).toHaveBeenCalled();
  });

  it("disables retry while an answer is pending", async () => {
    const retry = vi.fn();
    render(
      <AskUserCard
        question={textQuestion}
        onSubmit={vi.fn()}
        error="失败"
        onRetry={retry}
        isSubmitting
      />,
    );
    expect(screen.getByRole("button", { name: "重试" })).toBeDisabled();
    await userEvent.click(screen.getByRole("button", { name: "重试" }));
    expect(retry).not.toHaveBeenCalled();
  });

  it("does not submit empty text", async () => {
    const onSubmit = vi.fn();
    const user = userEvent.setup();
    render(<AskUserCard question={textQuestion} onSubmit={onSubmit} />);

    await user.click(screen.getByText("提交"));
    expect(onSubmit).not.toHaveBeenCalled();
  });
});
