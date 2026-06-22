import { useState, useEffect } from 'react';
import { useNavigate, useParams } from 'react-router';
import {
  trainingApi,
  type TrainingPlan,
  type TrainingLog,
  type TrainingProgress,
} from '../services/trainingService';

export function TrainingPage() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const [plan, setPlan] = useState<TrainingPlan | null>(null);
  const [todayTask, setTodayTask] = useState<TrainingLog | null>(null);
  const [progress, setProgress] = useState<TrainingProgress | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [notes, setNotes] = useState('');
  const [exerciseStates, setExerciseStates] = useState<Record<string, boolean>>({});

  useEffect(() => {
    if (!id) {
      navigate('/dashboard');
      return;
    }

    const loadData = async () => {
      try {
        const [planData, taskData, progressData] = await Promise.all([
          trainingApi.getPlan(id),
          trainingApi.getTodayTask(id),
          trainingApi.getProgress(id),
        ]);
        setPlan(planData);
        setTodayTask(taskData);
        setProgress(progressData);

        // Initialize exercise states from task
        const states: Record<string, boolean> = {};
        const exercises = taskData.exercises || [];
        exercises.forEach((ex: any) => {
          states[ex.name] = ex.completed || false;
        });
        setExerciseStates(states);
        setNotes(taskData.notes || '');
      } catch {
        // Handle error
      } finally {
        setIsLoading(false);
      }
    };

    loadData();
  }, [id, navigate]);

  const handleToggleExercise = (name: string) => {
    setExerciseStates((prev) => ({ ...prev, [name]: !prev[name] }));
  };

  const handleCheckIn = async () => {
    if (!id) return;
    try {
      await trainingApi.checkIn(id);
      // Reload progress
      const progressData = await trainingApi.getProgress(id);
      setProgress(progressData);
      setTodayTask((prev) => prev ? { ...prev, is_checked_in: true } : null);
    } catch {
      // Handle error
    }
  };

  const handleSaveLog = async () => {
    if (!id) return;
    const exercises = Object.entries(exerciseStates).map(([name, completed]) => ({
      name,
      completed,
    }));
    try {
      await trainingApi.updateLog(id, notes, exercises);
    } catch {
      // Handle error
    }
  };

  if (isLoading) {
    return (
      <div className="flex items-center justify-center h-screen">
        <div className="text-gray-500">加载中...</div>
      </div>
    );
  }

  if (!plan) {
    return (
      <div className="flex items-center justify-center h-screen">
        <div className="text-center">
          <p className="text-gray-500 mb-4">未找到训练计划</p>
          <button onClick={() => navigate('/dashboard')} className="text-blue-600 hover:underline">
            返回首页
          </button>
        </div>
      </div>
    );
  }

  // Get current phase exercises
  const currentPhase = plan.phases?.find((p) => p.week === plan.current_week);
  const exercises = currentPhase?.exercises || [];

  return (
    <div className="min-h-screen bg-gray-50">
      {/* Header */}
      <div className="bg-white shadow">
        <div className="mx-auto max-w-7xl px-4 py-4 sm:px-6 lg:px-8">
          <div className="flex items-center gap-3">
            <button onClick={() => navigate('/dashboard')} className="text-gray-500 hover:text-gray-700">
              ← 返回
            </button>
            <h1 className="text-xl font-bold text-gray-900">训练计划</h1>
          </div>
        </div>
      </div>

      <div className="mx-auto max-w-7xl px-4 py-8 sm:px-6 lg:px-8">
        <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
          {/* Main content */}
          <div className="lg:col-span-2 space-y-6">
            {/* Plan overview */}
            <div className="rounded-lg bg-white p-6 shadow">
              <h2 className="text-lg font-semibold text-gray-900 mb-2">{plan.goal}</h2>
              <div className="flex gap-4 text-sm text-gray-600">
                <span>周期：{plan.duration_weeks} 周</span>
                <span>当前：第 {plan.current_week} 周</span>
              </div>
              {currentPhase && (
                <p className="text-sm text-gray-500 mt-2">本阶段重点：{currentPhase.focus}</p>
              )}
            </div>

            {/* Today's tasks */}
            <div className="rounded-lg bg-white p-6 shadow">
              <h3 className="text-lg font-semibold text-gray-900 mb-4">今日训练</h3>

              {exercises.length === 0 ? (
                <p className="text-gray-500 text-sm">今天是休息日 🎉</p>
              ) : (
                <div className="space-y-3">
                  {exercises.map((exercise, i) => (
                    <div
                      key={i}
                      className={`flex items-center gap-3 p-3 rounded-lg border transition-colors ${
                        exerciseStates[exercise.name]
                          ? 'bg-green-50 border-green-200'
                          : 'bg-white border-gray-200'
                      }`}
                    >
                      <input
                        type="checkbox"
                        checked={exerciseStates[exercise.name] || false}
                        onChange={() => handleToggleExercise(exercise.name)}
                        className="h-5 w-5 rounded border-gray-300 text-blue-600"
                      />
                      <div className="flex-1">
                        <div className="font-medium text-sm text-gray-900">{exercise.name}</div>
                        <div className="text-xs text-gray-500">
                          {exercise.sets} 组 × {exercise.reps}
                        </div>
                        {exercise.notes && (
                          <div className="text-xs text-orange-600 mt-1">⚠️ {exercise.notes}</div>
                        )}
                      </div>
                    </div>
                  ))}
                </div>
              )}

              {/* Notes */}
              <div className="mt-4">
                <label className="text-sm text-gray-600 mb-1 block">训练感受</label>
                <textarea
                  value={notes}
                  onChange={(e) => setNotes(e.target.value)}
                  placeholder="记录今天的训练感受..."
                  className="w-full rounded-lg border p-3 text-sm resize-none"
                  rows={3}
                />
              </div>

              {/* Actions */}
              <div className="flex gap-3 mt-4">
                <button
                  onClick={handleCheckIn}
                  disabled={todayTask?.is_checked_in}
                  className="flex-1 rounded-lg bg-blue-600 px-4 py-2 text-sm font-medium text-white
                             hover:bg-blue-700 disabled:bg-gray-300 disabled:cursor-not-allowed"
                >
                  {todayTask?.is_checked_in ? '已打卡 ✓' : '打卡'}
                </button>
                <button
                  onClick={handleSaveLog}
                  className="flex-1 rounded-lg bg-gray-100 px-4 py-2 text-sm font-medium text-gray-700
                             hover:bg-gray-200"
                >
                  保存日志
                </button>
              </div>
            </div>
          </div>

          {/* Sidebar */}
          <div className="space-y-6">
            {/* Progress */}
            {progress && (
              <div className="rounded-lg bg-white p-6 shadow">
                <h3 className="text-sm font-semibold text-gray-700 mb-4">训练进度</h3>

                <div className="space-y-4">
                  <div>
                    <div className="flex justify-between text-sm mb-1">
                      <span className="text-gray-600">连续打卡</span>
                      <span className="font-medium text-blue-600">{progress.consecutive_days} 天</span>
                    </div>
                  </div>

                  <div>
                    <div className="flex justify-between text-sm mb-1">
                      <span className="text-gray-600">累计打卡</span>
                      <span className="font-medium">{progress.total_checkins} 次</span>
                    </div>
                  </div>

                  <div>
                    <div className="flex justify-between text-sm mb-1">
                      <span className="text-gray-600">阶段进度</span>
                      <span className="font-medium">
                        {progress.current_week} / {progress.total_weeks} 周
                      </span>
                    </div>
                    <div className="h-2 bg-gray-200 rounded-full overflow-hidden">
                      <div
                        className="h-full bg-blue-500 rounded-full"
                        style={{ width: `${(progress.current_week / progress.total_weeks) * 100}%` }}
                      />
                    </div>
                  </div>
                </div>
              </div>
            )}

            {/* Phase list */}
            <div className="rounded-lg bg-white p-6 shadow">
              <h3 className="text-sm font-semibold text-gray-700 mb-3">训练阶段</h3>
              <div className="space-y-2">
                {plan.phases?.map((phase, i) => (
                  <div
                    key={i}
                    className={`p-2 rounded text-sm ${
                      phase.week === plan.current_week
                        ? 'bg-blue-50 text-blue-700 font-medium'
                        : 'text-gray-600'
                    }`}
                  >
                    第 {phase.week} 周：{phase.focus}
                  </div>
                ))}
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}
