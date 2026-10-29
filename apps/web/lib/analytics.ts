import { capturePosthog } from "./posthog";

// Umami に送る前に、ページの URL から秘密と検索語を外す。
// 保護者のページ(/guardian/{token})のトークンは、知っていれば保護者として同意やメッセージができてしまう。
// Umami の data-before-send は関数の名前で受け取るので、window に置く。インラインの script で読み込ませる。
export const umamiBeforeSendName = "umamiBeforeSend";

export const umamiBeforeSendScript = `window.${umamiBeforeSendName}=function(type,payload){
  var hide=function(v){return typeof v==="string"?v.split(/[?#]/)[0].replace(/\\/guardian\\/(?!paid(?:[/?#]|$))[^/?#]+/g,"/guardian/:token"):v;};
  if(payload){payload.url=hide(payload.url);payload.referrer=hide(payload.referrer);}
  return payload;
};`;

export type AnalyticsEvent =
  | "onboarding_completed"
  | "post_created"
  | "post_liked"
  | "search_submitted"
  | "race_record_created"
  | "race_record_updated";

// 本文・検索語・アカウントIDを受け取らず、操作名だけを送る。
// 計測が拒否・遮断されても、保存や画面遷移を失敗させない。
export function trackEvent(event: AnalyticsEvent): void {
  if (typeof window === "undefined") return;
  if (!["track-talk.netlify.app"].includes(window.location.hostname)) return;
  capturePosthog(event);
  const tracker = (window as Window & {
    umami?: { track: (name: string) => unknown };
  }).umami;
  try {
    if (tracker) void Promise.resolve(tracker.track(event)).catch(() => {});
  } catch {
    // 計測はベストエフォート。再送で無料枠を消費しない。
  }
}

// API が成功したときだけ呼ぶ。編集・取り消しを新規作成に含めない。
export function trackSuccessfulMutation(path: string, method = "GET"): void {
  const verb = method.toUpperCase();
  if (verb === "POST") {
    if (path === "/v1/posts" || /^\/v1\/injury\/groups\/[^/]+\/posts$/.test(path)) trackEvent("post_created");
    else if (/^\/v1\/posts\/[^/]+\/likes$/.test(path)) trackEvent("post_liked");
    else if (path === "/v1/me/race-records") trackEvent("race_record_created");
  } else if (verb === "PATCH" && /^\/v1\/me\/race-records\/[^/]+$/.test(path)) {
    trackEvent("race_record_updated");
  }
}
