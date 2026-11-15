/** @vitest-environment jsdom */
import { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeAll, beforeEach, expect, it, vi } from "vitest";

// 合成データのみ。実在の個人情報・本物のシークレットは使わない。
const SECRET = "SYNTHETIC-SECRET-0123456789";
const QR = "data:image/svg+xml;base64,c3ludGhldGljLXFy";

type SavedSession = { access_token: string; aal: "aal1" | "aal2" };

const mocks = vi.hoisted(() => ({
  options: null as null | {
    storageKey?: string;
    storage?: {
      getItem: (key: string) => string | null;
      setItem: (key: string, value: string) => void;
      removeItem: (key: string) => void;
    };
  },
  authCallback: null as null | ((event: string, session: { access_token: string } | null) => void),
  signInWithPassword: vi.fn(),
  listFactors: vi.fn(),
  enroll: vi.fn(),
  challengeAndVerify: vi.fn(),
  signOut: vi.fn(),
  getSession: vi.fn(),
  getAuthenticatorAssuranceLevel: vi.fn(),
}));

// Supabase の保存のふるまい（persistSession）の模倣。受け取った storage アダプタ経由でのみ読み書きする。
function readRaw(): { raw: string | null; broken: boolean } {
  let raw: string | null = null;
  try {
    raw = mocks.options?.storage?.getItem(mocks.options?.storageKey ?? "") ?? null;
  } catch {
    return { raw: null, broken: true };
  }
  if (!raw) return { raw: null, broken: false };
  try {
    JSON.parse(raw);
    return { raw, broken: false };
  } catch {
    return { raw, broken: true };
  }
}

function readSaved(): SavedSession | null {
  const { raw, broken } = readRaw();
  if (broken || !raw) return null;
  const parsed = JSON.parse(raw) as Partial<SavedSession>;
  if (typeof parsed.access_token !== "string") return null;
  if (parsed.aal !== "aal1" && parsed.aal !== "aal2") return null;
  return { access_token: parsed.access_token, aal: parsed.aal };
}

function writeSaved(next: SavedSession | null) {
  const { storage, storageKey } = mocks.options ?? {};
  if (!storage || !storageKey) return;
  if (next) storage.setItem(storageKey, JSON.stringify(next));
  else storage.removeItem(storageKey);
}

