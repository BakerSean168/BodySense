import { useState } from "react";
import {
  Home,
  ClipboardList,
  BarChart3,
  MessageSquare,
  Dumbbell,
  History,
  User,
  ChevronRight,
  ChevronLeft,
  Check,
  Upload,
  Send,
  Star,
  Flame,
  Award,
  Camera,
  FileText,
  AlertTriangle,
  Heart,
  Activity,
  Moon,
  Clock,
  Calendar,
  TrendingUp,
  ChevronDown,
  X,
  Circle,
} from "lucide-react";

// ============================================================
// BodySense — AI 体态健康助手 可交互原型
// 7 个核心页面：首页、信息收集、评估报告、咨询工作台、
//              训练计划、历史记录、个人档案
// ============================================================

const COLORS = {
  primary: "indigo",
  accent: "emerald",
  warm: "amber",
};

// ============================================================
// 1. 首页 / 引导页
// ============================================================
function LandingPage({ onStart }) {
  return (
    <div className="min-h-screen bg-gradient-to-br from-indigo-50 via-white to-emerald-50 flex flex-col items-center justify-center px-4">
      <div className="max-w-lg text-center space-y-6">
        <div className="inline-flex items-center justify-center w-20 h-20 rounded-full bg-indigo-100 mb-2">
          <Heart className="w-10 h-10 text-indigo-600" />
        </div>
        <h1 className="text-3xl font-bold text-gray-900">
          AI 体态健康助手
        </h1>
        <p className="text-gray-500 text-lg leading-relaxed">
          帮你读懂身体，科学改善体态。
          <br />
          通过 AI 对话，识别体态问题，获得个性化改善方案。
        </p>
        <div className="space-y-3 pt-4">
          <button
            onClick={onStart}
            className="w-full py-3.5 px-6 bg-indigo-600 text-white rounded-xl font-medium text-lg hover:bg-indigo-700 transition-colors shadow-lg shadow-indigo-200"
          >
            开始体态评估
          </button>
          <p className="text-xs text-gray-400">
            本产品的分析和建议仅供参考，不构成医疗诊断。
          </p>
        </div>
        <div className="grid grid-cols-3 gap-4 pt-8 text-center">
          {[
            { icon: MessageSquare, label: "AI 引导问诊" },
            { icon: Activity, label: "体态评估" },
            { icon: Dumbbell, label: "训练计划" },
          ].map(({ icon: Icon, label }) => (
            <div key={label} className="space-y-2">
              <div className="mx-auto w-12 h-12 rounded-xl bg-white shadow-sm flex items-center justify-center">
                <Icon className="w-5 h-5 text-indigo-500" />
              </div>
              <p className="text-sm text-gray-600">{label}</p>
            </div>
          ))}
        </div>
      </div>
    </div>
  );
}

// ============================================================
// 2. 信息收集页（分步表单）
// ============================================================
const STEPS = [
  {
    title: "基础信息",
    desc: "帮助我们了解你的基本情况",
  },
  {
    title: "上传材料",
    desc: "可选：上传体态照片或体检报告",
  },
  {
    title: "自我描述",
    desc: "用自己的话描述当前的身体感受",
  },
];

