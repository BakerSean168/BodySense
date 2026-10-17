import { useAuthStore } from "@/stores/authStore";
import { useBodyHypothesisController } from "../hooks/useBodyHypothesisController";

/** Owner changes discard drafts and prevent late receipts from entering another profile. */
export function BodyHypotheses({ canEdit }: { canEdit: boolean }) {
  const owner = useAuthStore((state) => state.user?.id);
  return owner ? (
    <HypothesisPanel key={owner} owner={owner} canEdit={canEdit} />
  ) : null;
}

function HypothesisPanel({
  owner,
  canEdit,
}: {
  owner: string;
  canEdit: boolean;
}) {
  const {
    records,
    groups,
    data,
    consistent,
    ready,
    draft,
    statement,
    setStatement,
    facts,
    setFacts,
    observations,
    setObservations,
    saving,
    error,
    notice,
    refresh,
    open,
    submit,
    cancel,
  } = useBodyHypothesisController(owner, canEdit);
  const toggle = (ids: string[], id: string) =>
    ids.includes(id) ? ids.filter((value) => value !== id) : [...ids, id];
  return (
    <section aria-label="我的猜测与相关记录" className="grid gap-3">
      <h3 className="text-sm font-medium">我的猜测</h3>
      <p className="bc-window-note">
        仅记录你本人提出的可能原因或疾病，不代表诊断。Agent
        的推测不会自动成为你的记录。
      </p>
      {notice && <p role="status">{notice}</p>}
      {error && (
        <p role="alert" className="bc-error">
          {error}
        </p>
      )}
      {records.isPending || groups.isPending ? (
        <p role="status">正在读取猜测与分组…</p>
      ) : null}
      {records.isError || groups.isError ? (
        <p role="alert" className="bc-error">
          猜测或相关记录读取失败，请重试。
        </p>
      ) : null}
      {data && groups.data && !consistent && (
        <p role="status">记录已更新，正在等待一致的最新记录。请重新读取。</p>
      )}
      <button
        type="button"
        className="bc-text-button"
        disabled={saving || records.isFetching || groups.isFetching}
        onClick={refresh}
      >
        重新读取猜测与分组
      </button>
      {ready && data ? (
        <>
          {!data.hypotheses.length && <p>还没有你提出的猜测。</p>}
          {data.hypotheses.map((record) => (
            <article key={record.id} className="bc-record-item">
              <p>{record.statement}</p>
              <p className="bc-window-note">
                用户本人猜测 ·{" "}
                {record.supersedes_hypothesis_id ? "更正后的版本" : "本人记录"}{" "}
                · {new Date(record.recorded_at).toLocaleString()}
              </p>
              <p className="bc-window-note">
                来源：本人直接编辑；原始陈述已留存。
              </p>
              {canEdit && !draft && (
                <div className="bc-record-actions">
                  <button
                    className="bc-text-button"
                    type="button"
                    onClick={() => open({ mode: "correct", target: record })}
                  >
                    更正猜测
                  </button>
                  <button
                    className="bc-text-button"
                    type="button"
                    onClick={() => open({ mode: "withdraw", target: record })}
                  >
                    撤回猜测
                  </button>
                </div>
              )}
            </article>
          ))}
        </>
      ) : null}
      {canEdit && !draft && (
        <button
          className="bc-button"
          type="button"
          disabled={!ready}
          onClick={() => open({ mode: "record" })}
        >
          添加我的猜测
        </button>
      )}
      {draft && (
        <form
          className="bc-equipment-form"
          aria-label="编辑我的猜测"
          onSubmit={(event) => {
            event.preventDefault();
            void submit();
          }}
        >
          {draft.mode === "withdraw" ? (
            <p>
              确认撤回「{statement}
              」？不会删除关联事实或观察，也不表示症状结束。
            </p>
          ) : (
            <>
              <label htmlFor="hypothesis-statement">我的完整猜测</label>
              <textarea
                id="hypothesis-statement"
                value={statement}
                maxLength={2000}
                rows={3}
                disabled={saving}
                required
                onChange={(event) => setStatement(event.target.value)}
              />
              <p className="bc-window-note">
                选择你认为相关的记录（每类最多 20
                条）。更正会以这里明确选择的关联为准。
              </p>
              <fieldset disabled={saving || !ready}>
                <legend>相关事实</legend>
                {data?.facts.map((fact) => (
                  <label key={fact.id}>
                    <input
                      type="checkbox"
                      checked={facts.includes(fact.id)}
                      disabled={!facts.includes(fact.id) && facts.length >= 20}
                      onChange={() => setFacts(toggle(facts, fact.id))}
                    />
                    {fact.statement}
                  </label>
                ))}
              </fieldset>
              <fieldset disabled={saving || !ready}>
                <legend>相关观察</legend>
                {data?.observations.map((observation) => (
                  <label key={observation.id}>
                    <input
                      type="checkbox"
                      checked={observations.includes(observation.id)}
                      disabled={
                        !observations.includes(observation.id) &&
                        observations.length >= 20
                      }
                      onChange={() =>
                        setObservations(toggle(observations, observation.id))
                      }
                    />
                    {observation.statement}
                  </label>
                ))}
              </fieldset>
            </>
          )}
          <div className="bc-record-actions">
            <button
              className="bc-button"
              type="submit"
              disabled={saving || !ready || !canEdit || !statement.trim()}
            >
              {saving
                ? "正在提交"
                : draft.mode === "record"
                  ? "保存猜测"
                  : draft.mode === "correct"
                    ? "保存猜测更正"
                    : "确认撤回猜测"}
            </button>
            <button
              className="bc-button"
              type="button"
              disabled={saving}
              onClick={cancel}
            >
              取消
            </button>
          </div>
        </form>
      )}
      <h3 className="text-sm font-medium">相关记录分组</h3>
      <p className="bc-window-note">
        只读派生视图，组织事实、观察、经历和用户猜测，不形成新的事实或诊断。
      </p>
      {ready && data && groups.data ? (
        groups.data.groups.length ? (
          groups.data.groups.map((group) => (
            <article key={group.id} className="bc-record-item">
              <h4>{group.display_title}</h4>
              {group.related_hypothesis_ids.map((id) => (
                <p key={id}>
                  用户猜测：
                  {data.hypotheses.find((row) => row.id === id)?.statement ??
                    "当前记录不可用"}
                </p>
              ))}
              {group.related_fact_ids.map((id) => (
                <p key={id}>
                  来源事实：
                  {data.facts.find((row) => row.id === id)?.statement ??
                    "当前记录不可用"}
                </p>
              ))}
              {group.related_observation_ids.map((id) => {
                const row = data.observations.find((row) => row.id === id);
                return (
                  <p key={id}>
                    来源观察：
                    {row
                      ? [row.statement, row.raw_value, row.raw_unit]
                          .filter(Boolean)
                          .join(" · ")
                      : "当前记录不可用"}
                  </p>
                );
              })}
              {group.related_episode_ids.map((id) => (
                <p key={id}>
                  相关经历：
                  {data.episodes.find((row) => row.id === id)?.symptom_type ??
                    "当前记录不可用"}
                </p>
              ))}
            </article>
          ))
        ) : (
          <p>暂无相关记录分组。</p>
        )
      ) : null}
    </section>
  );
}
