// @vitest-environment jsdom
import { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { L, loadEnglish, setActiveLocale } from "@/lib/i18n";
import { LanguageSwitch } from "./language-switch";
import { LocaleProvider, useLocale, useLocaleControls } from "./locale-provider";

// 表示言語の決め方（本体 #530）：未ログイン・オンボーディング中は端末の言語、それ以外はアカウントの言語。
// どちらも、選んだ瞬間に画面全体（ここでは Probe）が切り替わる。

const auth = vi.hoisted(() => ({
  session: null as unknown,
  profile: null as null | { locale?: string; nickname?: string | null; event?: string | null },
  token: "token" as string | null,
  refreshProfile: vi.fn(),
}));
vi.mock("@/components/auth-provider", () => ({ useAuth: () => auth }));
const nav = vi.hoisted(() => ({ pathname: "/" }));
vi.mock("next/navigation", () => ({ usePathname: () => nav.pathname }));
const apiFetch = vi.hoisted(() => vi.fn());
vi.mock("@/lib/api", async (original) => ({ ...(await original<typeof import("@/lib/api")>()), apiFetch }));

function Probe() {
  const locale = useLocale();
  return <p data-testid="probe">{`${locale}:${L("ホーム")}`}</p>;
}

function DeviceChooser() {
  const { chooseDeviceLocale } = useLocaleControls();
  return (
    <button type="button" data-testid="choose-en" onClick={() => chooseDeviceLocale("en")}>
      en
    </button>
  );
}

let root: Root;
let container: HTMLDivElement;
const probe = () => container.querySelector("[data-testid=probe]")?.textContent;

async function render(children: React.ReactNode) {
  await act(async () => root.render(<LocaleProvider>{children}</LocaleProvider>));
  // 英語の辞書は後から読み込むので、読み終わるまで待つ。
  await act(async () => {
    await loadEnglish();
    await new Promise((resolve) => setTimeout(resolve, 0));
  });
}

beforeEach(() => {
  (globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
  localStorage.clear();
  auth.session = null;
  auth.profile = null;
  auth.refreshProfile.mockReset();
  apiFetch.mockReset().mockResolvedValue({});
  nav.pathname = "/";
  container = document.createElement("div");
  document.body.appendChild(container);
  root = createRoot(container);
});
afterEach(async () => {
  await act(async () => root.unmount());
  container.remove();
  localStorage.clear();
  setActiveLocale("ja");
  document.documentElement.lang = "";
});

describe("LocaleProvider", () => {
  it("未ログインで英語を選ぶと、その場で画面全体が英語になり、端末に残る", async () => {
    await render(
      <>
        <Probe />
        <DeviceChooser />
      </>,
    );
    expect(probe()).toBe("ja:ホーム");
    await act(async () => {
      container.querySelector<HTMLButtonElement>("[data-testid=choose-en]")!.click();
    });
    await render(
      <>
        <Probe />
        <DeviceChooser />
      </>,
    );
    expect(probe()).toBe("en:Home");
    expect(localStorage.getItem("tracktalk-locale")).toBe("en");
    expect(document.documentElement.lang).toBe("en");
  });

  it("端末に英語が残っていれば、開き直しても英語で出す", async () => {
    localStorage.setItem("tracktalk-locale", "en");
    await render(<Probe />);
    expect(probe()).toBe("en:Home");
  });

  it("登録直後（ニックネーム・種目が未設定）は、DB の既定値 ja より端末で選んだ英語を優先する", async () => {
    localStorage.setItem("tracktalk-locale", "en");
    auth.session = {};
    auth.profile = { locale: "ja", nickname: null, event: null };
    await render(<Probe />);
    expect(probe()).toBe("en:Home");
  });

  it("設定を終えたアカウントは、アカウントの言語で出し、端末にも写す", async () => {
    localStorage.setItem("tracktalk-locale", "en");
    auth.session = {};
    auth.profile = { locale: "ja", nickname: "ランナー", event: "100m" };
    await render(<Probe />);
    expect(probe()).toBe("ja:ホーム");
    expect(localStorage.getItem("tracktalk-locale")).toBe("ja");
  });
});

describe("マイページの表示言語", () => {
  beforeEach(() => {
    auth.session = {};
    auth.profile = { locale: "ja", nickname: "ランナー", event: "100m" };
    nav.pathname = "/me";
  });

  const radio = (lang: string) => container.querySelector<HTMLButtonElement>(`[role=radio][lang=${lang}]`)!;
  const saveButton = () => [...container.querySelectorAll("button")].find((b) => b.textContent === "Save" || b.textContent === "保存");

  it("選ぶとすぐに切り替わり、「保存」でアカウントに保存する", async () => {
    await render(
      <>
        <Probe />
        <LanguageSwitch />
      </>,
    );
    expect(saveButton()).toBeUndefined();
    await act(async () => radio("en").click());
    await render(
      <>
        <Probe />
        <LanguageSwitch />
      </>,
    );
    expect(probe()).toBe("en:Home");
    expect(radio("en").getAttribute("aria-checked")).toBe("true");
    await act(async () => saveButton()!.click());
    expect(apiFetch).toHaveBeenCalledWith("/v1/me/locale", expect.objectContaining({ method: "PATCH", body: JSON.stringify({ locale: "en" }) }));
    expect(auth.refreshProfile).toHaveBeenCalled();
    expect(localStorage.getItem("tracktalk-locale")).toBe("en");
  });

  it("保存せずに画面を移ると、保存済みの言語に戻る", async () => {
    await render(<LanguageSwitch />);
    await act(async () => radio("en").click());
    nav.pathname = "/";
    await render(<Probe />);
    expect(probe()).toBe("ja:ホーム");
    expect(apiFetch).not.toHaveBeenCalled();
  });

  it("保存に失敗したら赤字で知らせ、選んだ言語の試し表示は残す", async () => {
    apiFetch.mockRejectedValue(new Error("422"));
    await render(<LanguageSwitch />);
    await act(async () => radio("en").click());
    await render(<LanguageSwitch />);
    await act(async () => saveButton()!.click());
    expect(container.querySelector("[role=alert]")?.textContent).toBe("Could not save. Please try again.");
    expect(radio("en").getAttribute("aria-checked")).toBe("true");
    expect(saveButton()).toBeDefined();
  });

  it("保存できていれば、プロフィールの取り直しに失敗しても失敗表示は出さない", async () => {
    auth.refreshProfile.mockRejectedValue(new Error("network"));
    await render(<LanguageSwitch />);
    await act(async () => radio("en").click());
    await render(<LanguageSwitch />);
    await act(async () => saveButton()!.click());
    expect(apiFetch).toHaveBeenCalledWith("/v1/me/locale", expect.anything());
    expect(container.querySelector("[role=alert]")).toBeNull();
    expect(localStorage.getItem("tracktalk-locale")).toBe("en");
  });
});