function InfoCollectionPage({ onComplete }) {
  const [step, setStep] = useState(0);
  const [form, setForm] = useState({
    gender: "",
    age: "",
    height: "",
    weight: "",
    occupation: "",
    sleepTime: "",
    wakeTime: "",
    exerciseType: "",
    exerciseFreq: "",
    injury: "",
  });
  const [selfDesc, setSelfDesc] = useState("");

  const updateForm = (key, val) => setForm((p) => ({ ...p, [key]: val }));

  return (
    <div className="min-h-screen bg-gray-50 px-4 py-8">
      <div className="max-w-xl mx-auto">
        {/* 步骤指示器 */}
        <div className="flex items-center justify-center mb-8 space-x-2">
          {STEPS.map((s, i) => (
            <div key={i} className="flex items-center">
              <div
                className={`w-8 h-8 rounded-full flex items-center justify-center text-sm font-medium ${
                  i <= step
                    ? "bg-indigo-600 text-white"
                    : "bg-gray-200 text-gray-500"
                }`}
              >
                {i < step ? <Check className="w-4 h-4" /> : i + 1}
              </div>
              {i < STEPS.length - 1 && (
                <div
                  className={`w-12 h-0.5 mx-1 ${
                    i < step ? "bg-indigo-600" : "bg-gray-200"
                  }`}
                />
              )}
            </div>
          ))}
        </div>
        <p className="text-center text-sm text-gray-500 mb-6">
          {STEPS[step].desc}
        </p>

        {/* Step 0: 基础信息 */}
        {step === 0 && (
          <div className="bg-white rounded-2xl p-6 shadow-sm space-y-5">
            <h2 className="text-lg font-semibold text-gray-900">
              {STEPS[0].title}
            </h2>
            <div>
              <label className="block text-sm text-gray-600 mb-2">性别</label>
              <div className="flex gap-3">
                {["男", "女", "其他"].map((g) => (
                  <button
                    key={g}
                    onClick={() => updateForm("gender", g)}
                    className={`flex-1 py-2.5 rounded-xl text-sm font-medium border transition-colors ${
                      form.gender === g
                        ? "bg-indigo-50 border-indigo-400 text-indigo-700"
                        : "border-gray-200 text-gray-600 hover:border-gray-300"
                    }`}
                  >
                    {g}
                  </button>
                ))}
              </div>
            </div>
            <div className="grid grid-cols-3 gap-3">
              <div>
                <label className="block text-sm text-gray-600 mb-1">年龄</label>
                <input
                  type="number"
                  placeholder="24"
                  value={form.age}
                  onChange={(e) => updateForm("age", e.target.value)}
                  className="w-full px-3 py-2.5 border border-gray-200 rounded-xl text-sm focus:outline-none focus:border-indigo-400"
                />
              </div>
              <div>
                <label className="block text-sm text-gray-600 mb-1">
                  身高(cm)
                </label>
                <input
                  type="number"
                  placeholder="175"
                  value={form.height}
                  onChange={(e) => updateForm("height", e.target.value)}
                  className="w-full px-3 py-2.5 border border-gray-200 rounded-xl text-sm focus:outline-none focus:border-indigo-400"
                />
              </div>
              <div>
                <label className="block text-sm text-gray-600 mb-1">
                  体重(kg)
                </label>
                <input
                  type="number"
                  placeholder="70"
                  value={form.weight}
                  onChange={(e) => updateForm("weight", e.target.value)}
                  className="w-full px-3 py-2.5 border border-gray-200 rounded-xl text-sm focus:outline-none focus:border-indigo-400"
                />
              </div>
            </div>
            <div>
              <label className="block text-sm text-gray-600 mb-1">
                职业/日常活动类型
              </label>
              <input
                type="text"
                placeholder="如：程序员，日均久坐 10h"
                value={form.occupation}
                onChange={(e) => updateForm("occupation", e.target.value)}
                className="w-full px-3 py-2.5 border border-gray-200 rounded-xl text-sm focus:outline-none focus:border-indigo-400"
              />
            </div>
            <div>
              <label className="block text-sm text-gray-600 mb-2">
                运动习惯
              </label>
              <div className="flex gap-3">
                {[
                  "几乎不运动",
                  "每周 1-2 次",
                  "每周 3-5 次",
                  "每天运动",
                ].map((f) => (
                  <button
                    key={f}
                    onClick={() => updateForm("exerciseFreq", f)}
                    className={`flex-1 py-2 rounded-lg text-xs font-medium border transition-colors ${
                      form.exerciseFreq === f
                        ? "bg-indigo-50 border-indigo-400 text-indigo-700"
                        : "border-gray-200 text-gray-500 hover:border-gray-300"
                    }`}
                  >
                    {f}
                  </button>
                ))}
              </div>
            </div>
            <div>
              <label className="block text-sm text-gray-600 mb-1">
                既往伤病/手术史（可选）
              </label>
              <input
                type="text"
                placeholder="如：右膝半月板损伤"
                value={form.injury}
                onChange={(e) => updateForm("injury", e.target.value)}
                className="w-full px-3 py-2.5 border border-gray-200 rounded-xl text-sm focus:outline-none focus:border-indigo-400"
              />
            </div>
          </div>
        )}

        {/* Step 1: 上传材料 */}
        {step === 1 && (
          <div className="bg-white rounded-2xl p-6 shadow-sm space-y-5">
            <h2 className="text-lg font-semibold text-gray-900">
              {STEPS[1].title}
            </h2>
            <p className="text-sm text-gray-500">
              此步骤为可选内容，你可以跳过。
            </p>
            <div className="border-2 border-dashed border-gray-200 rounded-2xl p-8 text-center hover:border-indigo-300 transition-colors cursor-pointer">
              <Camera className="w-8 h-8 text-gray-300 mx-auto mb-3" />
              <p className="text-sm font-medium text-gray-600">
                上传体态照片
              </p>
              <p className="text-xs text-gray-400 mt-1">
                正面、侧面、背面全身站姿照片
              </p>
            </div>
            <div className="border-2 border-dashed border-gray-200 rounded-2xl p-8 text-center hover:border-indigo-300 transition-colors cursor-pointer">
              <FileText className="w-8 h-8 text-gray-300 mx-auto mb-3" />
              <p className="text-sm font-medium text-gray-600">
                上传体检报告
              </p>
              <p className="text-xs text-gray-400 mt-1">
                支持图片或 PDF 格式
              </p>
            </div>
          </div>
        )}

        {/* Step 2: 自我描述 */}
        {step === 2 && (
          <div className="bg-white rounded-2xl p-6 shadow-sm space-y-5">
            <h2 className="text-lg font-semibold text-gray-900">
              {STEPS[2].title}
            </h2>
            <p className="text-sm text-gray-500">
              描述你目前的身体感受和困扰，AI 将基于这些信息进行分析。
            </p>
            <textarea
              rows={5}
              placeholder="例如：最近半年经常觉得肩膀酸，尤其是下午工作久了以后，脖子也会不舒服……"
              value={selfDesc}
              onChange={(e) => setSelfDesc(e.target.value)}
              className="w-full px-4 py-3 border border-gray-200 rounded-xl text-sm focus:outline-none focus:border-indigo-400 resize-none"
            />
            <div className="space-y-2">
              <p className="text-xs text-gray-400">引导问题：</p>
              {[
                "你目前最困扰的身体问题是什么？",
                "什么时候开始感觉到这个问题的？",
                "哪些动作会让你不舒服？",
              ].map((q) => (
                <p key={q} className="text-xs text-indigo-500 cursor-pointer">
                  {q}
                </p>
              ))}
            </div>
          </div>
        )}

        {/* 导航按钮 */}
        <div className="flex justify-between mt-6">
          <button
            onClick={() => (step > 0 ? setStep(step - 1) : null)}
            className={`flex items-center gap-1 px-4 py-2.5 rounded-xl text-sm font-medium transition-colors ${
              step > 0
                ? "text-gray-600 hover:bg-gray-100"
                : "text-gray-300 cursor-not-allowed"
            }`}
          >
            <ChevronLeft className="w-4 h-4" />
            上一步
          </button>
          {step < STEPS.length - 1 ? (
            <button
              onClick={() => setStep(step + 1)}
              className="flex items-center gap-1 px-5 py-2.5 bg-indigo-600 text-white rounded-xl text-sm font-medium hover:bg-indigo-700 transition-colors"
            >
              下一步
              <ChevronRight className="w-4 h-4" />
            </button>
          ) : (
            <button
              onClick={onComplete}
              className="flex items-center gap-1 px-5 py-2.5 bg-emerald-600 text-white rounded-xl text-sm font-medium hover:bg-emerald-700 transition-colors"
            >
              生成评估报告
              <Check className="w-4 h-4" />
            </button>
          )}
        </div>
      </div>
    </div>
  );
}

