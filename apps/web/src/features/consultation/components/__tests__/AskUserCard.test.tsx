import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { AskUserCard } from '../AskUserCard';
import type { AskUserQuestion } from '../../types/consultation';

const textQuestion: AskUserQuestion = {
  question: '你的年龄是多少？',
  answer_type: 'text',
  required: true,
  context: '用于评估体态问题',
};

const singleChoiceQuestion: AskUserQuestion = {
  question: '你的主要不适部位？',
  answer_type: 'single_choice',
  options: ['肩颈', '腰背', '膝踝'],
  required: true,
};

const multiChoiceQuestion: AskUserQuestion = {
  question: '你有哪些症状？',
  answer_type: 'multi_choice',
  options: ['疼痛', '僵硬', '麻木', '无力'],
  required: true,
};

const numberQuestion: AskUserQuestion = {
  question: '你的身高(cm)？',
  answer_type: 'number',
  required: true,
};

describe('AskUserCard', () => {
  it('renders question text', () => {
    render(<AskUserCard question={textQuestion} onSubmit={vi.fn()} />);
    expect(screen.getByText('你的年龄是多少？')).toBeDefined();
  });

  it('renders context when provided', () => {
    render(<AskUserCard question={textQuestion} onSubmit={vi.fn()} />);
    expect(screen.getByText('用于评估体态问题')).toBeDefined();
  });

  it('renders text input for text answer type', () => {
    render(<AskUserCard question={textQuestion} onSubmit={vi.fn()} />);
    expect(screen.getByPlaceholderText('输入你的回答...')).toBeDefined();
  });

  it('renders radio buttons for single_choice', () => {
    render(<AskUserCard question={singleChoiceQuestion} onSubmit={vi.fn()} />);
    expect(screen.getByText('肩颈')).toBeDefined();
    expect(screen.getByText('腰背')).toBeDefined();
    expect(screen.getByText('膝踝')).toBeDefined();
  });

  it('renders checkboxes for multi_choice', () => {
    render(<AskUserCard question={multiChoiceQuestion} onSubmit={vi.fn()} />);
    expect(screen.getByText('疼痛')).toBeDefined();
    expect(screen.getByText('僵硬')).toBeDefined();
    expect(screen.getByText('麻木')).toBeDefined();
    expect(screen.getByText('无力')).toBeDefined();
  });

  it('renders number input for number answer type', () => {
    render(<AskUserCard question={numberQuestion} onSubmit={vi.fn()} />);
    const input = screen.getByPlaceholderText('输入数字...');
    expect(input).toBeDefined();
    expect(input.getAttribute('type')).toBe('number');
  });

  it('calls onSubmit with text answer', async () => {
    const onSubmit = vi.fn();
    const user = userEvent.setup();
    render(<AskUserCard question={textQuestion} onSubmit={onSubmit} />);

    const input = screen.getByPlaceholderText('输入你的回答...');
    await user.type(input, '25');
    await user.click(screen.getByText('提交'));

    expect(onSubmit).toHaveBeenCalledWith({ text: '25' });
  });

  it('calls onSubmit with single choice answer', async () => {
    const onSubmit = vi.fn();
    const user = userEvent.setup();
    render(<AskUserCard question={singleChoiceQuestion} onSubmit={onSubmit} />);

    await user.click(screen.getByLabelText('肩颈'));
    await user.click(screen.getByText('提交'));

    expect(onSubmit).toHaveBeenCalledWith({ text: '肩颈', selected: ['肩颈'] });
  });

  it('calls onSubmit with multi choice answer', async () => {
    const onSubmit = vi.fn();
    const user = userEvent.setup();
    render(<AskUserCard question={multiChoiceQuestion} onSubmit={onSubmit} />);

    await user.click(screen.getByLabelText('疼痛'));
    await user.click(screen.getByLabelText('麻木'));
    await user.click(screen.getByText('提交'));

    expect(onSubmit).toHaveBeenCalledWith({ text: '疼痛, 麻木', selected: ['疼痛', '麻木'] });
  });

  it('disables submit button when isSubmitting', () => {
    render(<AskUserCard question={textQuestion} onSubmit={vi.fn()} isSubmitting={true} />);
    const button = screen.getByText('提交中...');
    expect(button).toBeDefined();
    expect(button.hasAttribute('disabled')).toBe(true);
  });

  it('shows error message when error prop is provided', () => {
    render(
      <AskUserCard
        question={textQuestion}
        onSubmit={vi.fn()}
        error="网络错误"
      />
    );
    expect(screen.getByText('提交失败：网络错误')).toBeDefined();
  });

  it('shows retry button when error and onRetry provided', async () => {
    const onRetry = vi.fn();
    const user = userEvent.setup();
    render(
      <AskUserCard
        question={textQuestion}
        onSubmit={vi.fn()}
        error="失败"
        onRetry={onRetry}
      />
    );

    await user.click(screen.getByText('重试'));
    expect(onRetry).toHaveBeenCalled();
  });

  it('does not submit empty text', async () => {
    const onSubmit = vi.fn();
    const user = userEvent.setup();
    render(<AskUserCard question={textQuestion} onSubmit={onSubmit} />);

    await user.click(screen.getByText('提交'));
    expect(onSubmit).not.toHaveBeenCalled();
  });
});
