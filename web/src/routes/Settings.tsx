import { createSignal, onMount, Show } from 'solid-js';
import {
  IconUser,
  IconDownload,
  IconDatabase,
  IconAlertTriangle,
  IconFileSpreadsheet,
  IconFileCode,
  IconLock,
  IconTrash,
} from '@tabler/icons-solidjs';
import { ApiClient } from '../api/client';
import { showToast } from '../components/Toast';

interface UserProfile {
  id: number;
  email: string;
  display_name: string;
  timezone: string;
  auth_method: string;
}

export function Settings() {
  const [profile, setProfile] = createSignal<UserProfile | null>(null);
  const [displayName, setDisplayName] = createSignal('');
  const [timezone, setTimezone] = createSignal('UTC');
  const [currentPassword, setCurrentPassword] = createSignal('');
  const [newPassword, setNewPassword] = createSignal('');
  const [exportFrom, setExportFrom] = createSignal('');
  const [exportTo, setExportTo] = createSignal('');
  const [deleteFrom, setDeleteFrom] = createSignal('');
  const [deleteTo, setDeleteTo] = createSignal('');
  const [savingProfile, setSavingProfile] = createSignal(false);
  const [updatingPw, setUpdatingPw] = createSignal(false);

  onMount(async () => {
    try {
      const u = await ApiClient.get<UserProfile>('/api/v1/users/me');
      setProfile(u);
      setDisplayName(u.display_name || '');
      setTimezone(u.timezone || 'UTC');
    } catch {
      showToast('Failed to load profile', 'error');
    }
  });

  const handleUpdateProfile = async (e: Event) => {
    e.preventDefault();
    setSavingProfile(true);
    try {
      const updated = await ApiClient.patch<UserProfile>('/api/v1/users/me', {
        display_name: displayName(),
        timezone: timezone(),
      });
      setProfile(updated);
      showToast('Profile updated successfully', 'success');
    } catch (err: unknown) {
      showToast(err instanceof Error ? err.message : 'Failed to update profile', 'error');
    } finally {
      setSavingProfile(false);
    }
  };

  const handleChangePassword = async (e: Event) => {
    e.preventDefault();
    if (newPassword().length < 8) {
      showToast('New password must be at least 8 characters', 'error');
      return;
    }

    setUpdatingPw(true);
    try {
      await ApiClient.patch('/api/v1/users/me', {
        current_password: currentPassword(),
        new_password: newPassword(),
      });
      setCurrentPassword('');
      setNewPassword('');
      showToast('Password changed successfully', 'success');
    } catch (err: unknown) {
      showToast(err instanceof Error ? err.message : 'Failed to change password', 'error');
    } finally {
      setUpdatingPw(false);
    }
  };

  const handleExport = (format: 'csv' | 'json') => {
    let url = `/api/v1/data/export?format=${format}`;
    if (exportFrom()) url += `&from=${exportFrom()}`;
    if (exportTo()) url += `&to=${exportTo()}`;
    window.open(url, '_blank');
    showToast(`Downloading ${format.toUpperCase()} archive...`, 'info');
  };

  const handleDeleteData = async () => {
    if (!confirm('DANGER: This will permanently erase your usage data records in the selected range. Continue?')) {
      return;
    }

    try {
      let url = '/api/v1/data';
      const params = [];
      if (deleteFrom()) params.push(`from=${deleteFrom()}`);
      if (deleteTo()) params.push(`to=${deleteTo()}`);
      if (params.length) url += `?${params.join('&')}`;

      await ApiClient.delete(url);
      showToast('Data records permanently erased', 'success');
    } catch (err: unknown) {
      showToast(err instanceof Error ? err.message : 'Failed to delete data', 'error');
    }
  };

  return (
    <div class="flex flex-col gap-6">
      {/* Page Header */}
      <div>
        <h1 class="text-2xl font-extrabold text-text-primary tracking-tight">Preferences & Security</h1>
        <p class="text-xs text-text-muted mt-1">
          Personal credentials, data sovereignty archives, and package cache configuration
        </p>
      </div>

      {/* 2x2 Bento Grid */}
      <div class="bento-grid">
        {/* Card 1: User Profile & Security */}
        <div class="bento-card col-span-12 desktop:col-span-6 flex flex-col justify-between">
          <div>
            <div class="flex items-center gap-2 mb-4">
              <IconUser size={20} class="text-primary" />
              <h3 class="text-base font-bold text-text-primary">User Profile & Authentication</h3>
            </div>

            <form onSubmit={handleUpdateProfile} class="flex flex-col gap-4">
              <div>
                <label class="block text-xs font-semibold text-text-muted mb-1">
                  Email Address (Read-only)
                </label>
                <input
                  type="text"
                  disabled
                  value={profile()?.email || ''}
                  class="w-full bg-white/5 border border-border-default rounded-sm px-3 py-2 text-xs font-mono text-text-dim cursor-not-allowed"
                />
              </div>

              <div>
                <label class="block text-xs font-semibold text-text-muted mb-1">Display Name</label>
                <input
                  type="text"
                  value={displayName()}
                  onInput={(e) => setDisplayName(e.currentTarget.value)}
                  placeholder="Your Name"
                  class="w-full bg-bg-card border border-border-default rounded-sm px-3 py-2 text-xs text-text-primary focus:border-border-focus focus:outline-none"
                />
              </div>

              <div>
                <label class="block text-xs font-semibold text-text-muted mb-1">Timezone</label>
                <input
                  type="text"
                  value={timezone()}
                  onInput={(e) => setTimezone(e.currentTarget.value)}
                  placeholder="e.g. UTC, America/New_York"
                  class="w-full bg-bg-card border border-border-default rounded-sm px-3 py-2 text-xs text-text-primary focus:border-border-focus focus:outline-none font-mono"
                />
              </div>

              <button
                type="submit"
                disabled={savingProfile()}
                class="bg-primary hover:bg-primary-hover text-white px-4 py-2 rounded-sm font-semibold text-xs transition-all self-start cursor-pointer shadow-sm disabled:opacity-50"
              >
                {savingProfile() ? 'Saving...' : 'Save Profile Changes'}
              </button>
            </form>

            {/* Password Section */}
            <div class="mt-6 pt-5 border-t border-border-default">
              <div class="flex items-center gap-2 mb-3">
                <IconLock size={16} class="text-text-muted" />
                <h4 class="text-xs font-bold text-text-primary uppercase tracking-wider">
                  Password Management
                </h4>
              </div>

              <Show
                when={profile()?.auth_method === 'local'}
                fallback={
                  <div class="text-xs text-text-muted bg-white/5 rounded p-3 border border-border-default">
                    Authenticated via external SSO/OIDC provider. Password lifecycle managed by identity authority.
                  </div>
                }
              >
                <form onSubmit={handleChangePassword} class="flex flex-col gap-3">
                  <div>
                    <label class="block text-[0.72rem] text-text-muted mb-1">Current Password</label>
                    <input
                      type="password"
                      required
                      value={currentPassword()}
                      onInput={(e) => setCurrentPassword(e.currentTarget.value)}
                      class="w-full bg-bg-card border border-border-default rounded-sm px-3 py-1.5 text-xs text-text-primary focus:border-border-focus focus:outline-none"
                    />
                  </div>
                  <div>
                    <label class="block text-[0.72rem] text-text-muted mb-1">New Password (min 8 characters)</label>
                    <input
                      type="password"
                      required
                      minLength={8}
                      value={newPassword()}
                      onInput={(e) => setNewPassword(e.currentTarget.value)}
                      class="w-full bg-bg-card border border-border-default rounded-sm px-3 py-1.5 text-xs text-text-primary focus:border-border-focus focus:outline-none"
                    />
                  </div>
                  <button
                    type="submit"
                    disabled={updatingPw()}
                    class="bg-white/10 hover:bg-white/15 text-text-primary px-3 py-1.5 rounded-sm font-semibold text-xs border border-border-default transition-all self-start cursor-pointer disabled:opacity-50"
                  >
                    {updatingPw() ? 'Updating...' : 'Update Password'}
                  </button>
                </form>
              </Show>
            </div>
          </div>
        </div>

        {/* Card 2: Data Sovereignty & Export */}
        <div class="bento-card col-span-12 desktop:col-span-6 flex flex-col justify-between">
          <div>
            <div class="flex items-center gap-2 mb-4">
              <IconDownload size={20} class="text-accent-cyan" />
              <h3 class="text-base font-bold text-text-primary">Data Sovereignty & Export</h3>
            </div>

            <p class="text-xs text-text-muted leading-relaxed mb-4">
              Generate full cryptographic data dumps of your personal telemetry.
              All screen time sessions, health connect metrics, and device logs are provided in standard formats.
            </p>

            <div class="bg-white/5 border border-border-default rounded-md p-4 mb-5">
              <div class="text-xs font-semibold text-text-primary mb-2">Optional Date Window</div>
              <div class="flex flex-wrap items-center gap-3">
                <div>
                  <label class="block text-[0.7rem] text-text-dim mb-1">From</label>
                  <input
                    type="date"
                    value={exportFrom()}
                    onInput={(e) => setExportFrom(e.currentTarget.value)}
                    class="bg-bg-card border border-border-default rounded-sm px-2.5 py-1 text-xs text-text-primary"
                  />
                </div>
                <div>
                  <label class="block text-[0.7rem] text-text-dim mb-1">To</label>
                  <input
                    type="date"
                    value={exportTo()}
                    onInput={(e) => setExportTo(e.currentTarget.value)}
                    class="bg-bg-card border border-border-default rounded-sm px-2.5 py-1 text-xs text-text-primary"
                  />
                </div>
              </div>
            </div>

            <div class="flex flex-wrap items-center gap-3">
              <button
                type="button"
                onClick={() => handleExport('csv')}
                class="px-4 py-2 rounded-sm bg-white/5 hover:bg-white/10 text-text-primary text-xs font-semibold border border-border-default transition-all flex items-center gap-2 cursor-pointer"
              >
                <IconFileSpreadsheet size={16} class="text-success" />
                <span>Download CSV Archive</span>
              </button>

              <button
                type="button"
                onClick={() => handleExport('json')}
                class="px-4 py-2 rounded-sm bg-white/5 hover:bg-white/10 text-text-primary text-xs font-semibold border border-border-default transition-all flex items-center gap-2 cursor-pointer"
              >
                <IconFileCode size={16} class="text-accent-cyan" />
                <span>Download JSON Dump</span>
              </button>
            </div>
          </div>

          <div class="pt-3 mt-6 border-t border-border-default text-xs text-text-dim">
            Direct database streaming via gzip compression
          </div>
        </div>

        {/* Card 3: Package Resolver Cache */}
        <div class="bento-card col-span-12 desktop:col-span-6 flex flex-col justify-between">
          <div>
            <div class="flex items-center gap-2 mb-4">
              <IconDatabase size={20} class="text-accent-purple" />
              <h3 class="text-base font-bold text-text-primary">Hub Metadata & Resolver Cache</h3>
            </div>

            <p class="text-xs text-text-muted leading-relaxed mb-4">
              The Hub maintains an asynchronous Play Store package resolver cache to translate raw Android package IDs into human-readable titles, developer tags, and app categories.
            </p>

            <div class="space-y-3">
              <div class="p-3 rounded-md bg-white/5 border border-border-default flex items-center justify-between text-xs">
                <div>
                  <div class="font-semibold text-text-primary">Resolver Engine</div>
                  <div class="text-[0.7rem] text-text-dim">Autonomous worker pool with SQLite cache</div>
                </div>
                <span class="bg-success-bg text-success px-2 py-0.5 rounded-full text-[0.7rem] font-semibold">
                  Active
                </span>
              </div>

              <div class="p-3 rounded-md bg-white/5 border border-border-default flex items-center justify-between text-xs">
                <div>
                  <div class="font-semibold text-text-primary">Scraper Fallback Policy</div>
                  <div class="text-[0.7rem] text-text-dim">Non-blocking heuristics + reverse-domain formatter</div>
                </div>
                <span class="text-text-muted font-mono text-[0.7rem]">Graceful</span>
              </div>
            </div>
          </div>

          <div class="pt-3 mt-6 border-t border-border-default text-xs text-text-dim">
            Custom application renames take absolute precedence over scraper metadata
          </div>
        </div>

        {/* Card 4: Danger Zone */}
        <div class="bento-card col-span-12 desktop:col-span-6 border-danger/30 flex flex-col justify-between">
          <div>
            <div class="flex items-center gap-2 mb-4">
              <IconAlertTriangle size={20} class="text-danger" />
              <h3 class="text-base font-bold text-danger">Danger Zone</h3>
            </div>

            <p class="text-xs text-text-muted leading-relaxed mb-4">
              Irreversibly delete stored sync records from the Life Dashboard Hub database.
              Device configurations and account credentials will remain intact.
            </p>

            <div class="bg-danger-bg/40 border border-danger/20 rounded-md p-4 mb-4">
              <div class="text-xs font-semibold text-danger mb-2">Target Date Range (Optional)</div>
              <div class="flex flex-wrap items-center gap-3">
                <div>
                  <label class="block text-[0.7rem] text-text-dim mb-1">From</label>
                  <input
                    type="date"
                    value={deleteFrom()}
                    onInput={(e) => setDeleteFrom(e.currentTarget.value)}
                    class="bg-bg-card border border-border-default rounded-sm px-2.5 py-1 text-xs text-text-primary"
                  />
                </div>
                <div>
                  <label class="block text-[0.7rem] text-text-dim mb-1">To</label>
                  <input
                    type="date"
                    value={deleteTo()}
                    onInput={(e) => setDeleteTo(e.currentTarget.value)}
                    class="bg-bg-card border border-border-default rounded-sm px-2.5 py-1 text-xs text-text-primary"
                  />
                </div>
              </div>
            </div>

            <button
              type="button"
              onClick={handleDeleteData}
              class="bg-danger hover:bg-danger/80 text-white px-4 py-2 rounded-sm font-semibold text-xs transition-all flex items-center gap-2 cursor-pointer shadow-sm"
            >
              <IconTrash size={16} />
              <span>Permanently Delete Records</span>
            </button>
          </div>

          <div class="pt-3 mt-6 border-t border-danger/20 text-xs text-danger/80">
            Warning: Record deletion cannot be undone. Always export a backup first.
          </div>
        </div>
      </div>
    </div>
  );
}
