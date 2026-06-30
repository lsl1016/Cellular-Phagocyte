// 极简 DOM 构建辅助。

type Props = Record<string, string> & { className?: string; text?: string };

export function h<K extends keyof HTMLElementTagNameMap>(
  tag: K,
  props: Partial<Props> = {},
  children: (Node | string)[] = [],
): HTMLElementTagNameMap[K] {
  const el = document.createElement(tag);
  for (const [k, v] of Object.entries(props)) {
    if (v === undefined) continue;
    if (k === 'className') el.className = v;
    else if (k === 'text') el.textContent = v;
    else el.setAttribute(k, v);
  }
  for (const c of children) {
    el.append(typeof c === 'string' ? document.createTextNode(c) : c);
  }
  return el;
}

export function button(label: string, onClick: () => void, className = 'btn'): HTMLButtonElement {
  const b = h('button', { className, text: label });
  b.addEventListener('click', onClick);
  return b;
}

export function clear(node: HTMLElement): void {
  node.replaceChildren();
}

let toastTimer: ReturnType<typeof setTimeout> | null = null;
export function toast(message: string, ms = 2200): void {
  const el = document.getElementById('toast');
  if (!el) return;
  el.textContent = message;
  el.classList.remove('hidden');
  if (toastTimer) clearTimeout(toastTimer);
  toastTimer = setTimeout(() => el.classList.add('hidden'), ms);
}
