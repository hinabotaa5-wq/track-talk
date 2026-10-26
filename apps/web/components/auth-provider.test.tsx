/** @vitest-environment jsdom */
import { act } from "react";
import { createRoot } from "react-dom/client";
import { afterEach, expect, it, vi } from "vitest";
import { AuthProvider, useAuth } from "./auth-provider";

const state = vi.hoisted(() => ({
  callback: null as null | ((event: string, session: unknown) => void),
  unsubscribe: vi.fn(),
  fetch: vi.fn(),
}));
vi.mock("@/lib/supabase", () => ({ createClient: () => ({ auth: {
  onAuthStateChange: (callback: typeof state.callback) => {
    state.callback = callback;
    return { data: { subscription: { unsubscribe: state.unsubscribe } } };
  },
} }) }));
vi.mock("@/lib/api", () => ({ apiFetch: state.fetch }));

afterEach(() => { vi.unstubAllEnvs(); vi.clearAllMocks(); state.callback = null; });

it("keeps the page usable during auth startup, loads the initial profile once, and unsubscribes", async () => {
  (globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
  vi.stubEnv("NEXT_PUBLIC_SUPABASE_URL", "https://example.supabase.co");
  vi.stubEnv("NEXT_PUBLIC_SUPABASE_ANON_KEY", "test-key");
  state.fetch.mockResolvedValue({ id: "user-1", nickname: "runner" });
  const container = document.createElement("div");
  const root = createRoot(container);
  function Page() {
    const { token, profile, loading } = useAuth();
    return <><button>検索</button><span>{loading ? "loading" : `${token}:${profile?.nickname ?? "guest"}`}</span></>;
  }
  try {
    await act(async () => root.render(<AuthProvider><Page /></AuthProvider>));
    expect(container.querySelector("button")?.disabled).toBe(false);
    expect(container.textContent).toContain("loading");
    expect(state.callback).toBeTypeOf("function");
    await act(async () => state.callback?.("INITIAL_SESSION", { access_token: "token-1" }));
    expect(state.fetch).toHaveBeenCalledExactlyOnceWith("/v1/me", { token: "token-1" });
    expect(container.textContent).toContain("token-1:runner");
    await act(async () => state.callback?.("SIGNED_OUT", null));
    expect(container.textContent).toContain("null:guest");
  } finally {
    await act(async () => root.unmount());
  }
  expect(state.unsubscribe).toHaveBeenCalledOnce();
});

it("プロフィールが遅くてもセッションを先に使え、ログアウト後の遅い応答は捨てる", async () => {
  (globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
  vi.stubEnv("NEXT_PUBLIC_SUPABASE_URL", "https://example.supabase.co");
  vi.stubEnv("NEXT_PUBLIC_SUPABASE_ANON_KEY", "test-key");
  let finish!: (profile: unknown) => void;
  state.fetch.mockReturnValue(new Promise((resolve) => { finish = resolve; }));
  const container = document.createElement("div");
  const root = createRoot(container);
  function Page() {
    const { sessionLoading, loading, token, profile } = useAuth();
    return <span>{`${sessionLoading}:${loading}:${token}:${profile?.nickname ?? "none"}`}</span>;
  }
  try {
    await act(async () => root.render(<AuthProvider><Page /></AuthProvider>));
    await act(async () => state.callback?.("INITIAL_SESSION", { access_token: "token-1" }));
    expect(container.textContent).toBe("false:true:token-1:none");
    await act(async () => state.callback?.("SIGNED_OUT", null));
    await act(async () => finish({ nickname: "previous-user" }));
    expect(container.textContent).toBe("false:false:null:none");
  } finally { await act(async () => root.unmount()); }
});
