import { useState } from 'react';
import type { AskUserQuestion } from '../types/consultation';

interface AskUserCardProps {
  question: AskUserQuestion;
  onSubmit: (answer: unknown) => void;
  isSubmitting?: boolean;
  error?: string | null;
  onRetry?: () => void;
}

/**
 * Renders a pending ask_user interaction card.
 * Supports text, single_choice, multi_choice, number, and date answer types.
 */
export function AskUserCard({ question, onSubmit, isSubmitting = false, error, onRetry }: AskUserCardProps) {
  const [textAnswer, setTextAnswer] = useState('');
  const [selectedSingle, setSelectedSingle] = useState<string>('');
  const [selectedMulti, setSelectedMulti] = useState<string[]>([]);

  const handleSubmit = () => {
    switch (question.answer_type) {
      case 'text':
        if (textAnswer.trim()) onSubmit({ text: textAnswer.trim() });
        break;
      case 'single_choice':
        if (selectedSingle) onSubmit({ text: selectedSingle, selected: [selectedSingle] });
        break;
      case 'multi_choice':
        if (selectedMulti.length > 0) onSubmit({ text: selectedMulti.join(', '), selected: selectedMulti });
        break;
      case 'number':
        if (textAnswer.trim()) onSubmit({ text: textAnswer.trim(), value: Number(textAnswer.trim()) });
        break;
      case 'date':
        if (textAnswer.trim()) onSubmit({ text: textAnswer.trim() });
        break;
    }
  };

  return (
    <div className="rounded-lg border bg-blue-50 p-4 my-2">
      <p className="text-sm font-medium text-blue-900 mb-3">{question.question}</p>

      {question.context && (
        <p className="text-xs text-blue-700 mb-3">{question.context}</p>
      )}

      {/* Error state */}
      {error && (
        <div className="rounded bg-red-50 border border-red-200 p-3 mb-3">
          <p className="text-sm text-red-700">提交失败：{error}</p>
          {onRetry && (
            <button
              className="mt-2 text-sm text-red-600 underline hover:text-red-800"
              onClick={onRetry}
            >
              重试
            </button>
          )}
        </div>
      )}

      {/* Input by answer type */}
      {(question.answer_type === 'text' || question.answer_type === 'number' || question.answer_type === 'date') && (
        <input
          type={question.answer_type === 'number' ? 'number' : question.answer_type === 'date' ? 'date' : 'text'}
          className="w-full rounded border px-3 py-2 text-sm mb-3"
          placeholder={question.answer_type === 'text' ? '输入你的回答...' : question.answer_type === 'number' ? '输入数字...' : '选择日期...'}
          value={textAnswer}
          onChange={(e) => setTextAnswer(e.target.value)}
          disabled={isSubmitting}
        />
      )}

      {question.answer_type === 'single_choice' && question.options && (
        <div className="space-y-2 mb-3">
          {question.options.map((opt) => (
            <label key={opt} className="flex items-center gap-2 text-sm">
              <input
                type="radio"
                name="ask_user_single"
                value={opt}
                checked={selectedSingle === opt}
                onChange={() => setSelectedSingle(opt)}
                disabled={isSubmitting}
              />
              {opt}
            </label>
          ))}
        </div>
      )}

      {question.answer_type === 'multi_choice' && question.options && (
        <div className="space-y-2 mb-3">
          {question.options.map((opt) => (
            <label key={opt} className="flex items-center gap-2 text-sm">
              <input
                type="checkbox"
                checked={selectedMulti.includes(opt)}
                onChange={(e) => {
                  if (e.target.checked) {
                    setSelectedMulti([...selectedMulti, opt]);
                  } else {
                    setSelectedMulti(selectedMulti.filter((o) => o !== opt));
                  }
                }}
                disabled={isSubmitting}
              />
              {opt}
            </label>
          ))}
        </div>
      )}

      <button
        className="rounded bg-blue-600 px-4 py-2 text-sm text-white hover:bg-blue-700 disabled:opacity-50"
        onClick={handleSubmit}
        disabled={isSubmitting}
      >
        {isSubmitting ? '提交中...' : '提交'}
      </button>
    </div>
  );
}