// ============================================================
// 3. 评估报告页
// ============================================================
function AssessmentPage({ onConsult, onTrain }) {
  return (
    <div className="min-h-screen bg-gray-50 px-4 py-8">
      <div className="max-w-xl mx-auto space-y-5">
        <h2 className="text-xl font-bold text-gray-900">健康评估报告</h2>
        <p className="text-xs text-gray-400">生成时间：2026-06-19 10:30</p>

        {/* 健康等级 */}
        <div className="bg-white rounded-2xl p-6 shadow-sm text-center">
          <p className="text-sm text-gray-500 mb-2">综合健康等级</p>
          <div className="text-5xl font-bold text-amber-500 mb-2">B+</div>
          <div className="flex justify-center gap-1 mb-3">
            {[1, 2, 3, 4, 5].map((s) => (
              <Star
                key={s}
                className={`w-5 h-5 ${
                  s <= 3
                    ? "text-amber-400 fill-amber-400"
                    : "text-gray-200"
                }`}
              />
            ))}
          </div>
          <div className="grid grid-cols-3 gap-3 text-center">
            {[
              { label: "体态", score: 72 },
              { label: "运动能力", score: 80 },
              { label: "生活习惯", score: 68 },
            ].map(({ label, score }) => (
              <div key={label}>
                <p className="text-lg font-semibold text-gray-800">{score}</p>
                <p className="text-xs text-gray-400">{label}</p>
              </div>
            ))}
          </div>
        </div>

        {/* 主要问题识别 */}
        <div className="bg-white rounded-2xl p-6 shadow-sm space-y-4">
          <h3 className="text-sm font-semibold text-gray-900">
            主要问题识别
          </h3>
          {[
            {
              name: "上交叉综合征（圆肩/头前伸）",
              severity: "中度",
              color: "amber",
            },
            {
              name: "核心肌群稳定性不足",
              severity: "轻度",
              color: "emerald",
            },
            {
              name: "久坐导致的髋屈肌紧张",
              severity: "轻度",
              color: "emerald",
            },
          ].map((item) => (
            <div
              key={item.name}
              className="flex items-center justify-between p-3 bg-gray-50 rounded-xl"
            >
              <span className="text-sm text-gray-700">{item.name}</span>
              <span
                className={`text-xs font-medium px-2 py-0.5 rounded-full ${
                  item.color === "amber"
                    ? "bg-amber-100 text-amber-700"
                    : "bg-emerald-100 text-emerald-700"
                }`}
              >
                {item.severity}
              </span>
            </div>
          ))}
        </div>

        {/* 改善方向概要 */}
        <div className="bg-white rounded-2xl p-6 shadow-sm space-y-3">
          <h3 className="text-sm font-semibold text-gray-900">
            改善方案概要
          </h3>
          {[
            { icon: Dumbbell, text: "针对性拉伸和强化训练，改善圆肩和头前伸" },
            { icon: Moon, text: "优化工作间歇习惯，每 45 分钟起身活动" },
            {
              icon: Heart,
              text: "增加核心稳定性训练，每周 2-3 次",
            },
          ].map(({ icon: Icon, text }) => (
            <div key={text} className="flex items-start gap-3">
              <div className="w-8 h-8 rounded-lg bg-indigo-50 flex items-center justify-center flex-shrink-0 mt-0.5">
                <Icon className="w-4 h-4 text-indigo-500" />
              </div>
              <p className="text-sm text-gray-600">{text}</p>
            </div>
          ))}
        </div>

        {/* 免责声明 */}
        <div className="bg-amber-50 border border-amber-200 rounded-xl p-4 flex gap-3">
          <AlertTriangle className="w-5 h-5 text-amber-500 flex-shrink-0 mt-0.5" />
          <p className="text-xs text-amber-700 leading-relaxed">
            本报告仅供参考，不构成医疗诊断。如存在持续疼痛或严重不适，建议前往专业医疗机构就诊。
          </p>
        </div>

        {/* 操作按钮 */}
        <div className="flex gap-3">
          <button
            onClick={onConsult}
            className="flex-1 py-3 bg-indigo-600 text-white rounded-xl text-sm font-medium hover:bg-indigo-700 transition-colors"
          >
            进入咨询工作台
          </button>
          <button
            onClick={onTrain}
            className="flex-1 py-3 border border-indigo-200 text-indigo-600 rounded-xl text-sm font-medium hover:bg-indigo-50 transition-colors"
          >
            生成训练计划
          </button>
        </div>
      </div>
    </div>
  );
}

