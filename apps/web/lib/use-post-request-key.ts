import { useRef } from "react";

/**
 * 投稿の作成を冪等にするキー(#382)。フォームを開いている間は同じ値を返し、
 * 通信の失敗のあとに送り直しても、サーバーは最初に作った投稿を返す(二重投稿にならない)。
 * 投稿が成功したらフォームは別の画面に移るので、次に開いたときは新しいキーになる。
 */
export function usePostRequestKey(): () => string {
  const key = useRef<string | null>(null);
  return () => {
    key.current ??= crypto.randomUUID();
    return key.current;
  };
}
