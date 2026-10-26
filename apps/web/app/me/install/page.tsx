import { InstallInstructions } from "@/components/install-guide";

export default function InstallPage() {
  return <div className="mx-auto max-w-md space-y-4">
    <h1 className="text-xl font-bold">ホーム画面に追加して使う</h1>
    <p className="text-sm text-slate-300">ホーム画面のアイコンから、アプリのようにすぐ開けます。</p>
    <InstallInstructions />
  </div>;
}
