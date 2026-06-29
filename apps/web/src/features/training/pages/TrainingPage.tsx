import { useState, useEffect } from 'react';
import { useNavigate, useParams } from 'react-router';
import {
  trainingApi,
  type TrainingPlan,
  type TrainingLog,
  type TrainingProgress,
  type TrainingReassessmentResult,
} from '../services/trainingService';
import { MainLayout } from '@/components/layout/MainLayout';
import { Card } from '@/components/ui/Card';
import { Button } from '@/components/ui/button';
import { toast } from 'sonner';

export function TrainingPage() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const [plan, setPlan] = useState<TrainingPlan | null>(null);
  const [todayTask, setTodayTask] = useState<TrainingLog | null>(null);
  const [progress, setProgress] = useState<TrainingProgress | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [notes, setNotes] = useState('');
  const [exerciseStates, setExerciseStates] = useState<Record<string, boolean>>({});
  const [showReassessment, setShowReassessment] = useState(false);
  const [reassessmentFeedback, setReassessmentFeedback] = useState({
    symptom_changes: '',
    training_feeling: '',
    difficulties: '',
  });
  const [reassessmentResult, setReassessmentResult] = useState<TrainingReassessmentResult | null>(null);
  const [isReassessing, setIsReassessing] = useState(false);

  // Immersive Player State
  const [isPlayerOpen, setIsPlayerOpen] = useState(false);
  const [currentExerciseIndex, setCurrentExerciseIndex] = useState(0);
  const [currentSet, setCurrentSet] = useState(1);
  const [timerSeconds, setTimerSeconds] = useState(0);
  const [isTimerRunning, setIsTimerRunning] = useState(false);

  // Proposal modal state
  const [activeProposal, setActiveProposal] = useState<TrainingReassessmentResult | null>(null);

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
        exercises.forEach((ex) => {
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

  // Timer effect for rest countdown
  useEffect(() => {
    if (!isTimerRunning || timerSeconds <= 0) {
      if (timerSeconds === 0 && isTimerRunning) {
        setIsTimerRunning(false);
        toast('休息结束，开始下一组训练！', { icon: '💪' });
      }
      return;
    }
    const timer = setTimeout(() => {
      setTimerSeconds((prev) => prev - 1);
    }, 1000);
    return () => clearTimeout(timer);
  }, [isTimerRunning, timerSeconds]);

  const handleSaveLog = async () => {
    if (!id) return;
    const exercises = Object.entries(exerciseStates).map(([name, completed]) => ({
      name,
      completed,
    }));
    try {
      const res = await trainingApi.updateLog(id, notes, exercises);
      if (res.has_proposal && res.proposal) {
        setActiveProposal(res.proposal);
      } else {
        toast.success('日志已保存');
      }
    } catch {
      toast.error('日志保存失败，请稍后重试');
    }
  };

  const handleApplyProposal = async () => {
    if (!id || !plan || !activeProposal) return;
    try {
      const updatedPhases = plan.phases.map((p) => {
        if (p.week === plan.current_week) {
          return {
            ...p,
            focus: activeProposal.next_phase_plan.focus,
            exercises: activeProposal.next_phase_plan.exercises.map((e) => ({
              name: e.name,
              description: e.description,
              sets: String(e.sets),
              reps: String(e.reps),
              notes: e.notes,
            })),
          };
        }
        return p;
      });

      await trainingApi.updatePlanPhases(id, updatedPhases);
      
      setPlan({ ...plan, phases: updatedPhases });
      
      // Update exercise states for today
      const newStates: Record<string, boolean> = {};
      activeProposal.next_phase_plan.exercises.forEach((ex) => {
        newStates[ex.name] = false;
      });
      setExerciseStates(newStates);

      toast.success('计划调整方案已成功应用！');
      setActiveProposal(null);
    } catch (err) {
      toast.error('应用调整方案失败，请重试');
    }
  };

  const handleReassessment = async () => {
    if (!id) return;
    setIsReassessing(true);
    try {
      const result = await trainingApi.submitReassessment(id, reassessmentFeedback);
      setReassessmentResult(result);
    } catch {
      // Handle error
    } finally {
      setIsReassessing(false);
    }
  };

  if (isLoading) {
    return (
      <MainLayout>
        <div className="flex items-center justify-center min-h-[60vh]">
          <div className="text-center">
            <div className="animate-spin rounded-full h-12 w-12 border-b-2 border-primary-600 mx-auto"></div>
            <p className="mt-4 text-slate-500 font-medium">加载中 (Loading)...</p>
          </div>
        </div>
      </MainLayout>
    );
  }

  if (!plan) {
    return (
      <MainLayout>
        <div className="flex items-center justify-center min-h-[60vh]">
          <Card className="p-8 text-center shadow-xl max-w-sm w-full">
            <div className="w-16 h-16 bg-slate-100 rounded-full flex items-center justify-center mx-auto mb-4 text-slate-400">
              <svg className="w-8 h-8" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9.172 16.172a4 4 0 015.656 0M9 10h.01M15 10h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
              </svg>
            </div>
            <p className="text-lg font-bold text-slate-900 mb-2">未找到训练计划</p>
            <p className="text-slate-500 mb-6 text-sm">Plan not found.</p>
            <Button onClick={() => navigate('/dashboard')} className="w-full">
              返回首页 (Return to Dashboard)
            </Button>
          </Card>
        </div>
      </MainLayout>
    );
  }

  // Get current phase exercises
  const currentPhase = plan.phases?.find((p) => p.week === plan.current_week);
  const exercises = currentPhase?.exercises || [];

  return (
    <MainLayout>
      <div className="w-full space-y-8 animate-in fade-in slide-in-from-bottom-4 duration-500">
        {/* Header */}
        <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4 bg-slate-900 text-white p-8 rounded-3xl shadow-xl relative overflow-hidden">
          <div className="absolute top-0 right-0 w-64 h-64 bg-primary-500 rounded-full mix-blend-multiply filter blur-3xl opacity-30 -translate-y-1/2 translate-x-1/2"></div>
          <div className="absolute bottom-0 left-1/4 w-48 h-48 bg-indigo-500 rounded-full mix-blend-multiply filter blur-3xl opacity-30 translate-y-1/2 -translate-x-1/2"></div>
          
          <div className="relative z-10 flex items-center gap-4">
            <button
              onClick={() => navigate('/dashboard')}
              className="w-10 h-10 rounded-xl bg-white/10 hover:bg-white/20 border border-white/10 flex items-center justify-center text-white transition-colors backdrop-blur-md"
            >
              <svg className="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M10 19l-7-7m0 0l7-7m-7 7h18" />
              </svg>
            </button>
            <div>
              <h1 className="text-2xl font-bold tracking-tight mb-1">训练计划 (Training Plan)</h1>
              <p className="text-slate-300">Stick to the plan to achieve your health goals.</p>
            </div>
          </div>
        </div>

        <div className="grid grid-cols-1 lg:grid-cols-3 gap-8">
          {/* Main content */}
          <div className="lg:col-span-2 space-y-8">
            {/* Plan overview */}
            <Card className="p-8 border-none bg-gradient-to-br from-white to-slate-50 shadow-lg">
              <div className="flex items-start justify-between gap-4 mb-4">
                <h2 className="text-2xl font-bold text-slate-900">{plan.goal}</h2>
                <div className="bg-primary-100 text-primary-700 px-3 py-1 rounded-full text-sm font-bold whitespace-nowrap">
                  Week {plan.current_week} of {plan.duration_weeks}
                </div>
              </div>
              {currentPhase && (
                <div className="bg-indigo-50/50 border border-indigo-100 rounded-xl p-4">
                  <p className="text-sm font-semibold text-indigo-900 mb-1 flex items-center gap-2">
                    <svg className="w-4 h-4 text-indigo-500" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M13 10V3L4 14h7v7l9-11h-7z" />
                    </svg>
                    本阶段重点 (Current Focus)
                  </p>
                  <p className="text-indigo-700 text-sm">{currentPhase.focus}</p>
                </div>
              )}
            </Card>

            {/* Today's tasks */}
            <Card className="p-8 shadow-lg">
              <div className="flex items-center justify-between mb-6">
                <h3 className="text-xl font-bold text-slate-900 flex items-center gap-2">
                  <svg className="w-6 h-6 text-emerald-500" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M8 7V3m8 4V3m-9 8h10M5 21h14a2 2 0 002-2V7a2 2 0 00-2-2H5a2 2 0 00-2 2v12a2 2 0 002 2z" />
                  </svg>
                  今日训练 (Today's Training)
                </h3>
                {todayTask?.is_checked_in && (
                  <span className="bg-emerald-100 text-emerald-700 px-3 py-1 rounded-full text-xs font-bold flex items-center gap-1">
                    <svg className="w-3 h-3" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M5 13l4 4L19 7" />
                    </svg>
                    已打卡 (Completed)
                  </span>
                )}
              </div>

              {exercises.length === 0 ? (
                <div className="text-center py-12 bg-slate-50 rounded-2xl border-2 border-dashed border-slate-200">
                  <div className="text-4xl mb-2">🎉</div>
                  <p className="text-slate-600 font-medium">今天是休息日 (Rest Day)</p>
                  <p className="text-slate-400 text-sm mt-1">Take some time to recover.</p>
                </div>
              ) : (
                <div className="space-y-4">
                  {exercises.map((exercise, i) => {
                    const isCompleted = exerciseStates[exercise.name] || false;
                    return (
                      <div
                        key={i}
                        className={`group flex items-start gap-4 p-5 rounded-2xl border-2 transition-all duration-300 ${
                          isCompleted
                            ? 'bg-emerald-50 border-emerald-200 shadow-sm'
                            : 'bg-white border-slate-100 hover:border-primary-200 hover:shadow-md'
                        }`}
                      >
                        <button
                          onClick={() => handleToggleExercise(exercise.name)}
                          className={`mt-0.5 shrink-0 w-6 h-6 rounded-md flex items-center justify-center transition-colors ${
                            isCompleted ? 'bg-emerald-500 text-white' : 'border-2 border-slate-300 group-hover:border-primary-400 text-transparent'
                          }`}
                        >
                          <svg className="w-4 h-4" fill="currentColor" viewBox="0 0 20 20">
                            <path fillRule="evenodd" d="M16.707 5.293a1 1 0 010 1.414l-8 8a1 1 0 01-1.414 0l-4-4a1 1 0 011.414-1.414L8 12.586l7.293-7.293a1 1 0 011.414 0z" clipRule="evenodd" />
                          </svg>
                        </button>
                        <div className="flex-1">
                          <div className={`font-bold text-lg transition-colors ${isCompleted ? 'text-emerald-900' : 'text-slate-900'}`}>
                            {exercise.name}
                          </div>
                          <div className={`text-sm font-medium mt-1 ${isCompleted ? 'text-emerald-700' : 'text-primary-600'}`}>
                            {exercise.sets} 组 (Sets) × {exercise.reps}
                          </div>
                          {exercise.notes && (
                            <div className="text-sm text-amber-600 mt-2 bg-amber-50 px-3 py-2 rounded-lg inline-flex items-start gap-2">
                              <span className="shrink-0">⚠️</span>
                              <span>{exercise.notes}</span>
                            </div>
                          )}
                        </div>
                      </div>
                    );
                  })}
                </div>
              )}

              {/* Notes */}
              <div className="mt-8">
                <label className="text-sm font-bold text-slate-700 mb-2 block">训练感受 (Training Notes)</label>
                <textarea
                  value={notes}
                  onChange={(e) => setNotes(e.target.value)}
                  placeholder="记录今天的训练感受... (How did today's training feel?)"
                  className="w-full rounded-xl border border-slate-200 p-4 text-sm resize-none focus:ring-2 focus:ring-primary-500 focus:border-primary-500 transition-all bg-slate-50 focus:bg-white"
                  rows={3}
                />
              </div>

              {/* Actions */}
              <div className="flex flex-col sm:flex-row gap-4 mt-6 pt-6 border-t border-slate-100">
                {exercises.length > 0 && (
                  <Button
                    onClick={() => {
                      setCurrentExerciseIndex(0);
                      setCurrentSet(1);
                      setTimerSeconds(0);
                      setIsTimerRunning(false);
                      setIsPlayerOpen(true);
                    }}
                    className="flex-1 bg-gradient-to-r from-primary-600 to-indigo-600 hover:from-primary-700 hover:to-indigo-700 text-white border-none shadow-md"
                  >
                    开始训练 (Start Workout)
                  </Button>
                )}
                <Button
                  onClick={handleCheckIn}
                  disabled={todayTask?.is_checked_in || exercises.length === 0}
                  className={`flex-1 ${todayTask?.is_checked_in ? 'bg-emerald-500 hover:bg-emerald-600 shadow-emerald-500/20' : ''}`}
                >
                  {todayTask?.is_checked_in ? '已打卡 (Checked In) ✓' : '打卡 (Check In)'}
                </Button>
                <Button
                  variant="outline"
                  onClick={handleSaveLog}
                  className="flex-1"
                >
                  保存日志 (Save Notes)
                </Button>
              </div>
            </Card>
          </div>

          {/* Sidebar */}
          <div className="space-y-8">
            {/* Progress */}
            {progress && (
              <Card className="p-6 bg-slate-900 text-white shadow-xl border-none relative overflow-hidden">
                <div className="absolute top-0 right-0 w-32 h-32 bg-primary-500 rounded-full mix-blend-multiply filter blur-3xl opacity-20 -translate-y-1/2 translate-x-1/2"></div>
                <h3 className="text-lg font-bold text-white mb-6 flex items-center gap-2 relative z-10">
                  <svg className="w-5 h-5 text-primary-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M13 7h8m0 0v8m0-8l-8 8-4-4-6 6" />
                  </svg>
                  训练进度 (Progress)
                </h3>

                <div className="space-y-6 relative z-10">
                  <div className="grid grid-cols-2 gap-4">
                    <div className="bg-white/10 rounded-2xl p-4 backdrop-blur-sm border border-white/10">
                      <p className="text-slate-400 text-xs font-semibold mb-1 uppercase tracking-wider">连续打卡 (Streak)</p>
                      <p className="text-2xl font-black text-white">{progress.consecutive_days} <span className="text-sm font-medium text-slate-400">天</span></p>
                    </div>
                    <div className="bg-white/10 rounded-2xl p-4 backdrop-blur-sm border border-white/10">
                      <p className="text-slate-400 text-xs font-semibold mb-1 uppercase tracking-wider">累计打卡 (Total)</p>
                      <p className="text-2xl font-black text-white">{progress.total_checkins} <span className="text-sm font-medium text-slate-400">次</span></p>
                    </div>
                  </div>

                  <div>
                    <div className="flex justify-between text-sm font-medium mb-2">
                      <span className="text-slate-300">阶段进度 (Phase)</span>
                      <span className="text-primary-300">
                        {progress.current_week} / {progress.total_weeks} 周
                      </span>
                    </div>
                    <div className="h-3 bg-white/10 rounded-full overflow-hidden shadow-inner">
                      <div
                        className="h-full bg-gradient-to-r from-primary-400 to-indigo-400 rounded-full"
                        style={{ width: `${(progress.current_week / progress.total_weeks) * 100}%` }}
                      />
                    </div>
                  </div>
                </div>
              </Card>
            )}

            {/* Phase list */}
            <Card className="p-6">
              <h3 className="text-lg font-bold text-slate-900 mb-4 flex items-center gap-2">
                <svg className="w-5 h-5 text-indigo-500" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2m-3 7h3m-3 4h3m-6-4h.01M9 16h.01" />
                </svg>
                训练阶段 (Phases)
              </h3>
              <div className="space-y-3 relative before:absolute before:inset-0 before:ml-5 before:-translate-x-px md:before:mx-auto md:before:translate-x-0 before:h-full before:w-0.5 before:bg-gradient-to-b before:from-transparent before:via-slate-200 before:to-transparent">
                {plan.phases?.map((phase, i) => {
                  const isActive = phase.week === plan.current_week;
                  const isPast = phase.week < plan.current_week;
                  return (
                    <div
                      key={i}
                      className={`relative flex items-center justify-between md:justify-normal md:odd:flex-row-reverse group is-active`}
                    >
                      <div className={`flex items-center justify-center w-10 h-10 rounded-full border-4 shrink-0 md:order-1 md:group-odd:-translate-x-1/2 md:group-even:translate-x-1/2 shadow-sm ${
                        isActive ? 'bg-primary-500 border-white text-white' : 
                        isPast ? 'bg-emerald-500 border-white text-white' : 'bg-slate-100 border-white text-slate-400'
                      }`}>
                        <span className="text-sm font-bold">{phase.week}</span>
                      </div>
                      
                      <div className={`w-[calc(100%-4rem)] md:w-[calc(50%-2.5rem)] p-4 rounded-xl shadow-sm border ${
                        isActive ? 'bg-primary-50 border-primary-100' : 'bg-white border-slate-100'
                      }`}>
                        <div className={`font-bold text-sm mb-1 ${isActive ? 'text-primary-700' : 'text-slate-700'}`}>Week {phase.week}</div>
                        <div className={`text-xs ${isActive ? 'text-primary-600' : 'text-slate-500'}`}>{phase.focus}</div>
                      </div>
                    </div>
                  );
                })}
              </div>
            </Card>

            {/* Reassessment */}
            <Card className="p-6">
              <button
                onClick={() => setShowReassessment(!showReassessment)}
                className="w-full text-left font-bold text-slate-900 flex items-center justify-between group"
              >
                <span className="flex items-center gap-2">
                  <svg className="w-5 h-5 text-amber-500" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M11 5H6a2 2 0 00-2 2v11a2 2 0 002 2h11a2 2 0 002-2v-5m-1.414-9.414a2 2 0 112.828 2.828L11.828 15H9v-2.828l8.586-8.586z" />
                  </svg>
                  阶段性复评 (Reassessment)
                </span>
                <span className={`w-8 h-8 rounded-full bg-slate-50 flex items-center justify-center text-slate-400 group-hover:bg-slate-100 transition-colors ${showReassessment ? 'rotate-180' : ''}`}>
                  <svg className="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 9l-7 7-7-7" />
                  </svg>
                </span>
              </button>

              {showReassessment && (
                <div className="mt-6 space-y-4 animate-in fade-in slide-in-from-top-2">
                  {reassessmentResult ? (
                    <div className="space-y-4">
                      <div className="bg-primary-50 rounded-xl p-4 border border-primary-100">
                        <h4 className="text-sm font-bold text-primary-800 mb-2 flex items-center gap-2">
                          <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z" />
                          </svg>
                          分析结果 (Analysis)
                        </h4>
                        <p className="text-sm text-primary-700 leading-relaxed">{reassessmentResult.analysis}</p>
                      </div>
                      {reassessmentResult.motivation && (
                        <div className="bg-emerald-50 rounded-xl p-4 border border-emerald-100">
                          <p className="text-sm text-emerald-700 italic">"{reassessmentResult.motivation}"</p>
                        </div>
                      )}
                    </div>
                  ) : (
                    <>
                      <div>
                        <label className="text-sm font-bold text-slate-700 block mb-1">症状变化 (Symptom Changes)</label>
                        <textarea
                          value={reassessmentFeedback.symptom_changes}
                          onChange={(e) =>
                            setReassessmentFeedback({
                              ...reassessmentFeedback,
                              symptom_changes: e.target.value,
                            })
                          }
                          placeholder="描述症状是否有改善..."
                          className="w-full rounded-xl border-slate-200 p-3 text-sm focus:ring-primary-500 focus:border-primary-500"
                          rows={2}
                        />
                      </div>
                      <div>
                        <label className="text-sm font-bold text-slate-700 block mb-1">训练感受 (Training Feeling)</label>
                        <textarea
                          value={reassessmentFeedback.training_feeling}
                          onChange={(e) =>
                            setReassessmentFeedback({
                              ...reassessmentFeedback,
                              training_feeling: e.target.value,
                            })
                          }
                          placeholder="训练过程中的感受..."
                          className="w-full rounded-xl border-slate-200 p-3 text-sm focus:ring-primary-500 focus:border-primary-500"
                          rows={2}
                        />
                      </div>
                      <div>
                        <label className="text-sm font-bold text-slate-700 block mb-1">遇到困难 (Difficulties)</label>
                        <textarea
                          value={reassessmentFeedback.difficulties}
                          onChange={(e) =>
                            setReassessmentFeedback({
                              ...reassessmentFeedback,
                              difficulties: e.target.value,
                            })
                          }
                          placeholder="训练中遇到的困难..."
                          className="w-full rounded-xl border-slate-200 p-3 text-sm focus:ring-primary-500 focus:border-primary-500"
                          rows={2}
                        />
                      </div>
                      <Button
                        onClick={handleReassessment}
                        isLoading={isReassessing}
                        className="w-full"
                      >
                        {isReassessing ? '分析中 (Analyzing)...' : '提交复评 (Submit)'}
                      </Button>
                    </>
                  )}
                </div>
              )}
            </Card>
          </div>
        </div>
      </div>

      {/* Immersive Player Fullscreen Overlay */}
      {isPlayerOpen && (() => {
        const currentExercise = exercises[currentExerciseIndex];
        if (!currentExercise) return null;

        const totalExercises = exercises.length;
        const totalSetsNum = parseInt(currentExercise.sets) || 3;

        const handleNextExercise = () => {
          if (currentExerciseIndex < totalExercises - 1) {
            setCurrentExerciseIndex((prev) => prev + 1);
            setCurrentSet(1);
            setTimerSeconds(0);
            setIsTimerRunning(false);
          } else {
            setIsPlayerOpen(false);
            const newStates = { ...exerciseStates };
            exercises.forEach((ex) => {
              newStates[ex.name] = true;
            });
            setExerciseStates(newStates);
            toast.success('恭喜！您已完成今日全部训练！');
          }
        };

        const handlePrevExercise = () => {
          if (currentExerciseIndex > 0) {
            setCurrentExerciseIndex((prev) => prev - 1);
            setCurrentSet(1);
            setTimerSeconds(0);
            setIsTimerRunning(false);
          }
        };

        const handleCompleteSet = () => {
          if (currentSet < totalSetsNum) {
            setCurrentSet((prev) => prev + 1);
            setTimerSeconds(30);
            setIsTimerRunning(true);
          } else {
            handleNextExercise();
          }
        };

        return (
          <div className="fixed inset-0 z-50 bg-slate-950/95 backdrop-blur-lg flex flex-col justify-between text-white animate-in fade-in duration-300">
            {/* Top header bar */}
            <div className="px-6 py-4 flex items-center justify-between border-b border-white/10 bg-slate-900/50">
              <div className="flex items-center gap-2">
                <span className="bg-primary-500 text-white font-bold text-xs px-2.5 py-1 rounded-full uppercase tracking-wider">
                  训练中 (Workout Mode)
                </span>
                <span className="text-slate-400 text-sm">
                  {currentExerciseIndex + 1} / {totalExercises} 个动作
                </span>
              </div>
              <button
                onClick={() => setIsPlayerOpen(false)}
                className="w-10 h-10 rounded-full bg-white/10 hover:bg-white/20 flex items-center justify-center text-white transition-colors"
              >
                <svg className="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
                </svg>
              </button>
            </div>

            {/* Main layout: split left loop & details, right sidebar */}
            <div className="flex-1 flex flex-col md:flex-row min-h-0">
              {/* Left panel: active exercise player */}
              <div className="flex-1 flex flex-col justify-center p-6 md:p-12 relative">
                <div className="max-w-2xl w-full mx-auto space-y-8">
                  {/* Loop GIF placeholder */}
                  <div className="aspect-video w-full bg-slate-900 rounded-3xl overflow-hidden border border-white/10 shadow-2xl relative flex items-center justify-center group">
                    <div className="absolute inset-0 bg-gradient-to-t from-slate-950 via-transparent to-transparent opacity-65 pointer-events-none" />
                    
                    <div className="text-center space-y-3 z-10 p-6">
                      <div className="w-20 h-20 rounded-full bg-[#CD7B67]/20 border border-[#CD7B67]/40 flex items-center justify-center mx-auto text-[#CD7B67] animate-pulse">
                        <svg className="w-10 h-10" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M14.752 11.168l-3.197-2.132A1 1 0 0010 9.87v4.263a1 1 0 001.555.832l3.197-2.132a1 1 0 000-1.664z" />
                          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
                        </svg>
                      </div>
                      <p className="text-sm font-semibold text-[#CD7B67]">动作演示循环播放中 (Looping GIF)</p>
                      <p className="text-xs text-slate-400 max-w-sm mx-auto">
                        {currentExercise.description}
                      </p>
                    </div>
                  </div>

                  {/* Title & Info */}
                  <div className="space-y-3">
                    <h2 className="text-2xl md:text-3xl font-bold tracking-tight">{currentExercise.name}</h2>
                    <div className="flex flex-wrap gap-3 text-sm">
                      <span className="bg-white/10 px-3.5 py-1.5 rounded-full font-semibold">
                        目标：{currentExercise.sets} 组
                      </span>
                      <span className="bg-white/10 px-3.5 py-1.5 rounded-full font-semibold text-primary-400">
                        次数：{currentExercise.reps}
                      </span>
                      {currentExercise.notes && (
                        <span className="bg-amber-500/25 border border-amber-500/35 text-amber-300 px-3.5 py-1.5 rounded-full font-semibold">
                          💡 {currentExercise.notes}
                        </span>
                      )}
                    </div>
                  </div>
                </div>

                {/* Rest Timer Overlay */}
                {isTimerRunning && (
                  <div className="absolute inset-0 bg-slate-950/90 backdrop-blur-md flex flex-col items-center justify-center p-6 animate-in fade-in duration-300">
                    <div className="max-w-xs w-full text-center space-y-6">
                      <p className="text-xs font-bold uppercase tracking-widest text-[#CD7B67]">休息时间 (Rest Timer)</p>
                      
                      <div className="relative w-44 h-44 mx-auto flex items-center justify-center">
                        <div className="absolute inset-0 rounded-full border-8 border-white/10" />
                        <div className="absolute inset-0 rounded-full border-8 border-[#CD7B67] border-t-transparent animate-spin" />
                        <span className="text-5xl font-black tabular-nums tracking-tighter text-white">
                          {timerSeconds}s
                        </span>
                      </div>

                      <div className="space-y-4">
                        <p className="text-sm text-slate-400">下一个动作：{currentExercise.name} 第 {currentSet} 组</p>
                        <button
                          onClick={() => {
                            setIsTimerRunning(false);
                            setTimerSeconds(0);
                          }}
                          className="px-6 py-2 bg-white hover:bg-slate-100 text-slate-900 rounded-full text-xs font-bold transition-all shadow-md shadow-white/5"
                        >
                          跳过休息 (Skip Rest)
                        </button>
                      </div>
                    </div>
                  </div>
                )}
              </div>

              {/* Right panel: Sidebar with task lists & progress */}
              <div className="w-full md:w-80 border-t md:border-t-0 md:border-l border-white/10 bg-slate-900/40 p-6 flex flex-col justify-between shrink-0">
                <div>
                  <h3 className="text-xs font-bold uppercase tracking-wider text-slate-400 mb-4">今日训练进度</h3>
                  <div className="space-y-2 max-h-64 md:max-h-none overflow-y-auto pr-1">
                    {exercises.map((ex, idx) => (
                      <div
                        key={idx}
                        className={`flex items-center gap-3 p-3 rounded-xl border text-xs transition-all ${
                          idx === currentExerciseIndex
                            ? 'bg-primary-500/20 border-primary-500 text-white'
                            : idx < currentExerciseIndex
                            ? 'bg-emerald-500/10 border-emerald-500/20 text-emerald-400'
                            : 'bg-white/5 border-transparent text-slate-400'
                        }`}
                      >
                        <div className={`w-5 h-5 rounded-full flex items-center justify-center text-[10px] font-bold ${
                          idx === currentExerciseIndex
                            ? 'bg-primary-500 text-white'
                            : idx < currentExerciseIndex
                            ? 'bg-emerald-500 text-white'
                            : 'bg-white/10 text-slate-400'
                        }`}>
                          {idx < currentExerciseIndex ? '✓' : idx + 1}
                        </div>
                        <div className="flex-1 truncate font-semibold">{ex.name}</div>
                      </div>
                    ))}
                  </div>
                </div>

                {/* Complete set button / Next controls */}
                <div className="space-y-3 mt-6">
                  <div className="text-center text-xs font-medium text-slate-400">
                    当前动作进度：第 <span className="text-white font-bold">{currentSet}</span> / {totalSetsNum} 组
                  </div>
                  <button
                    onClick={handleCompleteSet}
                    className="w-full bg-gradient-to-r from-primary-500 to-[#CD7B67] hover:from-primary-600 hover:to-[#B65E49] text-white py-4 rounded-2xl text-sm font-bold shadow-lg shadow-primary-500/10 transition-all cursor-pointer flex items-center justify-center space-x-2"
                  >
                    <span>完成第 {currentSet} 组</span>
                    <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2.5} d="M5 13l4 4L19 7" />
                    </svg>
                  </button>

                  <div className="flex gap-2">
                    <button
                      onClick={handlePrevExercise}
                      disabled={currentExerciseIndex === 0}
                      className="flex-1 bg-white/10 hover:bg-white/15 disabled:opacity-40 disabled:cursor-not-allowed py-2.5 rounded-xl text-xs font-bold transition-all text-white border border-white/5"
                    >
                      上一个
                    </button>
                    <button
                      onClick={handleNextExercise}
                      className="flex-1 bg-white/10 hover:bg-white/15 py-2.5 rounded-xl text-xs font-bold transition-all text-white border border-white/5"
                    >
                      {currentExerciseIndex === totalExercises - 1 ? '完成训练' : '跳过动作'}
                    </button>
                  </div>
                </div>
              </div>
            </div>
          </div>
        );
      })()}

      {/* AI Adjustment Proposal Modal */}
      {activeProposal && (
        <div className="fixed inset-0 z-50 bg-slate-900/60 backdrop-blur-sm flex items-center justify-center p-4 animate-in fade-in duration-300">
          <Card className="max-w-lg w-full p-6 bg-white border border-slate-200 shadow-2xl rounded-2xl flex flex-col space-y-4 animate-in zoom-in-95 duration-300">
            <div className="flex items-center space-x-2.5 text-amber-600">
              <svg className="w-6 h-6 text-amber-500 shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
              </svg>
              <h3 className="font-bold text-lg text-slate-800">AI 计划微调建议</h3>
            </div>

            <div className="space-y-3 bg-slate-50 p-4 rounded-xl border border-slate-100 text-xs text-slate-600 leading-relaxed">
              <p className="font-semibold text-slate-700">【日志不适分析】</p>
              <p>{activeProposal.analysis}</p>
            </div>

            <div className="space-y-2">
              <p className="text-xs font-bold text-slate-400 uppercase tracking-widest">微调动作内容</p>
              <div className="space-y-2 max-h-48 overflow-y-auto pr-1">
                {activeProposal.adjustments?.exercise_changes?.map((change, idx) => (
                  <div key={idx} className="bg-amber-50/50 border border-amber-100/50 p-2.5 rounded-lg text-xs flex items-start gap-2">
                    <span className="bg-amber-500 text-white font-bold text-[9px] px-1.5 py-0.5 rounded shrink-0 uppercase">
                      {change.action === '替换' ? '替换' : change.action === '移除' ? '移除' : '新增'}
                    </span>
                    <div>
                      <span className="font-bold text-slate-800">{change.exercise}</span>
                      <span className="text-slate-500 ml-1.5">({change.reason})</span>
                    </div>
                  </div>
                ))}
              </div>
            </div>

            <div className="flex gap-3 pt-2">
              <button
                onClick={handleApplyProposal}
                className="flex-1 bg-gradient-to-r from-primary-600 to-[#CD7B67] hover:from-primary-700 hover:to-[#B65E49] text-white py-2.5 rounded-xl text-xs font-bold shadow-md shadow-primary-500/10 transition-colors cursor-pointer"
              >
                同意并应用 (Agree & Apply)
              </button>
              <button
                onClick={() => setActiveProposal(null)}
                className="flex-1 bg-slate-100 border border-slate-200 text-slate-700 py-2.5 rounded-xl text-xs font-bold hover:bg-slate-200 transition-colors cursor-pointer"
              >
                忽略建议 (Ignore)
              </button>
            </div>
          </Card>
        </div>
      )}
    </MainLayout>
  );
}
