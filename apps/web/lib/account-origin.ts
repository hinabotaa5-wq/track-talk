const key = "account-origin";

export function readAccountOrigin() {
  try {
    const path = sessionStorage.getItem(key) ?? "/";
    return path.startsWith("/") && !path.startsWith("//") && !path.startsWith("/me") && !path.startsWith("/login") ? path : "/";
  } catch { return "/"; }
}

export function rememberAccountOrigin(path: string) {
  try { sessionStorage.setItem(key, path); } catch { /* 保存できなくても画面は開く。 */ }
}
