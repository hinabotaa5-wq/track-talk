// 外部サイト（Stripe の口座登録など）から戻ったあと、画面が白いまま・ヘッダーがないまま
// 置いていかれないようにする判定。描画そのものはコンポーネント側（resume-recovery.tsx）が行い、
// ここには時刻と保存領域だけに依存する純粋な判定を置く。

// 文書の破棄（外部サイトへの移動・再読み込み・タブを閉じる）の記録。
export const resumePagehideKey = "tracktalk:resume-pagehide-at";
// 自動の取り直し（router.refresh）の実行記録。多重の取り直しで往復しないようにする。
const resumeRefreshKey = "tracktalk:resume-refresh-at";

// この間隔以上いなければ、タブの往復ではなく外部サイトへの往復とみなす。
// Stripe の口座登録は分単位かかる。短い往復では何もしない。
export const externalTripGapMs = 10_000;
// 非表示から戻ったときの同じ判定。タブ切り替えの直帰では何もしない。
export const hiddenTripGapMs = 15_000;
// 戻ってから判定までの待ち時間。起動画面（最大10秒）や読み込み中の骨組みが落ち着くのを待つ。
export const recoveryGraceMs = 8_000;
// 取り直しのあと、直ったか確かめるまでの待ち時間。
export const recoveryRecheckMs = 5_000;
// 自動の取り直しはこの間隔で1回まで。それ以上は利用者の再読み込みに任せる。
const refreshCooldownMs = 5 * 60_000;

type PickStorage = Pick<Storage, "getItem" | "setItem" | "removeItem">;

function readMs(storage: PickStorage, key: string): number | null {
  try {
    const raw = storage.getItem(key);
    if (raw === null) return null;
    const at = Number(raw);
    return Number.isFinite(at) ? at : null;
  } catch {
    return null;
  }
}

// 文書が捨てられるときに呼ぶ。アプリ内のタブ移動では pagehide は起きないので、
// ここに記録が残るのは外部への移動・再読み込み・破棄に限られる。
export function notePageHide(storage: PickStorage, now = Date.now()) {
  try {
    storage.setItem(resumePagehideKey, String(now));
  } catch {
    // 保存できない環境では、復帰の検出をあきらめるだけ。
  }
}

// 読み込み直後の文書で呼ぶ。外部サイト往復の長さの不在のあとに戻ってきたら true。
// 記録は使い捨てにする（同じ記録で二重に構えない）。
export function consumeExternalReturn(storage: PickStorage, now = Date.now()) {
  const at = readMs(storage, resumePagehideKey);
  try {
    storage.removeItem(resumePagehideKey);
  } catch {
    // 消せなくても判定は続ける。
  }
  if (at === null) return false;
  return now - at >= externalTripGapMs;
}

// 自動の取り直しをしてよい interval か。よい場合は実行記録を残して true。
// 保存できない環境では制限をかけず true（取り直し自体は無害なため）。
export function shouldRefreshAgain(storage: PickStorage, now = Date.now()) {
  const at = readMs(storage, resumeRefreshKey);
  if (at !== null && now - at < refreshCooldownMs) return false;
  try {
    storage.setItem(resumeRefreshKey, String(now));
  } catch {
    // 記録できなくても取り直しはする。
  }
  return true;
}

// 期待される header 要素がないか。アプリ内の header は components/header.tsx の1箇所だけ。
export function isHeaderMissing(doc: Document) {
  return doc.querySelector("header") === null;
}

// main の中身が要素ごと空か。読み込み中の骨組み・空の表示・失敗の表示はいずれも要素を持つので、
// 要素が1つもない main は「描画されなかった」とみなす（文章ノードだけでは復旧しない）。
export function isMainEmpty(doc: Document) {
  const main = doc.querySelector("main");
  if (!main) return true;
  return main.querySelector("*") === null;
}
