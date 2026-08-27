interface OtherLifestyleStepProps {
  nutrition: string;
  substances: string;
  recovery: string;
  onNutritionChange: (value: string) => void;
  onSubstancesChange: (value: string) => void;
  onRecoveryChange: (value: string) => void;
}

export function OtherLifestyleStep({
  nutrition,
  substances,
  recovery,
  onNutritionChange,
  onSubstancesChange,
  onRecoveryChange,
}: OtherLifestyleStepProps) {
  return (
    <div>
      <h2 className="mb-2 text-lg font-medium text-gray-900">还有哪些生活方式值得记录？</h2>
      <p className="mb-6 text-sm leading-6 text-gray-500">
        这里只收集可能影响身体状态判断的背景，不做精细热量或生活流水账。全部选填，后续也可以直接在对话里告诉 BodySense。
      </p>

      <div className="space-y-5">
        <div>
          <label htmlFor="nutritionPattern" className="mb-1 block text-sm font-medium text-gray-700">
            饮食节律
          </label>
          <textarea
            id="nutritionPattern"
            rows={3}
            value={nutrition}
            onChange={(event) => onNutritionChange(event.target.value)}
            placeholder="例如：通常三餐规律；夜班时会在凌晨加一餐；最近正在明显节食。"
            className="w-full rounded-md border border-gray-300 px-4 py-3 shadow-sm focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500"
          />
        </div>

        <div>
          <label htmlFor="substancesPattern" className="mb-1 block text-sm font-medium text-gray-700">
            酒精、烟草与咖啡因
          </label>
          <textarea
            id="substancesPattern"
            rows={3}
            value={substances}
            onChange={(event) => onSubstancesChange(event.target.value)}
            placeholder="例如：应酬时每周饮酒 1-2 次；每天咖啡约 2 杯；不吸烟。"
            className="w-full rounded-md border border-gray-300 px-4 py-3 shadow-sm focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500"
          />
        </div>

        <div>
          <label htmlFor="recoveryPattern" className="mb-1 block text-sm font-medium text-gray-700">
            恢复与压力
          </label>
          <textarea
            id="recoveryPattern"
            rows={3}
            value={recovery}
            onChange={(event) => onRecoveryChange(event.target.value)}
            placeholder="例如：最近工作节奏比较紧，工作日恢复感一般，周末会明显好一些。"
            className="w-full rounded-md border border-gray-300 px-4 py-3 shadow-sm focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500"
          />
        </div>
      </div>
    </div>
  );
}
