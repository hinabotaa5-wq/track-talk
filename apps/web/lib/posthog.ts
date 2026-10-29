import type { CaptureResult, PostHog, PostHogConfig } from "posthog-js";
import type { AnalyticsEvent } from "./analytics";
import { afterStartup } from "./after-startup";

const events = new Set([
  "$pageview", "onboarding_completed", "post_created", "post_liked",
  "search_submitted", "race_record_created", "race_record_updated",
]);

// クエリ・投稿ID・保護者リンク・相談やケガの画面を送らない。
const pages = new Set([
  "/", "/register", "/login", "/me", "/me/profile", "/me/posts", "/me/saved",
  "/memo", "/upload", "/competitions", "/competitions/major-hosts",
  "/competitions/major-domestic", "/competitions/regional", "/menu-contest",
  "/posts/new", "/posts/recruit", "/posts/practice", "/posts/video",
  "/posts/url", "/posts/youtube", "/posts/instagram", "/guide", "/guidelines",
  "/terms", "/privacy",
]);

export function analyticsPage(pathname: string): string | null {
  const path = pathname.split(/[?#]/)[0];
  if (pages.has(path)) return path;
  if (/^\/posts\/[^/]+$/.test(path)) return "/posts/:id";
  return null;
}

// SDK の標準プロパティにもURLや参照元が入るので、送信直前に許可したものだけ残す。
export function sanitizePosthogEvent(event: CaptureResult): CaptureResult | null {
  if (!events.has(event.event)) return null;
  const page = typeof event.properties.$pathname === "string" ? analyticsPage(event.properties.$pathname) : null;
  if (!page) return null;
  const properties: Record<string, unknown> = {
    $pathname: page,
    $current_url: `https://track-talk.netlify.app${page}`,
    $process_person_profile: false,
    $geoip_disable: true,
    $ip: "0.0.0.0",
  };
  for (const key of ["token", "distinct_id", "$device_id", "$session_id", "$window_id", "$lib", "$lib_version"]) {
    const value = event.properties[key];
    if (typeof value === "string") properties[key] = value;
  }
  return { uuid: event.uuid, event: event.event, timestamp: event.timestamp, properties };
}

export function posthogConfig(host: string): Partial<PostHogConfig> {
  return {
    api_host: host,
    persistence: "localStorage",
    person_profiles: "never",
    autocapture: false,
    capture_pageview: false,
    capture_pageleave: false,
    disable_session_recording: true,
    disable_surveys: true,
    disable_product_tours: true,
    disable_external_dependency_loading: true,
    advanced_disable_flags: true,
    capture_performance: false,
    capture_dead_clicks: false,
    capture_exceptions: false,
    capture_heatmaps: false,
    capture_webmcp: false,
    rageclick: false,
    save_referrer: false,
    save_campaign_params: false,
    before_send: sanitizePosthogEvent,
  };
}

let client: Promise<PostHog | null> | undefined;

function getClient(): Promise<PostHog | null> {
  const key = process.env.NEXT_PUBLIC_POSTHOG_KEY?.trim();
  const host = process.env.NEXT_PUBLIC_POSTHOG_HOST?.trim().replace(/\/+$/, "");
  // ホストも明示する。未設定なら通信もブラウザへの保存もしない。
  if (!key || !host || !["https://us.i.posthog.com", "https://eu.i.posthog.com"].includes(host)) return Promise.resolve(null);
  client ??= new Promise<void>((resolve) => { afterStartup(resolve); })
    .then(() => import("posthog-js/no-external"))
    .then(({ default: posthog }) => posthog.init(key, posthogConfig(host)))
    .catch(() => null);
  return client;
}

export function capturePosthog(event: AnalyticsEvent | "$pageview", pathname?: string): void {
  if (typeof window === "undefined" || !["track-talk.netlify.app"].includes(window.location.hostname)) return;
  const page = analyticsPage(pathname ?? window.location.pathname ?? "/");
  if (!page) return;
  void getClient().then((posthog) => {
    posthog?.capture(event, { $pathname: page });
  }).catch(() => {});
}
