import { createSignal, onMount, For, Show } from 'solid-js';
import {
  IconClock,
  IconCalendarStats,
  IconEyeOff,
  IconApps,
  IconFileSpreadsheet,
  IconRefresh,
  IconSearch,
  IconEdit,
} from '@tabler/icons-solidjs';
import { ApiClient } from '../api/client';
import { Chart } from '../components/Chart';
import { showToast } from '../components/Toast';
import { formatMinutes } from '../lib/format';
import { getDaysAgoISO, getTodayISO } from '../lib/dates';
import type { ChartConfiguration } from 'chart.js';

interface DailyReportResponse {
  from: string;
  to: string;
  days: {
    date: string;
    total_screen_time_minutes: number;
    device_breakdown?: { [deviceId: string]: number };
  }[];
  device_usage?: {
    device_id: number;
    device_name: string;
    total_minutes: number;
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
  label?: string;
}

export function ScreenTimeReport() {
  const [from, setFrom] = createSignal(getDaysAgoISO(6));
  const [to, setTo] = createSignal(getTodayISO());
  const [deviceId, setDeviceId] = createSignal('');
  const [devices, setDevices] = createSignal<DeviceOption[]>([]);
  const [dailyData, setDailyData] = createSignal<DailyReportResponse | null>(null);
  const [appsData, setAppsData] = createSignal<AppUsage[]>([]);
  const [loading, setLoading] = createSignal(false);
  const [searchQuery, setSearchQuery] = createSignal('');
  const [hideSystemUI, setHideSystemUI] = createSignal(true);

  const loadData = async () => {
    setLoading(true);
    try {
      let url = `/api/v1/reports/screen-time/daily?from=${from()}&to=${to()}`;
      if (deviceId()) url += `&device_id=${deviceId()}`;

      let appsUrl = `/api/v1/reports/screen-time/apps?from=${from()}&to=${to()}&limit=50`;
      if (deviceId()) appsUrl += `&device_id=${deviceId()}`;

      const [daily, apps] = await Promise.all([
        ApiClient.get<DailyReportResponse>(url),
        ApiClient.get<AppUsage[]>(appsUrl),
      ]);

      setDailyData(daily);
      setAppsData(apps || []);
    } catch (err) {
      console.error('Failed to load screen time report:', err);
      showToast('Failed to load screen time data', 'error');
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
            backgroundColor: '#6366f1',
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
          x: {
            grid: { display: false },
            ticks: { color: '#94a3b8' },
          },
          y: {
            beginAtZero: true,
            grid: { color: 'rgba(255, 255, 255, 0.05)' },
            ticks: {
              color: '#94a3b8',
              callback: (val) => `${val}h`,
            },
          },
        },
      },
    };
  };

  const totalPeriodMinutes = () => {
    const data = dailyData();
    if (!data || !data.days) return 0;
    return data.days.reduce((acc, curr) => acc + curr.total_screen_time_minutes, 0);
  };

  const dailyAverageMinutes = () => {
    const data = dailyData();
    if (!data || !data.days || data.days.length === 0) return 0;
    return Math.round(totalPeriodMinutes() / data.days.length);
  };

  const filteredApps = () => {
    return appsData().filter((a) => {
      if (
        hideSystemUI() &&
        (a.package_name.startsWith('com.android.systemui') ||
          a.package_name.startsWith('com.google.android.apps.nexuslauncher'))
      ) {
        return false;
      }
      if (searchQuery()) {
        const q = searchQuery().toLowerCase();
        return (
          a.app_name.toLowerCase().includes(q) || a.package_name.toLowerCase().includes(q)
        );
      }
      return true;
    });
  };

  const filteredSystemCount = () => {
    return appsData().filter(
      (a) =>
        a.package_name.startsWith('com.android.systemui') ||
        a.package_name.startsWith('com.google.android.apps.nexuslauncher')
    ).length;
  };

  const categories = () => {
    const list = appsData();
    const totals: Record<string, number> = {
      'Work & Dev': 0,
      'Media & Entertainment': 0,
      'Learning': 0,
      'Communication': 0,
      'Utilities & Other': 0,
    };
    let grandTotal = 0;

    for (const a of list) {
      grandTotal += a.minutes;
      const pkg = a.package_name.toLowerCase();
      if (
        pkg.includes('slack') ||
        pkg.includes('microsoft') ||
        pkg.includes('github') ||
        pkg.includes('obsidian') ||
        pkg.includes('notion')
      ) {
        totals['Work & Dev'] += a.minutes;
      } else if (
        pkg.includes('youtube') ||
        pkg.includes('netflix') ||
        pkg.includes('spotify') ||
        pkg.includes('tiktok') ||
        pkg.includes('instagram')
      ) {
        totals['Media & Entertainment'] += a.minutes;
      } else if (pkg.includes('duolingo') || pkg.includes('wikipedia') || pkg.includes('anki')) {
        totals['Learning'] += a.minutes;
      } else if (
        pkg.includes('whatsapp') ||
        pkg.includes('telegram') ||
        pkg.includes('discord') ||
        pkg.includes('signal')
      ) {
        totals['Communication'] += a.minutes;
      } else {
        totals['Utilities & Other'] += a.minutes;
      }
    }

    return Object.entries(totals).map(([name, mins]) => ({
      name,
      minutes: mins,
      percent: grandTotal > 0 ? Math.round((mins / grandTotal) * 100) : 0,
    }));
  };

  const handleRename = async (pkg: string, currentName: string) => {
    const newName = prompt(`Enter custom name for ${pkg}:`, currentName);
    if (!newName || newName.trim() === '' || newName === currentName) return;
    try {
      await ApiClient.post('/api/v1/reports/screen-time/aliases', {
        package_name: pkg,
        app_name: newName.trim(),
      });
      showToast('App name updated', 'success');
      await loadData();
    } catch (err: unknown) {
      showToast(err instanceof Error ? err.message : 'Failed to save app alias', 'error');
    }
  };

  const handleExportCSV = () => {
    const url = `/api/v1/data/export?format=csv&from=${from()}&to=${to()}`;
    window.open(url, '_blank');
    showToast('Exporting CSV report...', 'info');
  };

  return (
    <div class="flex flex-col gap-6">
      {/* Filters Bar */}
      <div class="bento-card flex flex-wrap items-center justify-between gap-4">
        <div class="flex flex-wrap items-center gap-3">
          <div class="flex items-center gap-2 text-xs">
            <span class="text-text-muted font-medium">From:</span>
            <input
              type="date"
              value={from()}
              onInput={(e) => setFrom(e.currentTarget.value)}
              class="bg-bg-card border border-border-default rounded-sm px-2.5 py-1 text-xs text-text-primary focus:border-border-focus focus:outline-none"
            />
          </div>

          <div class="flex items-center gap-2 text-xs">
            <span class="text-text-muted font-medium">To:</span>
            <input
              type="date"
              value={to()}
              onInput={(e) => setTo(e.currentTarget.value)}
              class="bg-bg-card border border-border-default rounded-sm px-2.5 py-1 text-xs text-text-primary focus:border-border-focus focus:outline-none"
            />
          </div>

          <div class="flex items-center gap-2 text-xs">
            <span class="text-text-muted font-medium">Device:</span>
            <select
              value={deviceId()}
              onChange={(e) => setDeviceId(e.currentTarget.value)}
              class="bg-bg-card border border-border-default rounded-sm px-2.5 py-1 text-xs text-text-primary focus:border-border-focus focus:outline-none cursor-pointer"
            >
              <option value="">All Devices</option>
              <For each={devices()}>
                {(d) => (
                  <option value={d.id}>
                    {d.display_name || d.label || `Device ${d.id}`}
                  </option>
                )}
              </For>
            </select>
          </div>

          <label class="flex items-center gap-1.5 text-xs text-text-muted cursor-pointer select-none">
            <input
              type="checkbox"
              checked={hideSystemUI()}
              onChange={(e) => setHideSystemUI(e.currentTarget.checked)}
              class="rounded border-border-default bg-bg-card text-primary focus:ring-0 cursor-pointer"
            />
            <IconEyeOff size={14} />
            <span>Hide System UI</span>
          </label>
        </div>

        <div class="flex items-center gap-2">
          <button
            type="button"
            onClick={handleExportCSV}
            class="px-3 py-1.5 rounded-sm bg-white/5 hover:bg-white/10 text-text-primary text-xs font-semibold border border-border-default transition-all flex items-center gap-1.5 cursor-pointer"
          >
            <IconFileSpreadsheet size={15} />
            <span>Export CSV</span>
          </button>
          <button
            type="button"
            onClick={loadData}
            class="px-3 py-1.5 rounded-sm bg-primary hover:bg-primary-hover text-white text-xs font-semibold transition-all flex items-center gap-1.5 cursor-pointer shadow-sm"
          >
            <IconRefresh size={15} class={loading() ? 'animate-spin' : ''} />
            <span>Refresh Report</span>
          </button>
        </div>
      </div>

      {/* 4 KPI Cards */}
      <div class="bento-grid">
        <div class="bento-card col-span-12 tablet:col-span-6 desktop:col-span-3">
          <div class="flex items-center gap-2 mb-2">
            <div class="w-8 h-8 rounded-sm bg-primary/15 text-primary-light flex items-center justify-center">
              <IconClock size={18} />
            </div>
            <span class="stat-label">Total Screen Time</span>
          </div>
          <div class="stat-value text-text-primary">
            {formatMinutes(totalPeriodMinutes())}
          </div>
          <div class="text-xs text-text-muted mt-1">Aggregated across active window</div>
        </div>

        <div class="bento-card col-span-12 tablet:col-span-6 desktop:col-span-3">
          <div class="flex items-center gap-2 mb-2">
            <div class="w-8 h-8 rounded-sm bg-accent-cyan/15 text-accent-cyan flex items-center justify-center">
              <IconCalendarStats size={18} />
            </div>
            <span class="stat-label">Daily Average</span>
          </div>
          <div class="stat-value text-accent-cyan">
            {formatMinutes(dailyAverageMinutes())}
          </div>
          <div class="text-xs text-text-muted mt-1">Mean active foreground quota</div>
        </div>

        <div class="bento-card col-span-12 tablet:col-span-6 desktop:col-span-3">
          <div class="flex items-center gap-2 mb-2">
            <div class="w-8 h-8 rounded-sm bg-warning/15 text-warning flex items-center justify-center">
              <IconEyeOff size={18} />
            </div>
            <span class="stat-label">Filtered System UI</span>
          </div>
          <div class="stat-value text-warning">
            {filteredSystemCount()} <span class="text-sm font-normal text-text-muted">pkgs</span>
          </div>
          <div class="text-xs text-text-muted mt-1">System overlays hidden</div>
        </div>

        <div class="bento-card col-span-12 tablet:col-span-6 desktop:col-span-3">
          <div class="flex items-center gap-2 mb-2">
            <div class="w-8 h-8 rounded-sm bg-success/15 text-success flex items-center justify-center">
              <IconApps size={18} />
            </div>
            <span class="stat-label">Unique Applications</span>
          </div>
          <div class="stat-value text-success">
            {filteredApps().length} <span class="text-sm font-normal text-text-muted">apps</span>
          </div>
          <div class="text-xs text-text-muted mt-1">Foreground packages executed</div>
        </div>

        {/* Daily Distribution Chart (col-8) */}
        <div class="bento-card col-span-12 desktop:col-span-8">
          <div class="flex items-center justify-between mb-4">
            <div>
              <h3 class="text-base font-bold text-text-primary">Daily Usage Distribution</h3>
              <p class="text-xs text-text-muted mt-0.5">
                Multi-day chronological breakdown of active hours
              </p>
            </div>
          </div>
          <div class="relative w-full h-[280px]">
            <Chart config={chartConfig()} height={280} />
          </div>
        </div>

        {/* Category Allocation (col-4) */}
        <div class="bento-card col-span-12 desktop:col-span-4 flex flex-col justify-between">
          <div>
            <div class="mb-4">
              <h3 class="text-base font-bold text-text-primary">Category Allocation</h3>
              <p class="text-xs text-text-muted mt-0.5">Heuristic cognitive distribution</p>
            </div>

            <div class="flex flex-col gap-3.5">
              <For each={categories()}>
                {(cat) => (
                  <div>
                    <div class="flex justify-between items-center text-xs mb-1">
                      <span class="font-medium text-text-primary">{cat.name}</span>
                      <span class="text-text-muted">
                        {formatMinutes(cat.minutes)} ({cat.percent}%)
                      </span>
                    </div>
                    <div class="h-1.5 w-full bg-primary/20 rounded-full overflow-hidden">
                      <div
                        class="h-full bg-primary rounded-full transition-all duration-300"
                        style={{ width: `${cat.percent}%` }}
                      />
                    </div>
                  </div>
                )}
              </For>
            </div>
          </div>

          <div class="pt-3 mt-3 border-t border-border-default text-xs text-text-dim text-center">
            Derived from package identifier patterns
          </div>
        </div>

        {/* Application Inventory Table (col-12) */}
        <div class="bento-card col-span-12">
          <div class="flex flex-wrap items-center justify-between gap-4 mb-4">
            <div>
              <h3 class="text-base font-bold text-text-primary">Application Inventory</h3>
              <p class="text-xs text-text-muted mt-0.5">
                Complete catalog of recorded foreground packages
              </p>
            </div>

            <div class="relative min-w-[240px]">
              <div class="absolute inset-y-0 left-0 pl-2.5 flex items-center pointer-events-none text-text-dim">
                <IconSearch size={15} />
              </div>
              <input
                type="text"
                placeholder="Search packages or names..."
                value={searchQuery()}
                onInput={(e) => setSearchQuery(e.currentTarget.value)}
                class="w-full bg-bg-card border border-border-default rounded-sm pl-8 pr-3 py-1.5 text-xs text-text-primary focus:border-border-focus focus:outline-none"
              />
            </div>
          </div>

          <Show
            when={filteredApps().length > 0}
            fallback={<div class="text-text-muted text-xs py-8 text-center">No applications matching filter</div>}
          >
            <div class="overflow-x-auto">
              <table class="w-full text-left text-xs">
                <thead>
                  <tr class="border-b border-border-default text-text-dim">
                    <th class="py-2.5 pr-4 font-semibold">APPLICATION</th>
                    <th class="py-2.5 px-4 font-semibold">PACKAGE ID</th>
                    <th class="py-2.5 px-4 font-semibold">SOURCE</th>
                    <th class="py-2.5 px-4 font-semibold text-right">ACTIVE TIME</th>
                    <th class="py-2.5 pl-4 font-semibold">USAGE SHARE</th>
                  </tr>
                </thead>
                <tbody class="divide-y border-border-default/50">
                  <For each={filteredApps()}>
                    {(app) => {
                      const maxM = filteredApps()[0]?.minutes || 1;
                      const sharePct = Math.min(100, Math.round((app.minutes / maxM) * 100));
                      return (
                        <tr class="hover:bg-white/5 transition-colors">
                          <td class="py-3 pr-4">
                            <div class="flex items-center gap-2">
                              <span class="font-semibold text-text-primary">{app.app_name}</span>
                              <button
                                type="button"
                                onClick={() => handleRename(app.package_name, app.app_name)}
                                class="text-primary hover:text-primary-light transition-colors cursor-pointer p-0.5"
                                title="Rename app"
                              >
                                <IconEdit size={13} />
                              </button>
                            </div>
                          </td>
                          <td class="py-3 px-4 font-mono text-text-muted text-[0.7rem]">
                            {app.package_name}
                          </td>
                          <td class="py-3 px-4">
                            <span class="bg-white/5 border border-border-default rounded px-1.5 py-0.5 text-[0.65rem] text-text-dim font-mono">
                              Play Store Cache
                            </span>
                          </td>
                          <td class="py-3 px-4 font-semibold text-text-primary text-right">
                            {formatMinutes(app.minutes)}
                          </td>
                          <td class="py-3 pl-4 w-32">
                            <div class="flex items-center gap-2">
                              <div class="h-1.5 flex-1 bg-primary/20 rounded-full overflow-hidden">
                                <div
                                  class="h-full bg-primary rounded-full"
                                  style={{ width: `${sharePct}%` }}
                                />
                              </div>
                              <span class="text-[0.65rem] text-text-dim font-medium w-8 text-right">
                                {sharePct}%
                              </span>
                            </div>
                          </td>
                        </tr>
                      );
                    }}
                  </For>
                </tbody>
              </table>
            </div>
          </Show>
        </div>
      </div>
    </div>
  );
}
