import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { DiagnosisPanel } from '../DiagnosisPanel';
import type { Diagnosis, TreatmentPlan, Citation } from '../../services/consultationService';

const sampleDiagnoses: Diagnosis[] = [
  {
    name: '上交叉综合征',
    confidence: '高',
    severity: '轻度',
    basis: '肩颈酸胀、久坐后明显',
    typical_symptoms: '圆肩、头前伸',
    differential: '需与肩袖损伤区分',
  },
  {
    name: '颈椎病',
    confidence: '中',
    severity: '中度',
    basis: '颈部不适',
    typical_symptoms: '颈部僵硬',
  },
];

const sampleCitations: Citation[] = [
  {
    title: '上交叉综合征的自测与改善',
    summary: '常见于久坐低头人群',
    body_markdown: '# 上交叉综合征\n\n详细内容...',
    category: 'definition',
    source_title: '体态健康手册',
    source_author: '张医生',
  },
  {
    title: '腰痛改善训练',
    summary: '腰部训练方法',
    category: 'exercise',
    source_title: '训练指南',
  },
];

const sampleTreatmentPlan: TreatmentPlan = {
  goal: '缓解肩颈酸胀',
  duration_weeks: 4,
  correction_exercises: [
    {
      name: '胸小肌拉伸',
      description: '靠墙或门框完成',
      sets: '2-3组',
      reps: '每次30秒',
      notes: '不要耸肩',
    },
  ],
  daily_habits: ['每45分钟起身活动'],
  expected_timeline: '4周可见改善',
  warning_signs: ['疼痛放射到手臂时就医'],
  citations: [
    {
      title: '胸小肌拉伸方法',
      summary: '拉伸要点',
      source_title: '训练手册',
    },
  ],
};

