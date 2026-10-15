import { redirect } from "next/navigation";

// 国内主要大会 is the main competitions tab now.
export default function Page() {
  redirect("/competitions");
}
