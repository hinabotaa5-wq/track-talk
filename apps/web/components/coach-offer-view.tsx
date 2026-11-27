import { L } from "@/lib/i18n";
import { offerTitles, type CoachDemoVideo } from "@/lib/coach-offer";

// コーチ詳細の「このコーチの添削で得られるもの」と「見本動画」（本体 #1086）。
// 得られるものはタイトルだけ（説明文は付けない）。固定4項目→追加項目（登録順、英語表示でも原文のまま）の順。
// 見本動画は別の節にし、0本なら節自体を出さない。
// コーチ詳細では、得られるものを申込みの入口の前に、見本動画を入口の後に置く（入口を下に押しやらない）。
export function CoachOfferSections({ items, videos }: { items?: readonly string[] | null; videos?: readonly CoachDemoVideo[] | null }) {
  return (
    <>
      <CoachOfferItems items={items} />
      <CoachDemoVideos videos={videos} />
    </>
  );
}

export function CoachOfferItems({ items }: { items?: readonly string[] | null }) {
  const titles = offerTitles(items);
  return (
    <section className="glass space-y-2 rounded-2xl border border-white/20 bg-white/10 p-4 text-sm" aria-labelledby="coach-offer-items">
      <h2 id="coach-offer-items" className="text-lg font-bold">{L("このコーチの添削で得られるもの")}</h2>
      <ul className="space-y-1.5" data-testid="coach-offer-items">
        {titles.map((title, i) => (
          <li key={`${i}-${title}`} className="border-l-2 border-blue-400/60 pl-2 break-words text-slate-100">{title}</li>
        ))}
      </ul>
    </section>
  );
}

export function CoachDemoVideos({ videos }: { videos?: readonly CoachDemoVideo[] | null }) {
  const demos = videos ?? [];
  return (
    <>
      {demos.length ? (
        <section className="glass space-y-2 rounded-2xl border border-white/20 bg-white/10 p-4 text-sm" aria-labelledby="coach-demo-videos">
          <h2 id="coach-demo-videos" className="text-lg font-bold">{L("見本動画")}</h2>
          <ul className="space-y-3" data-testid="coach-demo-videos">
            {demos.map((v, i) => (
              <li key={`${i}-${v.url}`}>
                <video src={v.url} poster={v.thumbnail_url || undefined} controls playsInline preload="metadata" aria-label={L("見本動画 {n}", { n: i + 1 })} className="mx-auto w-full max-w-[260px] max-h-[300px] rounded-lg bg-black" />
              </li>
            ))}
          </ul>
        </section>
      ) : null}
    </>
  );
}
