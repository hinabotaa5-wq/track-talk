/** @vitest-environment jsdom */
import { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeAll, beforeEach, expect, it, vi } from "vitest";

// 合成データのみ。実在の個人情報・本物のシークレットは使わない。
const SECRET = "SYNTHETIC-SECRET-0123456789";
const QR = "data:image/svg+xml;base64,c3ludGhldGljLXFy";

const mocks = vi.hoisted(() => ({
  signInWithPassword: vi.fn(),
  listFactors: vi.fn(),
  enroll: vi.fn(),
  challengeAndVerify: vi.fn(),
  signOut: vi.fn(),
}));
vi.mock("@supabase/supabase-js", () => ({
  createClient: () => ({
    auth: {
      signInWithPassword: (...args: unknown[]) => mocks.signInWithPassword(...args),
      mfa: {
        listFactors: (...args: unknown[]) => mocks.listFactors(...args),
        enroll: (...args: unknown[]) => mocks.enroll(...args),
        challengeAndVerify: (...args: unknown[]) => mocks.challengeAndVerify(...args),
      },
      signOut: (...args: unknown[]) => mocks.signOut(...args),
    },
  }),
}));

let IdentitySession: typeof import("./identity-session").IdentitySession;

beforeAll(async () => {
  vi.stubEnv("NEXT_PUBLIC_IDENTITY_API_URL", "https://vault-api.example.test");
  vi.stubEnv("NEXT_PUBLIC_IDENTITY_AUTH_URL", "https://vault-auth.example.test");
  vi.stubEnv("NEXT_PUBLIC_IDENTITY_AUTH_PUBLIC_KEY", "synthetic-public-key");
  vi.stubEnv("NEXT_PUBLIC_API_BASE_URL", "https://api.example.test");
  vi.stubEnv("NEXT_PUBLIC_SUPABASE_URL", "https://main.supabase.example.test");
  // MfaEnrollSteps がスマホ判定に使う。jsdom には無いので用意する（非タッチ＝QR表示）。
  Object.defineProperty(window, "matchMedia", {
    value: vi.fn(() => ({ matches: false, addEventListener: () => undefined, removeEventListener: () => undefined })),
    configurable: true,
  });
  const cryptoLike = globalThis.crypto as Crypto & { randomUUID?: () => string };
  if (typeof cryptoLike.randomUUID !== "function") {
    Object.defineProperty(cryptoLike, "randomUUID", { value: () => "synthetic-uuid-0123456789abcdef", configurable: true });
  }
  ({ IdentitySession } = await import("./identity-session"));
});

let container: HTMLDivElement;
let root: Root;
beforeEach(() => {
  (globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
  for (const fn of [mocks.signInWithPassword, mocks.listFactors, mocks.enroll, mocks.challengeAndVerify, mocks.signOut]) fn.mockReset();
  mocks.signOut.mockResolvedValue(undefined);
  container = document.createElement("div");
  document.body.appendChild(container);
  root = createRoot(container);
});
afterEach(async () => {
  await act(async () => root.unmount());
  container.remove();
  vi.restoreAllMocks();
});

async function type(input: Element, value: string) {
  await act(async () => {
    Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, "value")!.set!.call(input, value);
    input.dispatchEvent(new Event("input", { bubbles: true }));
  });
}

async function flush() {
  await act(async () => {
    await new Promise((r) => setTimeout(r, 0));
  });
}

function loginForm() {
  const email = container.querySelector('input[type="email"]')!;
  const password = container.querySelector('input[type="password"]')!;
  const submit = Array.from(container.querySelectorAll("button")).find((b) => b.textContent === "専用画面にログイン")!;
  return { email, password, submit };
}

async function doLogin() {
  const { email, password, submit } = loginForm();
  await type(email, "coach@example.test");
  await type(password, "synthetic-password");
  await act(async () => submit.click());
  await flush();
}

function renderSession() {
  return act(async () => {
    root.render(<IdentitySession>{(token: string) => <p>ログイン後の本体:{token}</p>}</IdentitySession>);
  });
}

it("qr付きenrollではQRと手順が出て、コード入力で検証して中身が表示される", async () => {
  mocks.signInWithPassword.mockResolvedValue({ error: null });
  mocks.listFactors.mockResolvedValue({ error: null, data: { totp: [] } });
  mocks.enroll.mockResolvedValue({ error: null, data: { id: "factor-1", totp: { secret: SECRET, qr_code: QR } } });
  mocks.challengeAndVerify.mockImplementation(async ({ code }: { code: string }) =>
    code === "123456" ? { error: null, data: { access_token: "vault-token-1" } } : { error: new Error("bad") },
  );
  await renderSession();
  await doLogin();

  const img = container.querySelector("img");
  expect(img?.getAttribute("src")).toBe(QR);
  expect(img?.getAttribute("alt")).toBe("二段階認証の QR コード");
  expect(container.textContent).toContain("認証アプリを用意する");
  // MfaEnrollSteps 自身の form に置き換わり、入れ子の form は無い。
  expect(container.querySelectorAll("form").length).toBe(1);

  const code = container.querySelector('input[placeholder="123456"]')!;
  await type(code, "123456");
  const submit = Array.from(container.querySelectorAll("button")).find((b) => b.textContent === "設定する")!;
  await act(async () => submit.click());
  await flush();

  expect(mocks.challengeAndVerify).toHaveBeenCalledWith({ factorId: "factor-1", code: "123456" });
  expect(container.textContent).toContain("ログイン後の本体:vault-token-1");
});

