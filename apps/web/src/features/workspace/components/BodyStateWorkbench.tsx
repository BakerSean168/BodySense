import { BodyFactRecords } from "@/features/body-canvas/components/BodyFactRecords";
import { BodyEpisodeAssociations } from "@/features/body-canvas/components/BodyEpisodeAssociations";
import { useBodyRecordsQuery } from "@/features/profile/hooks/useBodyRecordsQuery";
import { errorMessage } from "@/lib/api-client";

/** User statements, measurements and Episode identities keep their native semantics. */
export function BodyStateWorkbench({ canEdit }: { canEdit: boolean }) {
  const records = useBodyRecordsQuery();
  return (
    <div className="grid gap-4">
      <BodyFactRecords canEdit={canEdit} />
      <BodyEpisodeAssociations canEdit={canEdit} />
      {records.isPending ? (
        <p role="status">正在读取观察记录…</p>
      ) : records.isError ? (
        <div role="alert">
          <p>{errorMessage(records.error, "暂时无法读取观察记录")}</p>
          <button type="button" onClick={() => void records.refetch()}>
            重试
          </button>
        </div>
      ) : (
        <section aria-label="观察记录">
          {records.data.observations.length ? (
            records.data.observations.map((observation) => (
              <article key={observation.id}>
                <p>{observation.statement}</p>
                <p className="text-xs text-muted-foreground">
                  {[
                    observation.raw_value,
                    observation.raw_unit,
                    observation.method,
                  ]
                    .filter(Boolean)
                    .join(" · ")}
                </p>
              </article>
            ))
          ) : (
            <p className="text-xs text-muted-foreground">暂无观察记录</p>
          )}
        </section>
      )}
    </div>
  );
}
