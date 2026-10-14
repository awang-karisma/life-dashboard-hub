import { createSignal, Show } from 'solid-js';
import { A, useNavigate, useLocation } from '@solidjs/router';
import {
  IconChartBar,
  IconDeviceMobile,
  IconHeartbeat,
  IconRouter,
  IconUsers,
  IconSettings,
  IconSun,
  IconMoon,
  IconRefresh,
  IconPlus,
  IconLayoutDashboard,
  IconLogout,
} from '@tabler/icons-solidjs';
import { ApiClient } from '../api/client';
import { theme, toggleTheme } from '../lib/theme';
import { showToast } from './Toast';

export interface NavProps {
  user?: {
    email: string;
    display_name: string;
  };
}

export function Nav(props: NavProps) {
  const navigate = useNavigate();
  const location = useLocation();
  const [syncing, setSyncing] = createSignal(false);

  const handleLogout = async () => {
    try {
      await ApiClient.post('/api/v1/auth/logout');
    } catch {
      // Continue redirect
    }
    navigate('/login');
  };

  const handleSync = () => {
    if (syncing()) return;
    setSyncing(true);
    setTimeout(() => {
      setSyncing(false);
      showToast('Fleet telemetry refreshed', 'success');
    }, 700);
  };
  const pageSubtitle = () => {
    const p = location.pathname;
    if (p === '/' || p === '') return 'Executive Overview';
    if (p.startsWith('/reports/screen-time')) return 'Screen Time Report';
    if (p.startsWith('/reports/health')) return 'Health & Vitals';
    if (p.startsWith('/devices')) return 'Fleet & Telemetry';
    if (p.startsWith('/family')) return 'Family Hub';
    if (p.startsWith('/settings')) return 'Preferences';
    return '';
  };

  return (
    <nav class="sticky top-4 z-40 backdrop-blur-[20px] bg-bg-card border border-border-default rounded-lg max-w-[1380px] mx-auto mb-5 px-5 py-3 flex items-center justify-between gap-4">
      {/* Brand */}
      <div class="flex items-center gap-3">
        <A href="/" class="flex items-center gap-3 no-underline">
          <div class="w-[38px] h-[38px] rounded-[10px] bg-gradient-to-br from-indigo-500 to-cyan-500 flex items-center justify-center shrink-0 shadow-sm">
            <IconLayoutDashboard size={20} color="white" />
          </div>
          <div class="flex flex-col">
            <span class="text-base font-extrabold text-text-primary tracking-tight leading-tight">
              Life Dashboard
            </span>
            <Show when={pageSubtitle()}>
              <span class="text-[0.7rem] text-text-muted leading-tight">
                {pageSubtitle()}
              </span>
            </Show>
          </div>
        </A>
      </div>

      {/* Center Nav Capsule */}
      <div class="hidden tablet:flex items-center gap-0.5 bg-black/20 rounded-xl p-1 border border-border-default">
        <A href="/" end class="nav-link" activeClass="nav-link-active">
          <IconChartBar size={16} />
          <span>Overview</span>
        </A>
        <A href="/reports/screen-time" class="nav-link" activeClass="nav-link-active">
          <IconDeviceMobile size={16} />
          <span>Screen Time</span>
        </A>
        <A href="/reports/health" class="nav-link" activeClass="nav-link-active">
          <IconHeartbeat size={16} />
          <span>Health Vitals</span>
        </A>
        <A href="/devices" class="nav-link" activeClass="nav-link-active">
          <IconRouter size={16} />
          <span>Fleet Devices</span>
        </A>
        <A href="/family" class="nav-link" activeClass="nav-link-active">
          <IconUsers size={16} />
          <span>Family</span>
        </A>
        <A href="/settings" class="nav-link" activeClass="nav-link-active">
          <IconSettings size={16} />
          <span>Settings</span>
        </A>
      </div>

      {/* Actions */}
      <div class="flex items-center gap-2">
        {/* Theme Toggle */}
        <button
          type="button"
          onClick={toggleTheme}
          title={theme() === 'dark' ? 'Switch to light mode' : 'Switch to dark mode'}
          class="p-2 rounded-md text-text-muted hover:text-text-primary hover:bg-white/5 transition-colors cursor-pointer border border-transparent hover:border-border-default"
        >
          <Show when={theme() === 'dark'} fallback={<IconMoon size={18} />}>
            <IconSun size={18} />
          </Show>
        </button>

        {/* Sync Trigger */}
        <button
          type="button"
          onClick={handleSync}
          title="Refresh telemetry"
          class={`p-2 rounded-md text-text-muted hover:text-text-primary hover:bg-white/5 transition-colors cursor-pointer border border-transparent hover:border-border-default ${
            syncing() ? 'animate-spin' : ''
          }`}
        >
          <IconRefresh size={18} />
        </button>

        {/* Context CTA */}
        <Show
          when={location.pathname.startsWith('/family')}
          fallback={
            <A
              href="/devices/new"
              class="bg-primary hover:bg-primary-hover text-white px-3.5 py-1.5 rounded-sm font-semibold text-xs transition-all hover:-translate-y-px flex items-center gap-1.5 no-underline shadow-sm"
            >
              <IconPlus size={15} />
              <span>Pair Device</span>
            </A>
          }
        >
          <button
            type="button"
            onClick={() => {
              const inviteBtn = document.getElementById('open-invite-modal-btn');
              if (inviteBtn) inviteBtn.click();
              else showToast('Create a family first to invite members', 'info');
            }}
            class="bg-primary hover:bg-primary-hover text-white px-3.5 py-1.5 rounded-sm font-semibold text-xs transition-all hover:-translate-y-px flex items-center gap-1.5 cursor-pointer shadow-sm"
          >
            <IconPlus size={15} />
            <span>Invite Member</span>
          </button>
        </Show>

        {/* User / Logout */}
        <Show when={props.user}>
          <button
            type="button"
            onClick={handleLogout}
            title={`Log out (${props.user?.display_name || props.user?.email})`}
            class="p-2 rounded-md text-text-muted hover:text-danger hover:bg-danger-bg transition-colors cursor-pointer border border-transparent"
          >
            <IconLogout size={18} />
          </button>
        </Show>
      </div>
    </nav>
  );
}
