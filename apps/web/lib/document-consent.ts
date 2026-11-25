// Keep these in sync with document_consent.go; bump the changed document on revision.
export const documentConsentVersions = {
  terms: "terms-2026-11-25b",
  guidelines: "guidelines-2026-11-25",
  privacy: "privacy-2026-11-25",
} as const;

export type DocumentConsentVersions = Record<keyof typeof documentConsentVersions, string>;
const metadataKey = "document_consent_versions";
const storageKey = "tracktalk-document-consent";
const maxAgeMs = 60 * 60 * 1000;

function versionsFrom(value: unknown): DocumentConsentVersions | null {
  if (!value || typeof value !== "object") return null;
  const versions = value as Record<string, unknown>;
  return Object.keys(documentConsentVersions).every((key) => typeof versions[key] === "string" && versions[key] !== "")
    ? { terms: versions.terms as string, guidelines: versions.guidelines as string, privacy: versions.privacy as string }
    : null;
}

export function currentDocumentConsent(versions: DocumentConsentVersions | null) {
  return versions !== null && (Object.keys(documentConsentVersions) as (keyof DocumentConsentVersions)[])
    .every((key) => versions[key] === documentConsentVersions[key]);
}

export function documentConsentMetadata() {
  return { [metadataKey]: { ...documentConsentVersions } };
}

export function signupDocumentConsentVersions(metadata: Record<string, unknown> | null | undefined) {
  return versionsFrom(metadata?.[metadataKey]);
}

export function rememberDocumentConsent(now = Date.now()) {
  try {
    sessionStorage.setItem(storageKey, JSON.stringify({ versions: documentConsentVersions, expires: now + maxAgeMs }));
  } catch { /* Without storage, show the consent screen. */ }
}

export function takeDocumentConsent(now = Date.now()): DocumentConsentVersions | null {
  try {
    const stored = sessionStorage.getItem(storageKey);
    sessionStorage.removeItem(storageKey);
    if (!stored) return null;
    const value = JSON.parse(stored);
    return typeof value.expires === "number" && value.expires > now ? versionsFrom(value.versions) : null;
  } catch { return null; }
}
