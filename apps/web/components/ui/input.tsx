import { cn } from "@/lib/utils";
import type { InputHTMLAttributes } from "react";

export function Input({ className, ...props }: InputHTMLAttributes<HTMLInputElement>) {
  return (
    <input
      className={cn(
        "w-full rounded-md border border-zinc-300 bg-white px-3 py-2 text-base outline-none focus:border-blue-500",
        className,
      )}
      {...props}
    />
  );
}