// ============================================================
// 4. 咨询工作台（左右分栏）
// ============================================================
function ConsultationPage() {
  const [messages, setMessages] = useState([
    {
      role: "ai",
      text: "你好！我已经看过你的评估报告了。你目前最困扰的问题是什么？可以详细描述一下你的不适感受。",
    },
    {
      role: "user",
      text: "主要是肩膀和脖子不舒服，尤其是下午工作久了以后，感觉肩膀很紧，有时候头也会疼。",
    },
    {
      role: "ai",
      text: "了解了。你说的'肩膀紧'，是以下哪种感觉？\n\n① 酸胀感，按压时会舒服一些\n② 尖锐的刺痛，特定角度会加重\n③ 持续的钝痛，活动后反而减轻\n④ 其他（请描述）",
    },
  ]);
  const [input, setInput] = useState("");
  const [showDiagnosis, setShowDiagnosis] = useState(true);

  const sendMessage = () => {
    if (!input.trim()) return;
    setMessages((m) => [
      ...m,
      { role: "user", text: input },
      {
        role: "ai",
        text: "感谢你的详细描述。结合你的久坐工作习惯和症状表现，我来做一个初步分析……",
      },
    ]);
    setInput("");
  };

  return (
    <div className="h-screen flex flex-col bg-gray-50">
      <div className="px-4 py-3 bg-white border-b flex items-center justify-between">
        <h2 className="text-sm font-semibold text-gray-900">咨询工作台</h2>
        <span className="text-xs text-gray-400">会话 #001</span>
      </div>
      <div className="flex-1 flex overflow-hidden">
        {/* 左侧对话区 */}
        <div className="flex-1 flex flex-col border-r">
          <div className="flex-1 overflow-y-auto p-4 space-y-4">
            {messages.map((msg, i) => (
              <div
                key={i}
                className={`flex ${
                  msg.role === "user" ? "justify-end" : "justify-start"
                }`}
              >
                <div
                  className={`max-w-[80%] px-4 py-3 rounded-2xl text-sm leading-relaxed whitespace-pre-line ${
                    msg.role === "user"
                      ? "bg-indigo-600 text-white rounded-br-sm"
                      : "bg-white shadow-sm text-gray-700 rounded-bl-sm"
                  }`}
                >
                  {msg.text}
                  {msg.role === "ai" &&
                    msg.text.includes("哪种感觉") && (
                      <div className="mt-2 space-y-1.5">
                        {[
                          "① 酸胀感，按压时会舒服一些",
                          "② 尖锐的刺痛，特定角度会加重",
                          "③ 持续的钝痛，活动后反而减轻",
                          "④ 其他",
                        ].map((opt) => (
                          <button
                            key={opt}
                            onClick={() => {
                              setMessages((m) => [
                                ...m,
                                { role: "user", text: opt },
                              ]);
                            }}
                            className="block w-full text-left px-3 py-2 bg-indigo-50 hover:bg-indigo-100 rounded-lg text-xs text-indigo-700 transition-colors"
                          >
                            {opt}
                          </button>
                        ))}
                      </div>
                    )}
                </div>
              </div>
            ))}
          </div>
          {/* 输入框 */}
          <div className="p-3 bg-white border-t">
            <div className="flex items-center gap-2">
              <input
                value={input}
                onChange={(e) => setInput(e.target.value)}
                onKeyDown={(e) => e.key === "Enter" && sendMessage()}
                placeholder="描述你的症状或问题..."
                className="flex-1 px-4 py-2.5 border border-gray-200 rounded-xl text-sm focus:outline-none focus:border-indigo-400"
              />
              <button
                onClick={sendMessage}
                className="p-2.5 bg-indigo-600 text-white rounded-xl hover:bg-indigo-700 transition-colors"
              >
                <Send className="w-4 h-4" />
              </button>
            </div>
          </div>
        </div>

        {/* 右侧信息面板 */}
        <div className="w-80 bg-white overflow-y-auto p-4 space-y-4 hidden lg:block">
          {/* 结构化信息区 */}
          <div className="space-y-3">
            <h3 className="text-xs font-semibold text-gray-500 uppercase tracking-wider">
              AI 提取的结构化信息
            </h3>
            <div className="space-y-2">
              {[
                {
                  label: "涉及部位",
                  value: "肩部、颈部",
                  color: "indigo",
                },
                {
                  label: "症状类型",
                  value: "肌肉紧张感",
                  color: "amber",
                },
                {
                  label: "触发场景",
                  value: "久坐后（下午加重）",
                  color: "rose",
                },
                {
                  label: "持续时间",
                  value: "近半年",
                  color: "gray",
                },
              ].map(({ label, value, color }) => (
                <div
                  key={label}
                  className="flex items-center justify-between p-2.5 bg-gray-50 rounded-lg"
                >
                  <div>
                    <p className="text-xs text-gray-400">{label}</p>
                    <p className="text-sm text-gray-700">{value}</p>
                  </div>
                  <button className="text-xs text-indigo-500">修改</button>
                </div>
              ))}
            </div>
          </div>

          {/* 可能性判断 */}
          {showDiagnosis && (
            <div className="space-y-3">
              <h3 className="text-xs font-semibold text-gray-500 uppercase tracking-wider">
                可能性判断
              </h3>
              {[
                {
                  name: "上交叉综合征",
                  confidence: "高",
                  match: 85,
                },
                {
                  name: "颈型颈椎病（早期）",
                  confidence: "中",
                  match: 45,
                },
              ].map((d) => (
                <div key={d.name} className="p-3 bg-gray-50 rounded-xl">
                  <div className="flex items-center justify-between mb-1">
                    <span className="text-sm font-medium text-gray-800">
                      {d.name}
                    </span>
                    <span
                      className={`text-xs px-1.5 py-0.5 rounded-full ${
                        d.confidence === "高"
                          ? "bg-emerald-100 text-emerald-700"
                          : "bg-amber-100 text-amber-700"
                      }`}
                    >
                      置信度：{d.confidence}
                    </span>
                  </div>
                  <div className="w-full bg-gray-200 rounded-full h-1.5">
                    <div
                      className="bg-indigo-500 h-1.5 rounded-full"
                      style={{ width: `${d.match}%` }}
                    />
                  </div>
                  <p className="text-xs text-gray-400 mt-1">
                    匹配度 {d.match}%
                  </p>
                </div>
              ))}
              <button
                onClick={() => setShowDiagnosis(false)}
                className="w-full py-2 bg-indigo-600 text-white rounded-xl text-sm font-medium hover:bg-indigo-700 transition-colors"
              >
                确认诊断，生成改善方案
              </button>
            </div>
          )}

          {/* 人体可视化（简化示意） */}
          <div className="space-y-3">
            <h3 className="text-xs font-semibold text-gray-500 uppercase tracking-wider">
              身体可视化
            </h3>
            <div className="bg-gray-50 rounded-xl p-4 flex justify-center">
              <svg
                viewBox="0 0 100 200"
                className="w-24 h-48"
                fill="none"
                stroke="#cbd5e1"
                strokeWidth="1.5"
              >
                {/* 头 */}
                <circle cx="50" cy="22" r="14" />
                {/* 脖子 - 高亮 */}
                <line
                  x1="50"
                  y1="36"
                  x2="50"
                  y2="48"
                  stroke="#ef4444"
                  strokeWidth="3"
                />
                {/* 肩膀 - 高亮 */}
                <line
                  x1="25"
                  y1="55"
                  x2="75"
                  y2="55"
                  stroke="#ef4444"
                  strokeWidth="3"
                />
                {/* 身体 */}
                <line x1="50" y1="48" x2="50" y2="120" />
                {/* 手臂 */}
                <line x1="25" y1="55" x2="20" y2="100" />
                <line x1="75" y1="55" x2="80" y2="100" />
                {/* 腿 */}
                <line x1="50" y1="120" x2="35" y2="185" />
                <line x1="50" y1="120" x2="65" y2="185" />
              </svg>
            </div>
            <p className="text-xs text-red-500 text-center">
              红色区域为当前讨论涉及的部位
            </p>
          </div>
        </div>
      </div>
    </div>
  );
}

