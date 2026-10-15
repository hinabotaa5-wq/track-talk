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
  const title = typeof data.title === "string" && data.title ? data.title : "TrackTalk";
  const options = {
    body: typeof data.body === "string" ? data.body : "",
    icon: "/icon",
    badge: "/icon",
    tag: typeof data.tag === "string" && data.tag ? data.tag : undefined,
    data: { url: typeof data.url === "string" ? data.url : "/" },
  };
  event.waitUntil(self.registration.showNotification(title, options));
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
    self.clients.matchAll({ type: "window", includeUncontrolled: true }).then((windows) => {
      for (const client of windows) {
        if (client.url === target.href && "focus" in client) return client.focus();
      }
      return self.clients.openWindow(target.href);
    }),
  );
});
