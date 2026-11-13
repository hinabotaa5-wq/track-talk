const key = "account-origin";

export function readAccountOrigin() {
  try {
    const path = sessionStorage.getItem(key) ?? "/coaching";
    return path.startsWith("/") && !path.startsWith("//") && !/^\/(me|login)([/?]|$)/.test(path) ? path : "/coaching";
  } catch { return "/coaching"; }
}

export function rememberAccountOrigin(path: string) {
  try { sessionStorage.setItem(key, path); } catch { /* 保存できなくても画面は開く。 */ }
}
