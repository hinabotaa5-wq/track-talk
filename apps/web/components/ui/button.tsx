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
        "inline-flex items-center justify-center rounded-full px-4.5 py-2 text-sm font-semibold transition-all duration-150 cursor-pointer disabled:pointer-events-none disabled:opacity-50 active:scale-[0.97] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent",
        variant === "primary" && "bg-accent text-accent-foreground shadow-xs hover:bg-accent-hover active:bg-accent",
        variant === "secondary" && "border border-border bg-surface text-foreground shadow-xs hover:bg-surface-raised active:bg-field",
        variant === "ghost" && "text-foreground hover:bg-surface-raised active:bg-surface-raised/60",
        variant === "danger" && "bg-danger-strong text-danger-foreground shadow-xs hover:bg-danger-hover active:bg-danger-strong",
        className,
      )}
      {...props}
    />
  );
}
