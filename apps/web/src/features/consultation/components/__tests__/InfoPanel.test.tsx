import { describe, it, expect, vi } from 'vitest';
import { render, screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { InfoPanel } from '../InfoPanel';
import type { ExtractedInfo } from '../../services/consultationService';

// Mock BodyVisualization since it may have SVG/DOM dependencies
vi.mock('../BodyVisualization', () => ({
  BodyVisualization: ({ highlightedParts }: { highlightedParts: string[] }) => (
    <div data-testid="body-viz">
      {highlightedParts.map((p: string) => (
        <span key={p}>{p}</span>
      ))}
    </div>
  ),
}));

const sampleInfo: ExtractedInfo[] = [
  {
    body_part: '肩部',
    symptom_type: '酸胀',
    duration: '2周',
    trigger: '久坐后',
    relief: '按压后缓解',
    severity: '轻度',
  },
  {
    body_part: '腰部',
    symptom_type: '疼痛',
    duration: '1个月',
    severity: '中度',
  },
];

describe('InfoPanel', () => {
  it('renders empty state message when no info', () => {
    render(<InfoPanel extractedInfo={[]} />);
    expect(screen.getByText(/对话中提到的症状信息会在这里显示/)).toBeInTheDocument();
  });

  it('renders info cards for each extracted info', () => {
    render(<InfoPanel extractedInfo={sampleInfo} />);
    // Body part names appear in both the card and the mocked BodyVisualization
    expect(screen.getAllByText('肩部').length).toBeGreaterThanOrEqual(1);
    expect(screen.getAllByText('腰部').length).toBeGreaterThanOrEqual(1);
  });

  it('displays symptom details in view mode', () => {
    render(<InfoPanel extractedInfo={[sampleInfo[0]]} />);
    expect(screen.getByText(/症状：酸胀/)).toBeInTheDocument();
    expect(screen.getByText(/持续时间：2周/)).toBeInTheDocument();
    expect(screen.getByText(/触发场景：久坐后/)).toBeInTheDocument();
    expect(screen.getByText(/缓解方式：按压后缓解/)).toBeInTheDocument();
  });

  it('displays severity badge', () => {
    render(<InfoPanel extractedInfo={[sampleInfo[0]]} />);
    expect(screen.getByText('轻度')).toBeInTheDocument();
  });

  it('calls onConfirm when confirm button is clicked', async () => {
    const onConfirm = vi.fn();
    const user = userEvent.setup();
    render(<InfoPanel extractedInfo={[sampleInfo[0]]} onConfirm={onConfirm} />);

    await user.click(screen.getByText('确认'));
    expect(onConfirm).toHaveBeenCalledWith(sampleInfo[0]);
  });

  it('enters edit mode when modify button is clicked', async () => {
    const user = userEvent.setup();
    render(<InfoPanel extractedInfo={[sampleInfo[0]]} />);

    await user.click(screen.getByText('修改'));

    // Should show edit form inputs
    expect(screen.getByDisplayValue('肩部')).toBeInTheDocument();
    expect(screen.getByDisplayValue('酸胀')).toBeInTheDocument();
    // Should show save/cancel buttons
    expect(screen.getByText('保存')).toBeInTheDocument();
    expect(screen.getByText('取消')).toBeInTheDocument();
  });

  it('calls onModify with updated data when save is clicked', async () => {
    const onModify = vi.fn();
    const user = userEvent.setup();
    render(<InfoPanel extractedInfo={[sampleInfo[0]]} onModify={onModify} />);

    // Enter edit mode
    await user.click(screen.getByText('修改'));

    // Change body part
    const bodyPartInput = screen.getByDisplayValue('肩部');
    await user.clear(bodyPartInput);
    await user.type(bodyPartInput, '颈部');

    // Save
    await user.click(screen.getByText('保存'));

    expect(onModify).toHaveBeenCalledWith(0, expect.objectContaining({ body_part: '颈部' }));
  });

  it('exits edit mode when cancel is clicked', async () => {
    const onModify = vi.fn();
    const user = userEvent.setup();
    render(<InfoPanel extractedInfo={[sampleInfo[0]]} onModify={onModify} />);

    await user.click(screen.getByText('修改'));
    expect(screen.getByText('保存')).toBeInTheDocument();

    await user.click(screen.getByText('取消'));
    expect(screen.queryByText('保存')).not.toBeInTheDocument();
    expect(onModify).not.toHaveBeenCalled();
  });

  it('passes unique body parts to BodyVisualization', () => {
    render(<InfoPanel extractedInfo={sampleInfo} />);
    const viz = screen.getByTestId('body-viz');
    expect(within(viz).getByText('肩部')).toBeInTheDocument();
    expect(within(viz).getByText('腰部')).toBeInTheDocument();
  });

  it('renders medical disclaimer', () => {
    render(<InfoPanel extractedInfo={[]} />);
    expect(screen.getByText(/本分析仅供参考/)).toBeInTheDocument();
  });
});
