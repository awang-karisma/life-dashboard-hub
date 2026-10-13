import { createSignal, onMount, onCleanup, For, Show } from 'solid-js';
import { IconTerminal2, IconBroadcast } from '@tabler/icons-solidjs';

export interface IngestionEvent {
  timestamp: string;
  device_id: number;
  device_name: string;
  source: string;
  sha256: string;
  records: number;
  tombstones: number;
  latency_ms: number;
  status: string;
}

export function IngestionStream() {
  const [events, setEvents] = createSignal<IngestionEvent[]>([]);
  const [connected, setConnected] = createSignal(false);
  let es: EventSource | null = null;

  onMount(() => {
    try {
      es = new EventSource('/api/v1/events/ingestion');
      es.onopen = () => {
        setConnected(true);
      };
      es.onmessage = (e) => {
        try {
          const item: IngestionEvent = JSON.parse(e.data);
          setEvents((prev) => [item, ...prev].slice(0, 50));
        } catch {
          // ignore heartbeat / unparseable
        }
      };
      es.onerror = () => {
        setConnected(false);
      };
    } catch {
      setConnected(false);
    }
  });

  onCleanup(() => {
    if (es) {
      es.close();
      es = null;
    }
  });

  const formatTime = (ts: string) => {
    try {
      const d = new Date(ts);
      return d.toTimeString().split(' ')[0] || ts;
    } catch {
      return ts;
    }
  };

  const getEventClass = (status: string) => {
    if (status === 'PAYLOAD_OK' || status === 'PING_OK') return 'text-success';
    if (status.includes('ERROR') || status.includes('FAIL')) return 'text-danger';
    return 'text-text-muted';
  };

  return (
    <div class="bento-card col-span-12 desktop:col-span-8">
      <div class="flex items-center justify-between mb-3">
        <div class="flex items-center gap-2">
          <IconTerminal2 size={18} class="text-primary" />
          <h3 class="text-sm font-bold text-text-primary uppercase tracking-wider">
            Live Ingestion Stream
          </h3>
        </div>
        <div class="flex items-center gap-2">
          <span
            class={`w-2 h-2 rounded-full ${
              connected() ? 'bg-success shadow-[0_0_8px_var(--color-success)]' : 'bg-warning'
            }`}
          />
          <span class="text-[0.7rem] text-text-muted font-medium">
            {connected() ? 'SSE Connected' : 'Reconnecting...'}
          </span>
        </div>
      </div>

      <div class="bg-black/40 border border-border-default rounded-[10px] p-4 font-mono text-xs max-h-80 min-h-[180px] overflow-y-auto flex flex-col gap-1.5 selection:bg-primary/30">
        <Show
          when={events().length > 0}
          fallback={
            <div class="text-text-dim py-8 text-center flex flex-col items-center justify-center gap-2">
              <IconBroadcast size={24} class="animate-pulse text-text-muted/60" />
              <span>Awaiting live webhook ingestion events from connected companions...</span>
            </div>
          }
        >
          <For each={events()}>
            {(ev) => (
              <div class="leading-relaxed break-all flex items-start gap-2 border-b border-white/5 pb-1 last:border-b-0">
                <span class="text-text-dim shrink-0">[{formatTime(ev.timestamp)}]</span>
                <span class={`font-semibold shrink-0 ${getEventClass(ev.status)}`}>
                  {ev.status}:
                </span>
                <span class="text-text-primary">
                  dev=<span class="text-accent-cyan">{ev.device_name || ev.device_id}</span>
                </span>
                <span class="text-text-muted">src={ev.source || 'default'}</span>
                <span class="text-text-dim">
                  sha={ev.sha256 ? ev.sha256.slice(0, 8) : 'none'}
                </span>
                <span class="text-text-primary">recs={ev.records}</span>
                <Show when={ev.tombstones > 0}>
                  <span class="text-warning">del={ev.tombstones}</span>
                </Show>
                <span class="text-text-dim ml-auto shrink-0">({ev.latency_ms}ms)</span>
              </div>
            )}
          </For>
        </Show>
      </div>
    </div>
  );
}
