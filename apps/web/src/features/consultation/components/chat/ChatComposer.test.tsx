import { createRef } from "react";
import { describe, expect, it, vi } from "vitest";
import { fireEvent, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { ChatInputArea } from "./ChatComposer";

function setup(overrides: Partial<Parameters<typeof ChatInputArea>[0]> = {}) {
  const send = vi.fn();
  const onKeyDown = vi.fn();
  const setInputText = vi.fn();
  const props: Parameters<typeof ChatInputArea>[0] = {
    inputText: "",
    setInputText,
    isComposerLocked: false,
    handleKeyDown: onKeyDown,
    handleSend: send,
    textareaRef: createRef<HTMLTextAreaElement>(),
    pendingImages: [],
    onAddImages: vi.fn(),
    onRemoveImage: vi.fn(),
    isUploadingImage: false,
    ...overrides,
  };
  const result = render(<ChatInputArea {...props} />);
  return { ...result, send, onKeyDown, setInputText };
}

describe("Chat composer", () => {
  it("keeps Send disabled with no content, when locked or while uploading", async () => {
    const { rerender, send } = setup();
    const button = screen.getByRole("button", { name: "发送" });
    expect(button).toBeDisabled();
    const props: Parameters<typeof ChatInputArea>[0] = {
      inputText: "有新的身体感受",
      setInputText: vi.fn(),
      isComposerLocked: false,
      handleKeyDown: vi.fn(),
      handleSend: send,
      textareaRef: createRef<HTMLTextAreaElement>(),
      pendingImages: [],
      onAddImages: vi.fn(),
      onRemoveImage: vi.fn(),
      isUploadingImage: true,
    };
    rerender(<ChatInputArea {...props} />);
    expect(button).toBeDisabled();
    rerender(<ChatInputArea {...props} isComposerLocked />);
    expect(button).toBeDisabled();
  });

  it("uses the caller's send and keyboard handlers without directly invoking an API", async () => {
    const user = userEvent.setup();
    const { send, onKeyDown, setInputText } = setup({ inputText: "肩膀不适" });
    await user.click(screen.getByRole("button", { name: "发送" }));
    expect(send).toHaveBeenCalledTimes(1);
    const textbox = screen.getByPlaceholderText(
      "和 BodySense 说说你的身体感受…",
    );
    fireEvent.keyDown(textbox, { key: "Enter", code: "Enter" });
    expect(onKeyDown).toHaveBeenCalledTimes(1);
    await user.type(textbox, "测试");
    expect(setInputText).toHaveBeenCalled();
  });
});
