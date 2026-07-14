export type ToastKind = 'success' | 'error';

export interface ToastMsg {
  id: number;
  message: string;
  kind: ToastKind;
}

let nextId = 1;
let listener: ((t: ToastMsg) => void) | null = null;

/*
toast() can be called from anywhere (event handlers, async flows);
the single ToastHost mounted in main.tsx renders the queue.
*/
export function toast(message: string, kind: ToastKind = 'success') {
  listener?.({ id: nextId++, message, kind });
}

export function subscribeToasts(fn: ((t: ToastMsg) => void) | null) {
  listener = fn;
}