vi.mock("@supabase/supabase-js", () => ({
  createClient: (_url: unknown, _key: unknown, options?: { auth?: typeof mocks.options }) => {
    mocks.options = options?.auth ?? null;
    return {
      auth: {
        signInWithPassword: (...args: unknown[]) => mocks.signInWithPassword(...args),
        getSession: (...args: unknown[]) => mocks.getSession(...args),
        mfa: {
          listFactors: (...args: unknown[]) => mocks.listFactors(...args),
          enroll: (...args: unknown[]) => mocks.enroll(...args),
          challengeAndVerify: (...args: unknown[]) => mocks.challengeAndVerify(...args),
          getAuthenticatorAssuranceLevel: (...args: unknown[]) => mocks.getAuthenticatorAssuranceLevel(...args),
        },
        signOut: (...args: unknown[]) => mocks.signOut(...args),
        onAuthStateChange: (callback: (event: string, session: { access_token: string } | null) => void) => {
          mocks.authCallback = callback;
          return {
            data: {
              subscription: {
                unsubscribe: () => {
                  if (mocks.authCallback === callback) mocks.authCallback = null;
                },
              },
            },
          };
        },
      },
    };
  },
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
  for (const fn of [
    mocks.signInWithPassword, mocks.listFactors, mocks.enroll, mocks.challengeAndVerify,
    mocks.signOut, mocks.getSession, mocks.getAuthenticatorAssuranceLevel,
  ]) fn.mockReset();
  mocks.options = null;
  mocks.authCallback = null;
  window.sessionStorage.clear();
  window.localStorage.clear();
  mocks.listFactors.mockResolvedValue({ error: null, data: { totp: [] } });
  mocks.getSession.mockImplementation(async () => {
    const { raw, broken } = readRaw();
    if (broken) return { data: { session: null }, error: new Error("stored session is not valid") };
    if (!raw) return { data: { session: null }, error: null };
    const saved = readSaved();
    return { data: { session: saved ? { access_token: saved.access_token } : null }, error: null };
  });
  mocks.getAuthenticatorAssuranceLevel.mockImplementation(async () => {
    const saved = readSaved();
    return {
      data: { currentLevel: saved?.aal ?? null, nextLevel: saved?.aal === "aal1" ? "aal2" : null },
      error: null,
    };
  });
  mocks.signInWithPassword.mockImplementation(async () => {
    writeSaved({ access_token: "vault-aal1-session", aal: "aal1" });
    return { error: null };
  });
  mocks.enroll.mockResolvedValue({ error: null, data: { id: "factor-9", totp: { secret: SECRET, qr_code: QR } } });
  mocks.challengeAndVerify.mockImplementation(async () => {
    writeSaved({ access_token: "vault-aal2-session", aal: "aal2" });
    return { error: null, data: { access_token: "vault-aal2-session" } };
  });
  mocks.signOut.mockImplementation(async () => {
    writeSaved(null);
    mocks.authCallback?.("SIGNED_OUT", null);
    return { error: null };
  });
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

async function doVerify(codeValue = "123456") {
  const code = container.querySelector('input[placeholder="123456"]')!;
  await type(code, codeValue);
  const submit = Array.from(container.querySelectorAll("button")).find((b) => b.textContent === "設定する")!;
  await act(async () => submit.click());
  await flush();
}

function storageKey() {
  const key = mocks.options?.storageKey;
  expect(key).toBeTruthy();
  return key as string;
}

function renderSession() {
  return act(async () => {
    root.render(<IdentitySession>{(token: string) => <p>ログイン後の本体:{token}</p>}</IdentitySession>);
  });
}

// 再読み込み相当。同じタブの sessionStorage は残り、コンポーネントは作り直す。
async function remount() {
  await act(async () => root.unmount());
  container.remove();
  container = document.createElement("div");
  document.body.appendChild(container);
  root = createRoot(container);
  await renderSession();
  await flush();
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

it("QR・設定キー・トークンをURL・コンソール・localStorageに書き込まない", async () => {
  const seen: unknown[][] = [];
  for (const method of ["log", "warn", "error", "info", "debug"] as const) {
    vi.spyOn(console, method).mockImplementation((...args: unknown[]) => {
      seen.push(args);
    });
  }
  const hrefBefore = window.location.href;

  await renderSession();
  await flush();
  await doLogin();
  const key = storageKey();
  // パスワード段階の aal1 セッションは同一タブの復元のために保存される。
  expect(window.sessionStorage.getItem(key)).not.toBeNull();
  await doVerify("123456");
  expect(container.textContent).toContain("ログイン後の本体:vault-aal2-session");

  expect(window.location.href).toBe(hrefBefore);
  expect(window.location.href).not.toContain(SECRET);
  // localStorage には何も書かない（実状態で確認する。Storage のメソッドスパイは
  // この jsdom では素通りしないため、保存先の実内容で確かめる）。
  expect(window.localStorage.length).toBe(0);
  // 保存された中身に QR と設定キーが混ざらない。
  const stored = window.sessionStorage.getItem(key) ?? "";
  expect(stored).not.toContain(SECRET);
  expect(stored).not.toContain(QR);
  expect(JSON.parse(stored).access_token).toBe("vault-aal2-session");
  for (const args of seen) {
    const text = JSON.stringify(args);
    expect(text).not.toContain(SECRET);
    expect(text).not.toContain(QR);
    expect(text).not.toContain("vault-aal2-session");
  }
});

it("aal2 まで完了した後に再マウントするとパスワード・TOTPを求めず children が表示される", async () => {
  await renderSession();
  await flush();
  await doLogin();
  await doVerify("123456");
  expect(container.textContent).toContain("ログイン後の本体:vault-aal2-session");

  await remount();

  expect(container.querySelector('input[type="email"]')).toBeNull();
  expect(container.querySelector('input[type="password"]')).toBeNull();
  expect(container.textContent).toContain("ログイン後の本体:vault-aal2-session");
  // 再認証はしていない。
  expect(mocks.signInWithPassword).toHaveBeenCalledTimes(1);
  expect(mocks.challengeAndVerify).toHaveBeenCalledTimes(1);
});

it("保存された aal1 セッションでは children が表示されず保存が消えてログインフォームに戻る", async () => {
  await renderSession();
  await flush();
  await doLogin();
  const key = storageKey();
  // パスワード段階の aal1 セッションが保存されたまま中断した状態。
  expect(window.sessionStorage.getItem(key)).not.toBeNull();
  expect(container.textContent).not.toContain("ログイン後の本体");

  await remount();

  expect(window.sessionStorage.getItem(key)).toBeNull();
  expect(container.textContent).not.toContain("ログイン後の本体");
  expect(container.querySelector('input[type="email"]')).not.toBeNull();
  expect(container.querySelector('input[type="password"]')).not.toBeNull();
});

it("壊れた保存では保存が消えてログインフォームに戻る", async () => {
  await renderSession();
  await flush();
  const key = storageKey();
  window.sessionStorage.setItem(key, "壊れた保存{{{");

  await remount();

  expect(window.sessionStorage.getItem(key)).toBeNull();
  expect(container.textContent).not.toContain("ログイン後の本体");
  expect(container.querySelector('input[type="email"]')).not.toBeNull();
});

it("期限切れのセッションでは保存が消えてログインフォームに戻る", async () => {
  await renderSession();
  await flush();
  await doLogin();
  await doVerify("123456");
  const key = storageKey();
  expect(window.sessionStorage.getItem(key)).not.toBeNull();

  // 更新に失敗した期限切れ相当。次の一回だけ getSession が失敗する。
  mocks.getSession.mockRejectedValueOnce(new Error("refresh failed"));
  await remount();

  expect(window.sessionStorage.getItem(key)).toBeNull();
  expect(container.textContent).not.toContain("ログイン後の本体");
  expect(container.querySelector('input[type="email"]')).not.toBeNull();
});

it("ログアウトすると保存が消え再マウントしても復元されない", async () => {
  await renderSession();
  await flush();
  await doLogin();
  await doVerify("123456");
  const key = storageKey();
  expect(window.sessionStorage.getItem(key)).not.toBeNull();

  const logout = Array.from(container.querySelectorAll("button")).find((b) => b.textContent === "専用画面からログアウト")!;
  await act(async () => logout.click());
  await flush();

  expect(mocks.signOut).toHaveBeenCalledWith({ scope: "local" });
  expect(window.sessionStorage.getItem(key)).toBeNull();
  expect(container.textContent).not.toContain("ログイン後の本体");
  expect(container.querySelector('input[type="email"]')).not.toBeNull();

  await remount();

  expect(container.textContent).not.toContain("ログイン後の本体");
  expect(container.querySelector('input[type="email"]')).not.toBeNull();
});

it("storage アダプタは sessionStorage だけを使い localStorage に何も書かない", async () => {
  await renderSession();
  await flush();

  const storage = mocks.options?.storage;
  const key = storageKey();
  // 本体アプリの Supabase 既定キー（sb-…）とは別の専用キー。
  expect(key.startsWith("sb-")).toBe(false);
  // アダプタを直接叩いても裏付けは sessionStorage だけ。
  storage?.setItem("probe-808", "1");
  expect(window.sessionStorage.getItem("probe-808")).toBe("1");
  expect(window.localStorage.getItem("probe-808")).toBeNull();
  window.sessionStorage.removeItem("probe-808");

  await doLogin();
  await doVerify("123456");
  await remount();

  expect(container.textContent).toContain("ログイン後の本体:vault-aal2-session");
  // Storage のメソッドスパイはこの jsdom では素通りしないため実状態で確かめる。
  expect(window.localStorage.length).toBe(0);
  expect(window.sessionStorage.getItem(key)).not.toBeNull();
});

it("TOKEN_REFRESHED で children へ渡るトークンが新しいものに差し替わる", async () => {
  await renderSession();
  await flush();
  await doLogin();
  await doVerify("123456");
  expect(container.textContent).toContain("ログイン後の本体:vault-aal2-session");

  await act(async () => {
    mocks.authCallback?.("TOKEN_REFRESHED", { access_token: "vault-aal2-refreshed" });
  });
  await flush();

  expect(container.textContent).toContain("ログイン後の本体:vault-aal2-refreshed");
  expect(container.textContent).not.toContain("ログイン後の本体:vault-aal2-session");
});

it("トークンやセッションの内容をコンソールログ・URLに出力しない", async () => {
  const seen: unknown[][] = [];
  for (const method of ["log", "warn", "error", "info", "debug"] as const) {
    vi.spyOn(console, method).mockImplementation((...args: unknown[]) => {
      seen.push(args);
    });
  }
  const hrefBefore = window.location.href;

  await renderSession();
  await flush();
  await doLogin();
  await doVerify("123456");
  await act(async () => {
    mocks.authCallback?.("TOKEN_REFRESHED", { access_token: "vault-aal2-refreshed" });
  });
  await flush();
  await remount();

  expect(window.location.href).toBe(hrefBefore);
  for (const args of seen) {
    const text = JSON.stringify(args);
    for (const secret of ["vault-aal1-session", "vault-aal2-session", "vault-aal2-refreshed"]) {
      expect(text).not.toContain(secret);
      expect(window.location.href).not.toContain(secret);
    }
  }
});