describe('DiagnosisPanel', () => {
  describe('diagnosis view', () => {
    it('returns null when no diagnoses and no treatment plan', () => {
      const { container } = render(
        <DiagnosisPanel
          diagnoses={[]}
          treatmentPlan={null}
          onConfirmAndGenerateTreatment={vi.fn()}
        />,
      );
      expect(container.innerHTML).toBe('');
    });

    it('renders diagnosis cards', () => {
      render(
        <DiagnosisPanel
          diagnoses={sampleDiagnoses}
          treatmentPlan={null}
          onConfirmAndGenerateTreatment={vi.fn()}
        />,
      );
      expect(screen.getByText('上交叉综合征')).toBeInTheDocument();
      expect(screen.getByText('颈椎病')).toBeInTheDocument();
    });

    it('displays confidence and severity badges', () => {
      render(
        <DiagnosisPanel
          diagnoses={[sampleDiagnoses[0]]}
          treatmentPlan={null}
          onConfirmAndGenerateTreatment={vi.fn()}
        />,
      );
      expect(screen.getByText(/置信度：高/)).toBeInTheDocument();
      expect(screen.getByText('轻度')).toBeInTheDocument();
    });

    it('displays basis and typical symptoms', () => {
      render(
        <DiagnosisPanel
          diagnoses={[sampleDiagnoses[0]]}
          treatmentPlan={null}
          onConfirmAndGenerateTreatment={vi.fn()}
        />,
      );
      expect(screen.getByText(/肩颈酸胀、久坐后明显/)).toBeInTheDocument();
      expect(screen.getByText(/典型表现：圆肩、头前伸/)).toBeInTheDocument();
      expect(screen.getByText(/区别说明：需与肩袖损伤区分/)).toBeInTheDocument();
    });

    it('disables confirm button when no diagnosis selected', () => {
      render(
        <DiagnosisPanel
          diagnoses={sampleDiagnoses}
          treatmentPlan={null}
          onConfirmAndGenerateTreatment={vi.fn()}
        />,
      );
      const button = screen.getByText('确认诊断并生成改善方案');
      expect(button).toBeDisabled();
    });

    it('enables confirm button after selecting a diagnosis and calls callback', async () => {
      const onConfirm = vi.fn();
      const user = userEvent.setup();
      render(
        <DiagnosisPanel
          diagnoses={sampleDiagnoses}
          treatmentPlan={null}
          onConfirmAndGenerateTreatment={onConfirm}
        />,
      );

      // Click on first diagnosis card
      await user.click(screen.getByText('上交叉综合征'));

      // Button should be enabled
      const button = screen.getByText('确认诊断并生成改善方案');
      expect(button).not.toBeDisabled();

      // Click confirm
      await user.click(button);
      expect(onConfirm).toHaveBeenCalledWith(sampleDiagnoses[0]);
    });

    it('shows loading state when isGeneratingTreatment is true', () => {
      render(
        <DiagnosisPanel
          diagnoses={sampleDiagnoses}
          treatmentPlan={null}
          onConfirmAndGenerateTreatment={vi.fn()}
          isGeneratingTreatment
        />,
      );
      expect(screen.getByText('生成中...')).toBeInTheDocument();
    });

    it('renders citations with matching logic', () => {
      render(
        <DiagnosisPanel
          diagnoses={sampleDiagnoses}
          citations={sampleCitations}
          treatmentPlan={null}
          onConfirmAndGenerateTreatment={vi.fn()}
        />,
      );
      // "上交叉综合征" diagnosis should match "上交叉综合征的自测与改善" citation
      // Multiple diagnosis cards may show citation tags
      expect(screen.getAllByText(/知识库来源/).length).toBeGreaterThanOrEqual(1);
    });

    it('renders expandable citations list', async () => {
      const user = userEvent.setup();
      render(
        <DiagnosisPanel
          diagnoses={sampleDiagnoses}
          citations={sampleCitations}
          treatmentPlan={null}
          onConfirmAndGenerateTreatment={vi.fn()}
        />,
      );

      expect(screen.getByText(/知识库参考来源/)).toBeInTheDocument();
      expect(screen.getByText('上交叉综合征的自测与改善')).toBeInTheDocument();

      // Click to expand
      await user.click(screen.getByText('上交叉综合征的自测与改善'));
      expect(screen.getByText(/详细内容/)).toBeInTheDocument();
    });
  });

  describe('treatment plan view', () => {
    it('renders treatment plan when provided', () => {
      render(
        <DiagnosisPanel
          diagnoses={[]}
          treatmentPlan={sampleTreatmentPlan}
          onConfirmAndGenerateTreatment={vi.fn()}
        />,
      );
      expect(screen.getByText('改善方案')).toBeInTheDocument();
      expect(screen.getByText(/缓解肩颈酸胀/)).toBeInTheDocument();
      expect(screen.getByText('胸小肌拉伸')).toBeInTheDocument();
      expect(screen.getByText(/每45分钟起身活动/)).toBeInTheDocument();
    });

    it('renders exercise details', () => {
      render(
        <DiagnosisPanel
          diagnoses={[]}
          treatmentPlan={sampleTreatmentPlan}
          onConfirmAndGenerateTreatment={vi.fn()}
        />,
      );
      expect(screen.getByText(/靠墙或门框完成/)).toBeInTheDocument();
      expect(screen.getByText(/组数：2-3组/)).toBeInTheDocument();
      expect(screen.getByText(/次数：每次30秒/)).toBeInTheDocument();
      expect(screen.getByText(/不要耸肩/)).toBeInTheDocument();
    });

    it('renders warning signs', () => {
      render(
        <DiagnosisPanel
          diagnoses={[]}
          treatmentPlan={sampleTreatmentPlan}
          onConfirmAndGenerateTreatment={vi.fn()}
        />,
      );
      expect(screen.getByText(/警示信号/)).toBeInTheDocument();
      expect(screen.getByText(/疼痛放射到手臂时就医/)).toBeInTheDocument();
    });

    it('renders treatment citations', () => {
      render(
        <DiagnosisPanel
          diagnoses={[]}
          treatmentPlan={sampleTreatmentPlan}
          onConfirmAndGenerateTreatment={vi.fn()}
        />,
      );
      expect(screen.getByText(/方案科学依据/)).toBeInTheDocument();
    });

    it('renders medical disclaimer', () => {
      render(
        <DiagnosisPanel
          diagnoses={[]}
          treatmentPlan={sampleTreatmentPlan}
          onConfirmAndGenerateTreatment={vi.fn()}
        />,
      );
      expect(screen.getByText(/本方案仅供参考/)).toBeInTheDocument();
    });
  });
});
