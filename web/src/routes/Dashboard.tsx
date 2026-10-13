import { createSignal, onMount, createEffect, For, Show } from 'solid-js';
import { A } from '@solidjs/router';
import {
  IconDeviceMobile,
  IconWalk,
  IconMoon,
  IconRouter,
  IconTarget,
  IconBulb,
  IconSunrise,
  IconArrowRight,
  IconChevronRight,
} from '@tabler/icons-solidjs';
import { ApiClient } from '../api/client';
import { Chart } from '../components/Chart';
import { PulseDot } from '../components/PulseDot';
import { formatMinutes, formatSteps } from '../lib/format';
import { getDaysAgoISO, getTodayISO, formatDateTime } from '../lib/dates';
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

interface SleepSessionSummary {
  id: number;
  day: string;
  duration_minutes: number;
  deep_sleep_minutes?: number;
  rem_sleep_minutes?: number;
}

interface AppSummaryItem {
  package_name: string;
  app_name: string;
  minutes: number;
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
  const [range, setRange] = createSignal<'7d' | 'today' | '30d'>('7d');
  const [todayMinutes, setTodayMinutes] = createSignal(0);
  const [todaySteps, setTodaySteps] = createSignal(0);
  const [todaySleepMinutes, setTodaySleepMinutes] = createSignal(0);
  const [devices, setDevices] = createSignal<DeviceItem[]>([]);
  const [trendDays, setTrendDays] = createSignal<DailySummary[]>([]);
  const [topApps, setTopApps] = createSignal<AppSummaryItem[]>([]);

  const fetchData = async () => {
    try {
      const to = getTodayISO();
      let from = to;
      if (range() === '7d') from = getDaysAgoISO(6);
      else if (range() === '30d') from = getDaysAgoISO(29);

      const [stResp, healthResp, syncResp, sleepResp, appsResp] = await Promise.all([
        ApiClient.get<{ days: DailySummary[] }>(`/api/v1/reports/screen-time/daily?from=${from}&to=${to}`),
        ApiClient.get<HealthSummary[]>(`/api/v1/reports/health/daily?from=${from}&to=${to}`),
        ApiClient.get<{ devices: DeviceItem[] }>('/api/v1/reports/sync-status'),
        ApiClient.get<SleepSessionSummary[]>(`/api/v1/reports/health/sleep?from=${from}&to=${to}`),
        ApiClient.get<AppSummaryItem[]>(`/api/v1/reports/screen-time/apps?from=${from}&to=${to}&limit=5`),
      ]);

      if (stResp && stResp.days) {
        setTrendDays(stResp.days);
        const todayItem = stResp.days.find((d) => d.date === to);
        setTodayMinutes(todayItem ? todayItem.total_screen_time_minutes : 0);
      }

      if (healthResp && Array.isArray(healthResp)) {
        const todayHealth = healthResp.find((d) => d.date === to);
        setTodaySteps(todayHealth ? todayHealth.steps : 0);
      }

      if (syncResp && syncResp.devices) {
        setDevices(syncResp.devices);
      }

      if (sleepResp && Array.isArray(sleepResp) && sleepResp.length > 0) {
        const lastSession = sleepResp[sleepResp.length - 1];
        setTodaySleepMinutes(lastSession.duration_minutes || 0);
      }

      if (appsResp && Array.isArray(appsResp)) {
        setTopApps(appsResp);
      }
    } catch (err) {
      console.error('Failed to load dashboard data:', err);
    }
  };

  createEffect(() => {
    range();
    fetchData();
  });