// ============================================================
// 5. 训练计划页
// ============================================================
function TrainingPage() {
  const [activeTab, setActiveTab] = useState("today");
  const [checkedItems, setCheckedItems] = useState({});

  const toggle = (id) =>
    setCheckedItems((p) => ({ ...p, [id]: !p[id] }));

  const todayExercises = [
    {
      id: 1,
      name: "胸椎伸展（泡沫轴）",
      sets: "3 组",
      reps: "每组 10 次",
      rest: "30 秒",
    },
    {
      id: 2,
      name: "靠墙天使",
      sets: "3 组",
      reps: "每组 12 次",
      rest: "30 秒",
    },
    {
      id: 3,
      name: "颈部回缩（下巴收缩）",
      sets: "3 组",
      reps: "每组 15 次",
      rest: "20 秒",
    },
    {
      id: 4,
      name: "死虫式",
      sets: "3 组",
      reps: "每组 10 次（每侧）",
      rest: "30 秒",
    },
    {
      id: 5,
      name: "猫牛式拉伸",
      sets: "2 组",
      reps: "每组 15 次",
      rest: "20 秒",
    },
  ];

  const checkedCount = Object.values(checkedItems).filter(Boolean).length;
  const totalCount = todayExercises.length;

  return (
    <div className="min-h-screen bg-gray-50">
      {/* 顶部进度 */}
      <div className="bg-white px-4 py-4 border-b">
        <div className="max-w-xl mx-auto">
          <div className="flex items-center justify-between mb-2">
            <h2 className="text-lg font-bold text-gray-900">训练计划</h2>
            <span className="text-xs text-gray-400">第 2 周 / 共 4 周</span>
          </div>
          <div className="flex gap-1">
            {["today", "plan", "progress"].map((tab) => (
              <button
                key={tab}
                onClick={() => setActiveTab(tab)}
                className={`flex-1 py-2 text-xs font-medium rounded-lg transition-colors ${
                  activeTab === tab
                    ? "bg-indigo-600 text-white"
                    : "text-gray-500 hover:bg-gray-100"
                }`}
              >
                {tab === "today"
                  ? "今日训练"
                  : tab === "plan"
                  ? "计划总览"
                  : "进度追踪"}
              </button>
            ))}
          </div>
        </div>
      </div>

      <div className="max-w-xl mx-auto px-4 py-6">
        {activeTab === "today" && (
          <div className="space-y-4">
            <div className="flex items-center justify-between">
              <p className="text-sm text-gray-500">
                6月19日 · 周四 · 训练日
              </p>
              <p className="text-sm font-medium text-indigo-600">
                {checkedCount}/{totalCount} 已完成
              </p>
            </div>

            {todayExercises.map((ex) => (
              <div
                key={ex.id}
                className={`bg-white rounded-xl p-4 shadow-sm flex items-center gap-4 transition-all ${
                  checkedItems[ex.id] ? "opacity-60" : ""
                }`}
              >
                <button
                  onClick={() => toggle(ex.id)}
                  className={`w-6 h-6 rounded-full border-2 flex items-center justify-center flex-shrink-0 transition-colors ${
                    checkedItems[ex.id]
                      ? "bg-emerald-500 border-emerald-500"
                      : "border-gray-300"
                  }`}
                >
                  {checkedItems[ex.id] && (
                    <Check className="w-3.5 h-3.5 text-white" />
                  )}
                </button>
                <div className="flex-1 min-w-0">
                  <p
                    className={`text-sm font-medium ${
                      checkedItems[ex.id]
                        ? "line-through text-gray-400"
                        : "text-gray-800"
                    }`}
                  >
                    {ex.name}
                  </p>
                  <p className="text-xs text-gray-400 mt-0.5">
                    {ex.sets} · {ex.reps} · 休息 {ex.rest}
                  </p>
                </div>
                <button className="text-xs text-indigo-500 flex-shrink-0">
                  详解
                </button>
              </div>
            ))}

            {checkedCount === totalCount && (
              <div className="bg-emerald-50 border border-emerald-200 rounded-xl p-4 text-center">
                <p className="text-emerald-700 font-medium">
                  今日训练全部完成！
                </p>
                <button className="mt-2 px-6 py-2 bg-emerald-600 text-white rounded-xl text-sm font-medium hover:bg-emerald-700 transition-colors">
                  打卡
                </button>
              </div>
            )}
          </div>
        )}

        {activeTab === "plan" && (
          <div className="space-y-4">
            <div className="bg-white rounded-xl p-5 shadow-sm">
              <h3 className="text-sm font-semibold text-gray-900 mb-3">
                改善圆肩 & 头前伸 · 4 周计划
              </h3>
              {[
                {
                  week: "第 1 周",
                  focus: "基础激活",
                  desc: "唤醒深层稳定肌群，建立正确运动模式",
                  done: true,
                },
                {
                  week: "第 2 周",
                  focus: "强化训练",
                  desc: "增加训练强度，强化中下斜方肌和深层颈屈肌",
                  done: false,
                  active: true,
                },
                {
                  week: "第 3 周",
                  focus: "功能整合",
                  desc: "将改善融入日常动作模式",
                  done: false,
                },
                {
                  week: "第 4 周",
                  focus: "巩固维持",
                  desc: "建立长期训练习惯，自我评估",
                  done: false,
                },
              ].map((w) => (
                <div
                  key={w.week}
                  className={`flex items-start gap-3 p-3 rounded-lg mb-2 ${
                    w.active ? "bg-indigo-50" : ""
                  }`}
                >
                  <div
                    className={`w-6 h-6 rounded-full flex items-center justify-center flex-shrink-0 mt-0.5 ${
                      w.done
                        ? "bg-emerald-500"
                        : w.active
                        ? "bg-indigo-600"
                        : "bg-gray-200"
                    }`}
                  >
                    {w.done ? (
                      <Check className="w-3.5 h-3.5 text-white" />
                    ) : (
                      <Circle className="w-3 h-3 text-white" />
                    )}
                  </div>
                  <div>
                    <p className="text-sm font-medium text-gray-800">
                      {w.week} — {w.focus}
                    </p>
                    <p className="text-xs text-gray-500 mt-0.5">{w.desc}</p>
                  </div>
                </div>
              ))}
            </div>
          </div>
        )}

        {activeTab === "progress" && (
          <div className="space-y-4">
            {/* 统计卡片 */}
            <div className="grid grid-cols-3 gap-3">
              {[
                { label: "连续打卡", value: "8 天", icon: Flame },
                { label: "本周完成率", value: "85%", icon: TrendingUp },
                { label: "总训练次数", value: "12", icon: Dumbbell },
              ].map(({ label, value, icon: Icon }) => (
                <div
                  key={label}
                  className="bg-white rounded-xl p-4 shadow-sm text-center"
                >
                  <Icon className="w-5 h-5 text-indigo-500 mx-auto mb-1" />
                  <p className="text-lg font-bold text-gray-800">{value}</p>
                  <p className="text-xs text-gray-400">{label}</p>
                </div>
              ))}
            </div>

            {/* 成就徽章 */}
            <div className="bg-white rounded-xl p-5 shadow-sm">
              <h3 className="text-sm font-semibold text-gray-900 mb-3">
                成就徽章
              </h3>
              <div className="flex gap-3">
                {[
                  { name: "初次打卡", earned: true },
                  { name: "连续 7 天", earned: true },
                  { name: "完成第 1 阶段", earned: false },
                  { name: "连续 30 天", earned: false },
                ].map((a) => (
                  <div
                    key={a.name}
                    className={`text-center ${
                      a.earned ? "" : "opacity-30"
                    }`}
                  >
                    <div
                      className={`w-12 h-12 rounded-full flex items-center justify-center mx-auto mb-1 ${
                        a.earned ? "bg-amber-100" : "bg-gray-100"
                      }`}
                    >
                      <Award
                        className={`w-6 h-6 ${
                          a.earned ? "text-amber-500" : "text-gray-400"
                        }`}
                      />
                    </div>
                    <p className="text-xs text-gray-600">{a.name}</p>
                  </div>
                ))}
              </div>
            </div>
          </div>
        )}
      </div>
    </div>
  );
}

