export function FailedRunStatusCard({ message }: { message?: string }) {
  return (
    <div className="flex justify-start" role="status">
      <div className="w-full max-w-[620px] rounded-xl border border-amber-300/12 bg-amber-300/[0.045] px-4 py-3 text-sm text-amber-100/75">
        <p className="font-semibold text-amber-100/90">本次处理已安全停止</p>
        <p className="mt-1 leading-5">
          {message || "这条消息没有处理完成。你可以补充信息后继续。"}
        </p>
      </div>
    </div>
  );
}
