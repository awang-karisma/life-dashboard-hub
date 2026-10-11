import { createSignal, onMount, For, Show } from 'solid-js';
import { A, useNavigate } from '@solidjs/router';
import { ApiClient } from '../api/client';
import { SyncStatus } from '../components/SyncStatus';

interface DeviceItem {
  id: number;
  label: string;
  display_name: string;
  device_model: string;
  collector_version: string;
  status: string;
  last_sync_at: string | null;
  last_sync_source: string;
  last_sync_error: string;
  created_at: string;
}

export function Devices() {
  const navigate = useNavigate();
  const [devices, setDevices] = createSignal<DeviceItem[]>([]);
  const [loading, setLoading] = createSignal(true);
  const [error, setError] = createSignal('');

  const loadDevices = async () => {
    try {
      const data = await ApiClient.get<DeviceItem[]>('/api/v1/devices');
      setDevices(data || []);
    } catch (err: unknown) {
      if (err instanceof Error) {
        setError(err.message);
      }
    } finally {
      setLoading(false);
    }
  };

  onMount(loadDevices);

  const handleRevoke = async (id: number) => {
    if (!confirm('Are you sure you want to revoke this device? It will no longer be able to submit data.')) return;
    try {
      await ApiClient.post(`/api/v1/devices/${id}/revoke`);
      await loadDevices();
    } catch (err: unknown) {
      alert(err instanceof Error ? err.message : 'Failed to revoke device');
    }
  };

  const handleDelete = async (id: number) => {
    if (!confirm('Are you sure you want to permanently delete this device and all its uploaded records?')) return;
    try {
      await ApiClient.delete(`/api/v1/devices/${id}`);
      await loadDevices();
    } catch (err: unknown) {
      alert(err instanceof Error ? err.message : 'Failed to delete device');
    }
  };

  const handleRegenerate = async (id: number) => {
    if (!confirm('Regenerating credentials will invalidate current device settings. Continue?')) return;
    try {
      const creds = await ApiClient.post<any>(`/api/v1/devices/${id}/regenerate`);
      // Navigate to setup view with regenerated credentials
      navigate('/devices/new', { state: { credentials: creds } });
    } catch (err: unknown) {
      alert(err instanceof Error ? err.message : 'Failed to regenerate credentials');
    }
  };

  return (
    <div>
      <div style={{ 'margin-bottom': '1.5rem', display: 'flex', 'justify-content': 'space-between', 'align-items': 'center', 'flex-wrap': 'wrap', gap: '1rem' }}>
        <div>
          <h1 style={{ 'font-size': '1.75rem', 'font-weight': 700 }}>Registered Devices</h1>
          <p style={{ color: 'var(--text-muted)', 'font-size': '0.875rem' }}>
            Manage companion app collectors sending data to your hub
          </p>
        </div>
        <A href="/devices/new" class="btn btn-primary">
          + Add New Device
        </A>
      </div>

      <Show when={error()}>
        <div style={{ background: '#fee2e2', color: '#b91c1c', padding: '0.75rem', 'border-radius': '0.5rem', 'margin-bottom': '1rem' }}>
          {error()}
        </div>
      </Show>

      <div class="card">
        <Show when={devices().length > 0} fallback={<div style={{ padding: '2rem', 'text-align': 'center', color: 'var(--text-muted)' }}>No devices registered yet.</div>}>
          <table class="table">
            <thead>
              <tr>
                <th>Device Name</th>
                <th>Label</th>
                <th>Model</th>
                <th>App Version</th>
                <th>Status</th>
                <th>Last Sync</th>
                <th style={{ 'text-align': 'right' }}>Actions</th>
              </tr>
            </thead>
            <tbody>
              <For each={devices()}>
                {(d) => (
                  <tr>
                    <td style={{ 'font-weight': 600 }}>{d.display_name}</td>
                    <td><code>{d.label}</code></td>
                    <td style={{ color: 'var(--text-muted)' }}>{d.device_model || '—'}</td>
                    <td style={{ color: 'var(--text-muted)' }}>{d.collector_version || '—'}</td>
                    <td>
                      <span class={`badge ${d.status === 'active' ? 'badge-green' : 'badge-red'}`}>
                        {d.status}
                      </span>
                    </td>
                    <td>
                      <SyncStatus lastSyncAt={d.last_sync_at} status={d.status} />
                    </td>
                    <td style={{ 'text-align': 'right' }}>
                      <div style={{ display: 'inline-flex', gap: '0.5rem' }}>
                        <button onClick={() => handleRegenerate(d.id)} class="btn btn-secondary" style={{ padding: '0.25rem 0.5rem', 'font-size': '0.75rem' }}>
                          Re-pair QR
                        </button>
                        {d.status === 'active' && (
                          <button onClick={() => handleRevoke(d.id)} class="btn btn-secondary" style={{ padding: '0.25rem 0.5rem', 'font-size': '0.75rem', color: '#b91c1c' }}>
                            Revoke
                          </button>
                        )}
                        <button onClick={() => handleDelete(d.id)} class="btn btn-danger" style={{ padding: '0.25rem 0.5rem', 'font-size': '0.75rem' }}>
                          Delete
                        </button>
                      </div>
                    </td>
                  </tr>
                )}
              </For>
            </tbody>
          </table>
        </Show>
      </div>
    </div>
  );
}
