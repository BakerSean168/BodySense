interface SleepStepProps {
  sleepTime: string;
  wakeTime: string;
  onSleepTimeChange: (value: string) => void;
  onWakeTimeChange: (value: string) => void;
}

export function SleepStep({
  sleepTime,
  wakeTime,
  onSleepTimeChange,
  onWakeTimeChange,
}: SleepStepProps) {
  return (
    <div>
      <h2 className="text-lg font-medium text-gray-900 mb-2">您的作息时间是？</h2>
      <p className="text-sm text-gray-500 mb-6">
        了解作息有助于判断肌肉恢复情况和安排最佳训练时间。
      </p>

      <div className="space-y-4">
        <div>
          <label htmlFor="sleepTime" className="block text-sm font-medium text-gray-700 mb-1">
            通常入睡时间
          </label>
          <input
            id="sleepTime"
            type="time"
            value={sleepTime}
            onChange={(e) => onSleepTimeChange(e.target.value)}
            className="w-full rounded-md border border-gray-300 px-4 py-3 shadow-sm focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500"
          />
        </div>

        <div>
          <label htmlFor="wakeTime" className="block text-sm font-medium text-gray-700 mb-1">
            通常起床时间
          </label>
          <input
            id="wakeTime"
            type="time"
            value={wakeTime}
            onChange={(e) => onWakeTimeChange(e.target.value)}
            className="w-full rounded-md border border-gray-300 px-4 py-3 shadow-sm focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500"
          />
        </div>
      </div>
    </div>
  );
}
