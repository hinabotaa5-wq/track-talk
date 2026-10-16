// 検索エンジン向けの構造化データ。"<" をエスケープして、本文に </script> が入っていても閉じないようにする。
export function JsonLd({ data }: { data: object }) {
  return (
    <script type="application/ld+json" dangerouslySetInnerHTML={{ __html: JSON.stringify(data).replace(/</g, "\\u003c") }} />
  );
}
