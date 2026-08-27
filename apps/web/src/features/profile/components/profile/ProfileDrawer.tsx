import { useEffect, useState } from "react";
import { useNavigate } from "react-router";
import { X } from "lucide-react";
import type { UserProfile } from "@/stores/profileStore";
import { useProfileStore } from "@/stores/profileStore";
import { useUploadStore } from "@/stores/uploadStore";
import { useAuthStore } from "@/stores/authStore";
import { Button } from "@/components/ui/Button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogTitle,
} from "@/components/ui/dialog";
import { ProfileView } from "./ProfileView";
import { ProfileEdit } from "./ProfileEdit";
import { BodyMetricsPanel } from "./BodyMetricsPanel";
import { LifestylePanel } from "./LifestylePanel";
import { HealthHistoryPanel } from "./HealthHistoryPanel";
import { FileUploader, UploadList } from "../uploads";
import { PrivacyPanel } from "../privacy/PrivacyPanel";

interface ProfileDrawerProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
}

type ProfileTab = "profile" | "lifestyle" | "history" | "uploads" | "privacy";

export function ProfileDrawer({ open, onOpenChange }: ProfileDrawerProps) {
  const navigate = useNavigate();
  const { profile, isLoading, error, fetchProfile, updateProfile, clearError } =
    useProfileStore();
  const { uploads, fetchUploads } = useUploadStore();
  const forgetSession = useAuthStore((state) => state.forgetSession);
  const [activeTab, setActiveTab] = useState<ProfileTab>("profile");
  const [isEditing, setIsEditing] = useState(false);

  useEffect(() => {
    if (!open) return;
    void fetchProfile();
    void fetchUploads();
  }, [open, fetchProfile, fetchUploads]);

  const handleSave = async (data: Partial<UserProfile>) => {
    await updateProfile(data);
    setIsEditing(false);
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent
        showCloseButton={false}
        className="top-0 right-0 left-auto flex h-dvh w-[min(46rem,96vw)] max-w-none translate-x-0 translate-y-0 flex-col gap-0 rounded-none border-l border-border bg-background p-0 sm:max-w-none data-open:slide-in-from-right data-closed:slide-out-to-right"
      >
        <div className="flex h-14 shrink-0 items-center justify-between border-b border-border px-4">
          <div className="min-w-0">
            <DialogTitle>身体档案</DialogTitle>
            <DialogDescription className="mt-0.5">
              稳定身份、身体测量、生活方式、健康历史、上传资料与隐私都在这里管理。
            </DialogDescription>
          </div>
          <Button
            type="button"
            size="icon-sm"
            variant="ghost"
            aria-label="关闭身体档案"
            onClick={() => onOpenChange(false)}
          >
            <X />
          </Button>
        </div>

        <div className="flex shrink-0 gap-1 border-b border-border px-4 py-2">
          {([
            ["profile", "基本档案"],
            ["lifestyle", "生活方式"],
            ["history", "健康历史"],
            ["uploads", `文件管理${uploads.length ? ` · ${uploads.length}` : ""}`],
            ["privacy", "数据与隐私"],
          ] as const).map(([tab, label]) => (
            <button
              key={tab}
              type="button"
              onClick={() => setActiveTab(tab)}
              className={`rounded-lg px-3 py-2 text-xs font-medium transition-colors ${
                activeTab === tab
                  ? "bg-muted text-foreground"
                  : "text-muted-foreground hover:bg-muted/60 hover:text-foreground"
              }`}
            >
              {label}
            </button>
          ))}
        </div>

        <div className="min-h-0 flex-1 overflow-y-auto p-4 sm:p-6">
          {error ? (
            <div className="mb-4 flex items-center justify-between rounded-xl border border-destructive/20 bg-destructive/10 p-3 text-sm text-destructive">
              <span>{error}</span>
              <button type="button" className="font-medium" onClick={clearError}>
                关闭
              </button>
            </div>
          ) : null}

          {activeTab === "profile" ? (
            isLoading && !profile ? (
              <div className="flex min-h-64 items-center justify-center text-sm text-muted-foreground">
                正在读取身体档案…
              </div>
            ) : profile && isEditing ? (
              <ProfileEdit
                profile={profile}
                onSave={handleSave}
                onCancel={() => setIsEditing(false)}
                isLoading={isLoading}
              />
            ) : profile ? (
              <div className="space-y-6">
                <ProfileView profile={profile} onEdit={() => setIsEditing(true)} />
                <BodyMetricsPanel />
              </div>
            ) : (
              <div className="rounded-2xl border border-dashed border-border p-8 text-center">
                <p className="text-sm font-semibold">身体档案尚未建立</p>
                <Button
                  className="mt-4"
                  onClick={() => {
                    onOpenChange(false);
                    navigate("/onboarding");
                  }}
                >
                  开始填写
                </Button>
              </div>
            )
          ) : null}

          {activeTab === "lifestyle" ? <LifestylePanel /> : null}

          {activeTab === "history" ? <HealthHistoryPanel /> : null}

          {activeTab === "uploads" ? (
            <div className="space-y-5">
              <div className="rounded-2xl border border-border p-4">
                <h3 className="mb-3 text-sm font-semibold">上传资料</h3>
                <FileUploader onUploadComplete={() => fetchUploads()} />
              </div>
              <div className="rounded-2xl border border-border p-4">
                <h3 className="mb-3 text-sm font-semibold">已上传文件</h3>
                <UploadList />
              </div>
            </div>
          ) : null}

          {activeTab === "privacy" ? (
            <PrivacyPanel
              onErasureAccepted={() => {
                forgetSession();
                onOpenChange(false);
                navigate("/login", { replace: true });
              }}
            />
          ) : null}
        </div>
      </DialogContent>
    </Dialog>
  );
}
