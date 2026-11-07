import { readFileSync } from "node:fs";
import { runInNewContext } from "node:vm";
import { expect, it, vi } from "vitest";

it("shows notifications only for the current device registration", async () => {
  const showNotification = vi.fn();
  let subscriptionId: string | undefined;
  let push!: (event: { data: { json: () => unknown }; waitUntil: (work: Promise<unknown>) => void }) => void;
  runInNewContext(readFileSync(new URL("../public/sw.js", import.meta.url), "utf8"), {
    self: {
      navigator: { locks: { request: (_: string, work: () => Promise<void>) => work() } },
      addEventListener: (event: string, handler: typeof push) => { if (event === "push") push = handler; },
      registration: { showNotification },
    },
    caches: {
      open: async () => ({
        match: async () => subscriptionId ? { json: async () => ({ subscriptionId }) } : undefined,
      }),
    },
  });
  async function deliver(id?: string) {
    let work: Promise<unknown> | undefined;
    push({
      data: { json: () => ({ title: "TrackTalk", body: "通知", subscription_id: id }) },
      waitUntil: (pending) => { work = pending; },
    });
    await work;
  }

  subscriptionId = "first";
  await deliver("first");
  expect(showNotification).toHaveBeenCalledTimes(1);

  subscriptionId = undefined;
  await deliver("first"); // ログアウト後の到達
  await deliver("first"); // 次の登録が失敗した後の到達
  await deliver();
  expect(showNotification).toHaveBeenCalledTimes(1);

  subscriptionId = "second";
  await deliver("first");
  await deliver("second");
  subscriptionId = "third";
  await deliver("first");
  await deliver("third");
  expect(showNotification).toHaveBeenCalledTimes(3);
});
