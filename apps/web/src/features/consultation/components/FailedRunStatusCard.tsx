export function FailedRunStatusCard({ message }: { message?: string }) {
  return (
    <div className="flex justify-start" role="status">
      <div className="max-w-[80%] rounded-xl border border-amber-200 bg-amber-50 px-4 py-3 text-sm text-amber-900">
        <p className="font-semibold">本次执行已安全停止</p>
        <p className="mt-1 leading-5">
          {message || "本次执行未完成。你可以继续输入，发起一次新的执行。"}
        </p>
      </div>
    </div>
  );
}
