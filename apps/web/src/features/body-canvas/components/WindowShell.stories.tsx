import { useState, type ReactNode } from "react";
import { useMediaQuery } from "@/hooks/useMediaQuery";
import type { Meta, StoryObj } from "@storybook/react-vite";
import { MessageCircle } from "lucide-react";
import { BusinessWindowHost } from "./BusinessWindowHost";
import { ChatHistoryMenu } from "../../consultation/components/chat/ChatHistoryMenu";
import { ChatWindowHeader } from "../../consultation/components/chat/ChatWindowHeader";
import { DiagnosisWindowHeader } from "../windows/diagnosis/DiagnosisWindowHeader";
import { TrainingWindowHeader } from "../windows/training/TrainingWindowHeader";
import { PersonWindowHeader } from "../windows/person/PersonWindowHeader";
import { selectTrainingPlan } from "../model/trainingPlanSelection";
import { makePlan } from "../model/__fixtures__/trainingPlanFixtures";
import type { BusinessWindowId } from "../model/businessWindows";
import type { ProfileCategoryId } from "../model/profileCategories";
import { windowCanvasDecorator } from "./windowStorySupport";

type PreviewWindow = Exclude<BusinessWindowId, "trainer">;

interface ShellPreviewProps {
  windowId: PreviewWindow;
}

const trainingPlans = [
  makePlan(),
  makePlan({
    planId: "storybook-plan-2",
    title: "活动度训练",
    status: "paused",
  }),
];

const chatHistory = {
  todayKey: "2026-10-21",
  days: [
    {
      day: "2026-10-21",
      label: "今天",
      count: 2,
      preview: "今天的身体记录",
      firstMessageId: "today",
    },
    {
      day: "2026-10-20",
      label: "昨天",
      count: 1,
      preview: "昨天的训练感受",
      firstMessageId: "yesterday",
    },
  ],
};

function ShellPreview({ windowId }: ShellPreviewProps) {
  const mobile = useMediaQuery("(max-width: 760px)");
  const [visible, setVisible] = useState(true);
  const [category, setCategory] = useState<ProfileCategoryId>("records");
  const [personOpen, setPersonOpen] = useState(false);
  const [conditionId, setConditionId] = useState("first");
  const [planId, setPlanId] = useState(trainingPlans[0].planId);
  const [chatDay, setChatDay] = useState(chatHistory.todayKey);

  let header: ReactNode;
  let actions: ReactNode = null;

  switch (windowId) {
    case "person":
      header = (
        <PersonWindowHeader
          identity={{ nickname: "Storybook 用户" }}
          open={personOpen}
          onToggle={() => setPersonOpen((current) => !current)}
          category={category}
          onCategoryChange={setCategory}
        />
      );
      break;
    case "chat":
      header = (
        <ChatWindowHeader
          history={chatHistory}
          selectedDay={chatDay}
          onSelectDay={setChatDay}
        />
      );
      actions = (
        <ChatHistoryMenu history={chatHistory} onSelectDay={setChatDay} />
      );
      break;
    case "diagnosis":
      header = (
        <DiagnosisWindowHeader
          view={{
            confirmed: [
              { conditionId: "first", name: "示例病情 A" },
              { conditionId: "second", name: "示例病情 B" },
            ],
          }}
          selectedConditionId={conditionId}
          onSelectCondition={setConditionId}
        />
      );
      actions = (
        <button
          className="bc-icon-button"
          type="button"
          aria-label="打开对话"
          title="打开对话"
        >
          <MessageCircle size={19} aria-hidden="true" />
        </button>
      );
      break;
    case "training":
      header = (
        <TrainingWindowHeader
          view={selectTrainingPlan(trainingPlans, planId)}
          onSelectPlan={setPlanId}
        />
      );
      actions = (
        <button
          className="bc-icon-button"
          type="button"
          aria-label="打开对话"
          title="打开对话"
        >
          <MessageCircle size={19} aria-hidden="true" />
        </button>
      );
      break;
  }

  // The body is deliberately a placeholder: this group is for inspecting the
  // production shell + real header widgets, not an imitation of app content.
  return visible ? (
    <BusinessWindowHost
      windowId={windowId}
      mobile={mobile}
      stackIndex={0}
      isActive
      onClose={() => setVisible(false)}
      headerLeft={header}
      headerActions={actions}
      hideTitle
    >
      <section className="bc-business-window-body" aria-label="窗口布局预览">
        <div className="bc-window-body">
          <p style={{ fontWeight: 650, marginBottom: 12 }}>共享窗口外壳预览</p>
          <p>
            此处仅展示真实窗口外壳与 Header。业务内容请查看 Diagnosis / Training
            Stories。
          </p>
          <p style={{ marginTop: 16, opacity: 0.65 }}>
            可试用 Header 选择器与右上角关闭按钮。
          </p>
        </div>
      </section>
    </BusinessWindowHost>
  ) : (
    <button
      type="button"
      onClick={() => setVisible(true)}
      style={{
        position: "absolute",
        top: 110,
        left: 40,
        padding: 12,
        background: "white",
        borderRadius: 8,
      }}
    >
      重新打开窗口
    </button>
  );
}

const meta = {
  title: "BodySense/Windows/01 Shell",
  component: ShellPreview,
  decorators: [windowCanvasDecorator],
  parameters: {
    layout: "fullscreen",
    docs: {
      description: {
        component:
          "复用正式 BusinessWindowHost 和各业务 Header。内容为占位区，不会调用后端 API。",
      },
    },
  },
} satisfies Meta<typeof ShellPreview>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Person: Story = { args: { windowId: "person" } };
export const Chat: Story = { args: { windowId: "chat" } };
export const Diagnosis: Story = { args: { windowId: "diagnosis" } };
export const Training: Story = { args: { windowId: "training" } };
