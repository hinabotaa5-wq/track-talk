import { useCallback, useRef, useState } from "react";

// 18歳未満が自分で払う動画添削・追加相談の、購入のたびの宣言。
// 文案は運営者が 2026-11-05 に確定。変えたら API の minorPurchaseDeclarationVersion と文面の定数も上げる・直す
// （API は記録に文面を写し、テストでこのファイルの文面と突き合わせる）。
export const minorPurchaseDeclarationLabel = "保護者に許可をもらいました";
export const minorPurchaseDeclarationNote = "未成年の方は、購入する前に保護者の許可を得てください。";

// 宣言の表示・操作の時刻（証跡として API に送り、記録に残す）。端末の時計による。
export type MinorDeclarationLog = { shownAt?: string; checkedAt?: string };

type MinorDeclarationBody = {
  guardian_permission?: boolean;
  guardian_permission_shown_at?: string;
  guardian_permission_checked_at?: string;
};

// API に送る本文。18歳以上には送らない（送っても使われない）。
export function minorDeclarationBody(minor: boolean, checked: boolean, log: MinorDeclarationLog = {}): MinorDeclarationBody {
  if (!minor) return {};
  const body: MinorDeclarationBody = { guardian_permission: checked };
  if (log.shownAt) body.guardian_permission_shown_at = log.shownAt;
  if (checked && log.checkedAt) body.guardian_permission_checked_at = log.checkedAt;
  return body;
}

// useMinorDeclaration は、宣言のチェックの状態と、表示した時刻・チェックを入れた時刻を持つ。
// 表示した時刻は最初に出した時のまま、チェックの時刻は最後にチェックを入れた時にする（外したら消す）。
export function useMinorDeclaration() {
  const [checked, setCheckedState] = useState(false);
  const log = useRef<MinorDeclarationLog>({});
  const setChecked = useCallback((value: boolean) => {
    setCheckedState(value);
    log.current = { ...log.current, checkedAt: value ? new Date().toISOString() : undefined };
  }, []);
  const markShown = useCallback(() => {
    if (!log.current.shownAt) log.current = { ...log.current, shownAt: new Date().toISOString() };
  }, []);
  const body = useCallback((minor: boolean) => minorDeclarationBody(minor, checked, log.current), [checked]);
  return { checked, setChecked, markShown, body };
}
