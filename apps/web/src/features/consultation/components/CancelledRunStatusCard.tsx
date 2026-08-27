export function CancelledRunStatusCard() {
  return (
    <div className="flex justify-start" role="status">
      <div className="w-full max-w-[620px] rounded-xl border border-white/[0.07] bg-white/[0.03] px-4 py-3 text-sm text-white/60">
        <p className="font-semibold text-white/78">本次处理已停止</p>
        <p className="mt-1 leading-5">
          你可以修改或补充信息后继续和 BodySense 对话。
        </p>
      </div>
    </div>
  );
}
