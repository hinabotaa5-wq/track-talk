import { InjuryNoteReport } from "./injury-note-report";
import { injuryPercent, injuryRecoveryRangeText, type InjuryPublicGroup, type InjuryTally } from "@/lib/injury";

const panel = "space-y-3 rounded-2xl border border-white/20 glass bg-white/10 p-4";

function waitingText(people: number, min: number) {
  return `いま${people}人。${min}人そろうと表示します。`;
}

function Bars({ title, tally, min }: { title: string; tally: InjuryTally; min: number }) {
  return (
    <div className="space-y-2">
      <h3 className="text-sm font-bold text-slate-100">{title}</h3>
      {tally.items.length === 0 ? (
        <p className="text-sm text-slate-300">{waitingText(tally.people, min)}</p>
      ) : (
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
      )}
    </div>
  );
}

// グループの公開ページの中身。ログインしていなくても、検索から来た人にも見える。
// 集計は少人数だと誰のことかわかるので、min_people 人そろうまでは出さない(API 側でも出さない)。
export function InjuryPublicSummary({ group }: { group: InjuryPublicGroup }) {
  const min = group.min_people;
  const anyRecovery = group.recovery.some((r) => r.stages.some((s) => s.low_days !== null));
  return (
    <div className="space-y-4">
      <section className={panel} aria-labelledby="injury-recovery">
        <h2 id="injury-recovery" className="text-base font-bold">
          復帰までの目安
        </h2>
        <p className="text-sm text-slate-300">ケガをしてから、その段階に着くまで。真ん中の半分の人が入る幅です。</p>
        {anyRecovery ? (
          <div className="space-y-3">
            {group.recovery.map((r) => (
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
                          <span className="text-slate-400">{s.people > 0 ? `あと${min - s.people}人で表示` : "まだ記録がありません"}</span>
                        )}
                      </dd>
                    </div>
                  ))}
                </dl>
              </div>
            ))}
          </div>
        ) : (
          <p className="text-sm text-slate-300">まだ記録が少ないため、目安は出ていません。同じ重さで{min}人そろうと表示します。</p>
        )}
      </section>

      <section className={panel} aria-labelledby="injury-prevention">
        <h2 id="injury-prevention" className="text-base font-bold">
          どんなときにケガをしたか
        </h2>
        <p className="text-sm text-slate-300">予防のヒントに。参加した人がケガをしたときの答えです。</p>
        <Bars title="ケガの前に変わったこと" tally={group.pre_changes} min={min} />
        <Bars title="そのときしていたこと" tally={group.activities} min={min} />
        <Bars title="場所" tally={group.places} min={min} />
        <Bars title="種目" tally={group.events} min={min} />
      </section>

      <section className={panel} aria-labelledby="injury-actions">
        <h2 id="injury-actions" className="text-base font-bold">
          復帰までに意識的にやったこと
        </h2>
        <Bars title="やった人の割合" tally={group.actions} min={min} />
      </section>

      <section className={panel} aria-labelledby="injury-notes">
        <h2 id="injury-notes" className="text-base font-bold">
          次の人への一言
        </h2>
        {group.notes.length === 0 ? (
          <p className="text-sm text-slate-300">まだありません。段階を変えるときに書いた一言のうち、公開してよいと選ばれたものが載ります。</p>
        ) : (
          <ul className="space-y-2">
            {group.notes.map((n) => (
              <li key={n.id} className="rounded-xl border border-white/15 bg-white/5 p-3">
                <p className="whitespace-pre-wrap text-sm">{n.note}</p>
                <div className="mt-1.5 flex items-center justify-between gap-3">
                  <p className="text-xs text-slate-300">
                    {n.stage_label}になったとき{n.event_label ? `・${n.event_label}` : ""}
                  </p>
                  <InjuryNoteReport id={n.id} part={group.body_part} />
                </div>
              </li>
            ))}
          </ul>
        )}
      </section>

      <p className="text-xs text-slate-400">
        ここの数字は、参加した人の記録をまとめたものです。診断や治療のかわりにはなりません。痛みが続くときは病院でみてもらってください。
      </p>
    </div>
  );
}
