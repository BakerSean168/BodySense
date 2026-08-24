export function CancelledRunStatusCard() {
  return (
    <div className="flex justify-start" role="status">
      <div className="max-w-[80%] rounded-xl border border-slate-200 bg-slate-50 px-4 py-3 text-sm text-slate-700">
        <p className="font-semibold">本次执行已取消</p>
        <p className="mt-1 leading-5">
          已停止当前运行；你可以修改或补充信息后继续咨询。
        </p>
      </div>
    </div>
  );
}
