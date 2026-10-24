import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { analyticsPage, posthogConfig, sanitizePosthogEvent } from "./posthog";

const sdk = vi.hoisted(() => ({ init: vi.fn(), capture: vi.fn() }));
vi.mock("posthog-js/no-external", () => ({ default: sdk }));

beforeEach(() => {
  vi.resetModules();
  sdk.init.mockReset().mockReturnValue(sdk);
  sdk.capture.mockReset();
  vi.stubEnv("NEXT_PUBLIC_POSTHOG_KEY", "phc_public");
  vi.stubEnv("NEXT_PUBLIC_POSTHOG_HOST", "https://eu.i.posthog.com");
  vi.stubGlobal("window", { location: { hostname: "tracktalk.jp", pathname: "/" } });
});
afterEach(() => { vi.unstubAllGlobals(); vi.unstubAllEnvs(); });

describe("PostHog data boundaries", () => {
  it("groups post identifiers and removes query and hash values", () => {
    expect(analyticsPage("/posts/private-id?q=secret#token")).toBe("/posts/:id");
    expect(analyticsPage("/?q=secret")).toBe("/");
  });

  it.each(["/guardian/secret", "/injury/knee", "/me/injury", "/coaching/personal/123", "/admin", "/auth/callback", "/unknown"])("excludes sensitive or unknown pages: %s", (path) => {
    expect(analyticsPage(path)).toBeNull();
  });

  it("keeps the ingestion token and anonymous identifiers while stripping all other content", () => {
    const clean = sanitizePosthogEvent({
      uuid: "event-id", event: "post_created",
      $set: { email: "secret@example.com" }, $set_once: { secret: "private" },
      properties: {
        token: "phc_public", distinct_id: "anonymous", $session_id: "session",
        $pathname: "/posts/private-id", $current_url: "https://tracktalk.jp/?q=secret",
        $referrer: "https://tracktalk.jp/guardian/secret", $initial_referrer: "private",
        $set: { email: "secret@example.com" }, body: "private", $title: "private",
      },
    });
    expect(clean).toEqual({
      uuid: "event-id", event: "post_created", timestamp: undefined,
      properties: {
        token: "phc_public", distinct_id: "anonymous", $session_id: "session",
        $pathname: "/posts/:id", $current_url: "https://tracktalk.jp/posts/:id",
        $process_person_profile: false, $geoip_disable: true, $ip: "0.0.0.0",
      },
    });
    expect(sanitizePosthogEvent({ uuid: "id", event: "$autocapture", properties: { $pathname: "/" } })).toBeNull();
    expect(sanitizePosthogEvent({ uuid: "id", event: "post_created", properties: { $pathname: "/guardian/secret" } })).toBeNull();
  });

  it("disables automatic collection, recordings and additional products", () => {
    expect(posthogConfig("https://eu.i.posthog.com")).toMatchObject({
      persistence: "localStorage", person_profiles: "never", autocapture: false,
      capture_pageview: false, capture_pageleave: false, disable_session_recording: true,
      disable_surveys: true, disable_product_tours: true, advanced_disable_flags: true,
      disable_external_dependency_loading: true, capture_exceptions: false,
      capture_performance: false, capture_heatmaps: false,
    });
  });
});

describe("optional PostHog startup", () => {
  it.each(["NEXT_PUBLIC_POSTHOG_KEY", "NEXT_PUBLIC_POSTHOG_HOST"])("does not initialize when %s is absent", async (name) => {
    vi.stubEnv(name, "");
    const { capturePosthog } = await import("./posthog");
    capturePosthog("search_submitted");
    await new Promise((resolve) => setTimeout(resolve, 0));
    expect(sdk.init).not.toHaveBeenCalled();
    expect(sdk.capture).not.toHaveBeenCalled();
  });

  it("starts once and preserves the page at the time each event happened", async () => {
    const { capturePosthog } = await import("./posthog");
    capturePosthog("$pageview", "/register");
    capturePosthog("onboarding_completed", "/me/profile");
    await vi.waitFor(() => expect(sdk.capture).toHaveBeenCalledTimes(2));
    expect(sdk.init).toHaveBeenCalledTimes(1);
    expect(sdk.capture).toHaveBeenNthCalledWith(1, "$pageview", { $pathname: "/register" });
    expect(sdk.capture).toHaveBeenNthCalledWith(2, "onboarding_completed", { $pathname: "/me/profile" });
  });

  it("does not initialize for local development or excluded pages", async () => {
    const { capturePosthog } = await import("./posthog");
    capturePosthog("$pageview", "/guardian/secret");
    vi.stubGlobal("window", { location: { hostname: "localhost", pathname: "/" } });
    capturePosthog("search_submitted");
    await new Promise((resolve) => setTimeout(resolve, 0));
    expect(sdk.init).not.toHaveBeenCalled();
  });

  it("captures events on the verified Netlify production hostname", async () => {
    vi.stubGlobal("window", { location: { hostname: "track-talk.netlify.app", pathname: "/" } });
    const { capturePosthog } = await import("./posthog");
    capturePosthog("search_submitted");
    await vi.waitFor(() => expect(sdk.capture).toHaveBeenCalledExactlyOnceWith("search_submitted", { $pathname: "/" }));
  });

  it("does not propagate SDK loading or capture failures", async () => {
    sdk.init.mockImplementationOnce(() => { throw new Error("blocked"); });
    const { capturePosthog } = await import("./posthog");
    expect(() => capturePosthog("post_created")).not.toThrow();
    await new Promise((resolve) => setTimeout(resolve, 0));
    expect(sdk.capture).not.toHaveBeenCalled();
  });
});
