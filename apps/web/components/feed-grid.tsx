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

export function FeedGrid({ children }: { children: ReactNode }) {
  const columns = useSyncExternalStore(subscribe, columnCount, () => 2);
  const items = Children.toArray(children);
  return (
    <div className="flex items-start gap-3">
      {splitIntoColumns(items, columns).map((column, index) => (
        <div key={index} className="min-w-0 flex-1">
          {column}
        </div>
      ))}
    </div>
  );
}