// ============================================================
// 6. 历史记录页
// ============================================================
function HistoryPage() {
  return (
    <div className="min-h-screen bg-gray-50 px-4 py-8">
      <div className="max-w-xl mx-auto space-y-5">
        <h2 className="text-xl font-bold text-gray-900">历史记录</h2>

        {[
          {
            date: "2026-06-19",
            type: "咨询会话",
            title: "肩膀和颈部不适",
            status: "进行中",
          },
          {
            date: "2026-06-19",
            type: "评估报告",
            title: "初次健康评估",
            status: "已完成",
          },
          {
            date: "2026-06-15",
            type: "咨询会话",
            title: "腰部酸痛讨论",
            status: "已完成",
          },
        ].map((item, i) => (
          <div
            key={i}
            className="bg-white rounded-xl p-4 shadow-sm flex items-center gap-4 cursor-pointer hover:shadow-md transition-shadow"
          >
            <div
              className={`w-10 h-10 rounded-xl flex items-center justify-center flex-shrink-0 ${
                item.type === "咨询会话"
                  ? "bg-indigo-50"
                  : "bg-emerald-50"
              }`}
            >
              {item.type === "咨询会话" ? (
                <MessageSquare className="w-5 h-5 text-indigo-500" />
              ) : (
                <BarChart3 className="w-5 h-5 text-emerald-500" />
              )}
            </div>
            <div className="flex-1 min-w-0">
              <p className="text-sm font-medium text-gray-800">
                {item.title}
              </p>
              <p className="text-xs text-gray-400 mt-0.5">
                {item.date} · {item.type}
              </p>
            </div>
            <span
              className={`text-xs px-2 py-0.5 rounded-full ${
                item.status === "进行中"
                  ? "bg-indigo-100 text-indigo-700"
                  : "bg-gray-100 text-gray-500"
              }`}
            >
              {item.status}
            </span>
            <ChevronRight className="w-4 h-4 text-gray-300" />
          </div>
        ))}
      </div>
    </div>
  );
}

