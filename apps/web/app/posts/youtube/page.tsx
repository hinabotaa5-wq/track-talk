import { redirect } from "next/navigation";

// YouTube と Instagram の URL 投稿は /posts/url にまとめた。古いリンクから来た人もそちらへ送る。
export default function Page() {
  redirect("/posts/url");
}
