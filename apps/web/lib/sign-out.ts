import { apiFetch } from "./api";

// ログアウト・退会のときに、この端末に残る前の人のものを片付ける。共有の端末で次の人に見えないようにする。

// 人ごとの localStorage のキー(ケガの復帰のお祝いを出したか、レビューの足のサイズなど)。端末の設定(県など)は残す。
const personalKeyPrefixes = ["injury-celebrated:", "review-attrs"];

export function clearPersonalStorage(storage: Pick<Storage, "length" | "key" | "removeItem">) {
  const keys: string[] = [];
  for (let i = 0; i < storage.length; i++) {
    const key = storage.key(i);
    if (key && personalKeyPrefixes.some((prefix) => key.startsWith(prefix))) keys.push(key);
  }
  for (const key of keys) storage.removeItem(key);
}

// この端末の通知の購読を外す。API に知らせないと、ログアウトしたあとも前の人あての通知(保護者の同意や
// 添削の依頼など)がこの端末に届き続ける。token が無い(退会したあと)ときは、ブラウザ側だけ外す。
export async function unsubscribePushOnThisDevice(token: string | null) {
  let timer: ReturnType<typeof setTimeout> | undefined;
  try {
    await Promise.race([
      removePushSubscription(token),
      new Promise<void>((resolve) => { timer = setTimeout(resolve, 2000); }),
    ]);
  } finally {
    clearTimeout(timer);
  }
}

async function removePushSubscription(token: string | null) {
  if (typeof navigator === "undefined" || !navigator.serviceWorker) return;
  try {
    const registration = await navigator.serviceWorker.getRegistration();
    const subscription = await registration?.pushManager.getSubscription();
    if (!subscription) return;
    const unsubscribe = subscription.unsubscribe().catch(() => undefined);
    if (token) {
      await apiFetch<void>("/v1/me/push-subscriptions", {
        method: "DELETE",
        token,
        signal: AbortSignal.timeout(1500),
        body: JSON.stringify({ endpoint: subscription.endpoint }),
      }).catch(() => undefined);
    }
    await unsubscribe;
  } catch {
    // 通知を外せなくてもログアウトは止めない。
  }
}
