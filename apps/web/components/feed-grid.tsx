"use client";

import { Children, type ReactNode } from "react";

// 左右へ新しい順に配り、縦横の比率が違うカードも各列に詰めて並べる。
export function splitIntoColumns<T>(items: T[], columns: number) {
  const result: T[][] = Array.from({ length: columns }, () => []);
  items.forEach((item, index) => result[index % columns].push(item));
  return result;
}

export function FeedGrid({ children }: { children: ReactNode }) {
  const items = Children.toArray(children);
  return (
    <div className="flex items-start gap-3">
      {splitIntoColumns(items, 2).map((column, index) => (
        <div key={index} className="min-w-0 flex-1">
          {column}
        </div>
      ))}
    </div>
  );
}
