// TEMPORARY（本体 #565・#521 追補、運営者決定 2026-11-12）：
// 未ログイン時の言語選択プロンプト（components/locale-prompt.tsx）を米国（US）からのアクセスのときだけ出すため、
// Netlify の端でアクセス元の国コードだけを返す。IP やその他の位置情報は返さない。
// プロンプトを外すときはこのファイルも消す。
//
// IP からの判定なので、VPN などでは実際の所在地とずれることがある（運営者了承済み）。
// 国が取れないときは null を返し、画面側はプロンプトを出さない。

type GeoContext = { geo?: { country?: { code?: string | null } | null } | null };

export function countryCode(context: GeoContext | null | undefined): string | null {
  const code = context?.geo?.country?.code;
  return typeof code === "string" && /^[A-Za-z]{2}$/.test(code) ? code.toUpperCase() : null;
}

export default function geoCountry(_request: Request, context: GeoContext) {
  return new Response(JSON.stringify({ country: countryCode(context) }), {
    headers: {
      "Content-Type": "application/json; charset=utf-8",
      // 利用者ごとに違う答えなので、端でもブラウザでも残さない。
      "Cache-Control": "private, no-store",
    },
  });
}

export const config = { path: "/geo-country" };
