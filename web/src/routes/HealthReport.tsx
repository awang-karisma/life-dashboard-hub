import { createSignal, onMount, For, Show } from 'solid-js';
import { ApiClient } from '../api/client';
import { Chart } from '../components/Chart';
import { formatSteps, formatDistance, formatCalories, formatMinutes } from '../lib/format';
import { formatDate, formatDateTime, getDaysAgoISO, getTodayISO } from '../lib/dates';
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
            borderRadius: 4,
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
          y: { beginAtZero: true },
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
            backgroundColor: 'rgba(99, 102, 241, 0.1)',
            fill: true,
            tension: 0.3,
          },
        ],
      },
      options: {
        responsive: true,
        maintainAspectRatio: false,
        scales: {
          y: { beginAtZero: true, ticks: { callback: (val) => `${val}h` } },
        },
      },
    };
  };

  const totalSteps = () => dailyData().reduce((acc, c) => acc + c.steps, 0);
  const avgSteps = () => dailyData().length ? Math.round(totalSteps() / dailyData().length) : 0;
  const totalDistance = () => dailyData().reduce((acc, c) => acc + c.distance_meters, 0);
  const totalCalories = () => dailyData().reduce((acc, c) => acc + c.calories, 0);

  return (
    <div>
      <div style={{ 'margin-bottom': '1.5rem', display: 'flex', 'justify-content': 'space-between', 'align-items': 'center', 'flex-wrap': 'wrap', gap: '1rem' }}>
        <div>
          <h1 style={{ 'font-size': '1.75rem', 'font-weight': 700 }}>Health Connect Reports</h1>
          <p style={{ color: 'var(--text-muted)', 'font-size': '0.875rem' }}>
            Daily steps, distance, active calories, sleep duration, and trends
          </p>
        </div>

        <div style={{ display: 'flex', gap: '0.75rem', 'align-items': 'center' }}>
          <div>
            <label style={{ 'font-size': '0.75rem', color: 'var(--text-muted)', display: 'block' }}>From</label>
            <input type="date" class="input" value={from()} onInput={(e) => setFrom(e.currentTarget.value)} style={{ padding: '0.4rem 0.6rem' }} />
          </div>
          <div>
            <label style={{ 'font-size': '0.75rem', color: 'var(--text-muted)', display: 'block' }}>To</label>
            <input type="date" class="input" value={to()} onInput={(e) => setTo(e.currentTarget.value)} style={{ padding: '0.4rem 0.6rem' }} />
          </div>
          <button onClick={loadData} class="btn btn-primary" style={{ 'margin-top': '1.25rem', padding: '0.45rem 1rem' }}>
            Update
          </button>
        </div>
      </div>

      {/* Tabs */}
      <div style={{ display: 'flex', gap: '0.5rem', 'border-bottom': '1px solid var(--border)', 'margin-bottom': '1.5rem' }}>
        <button
          onClick={() => setActiveTab('activity')}
          class="btn"
          style={{
            background: 'none',
            border: 'none',
            'border-bottom': activeTab() === 'activity' ? '2px solid var(--primary)' : '2px solid transparent',
            color: activeTab() === 'activity' ? 'var(--primary)' : 'var(--text-muted)',
            'border-radius': 0,
            padding: '0.75rem 1.25rem',
          }}
        >
          🏃 Activity & Steps
        </button>
        <button
          onClick={() => setActiveTab('sleep')}
          class="btn"
          style={{
            background: 'none',
            border: 'none',
            'border-bottom': activeTab() === 'sleep' ? '2px solid var(--primary)' : '2px solid transparent',
            color: activeTab() === 'sleep' ? 'var(--primary)' : 'var(--text-muted)',
            'border-radius': 0,
            padding: '0.75rem 1.25rem',
          }}
        >
          🌙 Sleep
        </button>
        <button
          onClick={() => setActiveTab('trends')}
          class="btn"
          style={{
            background: 'none',
            border: 'none',
            'border-bottom': activeTab() === 'trends' ? '2px solid var(--primary)' : '2px solid transparent',
            color: activeTab() === 'trends' ? 'var(--primary)' : 'var(--text-muted)',
            'border-radius': 0,
            padding: '0.75rem 1.25rem',
          }}
        >
          📈 Weekly Trends
        </button>
      </div>

      <Show when={activeTab() === 'activity'}>
        <div class="grid-cols-4" style={{ 'margin-bottom': '1.5rem' }}>
          <div class="card">
            <div style={{ color: 'var(--text-muted)', 'font-size': '0.875rem' }}>Total Steps</div>
            <div style={{ 'font-size': '1.75rem', 'font-weight': 700, color: '#10b981', 'margin-top': '0.25rem' }}>
              {formatSteps(totalSteps())}
            </div>
          </div>
          <div class="card">
            <div style={{ color: 'var(--text-muted)', 'font-size': '0.875rem' }}>Daily Average</div>
            <div style={{ 'font-size': '1.75rem', 'font-weight': 700, 'margin-top': '0.25rem' }}>
              {formatSteps(avgSteps())}
            </div>
          </div>
          <div class="card">
            <div style={{ color: 'var(--text-muted)', 'font-size': '0.875rem' }}>Total Distance</div>
            <div style={{ 'font-size': '1.75rem', 'font-weight': 700, color: '#0284c7', 'margin-top': '0.25rem' }}>
              {formatDistance(totalDistance())}
            </div>
          </div>
          <div class="card">
            <div style={{ color: 'var(--text-muted)', 'font-size': '0.875rem' }}>Active Calories</div>
            <div style={{ 'font-size': '1.75rem', 'font-weight': 700, color: '#ea580c', 'margin-top': '0.25rem' }}>
              {formatCalories(totalCalories())}
            </div>
          </div>
        </div>

        <div class="card" style={{ 'margin-bottom': '1.5rem' }}>
          <h2 style={{ 'font-size': '1.125rem', 'font-weight': 600, 'margin-bottom': '1rem' }}>Daily Steps History</h2>
          <Show when={dailyData().length > 0} fallback={<div style={{ padding: '2rem', 'text-align': 'center', color: 'var(--text-muted)' }}>No step records in this range.</div>}>
            <Chart config={stepsChartConfig()} height={280} />
          </Show>
        </div>
      </Show>

      <Show when={activeTab() === 'sleep'}>
        <div class="card" style={{ 'margin-bottom': '1.5rem' }}>
          <h2 style={{ 'font-size': '1.125rem', 'font-weight': 600, 'margin-bottom': '1rem' }}>Sleep Duration Trend</h2>
          <Show when={sleepData().length > 0} fallback={<div style={{ padding: '2rem', 'text-align': 'center', color: 'var(--text-muted)' }}>No sleep records in this range.</div>}>
            <Chart config={sleepChartConfig()} height={260} />
          </Show>
        </div>

        <div class="card">
          <h2 style={{ 'font-size': '1.125rem', 'font-weight': 600, 'margin-bottom': '1rem' }}>Recent Sleep Sessions</h2>
          <Show when={sleepData().length > 0} fallback={<div style={{ padding: '2rem', 'text-align': 'center', color: 'var(--text-muted)' }}>No sleep data available.</div>}>
            <table class="table">
              <thead>
                <tr>
                  <th>Wake Date</th>
                  <th>Duration</th>
                  <th>Bedtime</th>
                  <th>Wake Time</th>
                </tr>
              </thead>
              <tbody>
                <For each={sleepData()}>
                  {(s) => (
                    <tr>
                      <td style={{ 'font-weight': 600 }}>{s.wake_date}</td>
                      <td>{formatMinutes(s.duration_minutes)}</td>
                      <td style={{ color: 'var(--text-muted)' }}>{formatDateTime(s.start_time)}</td>
                      <td style={{ color: 'var(--text-muted)' }}>{formatDateTime(s.end_time)}</td>
                    </tr>
                  )}
                </For>
              </tbody>
            </table>
          </Show>
        </div>
      </Show>

      <Show when={activeTab() === 'trends'}>
        <div class="card">
          <h2 style={{ 'font-size': '1.125rem', 'font-weight': 600, 'margin-bottom': '1rem' }}>Weekly Aggregate Summary</h2>
          <Show when={trendsData().length > 0} fallback={<div style={{ padding: '2rem', 'text-align': 'center', color: 'var(--text-muted)' }}>No weekly trend data available.</div>}>
            <table class="table">
              <thead>
                <tr>
                  <th>Period</th>
                  <th>Total Steps</th>
                  <th>Daily Avg Steps</th>
                  <th>Total Distance</th>
                  <th>Total Calories</th>
                  <th>Active Days</th>
                </tr>
              </thead>
              <tbody>
                <For each={trendsData()}>
                  {(t) => (
                    <tr>
                      <td style={{ 'font-weight': 600 }}>{t.period}</td>
                      <td style={{ color: '#10b981', 'font-weight': 600 }}>{formatSteps(t.steps_total)}</td>
                      <td>{formatSteps(Math.round(t.steps_avg))}</td>
                      <td>{formatDistance(t.distance_total)}</td>
                      <td>{formatCalories(t.calories_total)}</td>
                      <td>{t.days_count}</td>
                    </tr>
                  )}
                </For>
              </tbody>
            </table>
          </Show>
        </div>
      </Show>
    </div>
  );
}
