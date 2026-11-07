// TrackTalk の service worker。いまは Web Push の表示とタップだけを扱う。
// オフラインキャッシュはしない（古い画面が残る方が困るため）。

self.addEventListener("install", () => {
  self.skipWaiting();
});

self.addEventListener("activate", (event) => {
  event.waitUntil(self.clients.claim());
});

self.addEventListener("push", (event) => {
  let data = {};
  try {
    data = event.data ? event.data.json() : {};
  } catch {
    data = {};
  }
  const locks = self.navigator.locks;
  if (!locks || typeof data?.subscription_id !== "string") return;
  event.waitUntil(locks.request("tracktalk-push", async () => {
    const cache = await caches.open("tracktalk-push");
    const entry = await cache.match("/.tracktalk-push-binding");
    const binding = await entry?.json();
    if (binding?.subscriptionId !== data.subscription_id) return;

    const title = typeof data.title === "string" && data.title ? data.title : "TrackTalk";
    const options = {
      body: typeof data.body === "string" ? data.body : "",
      icon: "/icon",
      badge: "/icon",
      tag: typeof data.tag === "string" && data.tag ? data.tag : undefined,
      data: { url: typeof data.url === "string" ? data.url : "/" },
    };
    await self.registration.showNotification(title, options);
  }));
});

self.addEventListener("notificationclick", (event) => {
  event.notification.close();
  // 通知に入っている URL は自分のサイトの中だけ開く。
  let target = new URL("/", self.location.origin);
  try {
    const url = new URL(event.notification.data && event.notification.data.url, self.location.origin);
    if (url.origin === self.location.origin) target = url;
  } catch {
    // そのままトップを開く
  }
  event.waitUntil(
    self.clients.matchAll({ type: "window", includeUncontrolled: true }).then(async (windows) => {
      for (const client of windows) {
        if (client.url === target.href && "focus" in client) return client.focus();
      }
      // アプリがすでに開いているときは、その画面を通知の行き先へ移す。
      // (開いている画面を前に出すだけだと、行き先に飛ばない)
      for (const client of windows) {
        if (!("focus" in client) || !("navigate" in client)) continue;
        try {
          const focused = await client.focus();
          const moved = await focused.navigate(target.href);
          if (moved) return moved;
        } catch {
          // 移せなければ新しく開く
        }
      }
      return self.clients.openWindow(target.href);
    }),
  );
});
