import { createSignal, onMount, For, Show } from 'solid-js';
import { A, useNavigate } from '@solidjs/router';
import {
  IconRouter,
  IconRefresh,
  IconShieldLock,
  IconTrash,
  IconQrcode,
  IconInfoCircle,
  IconPlus,
} from '@tabler/icons-solidjs';
import { ApiClient } from '../api/client';
import { IngestionStream } from '../components/IngestionStream';
import { PulseDot } from '../components/PulseDot';
import { showToast } from '../components/Toast';
import { formatDateTime } from '../lib/dates';

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

  const loadDevices = async () => {
    try {
      const data = await ApiClient.get<DeviceItem[]>('/api/v1/devices');
      setDevices(data || []);
    } catch {
      showToast('Failed to load devices', 'error');
    } finally {
      setLoading(false);
    }
  };

  onMount(loadDevices);

  const handleRevoke = async (id: number) => {
    if (!confirm('Are you sure you want to revoke this device? It will no longer be able to submit data.')) return;
    try {
      await ApiClient.post(`/api/v1/devices/${id}/revoke`);
      showToast('Device revoked successfully', 'success');
      await loadDevices();
    } catch (err: unknown) {
      showToast(err instanceof Error ? err.message : 'Failed to revoke device', 'error');
    }
  };

  const handleDelete = async (id: number) => {
    if (!confirm('Are you sure you want to permanently delete this device and all its uploaded records?')) return;
    try {
      await ApiClient.delete(`/api/v1/devices/${id}`);
      showToast('Device deleted successfully', 'success');
      await loadDevices();
    } catch (err: unknown) {
      showToast(err instanceof Error ? err.message : 'Failed to delete device', 'error');
    }
  };

  const handleRegenerate = async (id: number) => {
    if (!confirm('Regenerating credentials will invalidate current device settings. Continue?')) return;
    try {
      const creds = await ApiClient.post<any>(`/api/v1/devices/${id}/regenerate`);
      showToast('Credentials regenerated', 'success');
      navigate('/devices/new', { state: { credentials: creds } });
    } catch (err: unknown) {
      showToast(err instanceof Error ? err.message : 'Failed to regenerate credentials', 'error');
    }
  };

  const totalRegistered = () => devices().length;
  const activeCount = () => devices().filter((d) => d.status === 'active').length;
  const revokedCount = () => devices().filter((d) => d.status === 'revoked').length;

  return (
    <div class="flex flex-col gap-6">
      {/* Fleet Metrics Banner */}
      <div class="bento-card flex flex-wrap items-center justify-between gap-4">
        <div class="flex items-center gap-4">
          <div class="w-10 h-10 rounded-md bg-accent-cyan/15 text-accent-cyan flex items-center justify-center">
            <IconRouter size={22} />
          </div>
          <div>
            <h2 class="text-base font-bold text-text-primary">Companion Fleet</h2>
            <div class="text-xs text-text-muted mt-0.5 flex items-center gap-2">
              <span>Total Registered: <strong class="text-text-primary">{totalRegistered()}</strong></span>
              <span>&bull;</span>
              <span>Active: <strong class="text-success">{activeCount()}</strong></span>
              <span>&bull;</span>
              <span>Revoked: <strong class="text-danger">{revokedCount()}</strong></span>
            </div>
          </div>
        </div>

        <A
          href="/devices/new"
          class="bg-primary hover:bg-primary-hover text-white px-3.5 py-2 rounded-sm font-semibold text-xs transition-all flex items-center gap-1.5 no-underline shadow-sm"
        >
          <IconPlus size={16} />
          <span>Enroll New Device</span>
        </A>
      </div>

      {/* Device Tiles Grid */}
      <div class="bento-card">
        <div class="flex items-center justify-between mb-4">
          <div>
            <h3 class="text-base font-bold text-text-primary">Registered Companion Nodes</h3>
            <p class="text-xs text-text-muted mt-0.5">Android background workers & sync tokens</p>
          </div>
        </div>

        <Show
          when={devices().length > 0}
          fallback={
            <div class="text-text-muted text-xs py-12 text-center flex flex-col items-center justify-center gap-3">
              <IconRouter size={32} class="text-text-dim" />
              <span>No companion devices registered yet.</span>
              <A
                href="/devices/new"
                class="text-primary-light hover:text-primary font-semibold no-underline"
              >
                Enroll your first device &rarr;
              </A>
            </div>
          }
        >
          <div class="grid grid-cols-1 tablet:grid-cols-2 desktop:grid-cols-3 gap-4">
            <For each={devices()}>
              {(d) => (
                <div class="bg-bg-card-elevated border border-border-default rounded-md p-4 flex flex-col justify-between transition-all hover:border-border-highlight">
                  <div>
                    {/* Header */}
                    <div class="flex items-start justify-between gap-2 mb-2">
                      <div>
                        <h4 class="font-bold text-text-primary text-sm">
                          {d.display_name || d.device_model || d.label}
                        </h4>
                        <div class="text-[0.7rem] font-mono text-primary-light">{d.label}</div>
                      </div>
                      <span
                        class={`rounded-full px-2 py-0.5 text-[0.68rem] font-semibold ${
                          d.status === 'active'
                            ? 'bg-success-bg text-success'
                            : 'bg-danger-bg text-danger'
                        }`}
                      >
                        {d.status === 'active' ? 'Active' : 'Revoked'}
                      </span>
                    </div>

                    {/* Metadata Specs */}
                    <div class="text-xs text-text-muted space-y-1.5 my-3 bg-white/5 rounded p-2.5">
                      <div class="flex justify-between">
                        <span class="text-text-dim">Model:</span>
                        <span class="text-text-primary">{d.device_model || '—'}</span>
                      </div>
                      <div class="flex justify-between">
                        <span class="text-text-dim">Agent:</span>
                        <span class="text-text-primary">{d.collector_version || 'v1.0.0'}</span>
                      </div>
                      <div class="flex justify-between">
                        <span class="text-text-dim">Protocol:</span>
                        <code class="text-[0.7rem] text-accent-cyan">{d.last_sync_source || 'periodic_worker'}</code>
                      </div>
                      <div class="flex justify-between">
                        <span class="text-text-dim">Battery:</span>
                        <span class="text-text-dim">—</span>
                      </div>
                    </div>

                    {/* Last Sync */}
                    <div class="text-xs flex items-center justify-between text-text-muted mb-3">
                      <span class="text-text-dim">Last Sync:</span>
                      <Show
                        when={d.last_sync_at}
                        fallback={<span class="text-text-dim">Never</span>}
                      >
                        <span class="inline-flex items-center gap-1.5 text-text-primary font-medium text-[0.72rem]">
                          <PulseDot />
                          <span>{formatDateTime(d.last_sync_at!)}</span>
                        </span>
                      </Show>
                    </div>
                  </div>

                  {/* Actions */}
                  <div class="flex items-center justify-end gap-1 pt-3 border-t border-border-default/60">
                    <button
                      type="button"
                      onClick={() => handleRegenerate(d.id)}
                      title="Regenerate credentials"
                      class="p-1.5 rounded text-text-muted hover:text-text-primary hover:bg-white/5 transition-colors cursor-pointer"
                    >
                      <IconRefresh size={16} />
                    </button>
                    <Show when={d.status === 'active'}>
                      <button
                        type="button"
                        onClick={() => handleRevoke(d.id)}
                        title="Revoke device access"
                        class="p-1.5 rounded text-text-muted hover:text-warning hover:bg-warning-bg transition-colors cursor-pointer"
                      >
                        <IconShieldLock size={16} />
                      </button>
                    </Show>
                    <button
                      type="button"
                      onClick={() => handleDelete(d.id)}
                      title="Delete device and records"
                      class="p-1.5 rounded text-text-muted hover:text-danger hover:bg-danger-bg transition-colors cursor-pointer"
                    >
                      <IconTrash size={16} />
                    </button>
                  </div>
                </div>
              )}
            </For>
          </div>
        </Show>
      </div>

      {/* Live Ingestion Stream & Companion Setup Callout */}
      <div class="bento-grid">
        {/* SSE Live Ingestion Stream */}
        <IngestionStream />

        {/* Companion Setup Callout (col-4) */}
        <div class="bento-card col-span-12 desktop:col-span-4 flex flex-col justify-between">
          <div>
            <div class="flex items-center gap-2 mb-3">
              <IconInfoCircle size={20} class="text-primary" />
              <h3 class="text-sm font-bold text-text-primary uppercase tracking-wider">
                Companion Enrollment
              </h3>
            </div>
            <p class="text-xs text-text-muted leading-relaxed mb-4">
              To pair an Android device, scan the secure QR code using the Life Dashboard Companion app.
              The collector will negotiate a dedicated device token and HMAC signing secret.
            </p>

            <div class="bg-white/5 rounded-md p-3 border border-border-default text-xs space-y-2 mb-4">
              <div class="flex items-start gap-2">
                <span class="w-4 h-4 rounded-full bg-primary/20 text-primary-light flex items-center justify-center text-[0.65rem] font-bold shrink-0 mt-0.5">1</span>
                <span class="text-text-muted">Generate enrollment QR token</span>
              </div>
              <div class="flex items-start gap-2">
                <span class="w-4 h-4 rounded-full bg-primary/20 text-primary-light flex items-center justify-center text-[0.65rem] font-bold shrink-0 mt-0.5">2</span>
                <span class="text-text-muted">Scan with Companion app camera</span>
              </div>
              <div class="flex items-start gap-2">
                <span class="w-4 h-4 rounded-full bg-primary/20 text-primary-light flex items-center justify-center text-[0.65rem] font-bold shrink-0 mt-0.5">3</span>
                <span class="text-text-muted">Automated background sync commences</span>
              </div>
            </div>
          </div>

          <A
            href="/devices/new"
            class="w-full bg-primary hover:bg-primary-hover text-white py-2 rounded-sm font-semibold text-xs transition-all flex items-center justify-center gap-2 no-underline shadow-sm"
          >
            <IconQrcode size={16} />
            <span>Open Enrollment QR</span>
          </A>
        </div>
      </div>
    </div>
  );
}
