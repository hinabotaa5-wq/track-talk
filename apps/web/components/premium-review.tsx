import { L } from "@/lib/i18n";
import { premiumSlotLabels } from "@/lib/premium";
import type { PremiumSection, PremiumVideoGroup } from "@/lib/types";

// プレミアム（本体 #1053）の表示部品。選手・コーチの両方の画面で使う。

// 一覧・詳細で、その依頼がプレミアムだと分かる印。
export function PremiumBadge() {
  return (
    <span data-testid="premium-badge" className="inline-flex shrink-0 items-center rounded-full border border-blue-400/50 bg-blue-500/15 px-2 py-0.5 text-[11px] font-bold text-blue-300">
      {L("プレミアム")}
    </span>
  );
}

// 動画を枠ごとにまとめて出す。レース全体の動画にはこの時のタイム・風速・コンディション、練習の動画には目的を併記する。
// 総括の根拠は枠名で引用するので、枠名をそのまま見出しにする。
export function PremiumVideoGroups({ groups, purgedNote }: { groups: PremiumVideoGroup[]; purgedNote?: string | null }) {
  return (
    <div data-testid="premium-video-groups" className="space-y-4">
      {groups.map((group) => (
        <section key={group.slot} aria-label={premiumSlotLabels[group.slot]} className="space-y-2">
          <h3 className="text-sm font-bold text-blue-300">
            {premiumSlotLabels[group.slot]}
            <span className="ml-1 text-xs font-normal text-slate-300">{L("{n}本", { n: group.videos.length })}</span>
          </h3>
          <ul className="space-y-3">
            {group.videos.map((video, i) => {
              const inputs = [
                { label: L("この時のタイム"), value: video.race_time },
                { label: L("風速"), value: video.wind_speed },
                { label: L("その時の体のコンディション"), value: video.body_condition },
                { label: L("その練習をした目的"), value: video.practice_purpose },
              ].filter((row): row is { label: string; value: string } => Boolean(row.value));
              return (
                <li key={video.position} className="space-y-2 rounded-xl border border-white/15 bg-white/5 p-3">
                  <p className="text-xs font-semibold text-slate-300">{L("{slot}の動画{n}", { slot: premiumSlotLabels[group.slot], n: i + 1 })}</p>
                  {video.video_url ? (
                    <video
                      src={video.video_url}
                      poster={video.thumbnail_url || undefined}
                      controls
                      playsInline
                      preload="metadata"
                      className="mx-auto w-full max-w-[260px] max-h-[300px] rounded-xl bg-black"
                    />
                  ) : purgedNote ? (
                    <p className="text-xs text-slate-300">{purgedNote}</p>
                  ) : null}
                  {inputs.length > 0 ? (
                    <dl className="space-y-1">
                      {inputs.map((row) => (
                        <div key={row.label} className="space-y-0.5">
                          <dt className="text-xs font-semibold text-slate-300">{row.label}</dt>
                          <dd className="whitespace-pre-wrap break-words text-sm">{row.value}</dd>
                        </div>
                      ))}
                    </dl>
                  ) : null}
                </li>
              );
            })}
          </ul>
        </section>
      ))}
    </div>
  );
}

// 総括の枠ごとのブロック。枠名を見出しにして、動画がある枠だけ（API が返した分だけ）出す。
export function PremiumSummarySections({ sections }: { sections: PremiumSection[] }) {
  return (
    <>
      {sections.map((section) => (
        <section key={section.slot} data-testid={`premium-section-${section.slot}`} className="space-y-1 text-sm break-words">
          <h3 className="text-xs font-semibold text-blue-300">{premiumSlotLabels[section.slot]}</h3>
          <p className="whitespace-pre-wrap text-slate-100">{section.body}</p>
        </section>
      ))}
    </>
  );
}
