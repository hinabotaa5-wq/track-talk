import { renderAppIcon } from "@/lib/app-icon";

export const contentType = "image/png";
export const size = { width: 180, height: 180 };

export default function AppleIcon() {
  return renderAppIcon(180);
}
