import { JSX, Show } from 'solid-js';
import { IconX } from '@tabler/icons-solidjs';

export interface ModalProps {
  open: boolean;
  onClose: () => void;
  title: string;
  children: JSX.Element;
}

export function Modal(props: ModalProps): JSX.Element {
  return (
    <Show when={props.open}>
      <div
        class="fixed inset-0 z-50 bg-black/60 backdrop-blur-sm flex items-center justify-center p-4 [animation:slide-up_0.2s_ease]"
        onClick={(e) => {
          if (e.target === e.currentTarget) {
            props.onClose();
          }
        }}
      >
        <div class="bento-card max-w-[480px] w-full bg-bg-card-elevated shadow-2xl relative">
          <div class="flex items-center justify-between mb-4">
            <h3 class="text-lg font-bold text-text-primary">{props.title}</h3>
            <button
              type="button"
              onClick={() => props.onClose()}
              class="text-text-muted hover:text-text-primary p-1 rounded-sm transition-colors cursor-pointer"
            >
              <IconX size={18} />
            </button>
          </div>
          <div>{props.children}</div>
        </div>
      </div>
    </Show>
  );
}
