import { TargetReport } from "./target-report";
import { injuryPercent, injuryRecoveryRangeText, type InjuryPublicGroup, type InjuryTally } from "@/lib/injury";

const panel = "space-y-3 rounded-2xl border border-white/20 glass bg-white/10 p-4";

function Waiting({ min }: { min: number }) {
  return <p className="text-sm text-slate-400">{min}人そろうと表示</p>;
}

// まだ集計が出ていない項目は見出しごと出さない。全部まだのときは、呼ぶ側で1行だけ出す。
function Bars({ title, tally }: { title: string; tally: InjuryTally }) {
  if (tally.items.length === 0) return null;
  return (
    <div className="space-y-2">
      <h3 className="text-sm font-bold text-slate-100">{title}</h3>
      <ul className="space-y-1.5">
        {tally.items.map((item) => {
          const percent = injuryPercent(item.people, tally.people);
          return (
            <li key={item.code} className="text-sm">
              <div className="flex justify-between gap-3">
                <span>{item.label}</span>
                <span className="shrink-0 text-slate-300">{percent}%</span>
              </div>
              <div className="mt-1 h-1.5 rounded-full bg-white/10" aria-hidden>
                <div className="h-1.5 rounded-full bg-blue-400" style={{ width: `${percent}%` }} />
              </div>
            </li>
          );
        })}
      </ul>
    </div>
  );
}

const hasItems = (...tallies: InjuryTally[]) => tallies.some((tally) => tally.items.length > 0);

// グループの公開ページの中身。ログインしていなくても、検索から来た人にも見える。
// 集計は少人数だと誰のことかわかるので、min_people 人そろうまでは出さない(API 側でも出さない)。
export function InjuryPublicSummary({ group }: { group: InjuryPublicGroup }) {
  const min = group.min_people;
  // 目安がひとつも出ていない重さは、行ごと省く。
  const recovery = group.recovery.filter((r) => r.stages.some((s) => s.low_days !== null));
  const prevention = [
    { title: "ケガの前に変わったこと", tally: group.pre_changes },
    { title: "そのときしていたこと", tally: group.activities },
    { title: "場所", tally: group.places },
    { title: "種目", tally: group.events },
  ];
  return (
    <div className="space-y-4">
      <section className={panel} aria-labelledby="injury-recovery">
        <h2 id="injury-recovery" className="text-base font-bold">
          復帰までの目安
        </h2>
        {recovery.length > 0 ? (
          <div className="space-y-3">
            {recovery.map((r) => (
              <div key={r.severity}>
                <h3 className="text-sm font-bold text-slate-100">{r.severity_label}</h3>
                <dl className="mt-1 grid grid-cols-[auto_1fr] gap-x-4 gap-y-1 text-sm">
                  {r.stages.map((s) => (
                    <div key={s.stage} className="contents">
                      <dt className="text-slate-300">{s.stage_label}</dt>
                      <dd>
                        {s.low_days !== null && s.high_days !== null ? (
                          <>
                            {injuryRecoveryRangeText(s.low_days, s.high_days)}
                            <span className="text-slate-300">({s.people}人)</span>
                          </>
                        ) : (
                          <span className="text-slate-400">—</span>
                        )}
                      </dd>
                    </div>
                  ))}
                </dl>
              </div>
            ))}
          </div>
        ) : (
          <Waiting min={min} />
        )}
      </section>

      <section className={panel} aria-labelledby="injury-prevention">
        <h2 id="injury-prevention" className="text-base font-bold">
          どんなときにケガをしたか
        </h2>
        {hasItems(...prevention.map((item) => item.tally)) ? (
          prevention.map((item) => <Bars key={item.title} title={item.title} tally={item.tally} />)
        ) : (
          <Waiting min={min} />
        )}
      </section>

      <section className={panel} aria-labelledby="injury-actions">
        <h2 id="injury-actions" className="text-base font-bold">
          復帰までにやったこと
        </h2>
        {hasItems(group.actions) ? <Bars title="やった人の割合" tally={group.actions} /> : <Waiting min={min} />}
      </section>

      <section className={panel} aria-labelledby="injury-notes">
        <h2 id="injury-notes" className="text-base font-bold">
          次の人への一言
        </h2>
        {group.notes.length === 0 ? (
          <p className="text-sm text-slate-400">まだありません</p>
        ) : (
          <ul className="space-y-2">
            {group.notes.map((n) => (
              <li key={n.id} className="rounded-xl border border-white/15 bg-white/5 p-3">
                <p className="whitespace-pre-wrap text-sm">{n.note}</p>
                <div className="mt-1.5 flex items-center justify-between gap-3">
                  <p className="text-xs text-slate-300">
                    {n.stage_label}になったとき{n.event_label ? `・${n.event_label}` : ""}
                  </p>
                  <TargetReport targetType="injury_note" id={n.id} next={`/injury/${group.body_part}`} noun="一言" />
                </div>
              </li>
            ))}
          </ul>
        )}
      </section>

      <p className="text-xs text-slate-400">診断のかわりにはなりません。痛みが続くときは病院へ。</p>
    </div>
  );
}
