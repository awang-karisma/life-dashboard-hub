import { createSignal, onMount, onCleanup, Show } from 'solid-js';
import { A, useLocation } from '@solidjs/router';
import {
  IconArrowLeft,
  IconQrcode,
  IconCopy,
  IconCheck,
} from '@tabler/icons-solidjs';
import { ApiClient } from '../api/client';
import { QRCode } from '../components/QRCode';
import { PulseDot } from '../components/PulseDot';
import { showToast } from '../components/Toast';

interface DeviceCredentials {
  id: number;
  label: string;
  display_name: string;
  webhook_url: string;
  webhook_secret: string;
  pairing_qr: string;
}

export function DeviceSetup() {
  const location = useLocation();
  const [label, setLabel] = createSignal('');
  const [displayName, setDisplayName] = createSignal('');
  const [credentials, setCredentials] = createSignal<DeviceCredentials | null>(null);
  const [isSynced, setIsSynced] = createSignal(false);
  const [loading, setLoading] = createSignal(false);
  let pollInterval: number | undefined;

  onMount(() => {
    const state = location.state;
    if (state && typeof state === 'object' && 'credentials' in state) {
      const raw = state.credentials;
      if (raw && typeof raw === 'object' && 'id' in raw && typeof raw.id === 'number') {
        const creds = raw as DeviceCredentials;
        setCredentials(creds);
        startPolling(creds.id);
      }
    }
  });

  onCleanup(() => {
    clearInterval(pollInterval);
  });

  const startPolling = (deviceId: number) => {
    clearInterval(pollInterval);
    pollInterval = window.setInterval(async () => {
      try {
        const d = await ApiClient.get<{ last_sync_at?: string }>(`/api/v1/devices/${deviceId}`);
        if (d && d.last_sync_at) {
          setIsSynced(true);
          showToast('Companion app connected successfully!', 'success');
          clearInterval(pollInterval);
        }
      } catch {
        // Ignore polling errors
      }
    }, 5000);
  };

  const handleCreate = async (e: Event) => {
    e.preventDefault();
    setLoading(true);

    try {
      const resp = await ApiClient.post<DeviceCredentials>('/api/v1/devices', {
        label: label(),
        display_name: displayName(),
      });
      setCredentials(resp);
      showToast('Enrollment credentials generated', 'success');
      startPolling(resp.id);
    } catch (err: unknown) {
      showToast(err instanceof Error ? err.message : 'Failed to create device', 'error');
    } finally {
      setLoading(false);
    }
  };

  const copyToClipboard = (text: string) => {
    navigator.clipboard.writeText(text);
    showToast('Copied to clipboard', 'success');
  };

  return (
    <div class="max-w-[720px] mx-auto flex flex-col gap-5">
      <div>
        <A
          href="/devices"
          class="text-xs text-text-muted hover:text-text-primary flex items-center gap-1.5 no-underline transition-colors mb-2"
        >
          <IconArrowLeft size={16} />
          <span>Back to Devices</span>
        </A>
        <h1 class="text-2xl font-extrabold text-text-primary tracking-tight">Pair New Device</h1>
        <p class="text-xs text-text-muted mt-1">
          Connect Life Dashboard Companion on your Android phone or tablet
        </p>
      </div>

      <Show
        when={credentials()}
        fallback={
          <div class="bento-card">
            <h2 class="text-base font-bold text-text-primary mb-4">Device Details</h2>
            <form onSubmit={handleCreate} class="flex flex-col gap-4">
              <div>
                <label class="block text-xs font-semibold text-text-muted mb-1">
                  Device Name
                </label>
                <input
                  type="text"
                  required
                  placeholder="e.g. My Pixel 8"
                  value={displayName()}
                  onInput={(e) => {
                    setDisplayName(e.currentTarget.value);
                    if (!label()) {
                      setLabel(
                        e.currentTarget.value
                          .toLowerCase()
                          .replace(/[^a-z0-9_-]/g, '-')
                          .slice(0, 32)
                      );
                    }
                  }}
                  class="w-full bg-bg-card border border-border-default rounded-sm px-3 py-2 text-sm text-text-primary focus:border-border-focus focus:outline-none"
                />
              </div>

              <div>
                <label class="block text-xs font-semibold text-text-muted mb-1">
                  Label (unique URL identifier)
                </label>
                <input
                  type="text"
                  required
                  placeholder="e.g. pixel-8"
                  value={label()}
                  onInput={(e) =>
                    setLabel(e.currentTarget.value.toLowerCase().replace(/[^a-z0-9_-]/g, '-'))
                  }
                  class="w-full bg-bg-card border border-border-default rounded-sm px-3 py-2 text-sm text-text-primary focus:border-border-focus focus:outline-none font-mono"
                />
                <span class="text-[0.7rem] text-text-dim mt-1 block">
                  Only lowercase letters, numbers, hyphens, and underscores (max 32 chars).
                </span>
              </div>

              <button
                type="submit"
                disabled={loading()}
                class="bg-primary hover:bg-primary-hover text-white py-2.5 rounded-sm font-semibold text-xs transition-all flex items-center justify-center gap-2 cursor-pointer shadow-sm mt-2 disabled:opacity-50"
              >
                <IconQrcode size={16} />
                <span>{loading() ? 'Generating credentials...' : 'Generate Pairing QR Code'}</span>
              </button>
            </form>
          </div>
        }
      >
        <div class="bento-card text-center flex flex-col items-center">
          <Show when={isSynced()}>
            <div class="w-full bg-success-bg border border-success/30 text-success p-3 rounded-md mb-5 font-semibold text-xs flex items-center justify-center gap-2">
              <IconCheck size={18} />
              <span>Connection verified! Companion app sent initial payload successfully.</span>
            </div>
          </Show>

          <h2 class="text-lg font-bold text-text-primary mb-1">
            Scan QR Code with Companion App
          </h2>
          <p class="text-xs text-text-muted max-w-md mx-auto mb-5">
            Open <strong>Life Dashboard Companion</strong> on Android &rarr; go to{' '}
            <strong>Settings</strong> &rarr; tap <strong>Scan QR Code</strong>
          </p>

          <div class="p-4 bg-white rounded-lg shadow-inner mb-5 inline-block">
            <QRCode text={credentials()!.pairing_qr} size={240} />
          </div>

          <div class="flex items-center justify-center gap-3 mb-6">
            <span
              class={`rounded-full px-3 py-1 text-xs font-semibold inline-flex items-center gap-1.5 ${
                isSynced()
                  ? 'bg-success-bg text-success'
                  : 'bg-warning-bg text-warning'
              }`}
            >
              <PulseDot color={isSynced() ? 'var(--color-success)' : 'var(--color-warning)'} />
              <span>{isSynced() ? 'Connected' : 'Waiting for initial sync...'}</span>
            </span>

            <button
              type="button"
              onClick={() => copyToClipboard(credentials()!.pairing_qr)}
              class="px-3 py-1 rounded-sm bg-white/5 hover:bg-white/10 text-text-primary text-xs font-semibold border border-border-default transition-all flex items-center gap-1.5 cursor-pointer"
            >
              <IconCopy size={14} />
              <span>Copy Pairing URI</span>
            </button>
          </div>

          <div class="w-full text-left bg-black/25 p-4 rounded-md border border-border-default text-xs space-y-3">
            <div class="font-bold text-text-primary">
              Manual Configuration (if QR scanner is unavailable):
            </div>
            <div>
              <div class="text-[0.7rem] text-text-dim mb-1">Webhook URL:</div>
              <div class="flex gap-2">
                <input
                  readonly
                  class="w-full bg-bg-card border border-border-default rounded-sm px-2.5 py-1 font-mono text-[0.75rem] text-text-primary"
                  value={credentials()!.webhook_url}
                />
                <button
                  type="button"
                  onClick={() => copyToClipboard(credentials()!.webhook_url)}
                  class="px-2.5 py-1 bg-white/10 hover:bg-white/15 text-text-primary rounded-sm shrink-0 cursor-pointer text-xs"
                >
                  Copy
                </button>
              </div>
            </div>

            <div>
              <div class="text-[0.7rem] text-text-dim mb-1">HMAC Secret:</div>
              <div class="flex gap-2">
                <input
                  readonly
                  class="w-full bg-bg-card border border-border-default rounded-sm px-2.5 py-1 font-mono text-[0.75rem] text-text-primary"
                  value={credentials()!.webhook_secret}
                />
                <button
                  type="button"
                  onClick={() => copyToClipboard(credentials()!.webhook_secret)}
                  class="px-2.5 py-1 bg-white/10 hover:bg-white/15 text-text-primary rounded-sm shrink-0 cursor-pointer text-xs"
                >
                  Copy
                </button>
              </div>
            </div>
          </div>

          <div class="mt-6 w-full">
            <A
              href="/devices"
              class="block w-full bg-primary hover:bg-primary-hover text-white py-2.5 rounded-sm font-semibold text-xs text-center no-underline shadow-sm transition-all"
            >
              Done & Return to Devices
            </A>
          </div>
        </div>
      </Show>
    </div>
  );
}
