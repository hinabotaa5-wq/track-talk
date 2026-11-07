import { apiFetch } from "./api";

// Web Push の購読まわり。通知はブラウザが対応していて、ホーム画面のアプリかブラウザで許可したときだけ届く。

export function pushSupported(): boolean {
  return (
    typeof window !== "undefined" &&
    "serviceWorker" in navigator &&
    "PushManager" in window &&
    "Notification" in window &&
    "caches" in window &&
    Boolean(navigator.locks)
  );
}

// VAPID の公開鍵（base64url）を applicationServerKey に渡せる形にする。
export function base64UrlToBytes(value: string): Uint8Array<ArrayBuffer> {
  const padded = value.replace(/-/g, "+").replace(/_/g, "/").padEnd(Math.ceil(value.length / 4) * 4, "=");
  const raw = atob(padded);
  const bytes = new Uint8Array(new ArrayBuffer(raw.length));
  for (let i = 0; i < raw.length; i++) bytes[i] = raw.charCodeAt(i);
  return bytes;
}

// 新しい購読を保存する前に、その紐付けを照合する版の worker を有効にする。
export async function ensurePushWorker(): Promise<ServiceWorkerRegistration> {
  const registration = await navigator.serviceWorker.register("/sw.js?v=2", { scope: "/" });
  const worker = registration.installing ?? registration.waiting ?? registration.active;
  if (!worker) throw new Error("通知を準備できませんでした");
  if (worker.state !== "activated") {
    await new Promise<void>((resolve, reject) => {
      function finish(error?: Error) {
        clearTimeout(timer);
        worker?.removeEventListener("statechange", check);
        if (error) reject(error);
        else resolve();
      }
      function check() {
        if (worker?.state === "activated") finish();
        if (worker?.state === "redundant") finish(new Error("通知を準備できませんでした"));
      }
      const timer = setTimeout(() => finish(new Error("通知を準備できませんでした")), 5000);
      worker.addEventListener("statechange", check);
      check();
    });
  }
  return registration;
}

export async function subscribePush(publicKey: string): Promise<PushSubscriptionJSON> {
  const registration = await ensurePushWorker();
  const existing = await registration.pushManager.getSubscription();
  if (existing && subscriptionMatchesKey(existing, publicKey, rememberedPushKey())) {
    rememberPushKey(publicKey);
    return existing.toJSON();
  }
  // 前の鍵で作った購読には、鍵を入れ替えたあと通知が届かない。外してから作り直す。
  if (existing) await existing.unsubscribe();
  const subscription = await registration.pushManager.subscribe({
    userVisibleOnly: true,
    applicationServerKey: base64UrlToBytes(publicKey),
  });
  rememberPushKey(publicKey);
  return subscription.toJSON();
}

// 購読を作ったときの公開鍵。iPhone の Safari は購読の鍵を教えないので、この端末に覚えておいて比べる。
const pushKeyStorage = "tracktalk-push-key";

function rememberedPushKey(): string | null {
  try {
    return window.localStorage.getItem(pushKeyStorage);
  } catch {
    return null;
  }
}

function rememberPushKey(publicKey: string) {
  try {
    window.localStorage.setItem(pushKeyStorage, publicKey);
  } catch {
    // 保存できない端末では、次に開いたときも作り直す。
  }
}

// 購読が、いまの VAPID 公開鍵で作られたものか。ブラウザが鍵を教えないときは、この端末に覚えた鍵で比べ、
// 覚えていなければ作り直す。
export function subscriptionMatchesKey(
  subscription: Pick<PushSubscription, "options">,
  publicKey: string,
  remembered: string | null = null,
): boolean {
  const key = subscription.options?.applicationServerKey;
  if (!key) return remembered === publicKey;
  const current = new Uint8Array(key);
  const expected = base64UrlToBytes(publicKey);
  return current.length === expected.length && current.every((value, i) => value === expected[i]);
}

const bindingCache = "tracktalk-push";
const bindingPath = "/.tracktalk-push-binding";

export async function withPushDeviceLock<T>(work: () => Promise<T>): Promise<T> {
  return navigator.locks ? await navigator.locks.request(bindingCache, work) : work();
}

async function readPushBinding(): Promise<{ userId: string; subscriptionId: string } | undefined> {
  const entry = await (await caches.open(bindingCache)).match(bindingPath);
  return entry?.json();
}

export async function clearPushBinding(): Promise<void> {
  if (typeof caches !== "undefined") await (await caches.open(bindingCache)).delete(bindingPath);
  const registration = await navigator.serviceWorker?.getRegistration("/");
  const notifications = await registration?.getNotifications?.();
  notifications?.forEach((notification) => notification.close());
}

