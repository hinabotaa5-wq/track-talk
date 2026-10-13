import { cn } from "@/lib/utils";
import type { ButtonHTMLAttributes } from "react";

export function Button({
  className,
  variant = "primary",
  ...props
}: ButtonHTMLAttributes<HTMLButtonElement> & {
  variant?: "primary" | "secondary" | "ghost" | "danger";
}) {
  return (
    <button
      className={cn(
        "inline-flex items-center justify-center rounded-full px-4.5 py-2 text-sm font-semibold transition-all duration-150 cursor-pointer disabled:pointer-events-none disabled:opacity-50 active:scale-[0.97] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500",
        variant === "primary" && "bg-blue-600 text-white shadow-xs hover:bg-blue-500 active:bg-blue-700",
        variant === "secondary" && "border border-white/25 glass bg-white/12 text-slate-200 shadow-xs hover:bg-white/16 hover:text-white active:bg-white/20",
        variant === "ghost" && "text-slate-300 hover:bg-white/16 hover:text-white active:bg-white/20",
        variant === "danger" && "bg-red-600 text-white shadow-xs hover:bg-red-500 active:bg-red-700",
        className,
      )}
      {...props}
    />
  );
}
