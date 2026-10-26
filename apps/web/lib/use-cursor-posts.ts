"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { apiFetch, ApiRequestError } from "@/lib/api";
import type { Meta, Post } from "@/lib/types";

const perPage = 24;

// cursorPostsPath は、カーソルで続きを読む一覧の URL を作る。
export function cursorPostsPath(base: string, cursor?: string) {
  const sep = base.includes("?") ? "&" : "?";
  return `${base}${sep}pagination=cursor&per_page=${perPage}${cursor ? `&cursor=${encodeURIComponent(cursor)}` : ""}`;
}

type State = { base: string; token: string; posts: Post[]; next: string; error: string; expired: boolean };

// useCursorPosts は、一覧を最初の1ページだけ読み、続きは loadMore で1ページずつ足す。
// 投稿が増えても、開いたときに全件を取りにいかない。base が null の間は読まない。
// reloadKey が変わると、同じ一覧でも最初から読み直す。
export function useCursorPosts(base: string | null, token: string | null | undefined, reloadKey?: string | null) {
  const [state, setState] = useState<State | null>(null);
  const [loadingMore, setLoadingMore] = useState(false);
  const pending = useRef(false);
  const current = base !== null && state?.base === base && state.token === token ? state : null;

  useEffect(() => {
    if (base === null || !token) return;
    const controller = new AbortController();
    apiFetch<{ posts: Post[]; meta: Meta }>(cursorPostsPath(base), { token, signal: controller.signal })
      .then((data) => {
        if (!controller.signal.aborted) setState({ base, token, posts: data.posts ?? [], next: data.meta.has_more ? data.meta.next_cursor ?? "" : "", error: "", expired: false });
      })
      .catch(() => {
        if (!controller.signal.aborted) setState({ base, token, posts: [], next: "", error: "投稿の取得に失敗しました", expired: false });
      });
    return () => controller.abort();
  }, [base, token, reloadKey]);

  const loadMore = useCallback(async () => {
    if (!current?.next || pending.current || !token) return;
    pending.current = true;
    setLoadingMore(true);
    const { base: from, next } = current;
    try {
      const data = await apiFetch<{ posts: Post[]; meta: Meta }>(cursorPostsPath(from, next), { token });
      setState((value) => {
        if (!value || value.base !== from || value.token !== token || value.next !== next) return value;
        // 読み込み中に削除や並び替えで重なった投稿は足さない。
        const seen = new Set(value.posts.map((post) => post.id));
        const added = (data.posts ?? []).filter((post) => !seen.has(post.id));
        return { ...value, posts: [...value.posts, ...added], next: data.meta.has_more && added.length ? data.meta.next_cursor ?? "" : "", error: "" };
      });
    } catch (error) {
      const expired = error instanceof ApiRequestError && error.code === "invalid_cursor";
      setState((value) => value && value.base === from && value.token === token ? { ...value, error: "続きの読み込みに失敗しました", expired } : value);
    } finally {
      pending.current = false;
      setLoadingMore(false);
    }
  }, [current, token]);

  return {
    posts: current?.posts ?? null,
    hasMore: !!current?.next,
    // 1ページ目が読めなかったときだけ一覧の代わりに出す。続きの失敗は moreError で出す。
    error: current && current.posts.length === 0 ? current.error : "",
    moreError: current && current.posts.length > 0 ? current.error : "",
    expired: !!current?.expired,
    loadingMore,
    loadMore,
  };
}