// ============================================================
// 7. 个人档案页
// ============================================================
function ProfilePage() {
  return (
    <div className="min-h-screen bg-gray-50 px-4 py-8">
      <div className="max-w-xl mx-auto space-y-5">
        <h2 className="text-xl font-bold text-gray-900">个人档案</h2>

        {/* 基本信息 */}
        <div className="bg-white rounded-2xl p-6 shadow-sm space-y-4">
          <h3 className="text-sm font-semibold text-gray-500">基本信息</h3>
          {[
            { label: "性别", value: "男" },
            { label: "年龄", value: "24 岁" },
            { label: "身高", value: "175 cm" },
            { label: "体重", value: "70 kg" },
            { label: "BMI", value: "22.9（正常）" },
            { label: "职业", value: "程序员，日均久坐 10h" },
          ].map(({ label, value }) => (
            <div key={label} className="flex justify-between items-center">
              <span className="text-sm text-gray-500">{label}</span>
              <span className="text-sm font-medium text-gray-800">
                {value}
              </span>
            </div>
          ))}
          <button className="w-full py-2 text-sm text-indigo-600 font-medium hover:bg-indigo-50 rounded-lg transition-colors">
            编辑基本信息
          </button>
        </div>

        {/* 生活习惯 */}
        <div className="bg-white rounded-2xl p-6 shadow-sm space-y-4">
          <h3 className="text-sm font-semibold text-gray-500">生活习惯</h3>
          {[
            { label: "作息", value: "凌晨 1 点入睡，8 点起床" },
            { label: "运动频率", value: "每周 3 次" },
            { label: "运动类型", value: "跑步" },
          ].map(({ label, value }) => (
            <div key={label} className="flex justify-between items-center">
              <span className="text-sm text-gray-500">{label}</span>
              <span className="text-sm font-medium text-gray-800">
                {value}
              </span>
            </div>
          ))}
        </div>

        {/* 已上传材料 */}
        <div className="bg-white rounded-2xl p-6 shadow-sm space-y-4">
          <h3 className="text-sm font-semibold text-gray-500">
            已上传材料
          </h3>
          <p className="text-sm text-gray-400">暂无上传材料</p>
          <button className="w-full py-2.5 border-2 border-dashed border-gray-200 text-sm text-gray-500 rounded-xl hover:border-indigo-300 hover:text-indigo-500 transition-colors">
            上传体态照片或体检报告
          </button>
        </div>

        {/* 操作区 */}
        <div className="space-y-2">
          <button className="w-full py-2.5 text-sm text-red-500 hover:bg-red-50 rounded-xl transition-colors">
            清除全部数据
          </button>
        </div>
      </div>
    </div>
  );
}

