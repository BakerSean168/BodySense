import { useState } from "react";
import { List, LoaderCircle, X } from "lucide-react";
import { useNavigate } from "react-router";
import { useAuthStore } from "@/stores/authStore";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogTitle,
} from "@/components/ui/dialog";
import { SHOW_POSE_BAR } from "../model/bodyCanvas";
import type { ThemePreference } from "./useCanvasAppearance";

const THEME_OPTIONS: ReadonlyArray<[ThemePreference, string]> = [
  ["light", "浅色"],
  ["dark", "深色"],
  ["system", "跟随系统"],
];

const CANVAS_OVERLAY_META: Record<
  "help",
  { title: string; description: string }
> = {
  help: {
    title: "让操作顺手一点",
    description: "每个快捷操作都有可见按钮，也支持按名称选择区域。",
  },
};

export function CanvasWorkspaceDialogs({
  canvasOverlay,
  onCanvasOverlayChange,
  onOpenRegionPicker,
  accountDialog,
  onAccountDialogChange,
  themePreference,
  onThemePreferenceChange,
  themeClass,
}: {
  canvasOverlay: "help" | null;
  onCanvasOverlayChange: (value: "help" | null) => void;
  onOpenRegionPicker: () => void;
  accountDialog: "settings" | "logout" | null;
  onAccountDialogChange: (value: "settings" | "logout" | null) => void;
  themePreference: ThemePreference;
  onThemePreferenceChange: (value: ThemePreference) => void;
  themeClass: string;
}) {
  const [loggingOut, setLoggingOut] = useState(false);
  const logout = useAuthStore((state) => state.logout);
  const navigate = useNavigate();
  const setCanvasOverlay = onCanvasOverlayChange;
  const setRegionListOpen = (open: boolean) => {
    if (open) onOpenRegionPicker();
  };
  const setAccountDialog = onAccountDialogChange;
  const setThemePreference = onThemePreferenceChange;
  return (
    <>
      <Dialog
        open={canvasOverlay !== null}
        onOpenChange={(open) => {
          if (!open) setCanvasOverlay(null);
        }}
      >
        <DialogContent
          showCloseButton={false}
          className={`${themeClass} bc-inventory-dialog`}
        >
          <div className="bc-inventory-heading">
            <div>
              <span className="bc-eyebrow">
                {canvasOverlay
                  ? `CANVAS TOOL / ${canvasOverlay.toUpperCase()}`
                  : "CANVAS TOOL"}
              </span>
              <DialogTitle className="bc-inventory-title">
                {canvasOverlay
                  ? CANVAS_OVERLAY_META[canvasOverlay].title
                  : "画布工具"}
              </DialogTitle>
              <DialogDescription>
                {canvasOverlay
                  ? CANVAS_OVERLAY_META[canvasOverlay].description
                  : ""}
              </DialogDescription>
            </div>
            <button
              className="bc-icon-button"
              aria-label="关闭当前面板"
              onClick={() => setCanvasOverlay(null)}
            >
              <X size={21} />
            </button>
          </div>
          <div className="bc-inventory-content">
            {canvasOverlay === "help" && (
              <div className="bc-keyboard-help">
                {[
                  ["V / B / S", "选择、标注涂抹、框选"],
                  ...(SHOW_POSE_BAR
                    ? [
                        ["1 / 2 / 3 / 4", "站立、跑步、跳跃、坐姿"],
                        ["F / Space", "暂停或继续动作"],
                      ]
                    : []),
                  ["/", "打开对话窗口"],
                  ["M / P / D / T", "个人档案、训练计划、诊断、训练执行"],
                  ["L / R", "精细解剖图层、复位视角"],
                  ["Z / Esc", "撤销选区、逐层返回"],
                ].map(([key, label]) => (
                  <div key={key}>
                    <kbd>{key}</kbd>
                    <span>{label}</span>
                  </div>
                ))}
                <p>
                  输入文字、使用输入法或打开模态面板时，不触发背景单键命令。涂抹标记跟随模型表面；清除选区不会删除身体记录。窗口关闭或通过
                  Dock 切换不会丢失选区、对话或未完成操作。
                </p>
                <button
                  className="bc-button"
                  onClick={() => {
                    setCanvasOverlay(null);
                    setRegionListOpen(true);
                  }}
                >
                  按名称选择身体区域
                  <List size={17} />
                </button>
              </div>
            )}
          </div>
        </DialogContent>
      </Dialog>
      <Dialog
        open={accountDialog !== null}
        onOpenChange={(open) => {
          if (!open && !loggingOut) setAccountDialog(null);
        }}
      >
        <DialogContent
          showCloseButton={false}
          className={`${themeClass} bc-account-dialog`}
          data-testid={
            accountDialog === "logout" ? "logout-confirm" : "settings-dialog"
          }
        >
          {accountDialog === "logout" ? (
            <>
              <DialogTitle className="bc-account-dialog-title">
                退出登录
              </DialogTitle>
              <DialogDescription className="bc-account-dialog-text">
                确定要退出当前账号吗？
                <small>退出后将回到登录页。</small>
              </DialogDescription>
              <div className="bc-account-dialog-actions">
                <button
                  type="button"
                  className="bc-button"
                  disabled={loggingOut}
                  onClick={() => setAccountDialog(null)}
                >
                  取消
                </button>
                <button
                  type="button"
                  className="bc-button bc-danger-button"
                  disabled={loggingOut}
                  onClick={async () => {
                    setLoggingOut(true);
                    try {
                      await logout();
                    } finally {
                      setLoggingOut(false);
                      setAccountDialog(null);
                      navigate("/login", { replace: true });
                    }
                  }}
                >
                  {loggingOut ? (
                    <LoaderCircle size={15} className="animate-spin" />
                  ) : null}
                  退出登录
                </button>
              </div>
            </>
          ) : (
            <>
              <DialogTitle className="bc-account-dialog-title">
                设置
              </DialogTitle>
              <DialogDescription className="bc-account-dialog-text">
                外观与应用偏好。
              </DialogDescription>
              <div className="bc-theme-field">
                <span id="bc-theme-label" className="bc-theme-label">
                  主题
                </span>
                <span className="bc-theme-hint">浅色 / 深色 / 跟随系统</span>
                <div
                  className="bc-theme-options"
                  role="radiogroup"
                  aria-labelledby="bc-theme-label"
                >
                  {THEME_OPTIONS.map(([value, label]) => (
                    <button
                      key={value}
                      type="button"
                      role="radio"
                      aria-checked={themePreference === value}
                      tabIndex={themePreference === value ? 0 : -1}
                      onClick={() => setThemePreference(value)}
                      onKeyDown={(event) => {
                        const step =
                          event.key === "ArrowRight" ||
                          event.key === "ArrowDown"
                            ? 1
                            : event.key === "ArrowLeft" ||
                                event.key === "ArrowUp"
                              ? -1
                              : 0;
                        if (!step) return;
                        event.preventDefault();
                        const index = THEME_OPTIONS.findIndex(
                          ([option]) => option === themePreference,
                        );
                        const next =
                          THEME_OPTIONS[
                            (index + step + THEME_OPTIONS.length) %
                              THEME_OPTIONS.length
                          ][0];
                        setThemePreference(next);
                        const group = event.currentTarget.parentElement;
                        requestAnimationFrame(() =>
                          group
                            ?.querySelector<HTMLButtonElement>(
                              "[aria-checked='true']",
                            )
                            ?.focus(),
                        );
                      }}
                    >
                      {label}
                    </button>
                  ))}
                </div>
              </div>
              <section className="bc-about" aria-labelledby="bc-about-label">
                <span id="bc-about-label" className="bc-theme-label">
                  关于
                </span>
                <p className="bc-about-text">
                  人体模型来自{" "}
                  <a
                    href="https://github.com/vixotic/Vanatome"
                    target="_blank"
                    rel="noreferrer"
                  >
                    Vanatome
                  </a>{" "}
                  Human Atlas 1.4.0，源自{" "}
                  <a
                    href="https://github.com/Z-Anatomy/Models"
                    target="_blank"
                    rel="noreferrer"
                  >
                    Z-Anatomy
                  </a>
                  ，按{" "}
                  <a
                    href="https://creativecommons.org/licenses/by-sa/4.0/"
                    target="_blank"
                    rel="noreferrer"
                  >
                    CC BY-SA 4.0
                  </a>{" "}
                  许可使用。肌肉层取自{" "}
                  <a
                    href="https://github.com/Z-Anatomy/Models-of-human-anatomy"
                    target="_blank"
                    rel="noreferrer"
                  >
                    Z-Anatomy
                  </a>
                  （Gauthier Kervyn、Marcin Zielinski 等，基于 BodyParts3D），由
                  BodySense 精简导出并调整材质，同样按 CC BY-SA 4.0 提供。
                </p>
              </section>
              <button
                type="button"
                className="bc-button bc-account-dialog-close"
                onClick={() => setAccountDialog(null)}
              >
                关闭
              </button>
            </>
          )}
        </DialogContent>
      </Dialog>
    </>
  );
}
