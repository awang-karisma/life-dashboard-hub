import { createSignal, onMount, For, Show } from 'solid-js';
import { ApiClient } from '../api/client';
import { Chart } from '../components/Chart';
import { AppUsageBar } from '../components/AppUsageBar';
import { formatMinutes } from '../lib/format';
import { getDaysAgoISO, getTodayISO } from '../lib/dates';
import type { ChartConfiguration } from 'chart.js';

interface DailyReportResponse {
  from: string;
  to: string;
  days: {
    date: string;
    total_screen_time_minutes: number;
    filtered_screen_time_minutes: number;
    device_count: number;
  }[];
  device_usage: {
    device_id: number;
    device_model: string;
    date: string;
    total_screen_time_minutes: number;
  }[];
}

interface AppUsage {
  package_name: string;
  app_name: string;
  minutes: number;
}

interface DeviceOption {
  id: number;
  display_name: string;
}

export function ScreenTimeReport() {
  const [from, setFrom] = createSignal(getDaysAgoISO(6));
  const [to, setTo] = createSignal(getTodayISO());
  const [deviceId, setDeviceId] = createSignal('');
  const [devices, setDevices] = createSignal<DeviceOption[]>([]);
  const [dailyData, setDailyData] = createSignal<DailyReportResponse | null>(null);
  const [appsData, setAppsData] = createSignal<AppUsage[]>([]);
  const [loading, setLoading] = createSignal(false);

  const loadData = async () => {
    setLoading(true);
    try {
      let url = `/api/v1/reports/screen-time/daily?from=${from()}&to=${to()}`;
      if (deviceId()) url += `&device_id=${deviceId()}`;

      let appsUrl = `/api/v1/reports/screen-time/apps?from=${from()}&to=${to()}&limit=20`;
      if (deviceId()) appsUrl += `&device_id=${deviceId()}`;

      const [daily, apps] = await Promise.all([
        ApiClient.get<DailyReportResponse>(url),
        ApiClient.get<AppUsage[]>(appsUrl),
      ]);

      setDailyData(daily);
      setAppsData(apps || []);
    } catch (err) {
      console.error('Failed to load screen time report:', err);
    } finally {
      setLoading(false);
    }
  };

  onMount(async () => {
    try {
      const devList = await ApiClient.get<DeviceOption[]>('/api/v1/devices');
      setDevices(devList || []);
    } catch (err) {
      console.error(err);
    }
    loadData();
  });

  const chartConfig = (): ChartConfiguration => {
    const data = dailyData();
    if (!data || !data.days) return { type: 'bar', data: { labels: [], datasets: [] } };

    const labels = data.days.map((d) => d.date.slice(5));
    const hours = data.days.map((d) => Math.round((d.total_screen_time_minutes / 60) * 10) / 10);

    return {
      type: 'bar',
      data: {
        labels,
        datasets: [
          {
            label: 'Total Screen Time (Hours)',
            data: hours,
            backgroundColor: '#3b82f6',
            borderRadius: 6,
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
            ticks: { callback: (val) => `${val}h` },
          },
        },
      },
    };
  };

  const maxAppMinutes = () => {
    const apps = appsData();
    if (!apps.length) return 1;
    return Math.max(...apps.map((a) => a.minutes), 1);
  };

  const totalPeriodMinutes = () => {
    const data = dailyData();
    if (!data || !data.days) return 0;
    return data.days.reduce((acc, curr) => acc + curr.total_screen_time_minutes, 0);
  };
  const handleRename = async (pkg: string, currentName: string) => {
    const newName = prompt(`Enter custom name for ${pkg}:`, currentName);
    if (!newName || newName.trim() === '' || newName === currentName) return;
    try {
      await ApiClient.post('/api/v1/reports/screen-time/aliases', {
        package_name: pkg,
        app_name: newName.trim(),
      });
      await loadData();
    } catch (err: unknown) {
      alert(err instanceof Error ? err.message : 'Failed to save app alias');
    }
  };


  return (
    <div>
      <div style={{ 'margin-bottom': '1.5rem', display: 'flex', 'justify-content': 'space-between', 'align-items': 'center', 'flex-wrap': 'wrap', gap: '1rem' }}>
        <div>
          <h1 style={{ 'font-size': '1.75rem', 'font-weight': 700 }}>Screen Time Report</h1>
          <p style={{ color: 'var(--text-muted)', 'font-size': '0.875rem' }}>
            Daily application usage and device screen time analysis
          </p>
        </div>

        <div style={{ display: 'flex', gap: '0.75rem', 'flex-wrap': 'wrap', 'align-items': 'center' }}>
          <div>
            <label style={{ 'font-size': '0.75rem', color: 'var(--text-muted)', display: 'block' }}>From</label>
            <input type="date" class="input" value={from()} onInput={(e) => setFrom(e.currentTarget.value)} style={{ padding: '0.4rem 0.6rem' }} />
          </div>
          <div>
            <label style={{ 'font-size': '0.75rem', color: 'var(--text-muted)', display: 'block' }}>To</label>
            <input type="date" class="input" value={to()} onInput={(e) => setTo(e.currentTarget.value)} style={{ padding: '0.4rem 0.6rem' }} />
          </div>
          <div>
            <label style={{ 'font-size': '0.75rem', color: 'var(--text-muted)', display: 'block' }}>Device</label>
            <select class="select" value={deviceId()} onChange={(e) => setDeviceId(e.currentTarget.value)} style={{ padding: '0.4rem 0.6rem' }}>
              <option value="">All Devices</option>
              <For each={devices()}>{(d) => <option value={d.id}>{d.display_name}</option>}</For>
            </select>
          </div>
          <button onClick={loadData} class="btn btn-primary" style={{ 'margin-top': '1.25rem', padding: '0.45rem 1rem' }}>
            Apply Filter
          </button>
        </div>
      </div>

      <div class="grid-cols-2" style={{ 'margin-bottom': '1.5rem' }}>
        <div class="card">
          <div style={{ color: 'var(--text-muted)', 'font-size': '0.875rem', 'font-weight': 500 }}>
            Total Period Usage
          </div>
          <div style={{ 'font-size': '2rem', 'font-weight': 700, color: 'var(--primary)', 'margin-top': '0.5rem' }}>
            {formatMinutes(totalPeriodMinutes())}
          </div>
          <div style={{ 'font-size': '0.75rem', color: 'var(--text-muted)', 'margin-top': '0.25rem' }}>
            Total reported device usage across selected date range
          </div>
        </div>

        <div class="card">
          <div style={{ color: 'var(--text-muted)', 'font-size': '0.875rem', 'font-weight': 500 }}>
            Daily Average
          </div>
          <div style={{ 'font-size': '2rem', 'font-weight': 700, 'margin-top': '0.5rem' }}>
            {formatMinutes(dailyData()?.days?.length ? Math.round(totalPeriodMinutes() / dailyData()!.days.length) : 0)}
          </div>
          <div style={{ 'font-size': '0.75rem', color: 'var(--text-muted)', 'margin-top': '0.25rem' }}>
            Average across {dailyData()?.days?.length || 0} active days
          </div>
        </div>
      </div>

      <div class="card" style={{ 'margin-bottom': '1.5rem' }}>
        <h2 style={{ 'font-size': '1.125rem', 'font-weight': 600, 'margin-bottom': '1rem' }}>Daily Usage Breakdown</h2>
        <Show when={dailyData() && dailyData()!.days.length > 0} fallback={<div style={{ padding: '2rem', 'text-align': 'center', color: 'var(--text-muted)' }}>No data for selected period.</div>}>
          <Chart config={chartConfig()} height={280} />
        </Show>
      </div>

      <div class="card">
        <h2 style={{ 'font-size': '1.125rem', 'font-weight': 600, 'margin-bottom': '1rem' }}>Top Applications</h2>
        <Show when={appsData().length > 0} fallback={<div style={{ padding: '2rem', 'text-align': 'center', color: 'var(--text-muted)' }}>No app usage recorded for selected range.</div>}>
          <For each={appsData()}>
            {(app) => (
              <AppUsageBar
                appName={app.app_name}
                packageName={app.package_name}
                minutes={app.minutes}
                maxMinutes={maxAppMinutes()}
                onRename={handleRename}
              />
            )}
          </For>
        </Show>
      </div>
    </div>
  );
}
