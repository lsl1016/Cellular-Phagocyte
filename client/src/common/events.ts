// 极简事件总线。

type Handler = (payload: unknown) => void;

export class EventBus {
  private map = new Map<string, Set<Handler>>();

  on(event: string, handler: Handler): () => void {
    let set = this.map.get(event);
    if (!set) {
      set = new Set();
      this.map.set(event, set);
    }
    set.add(handler);
    return () => set!.delete(handler);
  }

  emit(event: string, payload?: unknown): void {
    const set = this.map.get(event);
    if (!set) return;
    for (const h of set) h(payload);
  }

  clear(): void {
    this.map.clear();
  }
}