async function savePushBinding(token: string, userId: string, sub: PushSubscriptionJSON) {
  const { subscription_id: subscriptionId } = await apiFetch<{ subscription_id: string }>(
    "/v1/me/push-subscriptions?return_subscription_id=1",
    { method: "POST", token, body: JSON.stringify(sub), signal: AbortSignal.timeout(5000) },
  );
  if (!subscriptionId) throw new Error("通知の登録を確認できませんでした");
  await (await caches.open(bindingCache)).put(bindingPath, Response.json({ userId, subscriptionId }));
}

// 認証イベントの処理が待たされたときも、その時点のアカウントで登録する。
export async function currentPushSession(userId: string | null) {
  const { createClient } = await import("./supabase");
  const result = await createClient()?.auth.getSession();
  if (result?.error) throw result.error;
  const session = result?.data.session ?? null;
  if ((session?.user.id ?? null) !== userId) throw new Error("アカウントが切り替わりました");
  return session;
}

// VAPID の鍵を入れ替えたあと、前の鍵の購読を今の鍵で作り直して API に登録し直す。
// 鍵が変わっていなくても登録し直す。API は届かなかった宛先を消すので、消されたままにしない。
// OS の許可が granted のまま端末の購読だけが消えたとき、以前に有効化した端末だけ自動で再購読する。
// 許可はあるので OS ダイアログは出ない。未決定・拒否の人には出さず、既存の案内導線に任せる。
export async function refreshPushSubscription(userId: string | null): Promise<boolean> {
  if (!pushSupported()) return false;
  return withPushDeviceLock(async () => {
    const session = await currentPushSession(userId);
    if (!session) {
      await clearPushBinding();
      return false;
    }
    if ((await readPushBinding())?.userId !== session.user.id) await clearPushBinding();
    const existing = await currentPushSubscription();
    if (!existing) {
      return resubscribeIfPreviouslyEnabled(session.access_token);
    }
    await registerCurrentPushSubscription(session.access_token, session.user.id);
    return true;
  });
}

// OS の許可が granted のまま端末の購読だけが消えたとき、以前に有効化した端末だけ自動で再購読する。
// 許可はあるので OS ダイアログは出ない。未決定・拒否の人には出さず、既存の案内導線に任せる。
async function resubscribeIfPreviouslyEnabled(token: string): Promise<boolean> {
  if (!pushSupported() || Notification.permission !== "granted") return false;
  const { ever_subscribed } = await apiFetch<{ ever_subscribed: boolean }>("/v1/me/push-subscriptions/status", { token });
  if (!ever_subscribed) return false;
  const { public_key } = await apiFetch<{ public_key: string }>("/v1/push/public-key");
  if (!public_key) return false;
  const subscription = await subscribePush(public_key);
  await apiFetch<void>("/v1/me/push-subscriptions", { method: "POST", token, body: JSON.stringify(subscription) });
  return true;
}

async function registerCurrentPushSubscription(token: string, userId: string) {
  const { public_key } = await apiFetch<{ public_key: string }>("/v1/push/public-key", {
    signal: AbortSignal.timeout(5000),
  });
  const subscription = await subscribePush(public_key);
  await savePushBinding(token, userId, subscription);
}

export async function registerPushSubscription(userId: string): Promise<void> {
  await withPushDeviceLock(async () => {
    const session = await currentPushSession(userId);
    if (!session) throw new Error("ログインしてください");
    if ((await readPushBinding())?.userId !== userId) await clearPushBinding();
    await registerCurrentPushSubscription(session.access_token, userId);
  });
}

export async function disablePushSubscription(userId: string): Promise<void> {
  await withPushDeviceLock(async () => {
    const session = await currentPushSession(userId);
    await clearPushBinding();
    const subscription = await readPushSubscription();
    if (!subscription || !session) return;
    await deletePushBinding(session.access_token, subscription.endpoint);
    await subscription.unsubscribe();
  });
}

export function deletePushBinding(token: string, endpoint: string): Promise<void> {
  return apiFetch<void>("/v1/me/push-subscriptions", {
    method: "DELETE",
    token,
    signal: AbortSignal.timeout(5000),
    body: JSON.stringify({ endpoint }),
  });
}

// service worker を新しく登録せず、この端末の購読を確かめる。
export async function readPushSubscription(): Promise<PushSubscription | null> {
  if (!pushSupported()) return null;
  const registration = await navigator.serviceWorker.getRegistration("/");
  return (await registration?.pushManager.getSubscription()) ?? null;
}

export async function currentPushSubscription(): Promise<PushSubscription | null> {
  if (!pushSupported() || Notification.permission !== "granted") return null;
  return readPushSubscription();
}
