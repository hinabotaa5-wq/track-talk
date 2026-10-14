"use client";

import { motion } from "motion/react";
import { Heart, Bookmark, Flag } from "lucide-react";
import { cn } from "@/lib/utils";

const actionClass =
  "inline-flex items-center gap-2 rounded-full px-4 py-2 text-sm font-semibold transition-colors duration-200 shadow-xs cursor-pointer disabled:opacity-50";

const idleClass =
  "border border-white/20 glass bg-white/12 text-slate-300 hover:bg-white/16 hover:text-white";

export function LikeButton({
  liked,
  count,
  disabled,
  onClick,
  label = "いいね",
}: {
  liked: boolean;
  count: number;
  disabled?: boolean;
  onClick: () => void;
  label?: string;
}) {
  return (
    <motion.button
      type="button"
      whileTap={{ scale: 0.95 }}
      disabled={disabled}
      onClick={onClick}
      className={cn(
        actionClass,
        liked
          ? "bg-rose-950/60 text-rose-300 border border-rose-800/60 hover:bg-rose-900/60"
          : idleClass
      )}
    >
      <motion.span
        key={String(liked)}
        initial={liked ? { scale: 0.6 } : { scale: 1 }}
        animate={{ scale: 1 }}
        transition={{ type: "spring", stiffness: 500, damping: 15 }}
      >
        <Heart
          className={cn(
            "size-4 transition-colors",
            liked ? "fill-rose-500 text-rose-400" : "text-slate-200"
          )}
        />
      </motion.span>
      <span>{label}</span>
      <span className="font-bold tabular-nums">{count}</span>
    </motion.button>
  );
}

export function SaveButton({
  saved,
  disabled,
  onClick,
}: {
  saved: boolean;
  disabled?: boolean;
  onClick: () => void;
}) {
  return (
    <motion.button
      type="button"
      whileTap={{ scale: 0.95 }}
      disabled={disabled}
      onClick={onClick}
      className={cn(
        actionClass,
        saved
          ? "bg-blue-950/60 text-blue-400 border border-blue-800/60 hover:bg-blue-900/60"
          : idleClass
      )}
    >
      <motion.span
        key={String(saved)}
        initial={saved ? { scale: 0.6 } : { scale: 1 }}
        animate={{ scale: 1 }}
        transition={{ type: "spring", stiffness: 500, damping: 15 }}
      >
        <Bookmark
          className={cn(
            "size-4 transition-colors",
            saved ? "fill-blue-400 text-blue-400" : "text-slate-200"
          )}
        />
      </motion.span>
      <span>{saved ? "保存済み" : "保存"}</span>
    </motion.button>
  );
}

export function ReportButton({
  disabled,
  onClick,
}: {
  disabled?: boolean;
  onClick: () => void;
}) {
  return (
    <motion.button
      type="button"
      whileTap={{ scale: 0.95 }}
      disabled={disabled}
      onClick={onClick}
      className={cn(actionClass, idleClass)}
    >
      <Flag className="size-4 text-slate-200" />
      <span>通報</span>
    </motion.button>
  );
}
