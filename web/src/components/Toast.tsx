import { createSignal, For, Show } from 'solid-js';
import { IconCheck, IconAlertTriangle, IconInfoCircle } from '@tabler/icons-solidjs';

export type ToastType = 'success' | 'error' | 'info';

export interface ToastItem {
  id: number;
  message: string;
  type: ToastType;
}

const [toasts, setToasts] = createSignal<ToastItem[]>([]);
let nextId = 0;

export function showToast(message: string, type: ToastType = 'info'): void {
  const id = ++nextId;
  setToasts((prev) => [...prev, { id, message, type }]);
  setTimeout(() => {
    setToasts((prev) => prev.filter((t) => t.id !== id));
  }, 3000);
}

export function ToastContainer() {
  return (
    <div class="fixed bottom-6 right-6 z-50 flex flex-col gap-2 pointer-events-none">
      <For each={toasts()}>
        {(toast) => (
          <div
            class={`pointer-events-auto bg-bg-card-elevated border border-border-default rounded-md px-4 py-3 text-sm text-text-primary shadow-lg flex items-center gap-2.5 transition-all duration-300 [animation:slide-up_0.3s_ease] ${
              toast.type === 'success'
                ? 'border-l-4 border-l-success'
                : toast.type === 'error'
                ? 'border-l-4 border-l-danger'
                : 'border-l-4 border-l-primary'
            }`}
          >
            <Show when={toast.type === 'success'}>
              <IconCheck size={18} class="text-success shrink-0" />
            </Show>
            <Show when={toast.type === 'error'}>
              <IconAlertTriangle size={18} class="text-danger shrink-0" />
            </Show>
            <Show when={toast.type === 'info'}>
              <IconInfoCircle size={18} class="text-primary shrink-0" />
            </Show>
            <span>{toast.message}</span>
          </div>
        )}
      </For>
    </div>
  );
}

export { toasts };