it("qrが無いenrollでは設定キーのみの表示で登録を完了できる", async () => {
  mocks.signInWithPassword.mockResolvedValue({ error: null });
  mocks.listFactors.mockResolvedValue({ error: null, data: { totp: [] } });
  mocks.enroll.mockResolvedValue({ error: null, data: { id: "factor-2", totp: { secret: SECRET } } });
  mocks.challengeAndVerify.mockResolvedValue({ error: null, data: { access_token: "vault-token-2" } });
  await renderSession();
  await doLogin();

  expect(container.querySelector("img")).toBeNull();
  expect(container.textContent).toContain("認証アプリに次の設定キーを登録してください。");
  expect(container.textContent).toContain(SECRET);

  const [code] = Array.from(container.querySelectorAll("input"));
  await type(code, "654321");
  const submit = Array.from(container.querySelectorAll("button")).find((b) => b.textContent === "認証する")!;
  await act(async () => submit.click());
  await flush();

  expect(mocks.challengeAndVerify).toHaveBeenCalledWith({ factorId: "factor-2", code: "654321" });
  expect(container.textContent).toContain("ログイン後の本体:vault-token-2");
});

it("登録済みfactorのログインではenrollされず従来のコード入力だけが出る", async () => {
  mocks.signInWithPassword.mockResolvedValue({ error: null });
  mocks.listFactors.mockResolvedValue({ error: null, data: { totp: [{ id: "existing-1" }] } });
  mocks.challengeAndVerify.mockResolvedValue({ error: null, data: { access_token: "vault-token-3" } });
  await renderSession();
  await doLogin();

  expect(mocks.enroll).not.toHaveBeenCalled();
  expect(container.querySelector("img")).toBeNull();
  expect(container.textContent).not.toContain("設定キー");
  expect(container.textContent).toContain("認証アプリのコード");

  const [code] = Array.from(container.querySelectorAll("input"));
  await type(code, "111222");
  const submit = Array.from(container.querySelectorAll("button")).find((b) => b.textContent === "認証する")!;
  await act(async () => submit.click());
  await flush();

  expect(container.textContent).toContain("ログイン後の本体:vault-token-3");
});

it("enroll失敗では従来のエラーのまま先へ進まない", async () => {
  mocks.signInWithPassword.mockResolvedValue({ error: null });
  mocks.listFactors.mockResolvedValue({ error: null, data: { totp: [] } });
  mocks.enroll.mockResolvedValue({ error: new Error("enroll_failed") });
  await renderSession();
  await doLogin();

  expect(container.querySelector('[role="alert"]')?.textContent).toBe("専用アカウントでログインできませんでした");
  expect(container.querySelector("img")).toBeNull();
  expect(mocks.challengeAndVerify).not.toHaveBeenCalled();
  // ログイン画面に戻り、再試行できる。
  expect(container.querySelector('input[type="email"]')).not.toBeNull();
});

it("QRと設定キーをURL・コンソール・ストレージに書き込まない", async () => {
  const seen: unknown[][] = [];
  for (const method of ["log", "warn", "error", "info", "debug"] as const) {
    vi.spyOn(console, method).mockImplementation((...args: unknown[]) => {
      seen.push(args);
    });
  }
  const localSet = vi.spyOn(window.localStorage, "setItem");
  const sessionSet = vi.spyOn(window.sessionStorage, "setItem");
  const hrefBefore = window.location.href;

  mocks.signInWithPassword.mockResolvedValue({ error: null });
  mocks.listFactors.mockResolvedValue({ error: null, data: { totp: [] } });
  mocks.enroll.mockResolvedValue({ error: null, data: { id: "factor-5", totp: { secret: SECRET, qr_code: QR } } });
  await renderSession();
  await doLogin();

  expect(container.querySelector("img")).not.toBeNull();
  expect(window.location.href).toBe(hrefBefore);
  expect(window.location.href).not.toContain(SECRET);
  expect(localSet).not.toHaveBeenCalled();
  expect(sessionSet).not.toHaveBeenCalled();
  for (const args of seen) {
    expect(JSON.stringify(args)).not.toContain(SECRET);
    expect(JSON.stringify(args)).not.toContain(QR);
  }
});