  const chartConfig = (): ChartConfiguration => {
    const days = trendDays();
    const labels = days.map((d) => d.date.slice(5)); // MM-DD
    const data = days.map((d) => Math.round((d.total_screen_time_minutes / 60) * 10) / 10);
    const todayISO = getTodayISO();

    const backgroundColors = days.map((d) =>
      d.date === todayISO ? '#6366f1' : 'rgba(129, 140, 248, 0.4)'
    );

    return {
      type: 'bar',
      data: {
        labels,
        datasets: [
          {
            label: 'Screen Time (Hours)',
            data,
            backgroundColor: backgroundColors,
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

  const latestSyncTime = () => {
    const list = devices().filter((d) => d.last_sync_at);
    if (list.length === 0) return null;
    return list.sort((a, b) => new Date(b.last_sync_at!).getTime() - new Date(a.last_sync_at!).getTime())[0].last_sync_at;
  };

  const activeDeviceCount = () => devices().filter((d) => d.status === 'active').length;

  return (
    <div class="flex flex-col gap-6">
      {/* Controls Strip */}
      <div class="flex flex-wrap items-center justify-between gap-4">
        <div class="flex items-center gap-2 bg-black/20 p-1 rounded-full border border-border-default">
          <button
            type="button"
            onClick={() => setRange('today')}
            class={`px-3 py-1 rounded-full text-xs font-semibold cursor-pointer transition-colors ${
              range() === 'today' ? 'bg-primary text-white shadow-sm' : 'text-text-muted hover:text-text-primary'
            }`}
          >
            Today
          </button>
          <button
            type="button"
            onClick={() => setRange('7d')}
            class={`px-3 py-1 rounded-full text-xs font-semibold cursor-pointer transition-colors ${
              range() === '7d' ? 'bg-primary text-white shadow-sm' : 'text-text-muted hover:text-text-primary'
            }`}
          >
            7 Days
          </button>
          <button
            type="button"
            onClick={() => setRange('30d')}
            class={`px-3 py-1 rounded-full text-xs font-semibold cursor-pointer transition-colors ${
              range() === '30d' ? 'bg-primary text-white shadow-sm' : 'text-text-muted hover:text-text-primary'
            }`}
          >
            30 Days
          </button>
        </div>

        <div class="flex items-center gap-2 bg-bg-card border border-border-default rounded-full px-3 py-1 text-xs text-text-muted">
          <PulseDot />
          <span>
            Fleet Verified:{' '}
            <strong class="text-success font-semibold">
              {latestSyncTime() ? formatDateTime(latestSyncTime()!) : 'Awaiting sync'}
            </strong>
          </span>
        </div>
      </div>

      {/* 4 KPI Cards */}
      <div class="bento-grid">
        {/* KPI 1: Screen Time */}
        <div class="bento-card col-span-12 tablet:col-span-6 desktop:col-span-3 flex flex-col justify-between">
          <div>
            <div class="flex items-center justify-between mb-3">
              <div class="flex items-center gap-2">
                <div class="w-8 h-8 rounded-sm bg-primary/15 text-primary-light flex items-center justify-center">
                  <IconDeviceMobile size={18} />
                </div>
                <span class="stat-label">Screen Attention</span>
              </div>
            </div>
            <div class="stat-value text-text-primary mb-1">
              {formatMinutes(todayMinutes())}
            </div>
            <div class="text-xs text-text-muted">Today's foreground usage</div>
          </div>
          <div class="mt-4">
            <div class="h-1.5 w-full bg-primary/20 rounded-full overflow-hidden">
              <div
                class="h-full bg-primary rounded-full"
                style={{ width: `${Math.min(100, Math.round((todayMinutes() / 360) * 100))}%` }}
              />
            </div>
            <div class="flex justify-between text-[0.7rem] text-text-dim mt-1">
              <span>Target: 6h cap</span>
              <span>{Math.round((todayMinutes() / 360) * 100)}% utilized</span>
            </div>
          </div>
        </div>

        {/* KPI 2: Steps Cadence */}
        <div class="bento-card col-span-12 tablet:col-span-6 desktop:col-span-3 flex flex-col justify-between">
          <div>
            <div class="flex items-center justify-between mb-3">
              <div class="flex items-center gap-2">
                <div class="w-8 h-8 rounded-sm bg-success/15 text-success flex items-center justify-center">
                  <IconWalk size={18} />
                </div>
                <span class="stat-label">Physical Activity</span>
              </div>
            </div>
            <div class="stat-value text-success mb-1">
              {formatSteps(todaySteps())}
            </div>
            <div class="text-xs text-text-muted">Health Connect step count</div>
          </div>
          <div class="mt-4">
            <div class="h-1.5 w-full bg-success/20 rounded-full overflow-hidden">
              <div
                class="h-full bg-success rounded-full"
                style={{ width: `${Math.min(100, Math.round((todaySteps() / 10000) * 100))}%` }}
              />
            </div>
            <div class="flex justify-between text-[0.7rem] text-text-dim mt-1">
              <span>Daily Goal: 10,000</span>
              <span>{Math.max(0, 10000 - todaySteps()).toLocaleString()} to go</span>
            </div>
          </div>
        </div>

        {/* KPI 3: Sleep */}
        <div class="bento-card col-span-12 tablet:col-span-6 desktop:col-span-3 flex flex-col justify-between">
          <div>
            <div class="flex items-center justify-between mb-3">
              <div class="flex items-center gap-2">
                <div class="w-8 h-8 rounded-sm bg-accent-purple/15 text-accent-purple flex items-center justify-center">
                  <IconMoon size={18} />
                </div>
                <span class="stat-label">Rest & Recovery</span>
              </div>
            </div>
            <div class="stat-value text-accent-purple mb-1">
              {todaySleepMinutes() > 0 ? formatMinutes(Math.round(todaySleepMinutes())) : '—'}
            </div>
            <div class="text-xs text-text-muted">Latest sleep session</div>
          </div>
          <div class="mt-4">
            <div class="h-1.5 w-full bg-accent-purple/20 rounded-full overflow-hidden">
              <div
                class="h-full bg-accent-purple rounded-full"
                style={{ width: `${Math.min(100, Math.round((todaySleepMinutes() / 480) * 100))}%` }}
              />
            </div>
            <div class="flex justify-between text-[0.7rem] text-text-dim mt-1">
              <span>Target: 8h duration</span>
              <span>{Math.round((todaySleepMinutes() / 480) * 100)}% achieved</span>
            </div>
          </div>
        </div>

        {/* KPI 4: Fleet Telemetry */}
        <div class="bento-card col-span-12 tablet:col-span-6 desktop:col-span-3 flex flex-col justify-between">
          <div>
            <div class="flex items-center justify-between mb-3">
              <div class="flex items-center gap-2">
                <div class="w-8 h-8 rounded-sm bg-accent-cyan/15 text-accent-cyan flex items-center justify-center">
                  <IconRouter size={18} />
                </div>
                <span class="stat-label">Fleet Telemetry</span>
              </div>
            </div>
            <div class="stat-value text-accent-cyan mb-1">
              {activeDeviceCount()} <span class="text-sm font-normal text-text-muted">nodes</span>
            </div>
            <div class="text-xs text-text-muted">Active companion devices</div>
          </div>
          <div class="mt-4 flex flex-wrap gap-1.5">
            <For each={devices().slice(0, 3)}>
              {(d) => (
                <span class="bg-white/5 border border-border-default rounded-sm px-2 py-0.5 text-[0.7rem] text-text-primary">
                  {d.display_name || d.device_model || d.label}
                </span>
              )}
            </For>
          </div>
        </div>

        {/* 7-Day Trajectory Chart (col-8) */}
        <div class="bento-card col-span-12 desktop:col-span-8 flex flex-col justify-between">
          <div>
            <div class="flex items-center justify-between mb-3">
              <div>
                <h3 class="text-base font-bold text-text-primary">
                  Attention & Screen Time Trajectory
                </h3>
                <p class="text-xs text-text-muted mt-0.5">
                  Foreground aggregation across authenticated companions
                </p>
              </div>
              <span class="text-xs text-text-dim">Limit: 6h / day</span>
            </div>
            <div class="relative w-full h-[260px]">
              <Chart config={chartConfig()} height={260} />
            </div>
          </div>
          <div class="flex items-center justify-between pt-3 mt-3 border-t border-border-default text-xs">
            <span class="text-text-muted">
              Continuous companion telemetry
            </span>
            <A
              href="/reports/screen-time"
              class="text-primary-light hover:text-primary font-semibold flex items-center gap-1 no-underline"
            >
              <span>Detailed Breakdown</span>
              <IconChevronRight size={14} />
            </A>
          </div>
        </div>

        {/* Top Applications (col-4) */}
        <div class="bento-card col-span-12 desktop:col-span-4 flex flex-col justify-between">
          <div>
            <div class="flex items-center justify-between mb-3">
              <div>
                <h3 class="text-base font-bold text-text-primary">Top Applications</h3>
                <p class="text-xs text-text-muted mt-0.5">Resolved application metadata</p>
              </div>
              <span class="text-xs text-text-dim font-medium">Ranked</span>
            </div>

            <Show
              when={topApps().length > 0}
              fallback={<div class="text-text-muted text-xs py-8 text-center">No app data recorded for range</div>}
            >
              <div class="flex flex-col gap-3">
                <For each={topApps()}>
                  {(app) => {
                    const maxM = topApps()[0]?.minutes || 1;
                    const pct = Math.min(100, Math.round((app.minutes / maxM) * 100));
                    return (
                      <div>
                        <div class="flex justify-between items-center text-xs mb-1">
                          <span class="font-semibold text-text-primary truncate max-w-[160px]">
                            {app.app_name}
                          </span>
                          <span class="text-text-muted font-medium">
                            {formatMinutes(app.minutes)}
                          </span>
                        </div>
                        <div class="h-1.5 w-full bg-primary/20 rounded-full overflow-hidden">
                          <div
                            class="h-full bg-primary rounded-full transition-all duration-300"
                            style={{ width: `${pct}%` }}
                          />
                        </div>
                        <span class="text-[0.65rem] text-text-dim font-mono block mt-0.5 truncate">
                          {app.package_name}
                        </span>
                      </div>
                    );
                  }}
                </For>
              </div>
            </Show>
          </div>

          <div class="pt-3 mt-3 border-t border-border-default text-center">
            <A
              href="/reports/screen-time"
              class="text-xs text-text-muted hover:text-text-primary no-underline font-medium inline-flex items-center gap-1"
            >
              <span>View all recorded apps</span>
              <IconArrowRight size={13} />
            </A>
          </div>
        </div>

        {/* Fleet Sync Matrix (col-7) */}
        <div class="bento-card col-span-12 desktop:col-span-7">
          <div class="flex items-center justify-between mb-4">
            <div>
              <h3 class="text-base font-bold text-text-primary">
                Companion Fleet Synchronization
              </h3>
              <p class="text-xs text-text-muted mt-0.5">
                Automated background workers & Health Connect telemetry
              </p>
            </div>
            <A
              href="/devices"
              class="text-xs text-primary-light hover:text-primary font-semibold no-underline"
            >
              Manage Fleet &rarr;
            </A>
          </div>

          <Show
            when={devices().length > 0}
            fallback={<div class="text-text-muted text-xs py-8 text-center">No companion devices registered</div>}
          >
            <div class="overflow-x-auto">
              <table class="w-full text-left text-xs">
                <thead>
                  <tr class="border-b border-border-default text-text-dim">
                    <th class="py-2 pr-3 font-semibold">DEVICE MODEL</th>
                    <th class="py-2 px-3 font-semibold">BUILD / AGENT</th>
                    <th class="py-2 px-3 font-semibold">PROTOCOL</th>
                    <th class="py-2 pl-3 font-semibold">LAST SYNC</th>
                  </tr>
                </thead>
                <tbody class="divide-y border-border-default/50">
                  <For each={devices()}>
                    {(dev) => (
                      <tr class="hover:bg-white/5 transition-colors">
                        <td class="py-3 pr-3">
                          <div class="font-semibold text-text-primary">
                            {dev.display_name || dev.device_model || dev.label}
                          </div>
                          <div class="text-[0.7rem] text-primary-light font-mono">
                            {dev.label}
                          </div>
                        </td>
                        <td class="py-3 px-3 text-text-muted">
                          {dev.collector_version || 'v1.0.0'}
                        </td>
                        <td class="py-3 px-3">
                          <code class="px-1.5 py-0.5 rounded bg-white/5 text-[0.7rem] text-accent-cyan font-mono">
                            {dev.last_sync_source || 'periodic_worker'}
                          </code>
                        </td>
                        <td class="py-3 pl-3">
                          <Show
                            when={dev.last_sync_at}
                            fallback={<span class="text-text-dim">Never</span>}
                          >
                            <span class="inline-flex items-center gap-1.5 bg-success-bg text-success px-2 py-0.5 rounded-full font-medium text-[0.7rem]">
                              <PulseDot />
                              <span>{formatDateTime(dev.last_sync_at!)}</span>
                            </span>
                          </Show>
                        </td>
                      </tr>
                    )}
                  </For>
                </tbody>
              </table>
            </div>
          </Show>
        </div>

        {/* Focus Habits & Wellbeing (col-5) */}
        <div class="bento-card col-span-12 desktop:col-span-5 flex flex-col justify-between">
          <div>
            <div class="flex items-center justify-between mb-4">
              <div>
                <h3 class="text-base font-bold text-text-primary">Focus Habits & Wellbeing</h3>
                <p class="text-xs text-text-muted mt-0.5">Automated algorithmic insights</p>
              </div>
            </div>

            <div class="flex flex-col gap-3">
              <div class="p-3 rounded-md bg-white/5 border border-border-default flex items-start gap-3">
                <div class="w-8 h-8 rounded-sm bg-primary/20 text-primary-light flex items-center justify-center shrink-0">
                  <IconTarget size={18} />
                </div>
                <div>
                  <h4 class="text-xs font-bold text-text-primary">Digital Sunset</h4>
                  <p class="text-[0.75rem] text-text-muted mt-0.5">
                    Maintain screen boundaries 1 hour before scheduled sleep for optimal melatonin secretion.
                  </p>
                </div>
              </div>

              <div class="p-3 rounded-md bg-white/5 border border-border-default flex items-start gap-3">
                <div class="w-8 h-8 rounded-sm bg-success/20 text-success flex items-center justify-center shrink-0">
                  <IconSunrise size={18} />
                </div>
                <div>
                  <h4 class="text-xs font-bold text-text-primary">Morning Activity Anchor</h4>
                  <p class="text-[0.75rem] text-text-muted mt-0.5">
                    First 2,000 steps achieved before 10:00 AM strongly correlates with sustained daily focus.
                  </p>
                </div>
              </div>

              <div class="p-3 rounded-md bg-white/5 border border-border-default flex items-start gap-3">
                <div class="w-8 h-8 rounded-sm bg-warning/20 text-warning flex items-center justify-center shrink-0">
                  <IconBulb size={18} />
                </div>
                <div>
                  <h4 class="text-xs font-bold text-text-primary">Context Switching Cap</h4>
                  <p class="text-[0.75rem] text-text-muted mt-0.5">
                    Limit app switches per session to under 8 to preserve deep work momentum.
                  </p>
                </div>
              </div>
            </div>
          </div>

          <div class="pt-3 mt-3 border-t border-border-default text-xs text-text-dim text-center">
            Derived from multimodal sensor telemetry
          </div>
        </div>
      </div>
    </div>
  );
}
