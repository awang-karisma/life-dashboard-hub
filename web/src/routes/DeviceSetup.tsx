import { createSignal, onMount, onCleanup, Show } from 'solid-js';
import { A, useLocation } from '@solidjs/router';
import { ApiClient } from '../api/client';
import { QRCode } from '../components/QRCode';

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
  const [error, setError] = createSignal('');
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
          clearInterval(pollInterval);
        }
      } catch {
        // Ignore polling errors
      }
    }, 5000);
  };

  const handleCreate = async (e: Event) => {
    e.preventDefault();
    setError('');
    setLoading(true);

    try {
      const resp = await ApiClient.post<DeviceCredentials>('/api/v1/devices', {
        label: label(),
        display_name: displayName(),
      });
      setCredentials(resp);
      startPolling(resp.id);
    } catch (err: unknown) {
      if (err instanceof Error) {
        setError(err.message);
      } else {
        setError('Failed to create device');
      }
    } finally {
      setLoading(false);
    }
  };

  const copyToClipboard = (text: string) => {
    navigator.clipboard.writeText(text);
    alert('Copied to clipboard!');
  };

  return (
    <div style={{ 'max-width': '720px', margin: '0 auto' }}>
      <div style={{ 'margin-bottom': '1.5rem' }}>
        <A href="/devices" style={{ 'font-size': '0.875rem' }}>← Back to Devices</A>
        <h1 style={{ 'font-size': '1.75rem', 'font-weight': 700, 'margin-top': '0.5rem' }}>Pair New Device</h1>
        <p style={{ color: 'var(--text-muted)', 'font-size': '0.875rem' }}>
          Connect Life Dashboard Companion on your Android phone or tablet
        </p>
      </div>

      <Show when={error()}>
        <div style={{ background: '#fee2e2', color: '#b91c1c', padding: '0.75rem', 'border-radius': '0.5rem', 'margin-bottom': '1rem' }}>
          {error()}
        </div>
      </Show>

      <Show
        when={credentials()}
        fallback={
          <div class="card">
            <h2 style={{ 'font-size': '1.125rem', 'font-weight': 600, 'margin-bottom': '1rem' }}>Device Details</h2>
            <form onSubmit={handleCreate} style={{ display: 'flex', 'flex-direction': 'column', gap: '1rem' }}>
              <div>
                <label style={{ display: 'block', 'font-size': '0.875rem', 'font-weight': 500, 'margin-bottom': '0.35rem' }}>
                  Device Name
                </label>
                <input
                  type="text"
                  required
                  class="input"
                  placeholder="e.g. My Pixel 8"
                  value={displayName()}
                  onInput={(e) => {
                    setDisplayName(e.currentTarget.value);
                    if (!label()) {
                      setLabel(e.currentTarget.value.toLowerCase().replace(/[^a-z0-9_-]/g, '-').slice(0, 32));
                    }
                  }}
                />
              </div>

              <div>
                <label style={{ display: 'block', 'font-size': '0.875rem', 'font-weight': 500, 'margin-bottom': '0.35rem' }}>
                  Label (unique URL identifier)
                </label>
                <input
                  type="text"
                  required
                  class="input"
                  placeholder="e.g. pixel-8"
                  value={label()}
                  onInput={(e) => setLabel(e.currentTarget.value.toLowerCase().replace(/[^a-z0-9_-]/g, '-'))}
                />
                <span style={{ 'font-size': '0.75rem', color: 'var(--text-muted)' }}>
                  Only lowercase letters, numbers, hyphens, and underscores (max 32 chars).
                </span>
              </div>

              <button type="submit" disabled={loading()} class="btn btn-primary" style={{ 'margin-top': '0.5rem' }}>
                {loading() ? 'Generating credentials...' : 'Generate Pairing QR Code'}
              </button>
            </form>
          </div>
        }
      >
        <div class="card" style={{ 'text-align': 'center' }}>
          <Show when={isSynced()}>
            <div style={{ background: '#dcfce7', color: '#15803d', padding: '1rem', 'border-radius': '0.5rem', 'margin-bottom': '1.5rem', 'font-weight': 600 }}>
              🎉 Connection verified! Companion app sent initial payload successfully.
            </div>
          </Show>

          <h2 style={{ 'font-size': '1.25rem', 'font-weight': 600 }}>Scan QR Code with Companion App</h2>
          <p style={{ color: 'var(--text-muted)', 'font-size': '0.875rem', 'margin': '0.5rem 0 1.5rem 0' }}>
            Open <strong>Life Dashboard Companion</strong> on Android → go to <strong>Settings</strong> → tap <strong>Scan QR Code</strong>
          </p>

          <div style={{ 'margin-bottom': '1.5rem' }}>
            <QRCode text={credentials()!.pairing_qr} size={260} />
          </div>

          <div style={{ 'margin-bottom': '1.5rem', display: 'flex', 'align-items': 'center', 'justify-content': 'center', gap: '0.5rem' }}>
            <span class={`badge ${isSynced() ? 'badge-green' : 'badge-yellow'}`}>
              {isSynced() ? 'Connected' : 'Waiting for initial sync...'}
            </span>
            <button onClick={() => copyToClipboard(credentials()!.pairing_qr)} class="btn btn-secondary" style={{ 'font-size': '0.75rem', padding: '0.25rem 0.5rem' }}>
              📋 Copy Pairing URI
            </button>
          </div>

          <div style={{ 'text-align': 'left', background: '#f8fafc', padding: '1rem', 'border-radius': '0.5rem', 'border': '1px solid var(--border)' }}>
            <div style={{ 'font-weight': 600, 'font-size': '0.875rem', 'margin-bottom': '0.5rem' }}>
              Manual Configuration (if QR scanner is unavailable):
            </div>
            <div style={{ 'margin-bottom': '0.5rem' }}>
              <div style={{ 'font-size': '0.75rem', color: 'var(--text-muted)' }}>Webhook URL:</div>
              <div style={{ display: 'flex', gap: '0.5rem', 'margin-top': '0.25rem' }}>
                <input readonly class="input" style={{ 'font-family': 'monospace', 'font-size': '0.8rem' }} value={credentials()!.webhook_url} />
                <button onClick={() => copyToClipboard(credentials()!.webhook_url)} class="btn btn-secondary" style={{ 'font-size': '0.75rem' }}>Copy</button>
              </div>
            </div>
            <div>
              <div style={{ 'font-size': '0.75rem', color: 'var(--text-muted)' }}>HMAC Secret:</div>
              <div style={{ display: 'flex', gap: '0.5rem', 'margin-top': '0.25rem' }}>
                <input readonly class="input" style={{ 'font-family': 'monospace', 'font-size': '0.8rem' }} value={credentials()!.webhook_secret} />
                <button onClick={() => copyToClipboard(credentials()!.webhook_secret)} class="btn btn-secondary" style={{ 'font-size': '0.75rem' }}>Copy</button>
              </div>
            </div>
          </div>

          <div style={{ 'margin-top': '1.5rem' }}>
            <A href="/devices" class="btn btn-primary">
              Done & Return to Devices
            </A>
          </div>
        </div>
      </Show>
    </div>
  );
}
