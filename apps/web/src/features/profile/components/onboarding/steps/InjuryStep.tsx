interface InjuryStepProps {
  injuryHistory: string;
  selfDescription: string;
  onInjuryHistoryChange: (value: string) => void;
  onSelfDescriptionChange: (value: string) => void;
}

export function InjuryStep({
  injuryHistory,
  selfDescription,
  onInjuryHistoryChange,
  onSelfDescriptionChange,
}: InjuryStepProps) {
  return (
    <div>
      <h2 className="text-lg font-medium text-gray-900 mb-2">伤病史和自我描述</h2>
      <p className="text-sm text-gray-500 mb-6">
        了解您的伤病史有助于避免训练中造成二次伤害，自我描述能帮助我们更好地了解您。
      </p>

      <div className="space-y-4">
        <div>
          <label htmlFor="injuryHistory" className="block text-sm font-medium text-gray-700 mb-1">
            伤病史（选填）
          </label>
          <textarea
            id="injuryHistory"
            value={injuryHistory}
            onChange={(e) => onInjuryHistoryChange(e.target.value)}
            rows={3}
            placeholder="例如：膝盖半月板损伤、腰椎间盘突出..."
            className="w-full rounded-md border border-gray-300 px-4 py-3 shadow-sm focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500"
          />
        </div>

        <div>
          <label htmlFor="selfDescription" className="block text-sm font-medium text-gray-700 mb-1">
            自我描述（选填）
          </label>
          <textarea
            id="selfDescription"
            value={selfDescription}
            onChange={(e) => onSelfDescriptionChange(e.target.value)}
            rows={3}
            placeholder="简单描述您的身体状况、健身目标等..."
            className="w-full rounded-md border border-gray-300 px-4 py-3 shadow-sm focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500"
          />
        </div>
      </div>
    </div>
  );
}
