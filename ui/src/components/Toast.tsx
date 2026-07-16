import { useState, useEffect, useRef } from 'react';
import { subscribeToasts, type ToastMsg } from './toastBus';
import { XIcon } from './Icons';

const TOAST_MS = 3500;

function ToastItem({ toast, onDismiss }: { toast: ToastMsg; onDismiss: (id: number) => void }) {
  const timer = useRef<ReturnType<typeof setTimeout> | null>(null);

  // Hovering pauses the clock so longer messages can actually be read.
  const arm = (ms: number) => {
    timer.current = setTimeout(() => onDismiss(toast.id), ms);
  };
  const disarm = () => {
    if (timer.current) clearTimeout(timer.current);
    timer.current = null;
  };

  useEffect(() => {
    arm(TOAST_MS);
    return disarm;
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  return (
    <div
      className={`toast toast-${toast.kind}`}
      onMouseEnter={disarm}
      onMouseLeave={() => arm(1500)}
    >
      <span className="toast-message">{toast.message}</span>
      <button
        className="toast-dismiss"
        onClick={() => onDismiss(toast.id)}
        aria-label="Dismiss notification"
      >
        <XIcon size={11} />
      </button>
    </div>
  );
}

export function ToastHost() {
  const [toasts, setToasts] = useState<ToastMsg[]>([]);

  useEffect(() => {
    subscribeToasts(t => setToasts(prev => [...prev, t]));
    return () => subscribeToasts(null);
  }, []);

  if (toasts.length === 0) return null;

  const dismiss = (id: number) => setToasts(prev => prev.filter(x => x.id !== id));

  return (
    <div className="toast-host" role="status" aria-live="polite">
      {toasts.map(t => (
        <ToastItem key={t.id} toast={t} onDismiss={dismiss} />
      ))}
    </div>
  );
}
