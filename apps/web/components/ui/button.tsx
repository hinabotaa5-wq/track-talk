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
        variant === "primary" && "bg-blue-600 text-white shadow-xs hover:bg-blue-700 active:bg-blue-800",
        variant === "secondary" && "border border-zinc-200 bg-white text-zinc-800 shadow-xs hover:bg-zinc-50 hover:text-zinc-900 active:bg-zinc-100",
        variant === "ghost" && "text-zinc-700 hover:bg-zinc-100 active:bg-zinc-200/60",
        variant === "danger" && "bg-red-600 text-white shadow-xs hover:bg-red-700 active:bg-red-800",
        className,
      )}
      {...props}
    />
  );
}
