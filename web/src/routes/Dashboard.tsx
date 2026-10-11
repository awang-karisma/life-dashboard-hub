import { createSignal, onMount, For, Show } from 'solid-js';
import { A } from '@solidjs/router';
import { ApiClient } from '../api/client';
import { Chart } from '../components/Chart';
import { SyncStatus } from '../components/SyncStatus';
import { formatMinutes, formatSteps } from '../lib/format';
import { getDaysAgoISO, getTodayISO } from '../lib/dates';
import type { ChartConfiguration } from 'chart.js';

interface DailySummary {
  date: string;
  total_screen_time_minutes: number;
}

interface HealthSummary {
  date: string;
  steps: number;
  distance_meters: number;
  calories: number;
}

interface DeviceItem {
  id: number;
  label: string;
  display_name: string;
  device_model: string;
  collector_version: string;
  status: string;
  last_sync_at: string | null;
  last_sync_source: string;
  is_stale?: boolean;
}

export function Dashboard() {
  const [todayMinutes, setTodayMinutes] = createSignal(0);
  const [todaySteps, setTodaySteps] = createSignal(0);
  const [devices, setDevices] = createSignal<DeviceItem[]>([]);
  const [trendDays, setTrendDays] = createSignal<DailySummary[]>([]);
  const [loading, setLoading] = createSignal(true);

  onMount(async () => {
    try {
      const from = getDaysAgoISO(6);
      const to = getTodayISO();

      const [stResp, healthResp, syncResp] = await Promise.all([
        ApiClient.get<{ days: DailySummary[] }>(`/api/v1/reports/screen-time/daily?from=${from}&to=${to}`),
        ApiClient.get<HealthSummary[]>(`/api/v1/reports/health/daily?from=${from}&to=${to}`),
        ApiClient.get<{ devices: DeviceItem[] }>('/api/v1/reports/sync-status'),
      ]);

      if (stResp && stResp.days) {
        setTrendDays(stResp.days);
        const todayItem = stResp.days.find((d) => d.date === to);
        if (todayItem) {
          setTodayMinutes(todayItem.total_screen_time_minutes);
        }
      }

      if (healthResp && Array.isArray(healthResp)) {
        const todayHealth = healthResp.find((d) => d.date === to);
        if (todayHealth) {
          setTodaySteps(todayHealth.steps);
        }
      }

      if (syncResp && syncResp.devices) {
        setDevices(syncResp.devices);
      }
    } catch (err) {
      console.error('Failed to load dashboard data:', err);
    } finally {
      setLoading(false);
    }
  });

  const chartConfig = (): ChartConfiguration => {
    const days = trendDays();
    const labels = days.map((d) => d.date.slice(5)); // MM-DD
    const data = days.map((d) => Math.round((d.total_screen_time_minutes / 60) * 10) / 10); // hours

    return {
      type: 'bar',
      data: {
        labels,
        datasets: [
          {
            label: 'Screen Time (Hours)',
            data,
            backgroundColor: '#3b82f6',
            borderRadius: 4,
          },
        ],
      },
      options: {
        responsive: true,
        maintainAspectRatio: false,
        plugins: {
          legend: { display: false },
          tooltip: {
            callbacks: {
              label: (ctx) => `${ctx.raw} hours`,
            },
          },
        },
        scales: {
          y: {
            beginAtZero: true,
            ticks: {
              callback: (val) => `${val}h`,
            },
          },
        },
      },
    };
  };

  return (
    <div>
      <div style={{ 'margin-bottom': '1.5rem', display: 'flex', 'justify-content': 'space-between', 'align-items': 'center', 'flex-wrap': 'wrap', gap: '1rem' }}>
        <div>
          <h1 style={{ 'font-size': '1.75rem', 'font-weight': 700 }}>Overview</h1>
          <p style={{ color: 'var(--text-muted)', 'font-size': '0.875rem' }}>Today's activity and device synchronization</p>
        </div>
        <A href="/devices/new" class="btn btn-primary">
          + Add Device
        </A>
      </div>

      <div class="grid-cols-3" style={{ 'margin-bottom': '1.5rem' }}>
        <div class="card">
          <div style={{ color: 'var(--text-muted)', 'font-size': '0.875rem', 'font-weight': 500 }}>
            Today's Screen Time
          </div>
          <div style={{ 'font-size': '2rem', 'font-weight': 700, color: 'var(--primary)', 'margin-top': '0.5rem' }}>
            {formatMinutes(todayMinutes())}
          </div>
          <div style={{ 'font-size': '0.75rem', color: 'var(--text-muted)', 'margin-top': '0.25rem' }}>
            Reported total across all devices
          </div>
        </div>

        <div class="card">
          <div style={{ color: 'var(--text-muted)', 'font-size': '0.875rem', 'font-weight': 500 }}>
            Today's Steps
          </div>
          <div style={{ 'font-size': '2rem', 'font-weight': 700, color: '#10b981', 'margin-top': '0.5rem' }}>
            {formatSteps(todaySteps())}
          </div>
          <div style={{ 'font-size': '0.75rem', color: 'var(--text-muted)', 'margin-top': '0.25rem' }}>
            Health Connect aggregate
          </div>
        </div>

        <div class="card">
          <div style={{ color: 'var(--text-muted)', 'font-size': '0.875rem', 'font-weight': 500 }}>
            Active Devices
          </div>
          <div style={{ 'font-size': '2rem', 'font-weight': 700, 'margin-top': '0.5rem' }}>
            {devices().filter((d) => d.status === 'active').length}
          </div>
          <div style={{ 'font-size': '0.75rem', color: 'var(--text-muted)', 'margin-top': '0.25rem' }}>
            Connected companion collectors
          </div>
        </div>
      </div>

      <div class="card" style={{ 'margin-bottom': '1.5rem' }}>
        <div style={{ display: 'flex', 'justify-content': 'space-between', 'align-items': 'center', 'margin-bottom': '1rem' }}>
          <h2 style={{ 'font-size': '1.125rem', 'font-weight': 600 }}>7-Day Screen Time Trend</h2>
          <A href="/reports/screen-time" style={{ 'font-size': '0.875rem' }}>Detailed breakdown →</A>
        </div>
        <Show when={trendDays().length > 0} fallback={<div style={{ padding: '2rem', 'text-align': 'center', color: 'var(--text-muted)' }}>No usage data recorded yet.</div>}>
          <Chart config={chartConfig()} height={260} />
        </Show>
      </div>

      <div class="card">
        <div style={{ display: 'flex', 'justify-content': 'space-between', 'align-items': 'center', 'margin-bottom': '1rem' }}>
          <h2 style={{ 'font-size': '1.125rem', 'font-weight': 600 }}>Device Sync Status</h2>
          <A href="/devices" style={{ 'font-size': '0.875rem' }}>Manage devices →</A>
        </div>

        <Show when={devices().length > 0} fallback={<div style={{ padding: '2rem', 'text-align': 'center', color: 'var(--text-muted)' }}>No devices registered yet. Click "+ Add Device" above to pair one.</div>}>
          <table class="table">
            <thead>
              <tr>
                <th>Device</th>
                <th>Model</th>
                <th>Status</th>
                <th>Last Sync</th>
              </tr>
            </thead>
            <tbody>
              <For each={devices()}>
                {(d) => (
                  <tr>
                    <td style={{ 'font-weight': 600 }}>{d.display_name}</td>
                    <td style={{ color: 'var(--text-muted)' }}>{d.device_model || 'Unknown'}</td>
                    <td>
                      <span class={`badge ${d.status === 'active' ? 'badge-green' : 'badge-red'}`}>
                        {d.status}
                      </span>
                    </td>
                    <td>
                      <SyncStatus lastSyncAt={d.last_sync_at} status={d.status} />
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
