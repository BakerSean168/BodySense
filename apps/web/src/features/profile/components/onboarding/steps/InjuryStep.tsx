interface InjuryStepProps {
  value: string;
  onChange: (value: string) => void;
}

export function InjuryStep({ value, onChange }: InjuryStepProps) {
  return (
    <div>
      <h2 className="text-lg font-medium text-gray-900 mb-2">
        既往伤病与手术史
      </h2>
      <p className="text-sm text-gray-500 mb-6">
        只记录对后续身体判断可能有影响的既往情况。当前正在困扰您的症状和目标，可以在进入工作台后直接告诉
        BodySense。
      </p>

      <label
        htmlFor="injuryHistory"
        className="block text-sm font-medium text-gray-700 mb-1"
      >
        既往伤病、手术或长期遗留问题（选填）
      </label>
      <textarea
        id="injuryHistory"
        value={value}
        onChange={(event) => onChange(event.target.value)}
        rows={5}
        placeholder="例如：2024 年左膝扭伤，未手术，跑步量大时偶尔酸胀；或：无明确伤病史。"
        className="w-full rounded-md border border-gray-300 px-4 py-3 shadow-sm focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500"
      />
    </div>
  );
}
