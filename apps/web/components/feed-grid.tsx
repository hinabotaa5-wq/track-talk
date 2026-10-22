"use client";

import { Children, useSyncExternalStore, type ReactNode } from "react";

// 画面の幅ごとの列の数(feedGridClass と同じ: スマホ2列、md で3列、xl で4列)。
const wide = "(min-width: 80rem)";
const medium = "(min-width: 48rem)";

function subscribe(onChange: () => void) {
  const lists = [wide, medium].map((query) => window.matchMedia(query));
  lists.forEach((list) => list.addEventListener("change", onChange));
  return () => lists.forEach((list) => list.removeEventListener("change", onChange));
}

function columnCount() {
  if (window.matchMedia(wide).matches) return 4;
  if (window.matchMedia(medium).matches) return 3;
  return 2;
}

// 投稿を左から右へ、上から下へ新しい順に並べる。
// CSS の columns は1列目を上から下まで埋めてから次の列に移るので、2番目に新しい投稿が左の列の下に入り、
// 右の列には途中の投稿が並んで、順番がばらばらに見えていた。カードを左右に順番に配り、高さの違う
// カードはそれぞれの列の中で詰める。
export function splitIntoColumns<T>(items: T[], columns: number) {
  const result: T[][] = Array.from({ length: columns }, () => []);
  items.forEach((item, index) => result[index % columns].push(item));
  return result;
}

// 列の数ごとの見せ方。画面の幅がわからないあいだ(サーバーで描くとき)は3つとも描き、CSS で幅に合うものだけを見せる。
const layouts = [
  { columns: 2, className: "flex md:hidden" },
  { columns: 3, className: "hidden md:flex xl:hidden" },
  { columns: 4, className: "hidden xl:flex" },
] as const;

// layoutsFor は、描く列の数を返す。0(幅がわからない)ならすべて。
export function layoutsFor(columns: number) {
  return columns === 0 ? layouts : layouts.filter((layout) => layout.columns === columns);
}

export function FeedGrid({ children }: { children: ReactNode }) {
  // サーバーでは幅がわからないので 0。2列で描いて PC で組み直すと、読み込んだあとに一覧が動いて見えるため。
  const columns = useSyncExternalStore(subscribe, columnCount, () => 0);
  const items = Children.toArray(children);
  return (
    <>
      {layoutsFor(columns).map((layout) => (
        // key を列の数にして、幅がわかったあとも見えていた並びをそのまま使う(作り直さない)。
        <div key={layout.columns} className={`items-start gap-3 ${columns === 0 ? layout.className : "flex"}`}>
          {splitIntoColumns(items, layout.columns).map((column, index) => (
            <div key={index} className="min-w-0 flex-1">
              {column}
            </div>
          ))}
        </div>
      ))}
    </>
  );
}
