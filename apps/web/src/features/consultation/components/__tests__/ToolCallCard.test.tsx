/**
 * Tests for ToolCallCard — UI defense layer dedup and rendering.
 */

import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { ToolCallCard } from '../ToolCallCard';
import type { ToolCallInfo } from '../../types/consultation';

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

function makeToolCall(overrides: Partial<ToolCallInfo> = {}): ToolCallInfo {
  return {
    id: 'tc-1',
    tool: 'search_knowledge',
    args: { query: 'back pain' },
    status: 'running',
    ...overrides,
  };
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

describe('ToolCallCard', () => {
  it('renders nothing when toolCalls is empty', () => {
    const { container } = render(<ToolCallCard toolCalls={[]} />);
    expect(container.firstChild).toBeNull();
  });

  it('renders tool label for search_knowledge', () => {
    render(<ToolCallCard toolCalls={[makeToolCall()]} />);
    expect(screen.getByText('搜索知识库')).toBeDefined();
  });

  it('renders tool label for extract_symptom_info', () => {
    render(
      <ToolCallCard
        toolCalls={[makeToolCall({ tool: 'extract_symptom_info', args: { body_part: 'neck' } })]}
      />,
    );
    expect(screen.getByText('提取症状信息')).toBeDefined();
  });

  it('renders summary text for search_knowledge', () => {
    render(<ToolCallCard toolCalls={[makeToolCall({ args: { query: 'back pain' } })]} />);
    expect(screen.getByText(/back pain/)).toBeDefined();
  });

  it('renders summary text for extract_symptom_info', () => {
    render(
      <ToolCallCard
        toolCalls={[makeToolCall({ tool: 'extract_symptom_info', args: { body_part: 'neck' } })]}
      />,
    );
    expect(screen.getByText(/neck/)).toBeDefined();
  });

  // --- Dedup ---------------------------------------------------------------

  it('renders tool call only once when same tool_call_id appears twice', () => {
    const calls: ToolCallInfo[] = [
      makeToolCall({ id: 'tc-1', status: 'running' }),
      makeToolCall({ id: 'tc-1', status: 'running' }), // duplicate
    ];

    render(<ToolCallCard toolCalls={calls} />);
    // Only one search_knowledge label should be rendered
    const labels = screen.getAllByText('搜索知识库');
    expect(labels).toHaveLength(1);
  });

  it('shows latest status for duplicate tool_call_id', () => {
    const calls: ToolCallInfo[] = [
      makeToolCall({ id: 'tc-1', status: 'running' }),
      makeToolCall({ id: 'tc-1', status: 'completed', result: { found: 3 } }),
    ];

    render(<ToolCallCard toolCalls={calls} />);
    // The completed checkmark SVG should be present
    const container = document.querySelector('svg');
    expect(container).not.toBeNull();
  });

  // --- Running vs completed ---------------------------------------------------

  it('shows spinner for running tool call', () => {
    const { container } = render(<ToolCallCard toolCalls={[makeToolCall({ status: 'running' })]} />);
    // Running indicator: animate-spin class on the spinner element
    const spinner = container.querySelector('.animate-spin');
    expect(spinner).not.toBeNull();
  });

  it('shows checkmark for completed tool call', () => {
    const { container } = render(
      <ToolCallCard toolCalls={[makeToolCall({ status: 'completed', result: { found: 1 } })]} />,
    );
    // Completed: checkmark SVG present
    const svg = container.querySelector('svg');
    expect(svg).not.toBeNull();
  });

  // --- ask_user filtering --------------------------------------------------

  it('does not render ask_user tools', () => {
    render(
      <ToolCallCard
        toolCalls={[
          makeToolCall({ id: 'tc-1', tool: 'ask_user', args: { question: 'age?' } }),
          makeToolCall({ id: 'tc-2', tool: 'search_knowledge', args: { query: 'posture' } }),
        ]}
      />,
    );

    // ask_user label should NOT appear
    expect(screen.queryByText('向用户提问')).toBeNull();
    // search_knowledge label SHOULD appear
    expect(screen.getByText('搜索知识库')).toBeDefined();
  });

  it('renders nothing when all tools are ask_user', () => {
    const { container } = render(
      <ToolCallCard
        toolCalls={[makeToolCall({ id: 'tc-1', tool: 'ask_user', args: { question: 'age?' } })]}
      />,
    );
    expect(container.firstChild).toBeNull();
  });

  // --- Same summary, different IDs -----------------------------------------

  it('allows tool calls with same summary text but different IDs to coexist', () => {
    const calls: ToolCallInfo[] = [
      makeToolCall({ id: 'tc-1', args: { query: 'back pain' }, status: 'completed', result: { found: 1 } }),
      makeToolCall({ id: 'tc-2', tool: 'search_knowledge', args: { query: 'back pain' }, status: 'running' }),
    ];

    render(<ToolCallCard toolCalls={calls} />);
    const labels = screen.getAllByText('搜索知识库');
    expect(labels).toHaveLength(2);
  });

  // --- Sort: running before completed ---------------------------------------

  it('maintains original order within same status group', () => {
    const calls: ToolCallInfo[] = [
      makeToolCall({ id: 'tc-1', tool: 'search_knowledge', args: { query: 'A' }, status: 'completed', result: {} }),
      makeToolCall({ id: 'tc-2', tool: 'extract_symptom_info', args: { body_part: 'neck' }, status: 'completed', result: {} }),
    ];

    render(<ToolCallCard toolCalls={calls} />);
    const items = screen.getAllByText(/搜索知识库|提取症状信息/);
    expect(items[0].textContent).toBe('搜索知识库');
    expect(items[1].textContent).toBe('提取症状信息');
  });
});
