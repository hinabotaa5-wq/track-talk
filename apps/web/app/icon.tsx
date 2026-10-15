import { renderAppIcon } from "@/lib/app-icon";

export const contentType = "image/png";
export const size = { width: 512, height: 512 };

export default function Icon() {
  return renderAppIcon(512);
}
