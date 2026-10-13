import { createSignal, onMount, For, Show } from 'solid-js';
import {
  IconWalk,
  IconMapPin,
  IconFlame,
  IconMoon,
  IconHeartbeat,
  IconTrendingUp,
  IconActivity,
  IconRefresh,
} from '@tabler/icons-solidjs';
import { ApiClient } from '../api/client';
import { Chart } from '../components/Chart';
import { showToast } from '../components/Toast';
import { formatSteps, formatDistance, formatCalories, formatMinutes } from '../lib/format';
import { formatDateTime, getDaysAgoISO, getTodayISO } from '../lib/dates';
import type { ChartConfiguration } from 'chart.js';

interface HealthDaily {
  date: string;
  steps: number;
  distance_meters: number;
  calories: number;
}

interface TrendItem {
  period: string;
  steps_total: number;
  steps_avg: number;
  distance_total: number;
  calories_total: number;
  days_count: number;
}

interface SleepSession {
  record_id: string;
  wake_date: string;
  duration_minutes: number;
  start_time: string;
  end_time: string;
}

export function HealthReport() {
  const [activeTab, setActiveTab] = createSignal<'activity' | 'sleep' | 'trends'>('activity');
  const [from, setFrom] = createSignal(getDaysAgoISO(13));
  const [to, setTo] = createSignal(getTodayISO());
  const [dailyData, setDailyData] = createSignal<HealthDaily[]>([]);
  const [trendsData, setTrendsData] = createSignal<TrendItem[]>([]);
  const [sleepData, setSleepData] = createSignal<SleepSession[]>([]);
  const [loading, setLoading] = createSignal(false);

  const loadData = async () => {
    setLoading(true);
    try {
      const [daily, trends, sleep] = await Promise.all([
        ApiClient.get<HealthDaily[]>(`/api/v1/reports/health/daily?from=${from()}&to=${to()}`),
        ApiClient.get<TrendItem[]>(`/api/v1/reports/health/trends?period=weekly&from=${from()}&to=${to()}`),
        ApiClient.get<SleepSession[]>(`/api/v1/reports/health/sleep?from=${from()}&to=${to()}`),
      ]);
      setDailyData(daily || []);
      setTrendsData(trends || []);
      setSleepData(sleep || []);
    } catch (err) {
      console.error('Failed to load health reports:', err);
      showToast('Failed to load health data', 'error');
    } finally {
      setLoading(false);
    }
  };

  onMount(loadData);

  const stepsChartConfig = (): ChartConfiguration => {
    const data = dailyData();
    const labels = data.map((d) => d.date.slice(5));
    const steps = data.map((d) => d.steps);

    return {
      type: 'bar',
      data: {
        labels,
        datasets: [
          {
            label: 'Steps',
            data: steps,
            backgroundColor: '#10b981',
            borderRadius: 6,
          },
        ],
      },
      options: {
        responsive: true,
        maintainAspectRatio: false,
        plugins: {
          legend: { display: false },
        },
        scales: {
          x: {
            grid: { display: false },
            ticks: { color: '#94a3b8' },
          },
          y: {
            beginAtZero: true,
            grid: { color: 'rgba(255, 255, 255, 0.05)' },
            ticks: { color: '#94a3b8' },
          },
        },
      },
    };
  };

  const sleepChartConfig = (): ChartConfiguration => {
    const data = sleepData();
    const labels = data.map((s) => s.wake_date.slice(5));
    const hours = data.map((s) => Math.round((s.duration_minutes / 60) * 10) / 10);

    return {
      type: 'line',
      data: {
        labels,
        datasets: [
          {
            label: 'Sleep Duration (Hours)',
            data: hours,
            borderColor: '#6366f1',
            backgroundColor: 'rgba(99, 102, 241, 0.15)',
            fill: true,
            tension: 0.3,
            pointBackgroundColor: '#6366f1',
          },
        ],
      },
      options: {
        responsive: true,
        maintainAspectRatio: false,
        plugins: {
          legend: { display: false },
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

  const totalSteps = () => dailyData().reduce((acc, c) => acc + c.steps, 0);
  const avgSteps = () => (dailyData().length ? Math.round(totalSteps() / dailyData().length) : 0);
  const totalDistance = () => dailyData().reduce((acc, c) => acc + c.distance_meters, 0);
  const totalCalories = () => dailyData().reduce((acc, c) => acc + c.calories, 0);

  const todaySteps = () => {
    const today = getTodayISO();
    const hit = dailyData().find((d) => d.date === today);
    return hit ? hit.steps : (dailyData()[dailyData().length - 1]?.steps || 0);
  };

  const lastSleep = () => {
    const list = sleepData();
    return list.length > 0 ? list[list.length - 1] : null;
  };

  return (
    <div class="flex flex-col gap-6">
      {/* Tabs and Controls Strip */}
      <div class="flex flex-wrap items-center justify-between gap-4">
        {/* Segmented Pill Tabs */}
        <div class="flex items-center gap-1 bg-black/20 rounded-xl p-1 border border-border-default">
          <button
            type="button"
            onClick={() => setActiveTab('activity')}
            class={`px-3 py-1.5 rounded-[10px] text-xs font-semibold flex items-center gap-1.5 transition-colors cursor-pointer ${
              activeTab() === 'activity'
                ? 'bg-white/10 text-text-primary shadow-sm'
                : 'text-text-muted hover:text-text-primary'
            }`}
          >
            <IconWalk size={16} />
            <span>Activity & Steps</span>
          </button>
          <button
            type="button"
            onClick={() => setActiveTab('sleep')}
            class={`px-3 py-1.5 rounded-[10px] text-xs font-semibold flex items-center gap-1.5 transition-colors cursor-pointer ${
              activeTab() === 'sleep'
                ? 'bg-white/10 text-text-primary shadow-sm'
                : 'text-text-muted hover:text-text-primary'
            }`}
          >
            <IconMoon size={16} />
            <span>Sleep & Stages</span>
          </button>
          <button
            type="button"
            onClick={() => setActiveTab('trends')}
            class={`px-3 py-1.5 rounded-[10px] text-xs font-semibold flex items-center gap-1.5 transition-colors cursor-pointer ${
              activeTab() === 'trends'
                ? 'bg-white/10 text-text-primary shadow-sm'
                : 'text-text-muted hover:text-text-primary'
            }`}
          >
            <IconTrendingUp size={16} />
            <span>Weekly Trends</span>
          </button>
        </div>

        {/* Date Filter & Health Connect Indicator */}
        <div class="flex flex-wrap items-center gap-3">
          <div class="flex items-center gap-1.5 text-xs text-text-dim">
            <IconHeartbeat size={15} class="text-danger" />
            <span>Android Health Connect API</span>
          </div>

          <div class="flex items-center gap-2 text-xs">
            <input
              type="date"
              value={from()}
              onInput={(e) => setFrom(e.currentTarget.value)}
              class="bg-bg-card border border-border-default rounded-sm px-2.5 py-1 text-xs text-text-primary focus:border-border-focus focus:outline-none"
            />
            <span class="text-text-muted">to</span>
            <input
              type="date"
              value={to()}
              onInput={(e) => setTo(e.currentTarget.value)}
              class="bg-bg-card border border-border-default rounded-sm px-2.5 py-1 text-xs text-text-primary focus:border-border-focus focus:outline-none"
            />
          </div>

          <button
            type="button"
            onClick={loadData}
            class="px-3 py-1.5 rounded-sm bg-primary hover:bg-primary-hover text-white text-xs font-semibold transition-all flex items-center gap-1.5 cursor-pointer shadow-sm"
          >
            <IconRefresh size={14} class={loading() ? 'animate-spin' : ''} />
            <span>Update</span>
          </button>
        </div>
      </div>

      {/* 4 KPI Cards (Always Visible) */}
      <div class="bento-grid">
        <div class="bento-card col-span-12 tablet:col-span-6 desktop:col-span-3">
          <div class="flex items-center gap-2 mb-2">
            <div class="w-8 h-8 rounded-sm bg-success/15 text-success flex items-center justify-center">
              <IconWalk size={18} />
            </div>
            <span class="stat-label">Today's Steps</span>
          </div>
          <div class="stat-value text-success">
            {formatSteps(todaySteps())}
          </div>
          <div class="text-xs text-text-muted mt-1">Health Connect sensor count</div>
        </div>

        <div class="bento-card col-span-12 tablet:col-span-6 desktop:col-span-3">
          <div class="flex items-center gap-2 mb-2">
            <div class="w-8 h-8 rounded-sm bg-accent-cyan/15 text-accent-cyan flex items-center justify-center">
              <IconMapPin size={18} />
            </div>
            <span class="stat-label">Distance Traversed</span>
          </div>
          <div class="stat-value text-accent-cyan">
            {formatDistance(totalDistance())}
          </div>
          <div class="text-xs text-text-muted mt-1">Aggregated distance for range</div>
        </div>

        <div class="bento-card col-span-12 tablet:col-span-6 desktop:col-span-3">
          <div class="flex items-center gap-2 mb-2">
            <div class="w-8 h-8 rounded-sm bg-warning/15 text-warning flex items-center justify-center">
              <IconFlame size={18} />
            </div>
            <span class="stat-label">Active Caloric Output</span>
          </div>
          <div class="stat-value text-warning">
            {formatCalories(totalCalories())}
          </div>
          <div class="text-xs text-text-muted mt-1">Active metabolic expenditure</div>
        </div>

        <div class="bento-card col-span-12 tablet:col-span-6 desktop:col-span-3">
          <div class="flex items-center gap-2 mb-2">
            <div class="w-8 h-8 rounded-sm bg-primary/15 text-primary-light flex items-center justify-center">
              <IconMoon size={18} />
            </div>
            <span class="stat-label">Sleep Duration</span>
          </div>
          <div class="stat-value text-primary-light">
            {lastSleep() ? formatMinutes(Math.round(lastSleep()!.duration_minutes)) : '—'}
          </div>
          <div class="text-xs text-text-muted mt-1">
            {lastSleep() ? `Wake date: ${lastSleep()!.wake_date}` : 'No session recorded'}
          </div>
        </div>

        {/* Tab 1: Activity View */}
        <Show when={activeTab() === 'activity'}>
          <div class="bento-card col-span-12 desktop:col-span-8">
            <div class="flex items-center justify-between mb-4">
              <div>
                <h3 class="text-base font-bold text-text-primary">Daily Steps History</h3>
                <p class="text-xs text-text-muted mt-0.5">Continuous pedometer cadence</p>
              </div>
            </div>
            <Show
              when={dailyData().length > 0}
              fallback={<div class="text-text-muted text-xs py-12 text-center">No step records in this range</div>}
            >
              <div class="relative w-full h-[280px]">
                <Chart config={stepsChartConfig()} height={280} />
              </div>
            </Show>
          </div>

          <div class="bento-card col-span-12 desktop:col-span-4 flex flex-col justify-between">
            <div>
              <div class="mb-4">
                <h3 class="text-base font-bold text-text-primary">Basal Vitals Profile</h3>
                <p class="text-xs text-text-muted mt-0.5">Aggregated activity metrics</p>
              </div>

              <div class="flex flex-col gap-4">
                <div class="p-3 rounded-md bg-white/5 border border-border-default flex items-center justify-between">
                  <div class="flex items-center gap-2.5">
                    <IconActivity size={18} class="text-success" />
                    <span class="text-xs font-medium text-text-primary">Daily Step Avg</span>
                  </div>
                  <span class="text-sm font-bold text-success">{formatSteps(avgSteps())}</span>
                </div>

                <div class="p-3 rounded-md bg-white/5 border border-border-default flex items-center justify-between">
                  <div class="flex items-center gap-2.5">
                    <IconMapPin size={18} class="text-accent-cyan" />
                    <span class="text-xs font-medium text-text-primary">Total Distance</span>
                  </div>
                  <span class="text-sm font-bold text-accent-cyan">{formatDistance(totalDistance())}</span>
                </div>

                <div class="p-3 rounded-md bg-white/5 border border-border-default flex items-center justify-between">
                  <div class="flex items-center gap-2.5">
                    <IconFlame size={18} class="text-warning" />
                    <span class="text-xs font-medium text-text-primary">Burned Calories</span>
                  </div>
                  <span class="text-sm font-bold text-warning">{formatCalories(totalCalories())}</span>
                </div>
              </div>
            </div>

            <div class="pt-3 mt-4 border-t border-border-default text-xs text-text-dim text-center">
              Health Connect sync window: {dailyData().length} days
            </div>
          </div>
        </Show>

        {/* Tab 2: Sleep View */}
        <Show when={activeTab() === 'sleep'}>
          <div class="bento-card col-span-12">
            <div class="flex items-center justify-between mb-4">
              <div>
                <h3 class="text-base font-bold text-text-primary">Sleep Duration Trend</h3>
                <p class="text-xs text-text-muted mt-0.5">Nightly rest intervals</p>
              </div>
            </div>
            <Show
              when={sleepData().length > 0}
              fallback={<div class="text-text-muted text-xs py-12 text-center">No sleep records in this range</div>}
            >
              <div class="relative w-full h-[260px]">
                <Chart config={sleepChartConfig()} height={260} />
              </div>
            </Show>
          </div>

          <div class="bento-card col-span-12">
            <div class="mb-4">
              <h3 class="text-base font-bold text-text-primary">Recorded Sleep Sessions</h3>
              <p class="text-xs text-text-muted mt-0.5">Session intervals and efficiency</p>
            </div>

            <Show
              when={sleepData().length > 0}
              fallback={<div class="text-text-muted text-xs py-8 text-center">No sleep sessions recorded</div>}
            >
              <div class="grid grid-cols-1 tablet:grid-cols-2 desktop:grid-cols-3 gap-4">
                <For each={sleepData()}>
                  {(s) => {
                    const pct = Math.min(100, Math.round((s.duration_minutes / 480) * 100));
                    return (
                      <div class="bg-bg-card-elevated border border-border-default rounded-md p-4 flex flex-col justify-between">
                        <div>
                          <div class="flex items-center justify-between mb-2">
                            <span class="font-bold text-text-primary text-sm">{s.wake_date}</span>
                            <span class="bg-primary/15 text-primary-light px-2 py-0.5 rounded-full text-[0.7rem] font-semibold">
                              {pct}% of 8h goal
                            </span>
                          </div>
                          <div class="text-xl font-extrabold text-accent-purple mb-2">
                            {formatMinutes(s.duration_minutes)}
                          </div>
                          <div class="text-xs text-text-muted space-y-0.5 mb-3">
                            <div>Bedtime: {formatDateTime(s.start_time)}</div>
                            <div>Wake: {formatDateTime(s.end_time)}</div>
                          </div>
                        </div>
                        <div class="h-1.5 w-full bg-primary/20 rounded-full overflow-hidden">
                          <div
                            class="h-full bg-primary rounded-full"
                            style={{ width: `${pct}%` }}
                          />
                        </div>
                      </div>
                    );
                  }}
                </For>
              </div>
            </Show>
          </div>
        </Show>

        {/* Tab 3: Weekly Trends View */}
        <Show when={activeTab() === 'trends'}>
          <div class="bento-card col-span-12">
            <div class="mb-4">
              <h3 class="text-base font-bold text-text-primary">Weekly Aggregated Trends</h3>
              <p class="text-xs text-text-muted mt-0.5">Periodic summary of activity metrics</p>
            </div>

            <Show
              when={trendsData().length > 0}
              fallback={<div class="text-text-muted text-xs py-8 text-center">No trend data available</div>}
            >
              <div class="overflow-x-auto">
                <table class="w-full text-left text-xs">
                  <thead>
                    <tr class="border-b border-border-default text-text-dim">
                      <th class="py-2.5 pr-4 font-semibold">PERIOD</th>
                      <th class="py-2.5 px-4 font-semibold">ACTIVE DAYS</th>
                      <th class="py-2.5 px-4 font-semibold text-right">TOTAL STEPS</th>
                      <th class="py-2.5 px-4 font-semibold text-right">DAILY AVG STEPS</th>
                      <th class="py-2.5 px-4 font-semibold text-right">TOTAL DISTANCE</th>
                      <th class="py-2.5 pl-4 font-semibold text-right">CALORIES</th>
                    </tr>
                  </thead>
                  <tbody class="divide-y border-border-default/50">
                    <For each={trendsData()}>
                      {(t) => (
                        <tr class="hover:bg-white/5 transition-colors">
                          <td class="py-3 pr-4 font-semibold text-text-primary">{t.period}</td>
                          <td class="py-3 px-4 text-text-muted">{t.days_count}</td>
                          <td class="py-3 px-4 font-semibold text-success text-right">
                            {formatSteps(t.steps_total)}
                          </td>
                          <td class="py-3 px-4 text-text-primary text-right">
                            {formatSteps(Math.round(t.steps_avg))}
                          </td>
                          <td class="py-3 px-4 text-text-muted text-right">
                            {formatDistance(t.distance_total)}
                          </td>
                          <td class="py-3 pl-4 text-warning font-semibold text-right">
                            {formatCalories(t.calories_total)}
                          </td>
                        </tr>
                      )}
                    </For>
                  </tbody>
                </table>
              </div>
            </Show>
          </div>
        </Show>
      </div>
    </div>
  );
}
