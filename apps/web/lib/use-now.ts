import { useEffect, useState } from "react";

// useNow は、いまの時刻（ミリ秒）。サーバーで描くときとハイドレーションの間は null を返し、表示のあとで入れる。
// ページが CDN にキャッシュされると、HTML に焼き込んだ「いま」は古く、クライアントで計算し直した値と
// 食い違ってハイドレーションエラーになるので、時刻に依存する表示はこの値が入ってから出す。
export function useNow(): number | null {
  const [now, setNow] = useState<number | null>(null);
  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect
    setNow(Date.now());
  }, []);
  return now;
}
