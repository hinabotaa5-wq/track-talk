// 海外の事業者に個人情報を預けることへの同意(個人情報保護法28条)の前に示す説明。
// 登録画面、同意の画面、プライバシーポリシー第7条で同じ文を使う。中身を変えたら API の overseasConsentVersion も上げる。
export const usSystemReportUrl = "https://www.ppc.go.jp/enforcement/infoprovision/laws/offshore_report_america/";

export function OverseasConsentNotice() {
  return (
    <div className="space-y-2 text-sm leading-7 text-slate-300">
      <p>
        <span className="font-semibold text-slate-100">預ける国: </span>
        米国。ログインとデータベース（Supabase）、画像と動画（Cloudinary）、画面の配信（Netlify）、API（Google Cloud）、計測（Umami、PostHog）、自動操作の確認（Cloudflare）、通知の配信、支払い（Stripe）、感覚メモのまとめ（Anthropic）の事業者が米国の会社です。配信網（CDN）を使う事業者は、利用者に近い国のサーバーからも配信するため、処理する国を特定できません。
      </p>
      <p>
        <span className="font-semibold text-slate-100">米国の個人情報の制度: </span>
        日本の個人情報保護法にあたる、国全体で共通の法律はありません。金融（グラム・リーチ・ブライリー法）、医療（HIPAA）、電子通信（電子通信プライバシー法）など分野ごとの連邦の法律と、カリフォルニア州の消費者プライバシー法（CCPA）など州ごとの法律で守っています。個人情報保護委員会の調査では、OECD プライバシーガイドラインの8原則のうち、連邦の法律に規定が見当たらない原則があるとされています。米国は、APEC の越境プライバシールール（CBPR）に参加しています。
      </p>
      <p>
        <span className="font-semibold text-slate-100">事業者がとる措置: </span>
        各社は、利用規約とデータ処理の契約で、預かった情報を TrackTalk のサービスの提供のためだけに使うこと、通信と保存の暗号化やアクセスの制限などの安全管理をすること、漏えいなどがあったときに知らせることを約束しています。運営者は、このポリシーの利用目的を超えて使わせません。
      </p>
      <p>
        くわしくは、
        <a href={usSystemReportUrl} target="_blank" rel="noreferrer" className="text-blue-300 underline">
          個人情報保護委員会「外国における個人情報の保護に関する制度等の調査（米国）」
        </a>
        と、プライバシーポリシー第7条をご覧ください。
      </p>
    </div>
  );
}
