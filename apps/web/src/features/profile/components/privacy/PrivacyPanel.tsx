import { useMemo, useState } from "react";
import { ShieldAlert, Trash2 } from "lucide-react";
import { Button } from "@/components/ui/Button";
import { Card } from "@/components/ui/Card";
import { Input } from "@/components/ui/Input";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import {
  privacyApi,
  type PrivacyErasurePlan,
} from "../../services/privacyService";

interface PrivacyPanelProps {
  onErasureAccepted: () => void;
}

export function PrivacyPanel({ onErasureAccepted }: PrivacyPanelProps) {
  const [open, setOpen] = useState(false);
  const [plan, setPlan] = useState<PrivacyErasurePlan | null>(null);
  const [confirmation, setConfirmation] = useState("");
  const [loadingPlan, setLoadingPlan] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const directRecordCount = useMemo(
    () => plan?.counts.reduce((sum, item) => sum + item.count, 0) ?? 0,
    [plan],
  );

  const openErasureDialog = async () => {
    setLoadingPlan(true);
    setError(null);
    try {
      const nextPlan = await privacyApi.getErasurePlan();
      setPlan(nextPlan);
      setConfirmation("");
      setOpen(true);
    } catch (err) {
      setError(err instanceof Error ? err.message : "无法准备删除计划");
    } finally {
      setLoadingPlan(false);
    }
  };

  const requestErasure = async () => {
    if (!plan || confirmation !== plan.confirmation_phrase) return;
    setSubmitting(true);
    setError(null);
    try {
      await privacyApi.requestErasure(confirmation);
      setOpen(false);
      onErasureAccepted();
    } catch (err) {
      setError(err instanceof Error ? err.message : "提交删除请求失败");
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <div className="space-y-6">
      <Card className="p-6">
        <div className="flex items-start gap-3">
          <ShieldAlert className="mt-0.5 size-5 text-amber-600" />
          <div>
            <h2 className="font-semibold text-slate-900">数据边界说明</h2>
            <p className="mt-2 text-sm leading-6 text-slate-600">
              删除会话只会清除聊天历史和对应分享，不会删除 BodyState、诊断分析、治疗方案、训练记录、结果或上传文件。
              如需清除全部 BodySense 数据与账号，请使用下面的完整数据删除流程。
            </p>
          </div>
        </div>
      </Card>

      <Card className="border-red-200 p-6">
        <div className="flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between">
          <div className="max-w-2xl">
            <h2 className="font-semibold text-red-700">删除全部数据与账号</h2>
            <p className="mt-2 text-sm leading-6 text-slate-600">
              这是不可逆的隐私删除操作。它会撤销登录会话，删除身体档案、纵向健康状态、诊断、治疗、训练、结果、全部会话与分享，以及上传文件和派生分析。
              请求一旦被接受，即使浏览器断开，服务端也会继续执行或自动重试。
            </p>
          </div>
          <Button
            variant="destructive"
            onClick={() => void openErasureDialog()}
            isLoading={loadingPlan}
          >
            <Trash2 />
            删除全部数据
          </Button>
        </div>
        {error && !open && (
          <p className="mt-4 text-sm font-medium text-red-600">{error}</p>
        )}
      </Card>

      <Dialog open={open} onOpenChange={setOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>确认完整删除 BodySense 数据</DialogTitle>
            <DialogDescription>
              这不是“清空聊天”。该操作会删除你的账号及全部可关联健康数据，并立即撤销登录凭据。
            </DialogDescription>
          </DialogHeader>

          {plan && (
            <div className="space-y-4 text-sm">
              <div className="rounded-lg border border-slate-200 bg-slate-50 p-3 text-slate-700">
                当前 dry-run 识别到 {directRecordCount} 条直接归属记录；其级联子记录和用户上传对象也会一并删除。
              </div>
              <ul className="list-disc space-y-1 pl-5 text-slate-600">
                <li>账号、身体档案和 BodyState</li>
                <li>诊断、治疗、训练和结果记录</li>
                <li>全部会话、运行记录与公开分享</li>
                <li>上传文件、OCR/姿态分析等派生结果</li>
              </ul>
              <p className="text-xs leading-5 text-slate-500">
                完成后仅保留无法直接关联回账号的删除请求状态/时间审计，以及不属于你的全局知识库发布记录。
              </p>
              <div>
                <label
                  htmlFor="privacy-erasure-confirmation"
                  className="mb-2 block font-medium text-slate-800"
                >
                  输入 <code>{plan.confirmation_phrase}</code> 以确认
                </label>
                <Input
                  id="privacy-erasure-confirmation"
                  value={confirmation}
                  onChange={(event) => setConfirmation(event.target.value)}
                  autoComplete="off"
                  spellCheck={false}
                />
              </div>
              {error && <p className="font-medium text-red-600">{error}</p>}
            </div>
          )}

          <DialogFooter>
            <Button
              variant="outline"
              onClick={() => setOpen(false)}
              disabled={submitting}
            >
              取消
            </Button>
            <Button
              variant="destructive"
              onClick={() => void requestErasure()}
              disabled={!plan || confirmation !== plan.confirmation_phrase}
              isLoading={submitting}
            >
              确认并永久删除
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}
