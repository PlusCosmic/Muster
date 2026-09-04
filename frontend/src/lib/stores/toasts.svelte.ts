// Non-blocking toast notifications. Backend errors are plain strings; every
// api call in the UI funnels its rejection through `toastError`.

export type ToastKind = 'error' | 'success' | 'info';

export interface Toast {
  id: number;
  kind: ToastKind;
  message: string;
  detail?: string;
}

let nextId = 1;

class ToastStore {
  items = $state<Toast[]>([]);

  push(kind: ToastKind, message: string, detail?: string, ttlMs = 6000): number {
    const id = nextId++;
    this.items = [...this.items, { id, kind, message, detail }];
    if (ttlMs > 0) {
      setTimeout(() => this.dismiss(id), ttlMs);
    }
    return id;
  }

  dismiss(id: number) {
    this.items = this.items.filter((t) => t.id !== id);
  }

  clear() {
    this.items = [];
  }
}

export const toasts = new ToastStore();

/** Normalise whatever a rejected `invoke` threw into a display string. */
export function errorText(err: unknown): string {
  if (typeof err === 'string') return err;
  if (err instanceof Error) return err.message;
  if (err && typeof err === 'object') {
    const msg = (err as { message?: unknown }).message;
    if (typeof msg === 'string') return msg;
    try {
      return JSON.stringify(err);
    } catch {
      return String(err);
    }
  }
  return String(err);
}

export function toastError(context: string, err: unknown) {
  toasts.push('error', context, errorText(err), 9000);
}

export function toastSuccess(message: string, detail?: string) {
  toasts.push('success', message, detail, 5000);
}

export function toastInfo(message: string, detail?: string) {
  toasts.push('info', message, detail, 5000);
}
