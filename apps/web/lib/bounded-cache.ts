// 読み書きされた順で保持する。期限切れの値は読み出さない。
export class BoundedCache<T> {
  private entries = new Map<string, { value: T; at: number }>();
  constructor(private limit: number, private ttl: number) {}
  get(key: string): T | undefined {
    const entry = this.entries.get(key);
    if (!entry) return undefined;
    this.entries.delete(key);
    if (Date.now() - entry.at >= this.ttl) return undefined;
    this.entries.set(key, entry);
    return entry.value;
  }
  set(key: string, value: T) {
    this.entries.delete(key);
    this.entries.set(key, { value, at: Date.now() });
    for (const [oldKey, entry] of this.entries) {
      if (Date.now() - entry.at >= this.ttl) this.entries.delete(oldKey);
    }
    while (this.entries.size > this.limit) this.entries.delete(this.entries.keys().next().value!);
  }
  delete(key: string) { this.entries.delete(key); }
  clear() { this.entries.clear(); }
  keys() { return this.entries.keys(); }
}
