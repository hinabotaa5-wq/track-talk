// @vitest-environment jsdom
import { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { documentConsentMetadata, documentConsentVersions, signupDocumentConsentVersions, currentDocumentConsent, rememberDocumentConsent, takeDocumentConsent } from "./document-consent";
import { ProfileGate } from "@/components/profile-gate";
import ConsentPage from "@/app/consent/page";

const { state, apiFetch, replace, refreshProfile } = vi.hoisted(() => ({
  state: {
    session: { user: { id: "runner", user_metadata: {} as Record<string, unknown> } },
    profile: { overseas_consent: true, documents_consent: false, nickname: "runner", event: "100m" },
    loading: false, token: "token",
  },
  apiFetch: vi.fn(), replace: vi.fn(), refreshProfile: vi.fn(),
}));
vi.mock("next/navigation", () => ({ usePathname: () => "/coaches", useRouter: () => ({replace}), useSearchParams: () => new URLSearchParams("next=/coaches") }));
vi.mock("@/components/auth-provider", () => ({ useAuth: () => ({...state,refreshProfile}) }));
vi.mock("@/lib/api", () => ({ apiFetch, ApiRequestError: class extends Error {} }));
let root: Root;
let div: HTMLDivElement;
beforeEach(() => {
  sessionStorage.clear();vi.clearAllMocks();
  state.session.user.user_metadata = {};
  state.profile = { overseas_consent: true, documents_consent: false, nickname: "runner", event: "100m" };
  apiFetch.mockImplementation(async (path: string) => { if(path.endsWith("/documents")) state.profile={...state.profile,documents_consent:true}; });
  refreshProfile.mockResolvedValue(undefined);
  div=document.createElement("div");document.body.append(div);root=createRoot(div);
});
afterEach(async()=>{await act(async()=>root.unmount());div.remove();});

it("metadata and one-use tab markers carry the displayed versions and reject expired or malformed data", () => {
  expect(currentDocumentConsent(signupDocumentConsentVersions(documentConsentMetadata()))).toBe(true);
  expect(signupDocumentConsentVersions({})).toBeNull();
  rememberDocumentConsent(1000);
  expect(takeDocumentConsent(2000)).toEqual(documentConsentVersions);
  expect(takeDocumentConsent(2000)).toBeNull();
  rememberDocumentConsent(0);expect(takeDocumentConsent(3600001)).toBeNull();
  sessionStorage.setItem("tracktalk-document-consent","broken");expect(takeDocumentConsent()).toBeNull();
  expect(currentDocumentConsent({...documentConsentVersions,terms:"terms-old"})).toBe(false);
});

it("B1: existing overseas consent alone still requires the consent screen",async()=>{
  await act(async()=>root.render(<ProfileGate />));
  expect(replace).toHaveBeenCalledWith("/consent?next=%2Fcoaches");
  expect(apiFetch).not.toHaveBeenCalled();
});

it.each(["metadata","tab"])("B2: current %s versions are recorded on first read without a consent redirect",async(source)=>{
  if(source==="metadata")state.session.user.user_metadata=documentConsentMetadata();else rememberDocumentConsent();
  await act(async()=>root.render(<ProfileGate />));
  expect(apiFetch).toHaveBeenCalledWith("/v1/me/consents/documents",expect.objectContaining({method:"POST",body:JSON.stringify({versions:documentConsentVersions})}));
  expect(refreshProfile).toHaveBeenCalled();
  await act(async()=>root.render(<ProfileGate />));
  expect(replace).not.toHaveBeenCalled();
});

it("B2: old metadata is never automatically recorded",async()=>{
  state.session.user.user_metadata={document_consent_versions:{...documentConsentVersions,privacy:"privacy-old"}};
  await act(async()=>root.render(<ProfileGate />));
  expect(apiFetch).not.toHaveBeenCalled();expect(replace).toHaveBeenCalledWith("/consent?next=%2Fcoaches");
});

it("B3: both current consents allow use without recording or redirecting",async()=>{
  state.profile.documents_consent=true;
  await act(async()=>root.render(<ProfileGate />));
  expect(replace).not.toHaveBeenCalled();expect(apiFetch).not.toHaveBeenCalled();
});

it("B1: the consent screen posts both endpoints before continuing",async()=>{
  await act(async()=>root.render(<ConsentPage />));
  expect(replace).not.toHaveBeenCalled();
  const check=div.querySelector<HTMLInputElement>('input[type="checkbox"]');
  if(!check)throw new Error("checkbox missing");
  await act(async()=>check.click());
  const button=Array.from(div.querySelectorAll("button")).find(b=>b.textContent?.includes("同意して続ける"));
  if(!button)throw new Error("consent button missing");
  await act(async()=>button.click());
  expect(apiFetch).toHaveBeenCalledWith("/v1/me/consents/overseas",expect.objectContaining({method:"POST"}));
  expect(apiFetch).toHaveBeenCalledWith("/v1/me/consents/documents",expect.objectContaining({method:"POST"}));
  await act(async()=>root.render(<ConsentPage />));
  expect(replace).toHaveBeenCalledWith("/coaches");
});
