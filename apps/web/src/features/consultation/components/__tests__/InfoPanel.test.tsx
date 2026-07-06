/**
 * @vitest-environment jsdom
 */

import { describe, it, expect, vi } from 'vitest';
import { cleanup, render, screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { InfoPanel } from '../InfoPanel';
import type { HealthFeatures } from '../../types/consultation';

vi.mock('../BodyVisualization', () => ({
  BodyVisualization: ({ highlightedParts }: { highlightedParts: string[] }) => (
    <div data-testid="body-viz">
      {highlightedParts.map((part: string) => (
        <span key={part}>{part}</span>
      ))}
    </div>
  ),
}));

cleanup();

const sampleHealthFeatures: HealthFeatures = {
  posture_findings: [
    { label: '头前移', details: '用户自述感觉有些头前移', source: 'user_message' },
  ],
  discomforts: [
    { label: '酸胀', body_part: '肩部', value: '轻度', details: '久坐后明显', source: 'extracted_info' },
  ],
  negative_findings: [
    { label: '未报告相关不适', value: '无', details: '是否感觉颈部或肩部不适？', source: 'ask_user' },
  ],
  movement_limitations: [],
  red_flags: [],
  user_answers: [
    { label: '是否感觉颈部或肩部不适？', value: '无', details: '为了判断是否已有代偿', source: 'ask_user' },
  ],
};

describe('InfoPanel', () => {
  it('renders empty state when no health features exist', () => {
    render(
      <InfoPanel
        healthFeatures={{
          posture_findings: [],
          discomforts: [],
          negative_findings: [],
          movement_limitations: [],
          red_flags: [],
          user_answers: [],
        }}
      />,
    );

    expect(screen.getByText(/体态观察、补充回答和不适信息会在这里沉淀/)).toBeTruthy();
  });

  it('renders grouped health feature sections', () => {
    render(<InfoPanel healthFeatures={sampleHealthFeatures} />);

    expect(screen.getByText('姿态观察')).toBeTruthy();
    expect(screen.getByText('不适与症状')).toBeTruthy();
    expect(screen.getByText('阴性信息')).toBeTruthy();
    expect(screen.getByText('补充回答')).toBeTruthy();
    expect(screen.getByText('头前移')).toBeTruthy();
    expect(screen.getByText('酸胀')).toBeTruthy();
  });

  it('passes body parts to body visualization', () => {
    render(<InfoPanel healthFeatures={sampleHealthFeatures} />);

    const viz = screen.getAllByTestId('body-viz').at(-1);
    expect(viz).toBeTruthy();
    expect(within(viz as HTMLElement).getByText('肩部')).toBeTruthy();
  });

  it('calls onConfirm with category and index', async () => {
    const onConfirm = vi.fn();
    const user = userEvent.setup();

    render(<InfoPanel healthFeatures={sampleHealthFeatures} onConfirm={onConfirm} />);

    await user.click(screen.getAllByText('确认')[0]);

    expect(onConfirm).toHaveBeenCalledWith('posture_findings', 0);
  });

  it('calls onModify with category, index and item', async () => {
    const onModify = vi.fn();
    const user = userEvent.setup();

    render(<InfoPanel healthFeatures={sampleHealthFeatures} onModify={onModify} />);

    await user.click(screen.getAllByText('标记')[1]);

    expect(onModify).toHaveBeenCalledWith(
      'discomforts',
      0,
      expect.objectContaining({ label: '酸胀', body_part: '肩部' }),
    );
  });

  it('calls onDelete with category and index', async () => {
    const onDelete = vi.fn();
    const user = userEvent.setup();

    render(<InfoPanel healthFeatures={sampleHealthFeatures} onDelete={onDelete} />);

    await user.click(screen.getAllByText('删除')[2]);

    expect(onDelete).toHaveBeenCalledWith('negative_findings', 0);
  });
});
