import { redirect } from "next/navigation";

// 感覚メモは下のメニューの「メモ」に移った。ブックマークや通知から来た人をそこへ送る。
export default function MySenseRedirect() {
  redirect("/memo");
}