// ============================================================
// 主应用：页面导航
// ============================================================
const PAGES = [
  { key: "home", label: "首页", icon: Home },
  { key: "collect", label: "信息收集", icon: ClipboardList },
  { key: "assessment", label: "评估报告", icon: BarChart3 },
  { key: "consult", label: "咨询工作台", icon: MessageSquare },
  { key: "training", label: "训练计划", icon: Dumbbell },
  { key: "history", label: "历史记录", icon: History },
  { key: "profile", label: "个人档案", icon: User },
];

export default function BodySensePrototype() {
  const [page, setPage] = useState("home");

  const renderPage = () => {
    switch (page) {
      case "home":
        return <LandingPage onStart={() => setPage("collect")} />;
      case "collect":
        return <InfoCollectionPage onComplete={() => setPage("assessment")} />;
      case "assessment":
        return (
          <AssessmentPage
            onConsult={() => setPage("consult")}
            onTrain={() => setPage("training")}
          />
        );
      case "consult":
        return <ConsultationPage />;
      case "training":
        return <TrainingPage />;
      case "history":
        return <HistoryPage />;
      case "profile":
        return <ProfilePage />;
      default:
        return <LandingPage onStart={() => setPage("collect")} />;
    }
  };

  return (
    <div className="flex flex-col h-screen bg-gray-50">
      {/* 页面内容 */}
      <div className="flex-1 overflow-y-auto">{renderPage()}</div>

      {/* 底部导航栏 */}
      <div className="bg-white border-t px-2 py-2 flex items-center justify-around flex-shrink-0">
        {PAGES.map(({ key, label, icon: Icon }) => (
          <button
            key={key}
            onClick={() => setPage(key)}
            className={`flex flex-col items-center gap-0.5 px-2 py-1 rounded-lg transition-colors ${
              page === key ? "text-indigo-600" : "text-gray-400"
            }`}
          >
            <Icon className="w-5 h-5" />
            <span className="text-[10px]">{label}</span>
          </button>
        ))}
      </div>
    </div>
  );
}
