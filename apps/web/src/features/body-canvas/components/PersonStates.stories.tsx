import { useEffect, useState } from "react";
import { useMediaQuery } from "@/hooks/useMediaQuery";
import type { Meta, StoryObj } from "@storybook/react-vite";
import type { UserUpload } from "@/features/profile/types/upload.types";
import { useUploadStore } from "@/stores/uploadStore";
import { BusinessWindowHost } from "./BusinessWindowHost";
import { PersonWindow } from "../windows/person/PersonWindow";
import { PersonWindowHeader } from "../windows/person/PersonWindowHeader";
import type { ProfileCategoryId } from "../model/profileCategories";
import { windowCanvasDecorator } from "./windowStorySupport";

type PersonScenario = "loading" | "empty" | "media" | "reports" | "error";

const exampleUploads: UserUpload[] = [
  {
    id: "story-media-1",
    file_type: "consultation_photo",
    original_name: "体态对比-正面.jpg",
    file_size: 1_234_567,
    mime_type: "image/jpeg",
    source_availability: "available",
    ocr_result: null,
    ocr_status: "completed",
    analysis_status: "completed",
    analysis_result: null,
    created_at: "2026-10-20T09:00:00+08:00",
    updated_at: "2026-10-20T09:00:00+08:00",
  },
  {
    id: "story-media-2",
    file_type: "consultation_photo",
    original_name: "训练前后-侧面.png",
    file_size: 432_100,
    mime_type: "image/png",
    source_availability: "available",
    ocr_result: null,
    ocr_status: "processing",
    analysis_status: "processing",
    analysis_result: null,
    created_at: "2026-10-19T11:00:00+08:00",
    updated_at: "2026-10-19T11:00:00+08:00",
  },
  {
    id: "story-report-1",
    file_type: "report",
    original_name: "影像检查报告.pdf",
    file_size: 843_229,
    mime_type: "application/pdf",
    source_availability: "available",
    ocr_result: null,
    ocr_status: "completed",
    analysis_status: "completed",
    analysis_result: null,
    created_at: "2026-10-18T11:00:00+08:00",
    updated_at: "2026-10-18T11:00:00+08:00",
  },
];

function PersonScenarioContent({ scenario }: { scenario: PersonScenario }) {
  const mobile = useMediaQuery("(max-width: 760px)");
  // Replace only the Store's data transport for the lifetime of this Story.
  // Never allow uploads or deletes to reach a signed-in backend.
  const [previous] = useState(() => {
    const original = useUploadStore.getState();
    useUploadStore.setState({
      error: null,
      isLoading: false,
      uploads:
        scenario === "media" || scenario === "reports" ? exampleUploads : [],
      hasFetched:
        scenario === "media" || scenario === "reports" || scenario === "empty",
      fetchUploads: async () => {
        if (scenario === "loading") {
          await new Promise<void>(() => {});
        }
        if (scenario === "error") throw new Error("预览：文件读取失败");
      },
      uploadFile: async () => {
        throw new Error("Storybook 预览禁止上传");
      },
      deleteUpload: async () => {
        throw new Error("Storybook 预览禁止删除");
      },
    });
    return original;
  });
  useEffect(
    () => () => {
      useUploadStore.setState({
        error: previous.error,
        isLoading: previous.isLoading,
        uploads: previous.uploads,
        hasFetched: previous.hasFetched,
        fetchUploads: previous.fetchUploads,
        uploadFile: previous.uploadFile,
        deleteUpload: previous.deleteUpload,
      });
    },
    [previous],
  );

  const [closed, setClosed] = useState(false);
  const [category, setCategory] = useState<ProfileCategoryId>(
    scenario === "reports" ? "reports" : "media",
  );
  const [infoOpen, setInfoOpen] = useState(false);

  if (closed) {
    return (
      <button
        type="button"
        onClick={() => setClosed(false)}
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
  return (
    <BusinessWindowHost
      windowId="person"
      mobile={mobile}
      stackIndex={0}
      isActive
      hideTitle
      onClose={() => setClosed(true)}
      headerLeft={
        <PersonWindowHeader
          identity={{ nickname: "预览用户" }}
          open={infoOpen}
          onToggle={() => setInfoOpen((value) => !value)}
          category={category}
          onCategoryChange={setCategory}
        />
      }
    >
      <PersonWindow
        canEdit={false}
        category={category}
        onCategoryChange={setCategory}
        identityInfoOpen={infoOpen}
        onIdentityInfoOpenChange={setInfoOpen}
      />
    </BusinessWindowHost>
  );
}

function PersonPreview({ scenario }: { scenario: PersonScenario }) {
  // Remount the mocked Store adapter when Controls switches scenarios.
  return <PersonScenarioContent key={scenario} scenario={scenario} />;
}

const meta = {
  title: "BodySense/Windows/03 Person",
  component: PersonPreview,
  decorators: [windowCanvasDecorator],
  argTypes: {
    scenario: {
      control: "select",
      options: ["loading", "empty", "media", "reports", "error"],
    },
  },
  parameters: {
    layout: "fullscreen",
    docs: {
      description: {
        component:
          "真实 PersonWindow 和 Header，使用隔离的上传 Store 适配器；分类从媒体/报告开始，预览不请求或修改后端。",
      },
    },
  },
} satisfies Meta<typeof PersonPreview>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Loading: Story = { args: { scenario: "loading" } };
export const Empty: Story = { args: { scenario: "empty" } };
export const Media: Story = { args: { scenario: "media" } };
export const Reports: Story = { args: { scenario: "reports" } };
export const ErrorState: Story = { name: "Error", args: { scenario: "error" } };
