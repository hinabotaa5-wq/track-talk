// 表示したURLの数ではなく、ブラウザが実際に追加・置換した履歴を追う。
// このページを開く前のログイン画面や外部サイトの履歴は戻り先にしない。
const key = "tracktalkHeaderHistory";
type Entry = { href: string; previous: number | null };

export function trackHeaderHistory(history: History, location: Location) {
  const scope = crypto.randomUUID();
  let sequence = 0;
  const entries = new Map<number, Entry>();
  const originalPush = history.pushState;
  const originalReplace = history.replaceState;
  const href = () => location.pathname + location.search + location.hash;
  const currentId = (): number | null => {
    const marker = history.state?.[key];
    return marker?.scope === scope && entries.has(marker.id) ? marker.id : null;
  };
  const tagged = (data: unknown, id: number) => ({
    ...(data && typeof data === "object" ? data : {}),
    [key]: { scope, id },
  });

  originalReplace.call(history, tagged(history.state, sequence), "");
  entries.set(sequence, { href: href(), previous: null });

  const push: History["pushState"] = function (data, unused, url) {
    const previous = currentId();
    const id = ++sequence;
    originalPush.call(history, tagged(data, id), unused, url);
    entries.set(id, { href: href(), previous });
  };
  const replace: History["replaceState"] = function (data, unused, url) {
    const id = currentId() ?? ++sequence;
    const previous = entries.get(id)?.previous ?? null;
    originalReplace.call(history, tagged(data, id), unused, url);
    entries.set(id, { href: href(), previous });
  };
  history.pushState = push;
  history.replaceState = replace;

  return {
    canGoBack(signedIn: boolean) {
      const id = currentId();
      const previous = id === null ? null : entries.get(id)?.previous;
      if (previous == null) return false;
      const destination = entries.get(previous);
      if (!destination) return false;
      const path = destination.href.split(/[?#]/)[0];
      // 未ログイン時の「登録→ログインに戻る」は許可する。
      return !signedIn || !["/login", "/register", "/reset-password", "/auth/callback"].some(
        (authPath) => path === authPath || path.startsWith(`${authPath}/`),
      );
    },
    dispose() {
      if (history.pushState === push) history.pushState = originalPush;
      if (history.replaceState === replace) history.replaceState = originalReplace;
    },
  };
}
