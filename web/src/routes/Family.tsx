import { createSignal, onMount, For, Show } from 'solid-js';
import {
  IconUsers,
  IconShield,
  IconUserPlus,
  IconBabyCarriage,
  IconClock,
  IconPlus,
  IconCopy,
  IconCheck,
  IconPencil,
  IconTrash,
  IconX,
  IconSettings,
  IconArrowLeft,
  IconEye,
  IconShoe,
  IconFlame,
  IconActivity,
  IconDeviceMobile,
} from '@tabler/icons-solidjs';
import { ApiClient } from '../api/client';
import { Modal } from '../components/Modal';
import { showToast } from '../components/Toast';
import { formatMinutes } from '../lib/format';
import { getDaysAgoISO, getTodayISO, formatDate } from '../lib/dates';

interface CurrentUser {
  id: number;
  email: string;
  display_name: string;
}

interface FamilyMember {
  user_id: number;
  display_name: string;
  email: string;
  role: string;
  sharing: string;
  status: string;
  joined_at: string;
}

interface FamilyItem {
  id: number;
  name: string;
  role: string;
  sharing: string;
  members?: FamilyMember[];
}

interface FamilySettingsData {
  id?: number;
  family_id?: number;
  members_can_view_others: boolean;
}

interface MemberDailyReport {
  from: string;
  to: string;
  days: { date: string; total_screen_time_minutes: number }[];
}

interface MemberAppUsage {
  package_name: string;
  app_name: string;
  category: string;
  minutes: number;
}

interface MemberHealthDaily {
  date: string;
  steps: number;
  distance_meters: number;
  calories: number;
}

export function Family() {
  const [currentUser, setCurrentUser] = createSignal<CurrentUser | null>(null);
  const [families, setFamilies] = createSignal<FamilyItem[]>([]);
  const [selectedFamily, setSelectedFamily] = createSignal<FamilyItem | null>(null);
  const [familySettings, setFamilySettings] = createSignal<FamilySettingsData | null>(null);
  const [newFamilyName, setNewFamilyName] = createSignal('');
  const [renameName, setRenameName] = createSignal('');
  const [inviteEmail, setInviteEmail] = createSignal('');
  const [inviteRole, setInviteRole] = createSignal('adult');
  const [inviteLink, setInviteLink] = createSignal('');
  const [copiedLink, setCopiedLink] = createSignal(false);
  const [childName, setChildName] = createSignal('');
  const [childPassword, setChildPassword] = createSignal('');
  const [loading, setLoading] = createSignal(true);
  const [memberScreenTime, setMemberScreenTime] = createSignal<{ [userId: number]: number }>({});

  // Modals
  const [showInviteModal, setShowInviteModal] = createSignal(false);
  const [showChildModal, setShowChildModal] = createSignal(false);
  const [showCreateFamilyModal, setShowCreateFamilyModal] = createSignal(false);
  const [showRenameModal, setShowRenameModal] = createSignal(false);

  // Member telemetry drill-down
  const [viewingMember, setViewingMember] = createSignal<FamilyMember | null>(null);
  const [memberReportDays, setMemberReportDays] = createSignal<{ date: string; total_screen_time_minutes: number }[]>([]);
  const [memberApps, setMemberApps] = createSignal<MemberAppUsage[]>([]);
  const [memberHealth, setMemberHealth] = createSignal<MemberHealthDaily[]>([]);
  const [memberDataLoading, setMemberDataLoading] = createSignal(false);
  const [memberDateRange, setMemberDateRange] = createSignal(7); // days

  const loadFamilies = async () => {
    try {
      const list = await ApiClient.get<FamilyItem[]>('/api/v1/families');
      setFamilies(list || []);
      if (list && list.length > 0) {
        await selectFamily(list[0].id);
      } else {
        setSelectedFamily(null);
        setFamilySettings(null);
      }
    } catch {
      showToast('Failed to load families', 'error');
    } finally {
      setLoading(false);
    }
  };

  const selectFamily = async (id: number) => {
    try {
      const fam = await ApiClient.get<FamilyItem>(`/api/v1/families/${id}`);
      setSelectedFamily(fam);

      // Load family settings
      try {
        const s = await ApiClient.get<FamilySettingsData>(`/api/v1/families/${fam.id}/settings`);
        setFamilySettings(s);
      } catch {
        setFamilySettings(null);
      }

      if (fam.members) {
        for (const m of fam.members) {
          if (m.sharing !== 'none' || m.role === 'child') {
            try {
              const st = await ApiClient.get<any>(
                `/api/v1/families/${fam.id}/members/${m.user_id}/reports/screen-time/daily`
              );
              if (st && st.days && st.days.length > 0) {
                const latest = st.days[st.days.length - 1];
                setMemberScreenTime((prev) => ({
                  ...prev,
                  [m.user_id]: latest.total_screen_time_minutes,
                }));
              }
            } catch {
              // Not permitted or no data
            }
          }
        }
      }
    } catch {
      showToast('Failed to select household', 'error');
    }
  };

  onMount(async () => {
    try {
      const u = await ApiClient.get<CurrentUser>('/api/v1/users/me');
      setCurrentUser(u);
    } catch {
      // Handled by global auth redirect
    }
    await loadFamilies();
  });

  const handleCreateFamily = async (e: Event) => {
    e.preventDefault();
    if (!newFamilyName()) return;
    try {
      await ApiClient.post('/api/v1/families', { name: newFamilyName() });
      showToast('Family created successfully', 'success');
      setNewFamilyName('');
      setShowCreateFamilyModal(false);
      await loadFamilies();
    } catch (err: unknown) {
      showToast(err instanceof Error ? err.message : 'Failed to create family', 'error');
    }
  };

  const handleRenameFamily = async (e: Event) => {
    e.preventDefault();
    const fam = selectedFamily();
    const name = renameName().trim();
    if (!fam || !name) return;
    try {
      await ApiClient.patch(`/api/v1/families/${fam.id}`, { name });
      showToast('Family renamed successfully', 'success');
      setShowRenameModal(false);
      await loadFamilies();
    } catch (err: unknown) {
      showToast(err instanceof Error ? err.message : 'Failed to rename family', 'error');
    }
  };

  const handleDeleteFamily = async () => {
    const fam = selectedFamily();
    if (!fam) return;
    if (!window.confirm('Are you sure? This will remove all members.')) return;
    try {
      await ApiClient.delete(`/api/v1/families/${fam.id}`);
      showToast('Family deleted', 'success');
      setSelectedFamily(null);
      setViewingMember(null);
      await loadFamilies();
    } catch (err: unknown) {
      showToast(err instanceof Error ? err.message : 'Failed to delete family', 'error');
    }
  };

  const handleRemoveMember = async (userId: number) => {
    const fam = selectedFamily();
    if (!fam) return;
    if (!window.confirm('Are you sure you want to remove this member?')) return;
    try {
      await ApiClient.delete(`/api/v1/families/${fam.id}/members/${userId}`);
      showToast('Member removed', 'success');
      if (viewingMember()?.user_id === userId) {
        setViewingMember(null);
      }
      await selectFamily(fam.id);
    } catch (err: unknown) {
      showToast(err instanceof Error ? err.message : 'Failed to remove member', 'error');
    }
  };

  const handleUpdateSettings = async (key: string, value: boolean) => {
    const fam = selectedFamily();
    if (!fam) return;
    try {
      await ApiClient.patch(`/api/v1/families/${fam.id}/settings`, {
        [key]: value,
      });
      setFamilySettings((prev) => (prev ? { ...prev, [key]: value } : { members_can_view_others: value }));
      showToast('Family settings updated', 'success');
    } catch (err: unknown) {
      showToast(err instanceof Error ? err.message : 'Failed to update settings', 'error');
    }
  };

  const handleInvite = async (e: Event) => {
    e.preventDefault();
    const fam = selectedFamily();
    if (!fam) return;

    try {
      const res = await ApiClient.post<any>(`/api/v1/families/${fam.id}/invitations`, {
        email: inviteEmail(),
        role: inviteRole(),
      });
      if (res && res.invite_token) {
        const link = `${window.location.origin}/invite/${res.invite_token}`;
        setInviteLink(link);
      }
      showToast('Invitation token generated', 'success');
      setInviteEmail('');
    } catch (err: unknown) {
      showToast(err instanceof Error ? err.message : 'Failed to create invitation', 'error');
    }
  };

  const handleCreateChild = async (e: Event) => {
    e.preventDefault();
    const fam = selectedFamily();
    if (!fam) return;

    try {
      await ApiClient.post(`/api/v1/families/${fam.id}/children`, {
        display_name: childName(),
        password: childPassword(),
      });
      showToast(`Child account "${childName()}" created`, 'success');
      setChildName('');
      setChildPassword('');
      setShowChildModal(false);
      await selectFamily(fam.id);
    } catch (err: unknown) {
      showToast(err instanceof Error ? err.message : 'Failed to create child account', 'error');
    }
  };

  const handleUpdateSharing = async (userId: number, newSharing: string) => {
    const fam = selectedFamily();
    if (!fam) return;

    try {
      await ApiClient.patch(`/api/v1/families/${fam.id}/members/${userId}/sharing`, {
        sharing: newSharing,
      });
      showToast('Sharing level updated', 'success');
      await selectFamily(fam.id);
    } catch (err: unknown) {
      showToast(err instanceof Error ? err.message : 'Failed to update sharing', 'error');
    }
  };

  const copyInviteLink = () => {
    if (!inviteLink()) return;
    navigator.clipboard.writeText(inviteLink());
    setCopiedLink(true);
    showToast('Invitation link copied to clipboard', 'success');
    setTimeout(() => setCopiedLink(false), 2500);
  };

  const canViewMember = (m: FamilyMember) => {
    if (m.user_id === currentUser()?.id) return true;
    const role = selectedFamily()?.role;
    if (role === 'admin' || role === 'parent') return true;
    if (familySettings()?.members_can_view_others && m.sharing !== 'none') return true;
    return false;
  };

  const loadMemberData = async (m: FamilyMember, days: number = memberDateRange()) => {
    const fam = selectedFamily();
    if (!fam) return;
    setMemberDataLoading(true);
    const from = getDaysAgoISO(days);
    const to = getTodayISO();

    try {
      const stPromise = ApiClient.get<MemberDailyReport>(
        `/api/v1/families/${fam.id}/members/${m.user_id}/reports/screen-time/daily?from=${from}&to=${to}`
      ).catch(() => null);

      const appsPromise = ApiClient.get<any>(
        `/api/v1/families/${fam.id}/members/${m.user_id}/reports/screen-time/apps?from=${from}&to=${to}&limit=100`
      ).catch(() => null);

      const healthPromise = ApiClient.get<MemberHealthDaily[]>(
        `/api/v1/families/${fam.id}/members/${m.user_id}/reports/health/daily?from=${from}&to=${to}`
      ).catch(() => null);

      const [stRes, appsRes, healthRes] = await Promise.all([stPromise, appsPromise, healthPromise]);

      setMemberReportDays(stRes?.days || []);
      setMemberApps(Array.isArray(appsRes) ? appsRes : appsRes?.items || []);
      setMemberHealth(Array.isArray(healthRes) ? healthRes : []);
    } catch {
      showToast('Failed to load member telemetry data', 'error');
    } finally {
      setMemberDataLoading(false);
    }
  };

  const handleViewMember = (m: FamilyMember) => {
    setViewingMember(m);
    loadMemberData(m);
  };

  const totalMemberScreenTimeMins = () => {
    return memberReportDays().reduce((acc, d) => acc + (d.total_screen_time_minutes || 0), 0);
  };

  const totalMemberSteps = () => {
    return memberHealth().reduce((acc, h) => acc + (h.steps || 0), 0);
  };

  const totalMemberCalories = () => {
    return memberHealth().reduce((acc, h) => acc + (h.calories || 0), 0);
  };

  const totalMemberDistanceKm = () => {
    const meters = memberHealth().reduce((acc, h) => acc + (h.distance_meters || 0), 0);
    return (meters / 1000).toFixed(1);
  };

  const getAvatarColor = (name: string) => {
    const colors = ['bg-indigo-600', 'bg-purple-600', 'bg-cyan-600', 'bg-emerald-600', 'bg-amber-600', 'bg-rose-600'];
    let hash = 0;
    for (let i = 0; i < name.length; i++) hash = name.charCodeAt(i) + ((hash << 5) - hash);
    return colors[Math.abs(hash) % colors.length];
  };

  const getInitials = (name: string) => {
    const parts = name.trim().split(' ');
    if (parts.length >= 2) return `${parts[0][0]}${parts[1][0]}`.toUpperCase();
    return (name.slice(0, 2) || '??').toUpperCase();
  };

  return (
    <div class="flex flex-col gap-6">
      {/* Household Header */}
      <div class="flex flex-wrap items-center justify-between gap-4">
        <div>
          <div class="flex items-center gap-3">
            <h1 class="text-2xl font-extrabold text-text-primary tracking-tight">
              {selectedFamily()?.name || 'Family Hub'}
            </h1>

            {/* Rename Family Icon (Admin only) */}
            <Show when={selectedFamily()?.role === 'admin'}>
              <button
                type="button"
                onClick={() => {
                  setRenameName(selectedFamily()?.name || '');
                  setShowRenameModal(true);
                }}
                title="Rename Family"
                class="p-1 rounded-sm text-text-muted hover:text-text-primary hover:bg-white/5 transition-all cursor-pointer"
              >
                <IconPencil size={16} />
              </button>
            </Show>

            <Show when={families().length > 1}>
              <select
                value={selectedFamily()?.id || ''}
                onChange={(e) => selectFamily(Number(e.currentTarget.value))}
                class="bg-bg-card border border-border-default rounded-sm px-2.5 py-1 text-xs text-text-primary"
              >
                <For each={families()}>
                  {(f) => <option value={f.id}>{f.name}</option>}
                </For>
              </select>
            </Show>
          </div>
          <p class="text-xs text-text-muted mt-1">
            Household member roles, sharing boundaries, and telemetry permissions
          </p>
        </div>

        <div class="flex items-center gap-2">
          {/* New Family button: only visible when user has no family */}
          <Show when={families().length === 0}>
            <button
              type="button"
              onClick={() => setShowCreateFamilyModal(true)}
              class="px-3 py-1.5 rounded-sm bg-white/5 hover:bg-white/10 text-text-muted hover:text-text-primary text-xs font-semibold border border-border-default transition-all flex items-center gap-1.5 cursor-pointer"
            >
              <IconPlus size={15} />
              <span>New Family</span>
            </button>
          </Show>

          {/* Member actions and admin controls: visible only when a family exists */}
          <Show when={selectedFamily()}>
            {/* Delete Family Button (Admin only) */}
            <Show when={selectedFamily()?.role === 'admin'}>
              <button
                type="button"
                onClick={handleDeleteFamily}
                class="px-3 py-1.5 rounded-sm bg-danger/10 hover:bg-danger/20 text-danger text-xs font-semibold border border-danger/30 transition-all flex items-center gap-1.5 cursor-pointer"
              >
                <IconTrash size={15} />
                <span>Delete Family</span>
              </button>
            </Show>

            <button
              type="button"
              onClick={() => setShowChildModal(true)}
              class="px-3 py-1.5 rounded-sm bg-white/5 hover:bg-white/10 text-text-primary text-xs font-semibold border border-border-default transition-all flex items-center gap-1.5 cursor-pointer"
            >
              <IconBabyCarriage size={16} />
              <span>Create Child Account</span>
            </button>

            <button
              type="button"
              id="open-invite-modal-btn"
              onClick={() => setShowInviteModal(true)}
              class="bg-primary hover:bg-primary-hover text-white px-3.5 py-1.5 rounded-sm font-semibold text-xs transition-all flex items-center gap-1.5 cursor-pointer shadow-sm"
            >
              <IconUserPlus size={15} />
              <span>Invite Member</span>
            </button>
          </Show>
        </div>
      </div>

      {/* Main Content Area: Either Household Members or Viewing Specific Member */}
      <Show
        when={!viewingMember()}
        fallback={
          /* Member Dashboard Section */
          <div class="flex flex-col gap-6">
            <div class="bento-card col-span-12">
              <div class="flex flex-wrap items-center justify-between gap-4 pb-4 border-b border-border-default">
                <div class="flex items-center gap-3">
                  <button
                    type="button"
                    onClick={() => setViewingMember(null)}
                    class="px-2.5 py-1.5 rounded-sm bg-white/5 hover:bg-white/10 text-text-muted hover:text-text-primary text-xs font-semibold border border-border-default transition-all flex items-center gap-1.5 cursor-pointer"
                  >
                    <IconArrowLeft size={15} />
                    <span>Back to Members</span>
                  </button>

                  <div
                    class={`w-9 h-9 rounded-full flex items-center justify-center text-white font-bold text-xs shrink-0 shadow-sm ${getAvatarColor(
                      viewingMember()?.display_name || viewingMember()?.email || ''
                    )}`}
                  >
                    {getInitials(viewingMember()?.display_name || viewingMember()?.email || '')}
                  </div>

                  <div>
                    <div class="flex items-center gap-2">
                      <h2 class="text-base font-bold text-text-primary">
                        {viewingMember()?.display_name || viewingMember()?.email}
                      </h2>
                      <span class="rounded-full px-2 py-0.5 text-[0.68rem] font-semibold bg-white/10 text-text-muted">
                        {viewingMember()?.role}
                      </span>
                    </div>
                    <div class="text-xs text-text-muted font-mono">{viewingMember()?.email}</div>
                  </div>
                </div>

                {/* Date Range Selector */}
                <div class="flex items-center gap-1 bg-bg-card p-0.5 rounded-sm border border-border-default">
                  <For each={[7, 14, 30]}>
                    {(days) => (
                      <button
                        type="button"
                        onClick={() => {
                          setMemberDateRange(days);
                          if (viewingMember()) loadMemberData(viewingMember()!, days);
                        }}
                        class={`px-2.5 py-1 text-xs rounded-sm font-semibold transition-all cursor-pointer ${
                          memberDateRange() === days
                            ? 'bg-primary text-white shadow-sm'
                            : 'text-text-muted hover:text-text-primary'
                        }`}
                      >
                        {days}D
                      </button>
                    )}
                  </For>
                </div>
              </div>

              {/* Telemetry KPIs */}
              <Show
                when={!memberDataLoading()}
                fallback={<div class="py-12 text-center text-xs text-text-muted">Loading telemetry data...</div>}
              >
                <div class="grid grid-cols-2 md:grid-cols-4 gap-4 mt-4">
                  <div class="bg-bg-card-elevated border border-border-default rounded-md p-3.5">
                    <div class="flex items-center justify-between text-text-dim text-[0.7rem] uppercase tracking-wider mb-1">
                      <span>Screen Time</span>
                      <IconClock size={15} class="text-primary" />
                    </div>
                    <div class="text-lg font-extrabold text-text-primary">
                      {formatMinutes(totalMemberScreenTimeMins())}
                    </div>
                    <div class="text-[0.68rem] text-text-muted mt-0.5">Last {memberDateRange()} days total</div>
                  </div>

                  <div class="bg-bg-card-elevated border border-border-default rounded-md p-3.5">
                    <div class="flex items-center justify-between text-text-dim text-[0.7rem] uppercase tracking-wider mb-1">
                      <span>Total Steps</span>
                      <IconShoe size={15} class="text-cyan-400" />
                    </div>
                    <div class="text-lg font-extrabold text-text-primary">
                      {totalMemberSteps().toLocaleString()}
                    </div>
                    <div class="text-[0.68rem] text-text-muted mt-0.5">Across recorded days</div>
                  </div>

                  <div class="bg-bg-card-elevated border border-border-default rounded-md p-3.5">
                    <div class="flex items-center justify-between text-text-dim text-[0.7rem] uppercase tracking-wider mb-1">
                      <span>Active Calories</span>
                      <IconFlame size={15} class="text-amber-400" />
                    </div>
                    <div class="text-lg font-extrabold text-text-primary">
                      {Math.round(totalMemberCalories()).toLocaleString()} kcal
                    </div>
                    <div class="text-[0.68rem] text-text-muted mt-0.5">Estimated burn</div>
                  </div>

                  <div class="bg-bg-card-elevated border border-border-default rounded-md p-3.5">
                    <div class="flex items-center justify-between text-text-dim text-[0.7rem] uppercase tracking-wider mb-1">
                      <span>Distance</span>
                      <IconActivity size={15} class="text-emerald-400" />
                    </div>
                    <div class="text-lg font-extrabold text-text-primary">
                      {totalMemberDistanceKm()} km
                    </div>
                    <div class="text-[0.68rem] text-text-muted mt-0.5">Walking / running</div>
                  </div>
                </div>

                {/* Daily Breakdown and Application Inventory */}
                <div class="grid grid-cols-1 lg:grid-cols-12 gap-6 mt-6">
                  {/* Daily Screen Time */}
                  <div class="lg:col-span-5 flex flex-col gap-3">
                    <h3 class="text-sm font-bold text-text-primary flex items-center gap-1.5">
                      <IconClock size={16} class="text-primary" />
                      <span>Daily Screen Time</span>
                    </h3>
                    <div class="bg-bg-card-elevated border border-border-default rounded-md p-3 flex flex-col gap-2 max-h-[360px] overflow-y-auto">
                      <Show
                        when={memberReportDays().length > 0}
                        fallback={<div class="text-xs text-text-muted py-6 text-center">No daily records for this range</div>}
                      >
                        <For each={memberReportDays()}>
                          {(day) => (
                            <div class="flex items-center justify-between text-xs py-1 border-b border-border-default last:border-b-0">
                              <span class="text-text-muted font-mono">{formatDate(day.date)}</span>
                              <span class="font-bold text-text-primary">
                                {formatMinutes(day.total_screen_time_minutes)}
                              </span>
                            </div>
                          )}
                        </For>
                      </Show>
                    </div>
                  </div>

                  {/* Application Inventory */}
                  <div class="lg:col-span-7 flex flex-col gap-3">
                    <h3 class="text-sm font-bold text-text-primary flex items-center gap-1.5">
                      <IconDeviceMobile size={16} class="text-primary" />
                      <span>App Usage Inventory</span>
                      <span class="text-xs text-text-dim font-normal">({memberApps().length} apps)</span>
                    </h3>
                    <div class="bg-bg-card-elevated border border-border-default rounded-md overflow-hidden max-h-[360px] overflow-y-auto">
                      <Show
                        when={memberApps().length > 0}
                        fallback={<div class="text-xs text-text-muted py-8 text-center">No app usage records for this range</div>}
                      >
                        <table class="w-full text-left text-xs">
                          <thead class="bg-white/5 border-b border-border-default text-text-dim uppercase text-[0.68rem] tracking-wider sticky top-0">
                            <tr>
                              <th class="py-2 px-3">Application</th>
                              <th class="py-2 px-3">Category</th>
                              <th class="py-2 px-3 text-right">Usage</th>
                            </tr>
                          </thead>
                          <tbody class="divide-y divide-border-default">
                            <For each={memberApps()}>
                              {(app) => (
                                <tr class="hover:bg-white/[0.02]">
                                  <td class="py-2 px-3">
                                    <div class="font-semibold text-text-primary">
                                      {app.app_name || app.package_name}
                                    </div>
                                    <div class="text-[0.65rem] text-text-dim font-mono">{app.package_name}</div>
                                  </td>
                                  <td class="py-2 px-3 text-text-muted capitalize">
                                    {app.category || 'General'}
                                  </td>
                                  <td class="py-2 px-3 text-right font-bold text-text-primary whitespace-nowrap">
                                    {formatMinutes(app.minutes)}
                                  </td>
                                </tr>
                              )}
                            </For>
                          </tbody>
                        </table>
                      </Show>
                    </div>
                  </div>
                </div>
              </Show>
            </div>
          </div>
        }
      >
        {/* Members Grid */}
        <div class="bento-grid">
          <div class="bento-card col-span-12">
            <div class="flex items-center justify-between mb-4">
              <div class="flex items-center gap-2">
                <IconUsers size={20} class="text-primary" />
                <h3 class="text-base font-bold text-text-primary">Household Members</h3>
              </div>
              <span class="text-xs text-text-dim">
                {selectedFamily()?.members?.length || 0} active accounts
              </span>
            </div>

            <Show
              when={(selectedFamily()?.members?.length || 0) > 0}
              fallback={
                <div class="text-text-muted text-xs py-12 text-center flex flex-col items-center justify-center gap-3">
                  <IconUsers size={32} class="text-text-dim opacity-50" />
                  <span>No members in this family yet.</span>
                  <Show when={selectedFamily()}>
                    <button
                      type="button"
                      onClick={() => setShowInviteModal(true)}
                      class="text-primary-light hover:text-primary font-semibold text-xs cursor-pointer"
                    >
                      Invite your first member &rarr;
                    </button>
                  </Show>
                </div>
              }
            >
              <div class="flex flex-col gap-3">
                <For each={selectedFamily()?.members}>
                  {(m) => {
                    const displayName = m.display_name || m.email;
                    const isChild = m.role === 'child';
                    const isOrganizer = m.role === 'organizer' || m.role === 'admin';
                    const stMins = memberScreenTime()[m.user_id];

                    return (
                      <div class="bg-bg-card-elevated border border-border-default rounded-md p-4 flex flex-wrap items-center justify-between gap-4 transition-all hover:border-border-highlight">
                        <div class="flex items-center gap-3.5">
                          <div
                            class={`w-10 h-10 rounded-full flex items-center justify-center text-white font-bold text-sm shrink-0 shadow-sm ${getAvatarColor(
                              displayName
                            )}`}
                          >
                            {getInitials(displayName)}
                          </div>

                          <div>
                            <div class="flex items-center gap-2">
                              <span class="font-bold text-text-primary text-sm">{displayName}</span>
                              <span
                                class={`rounded-full px-2 py-0.5 text-[0.68rem] font-semibold flex items-center gap-1 ${
                                  isOrganizer
                                    ? 'bg-primary/20 text-primary-light'
                                    : isChild
                                    ? 'bg-amber-500/20 text-amber-300'
                                    : 'bg-white/10 text-text-muted'
                                }`}
                              >
                                <Show
                                  when={isChild}
                                  fallback={<IconShield size={12} />}
                                >
                                  <IconBabyCarriage size={12} />
                                </Show>
                                <span>{m.role}</span>
                              </span>
                            </div>
                            <div class="text-xs text-text-muted mt-0.5 font-mono">{m.email}</div>
                          </div>
                        </div>

                        <div class="flex items-center gap-4 flex-wrap">
                          {/* Today's Screen Time */}
                          <div class="text-right">
                            <div class="text-[0.7rem] text-text-dim uppercase tracking-wider flex items-center gap-1 justify-end">
                              <IconClock size={12} />
                              <span>Today's Time</span>
                            </div>
                            <div class="text-xs font-semibold text-text-primary mt-0.5">
                              {stMins !== undefined ? formatMinutes(stMins) : 'Hidden'}
                            </div>
                          </div>

                          {/* Sharing Level Selector */}
                          <div>
                            <label class="block text-[0.7rem] text-text-dim uppercase tracking-wider mb-1">
                              Sharing
                            </label>
                            <select
                              value={m.sharing}
                              onChange={(e) => handleUpdateSharing(m.user_id, e.currentTarget.value)}
                              class="bg-bg-card border border-border-default rounded-sm px-2 py-1 text-xs text-text-primary focus:border-border-focus focus:outline-none cursor-pointer"
                            >
                              <option value="none">None (Private)</option>
                              <option value="summary">Summary</option>
                              <option value="detailed">Full Details</option>
                            </select>
                          </div>

                          {/* View Member Data Button */}
                          <Show when={canViewMember(m)}>
                            <button
                              type="button"
                              onClick={() => handleViewMember(m)}
                              class="px-2.5 py-1 rounded-sm bg-white/5 hover:bg-white/10 text-text-muted hover:text-text-primary text-xs font-semibold border border-border-default transition-all flex items-center gap-1 cursor-pointer"
                            >
                              <IconEye size={14} />
                              <span>View Data</span>
                            </button>
                          </Show>

                          {/* Remove Member Button (Admin only, not self) */}
                          <Show when={selectedFamily()?.role === 'admin' && m.user_id !== currentUser()?.id}>
                            <button
                              type="button"
                              onClick={() => handleRemoveMember(m.user_id)}
                              title="Remove member"
                              class="p-1 rounded-sm text-text-muted hover:text-danger hover:bg-danger/10 transition-colors cursor-pointer"
                            >
                              <IconX size={15} />
                            </button>
                          </Show>
                        </div>
                      </div>
                    );
                  }}
                </For>
              </div>
            </Show>
          </div>
        </div>

        {/* Family Settings Panel (Admin only) */}
        <Show when={selectedFamily() && selectedFamily()?.role === 'admin'}>
          <div class="bento-grid">
            <div class="bento-card col-span-12">
              <div class="flex items-center gap-2 mb-4">
                <IconSettings size={20} class="text-primary" />
                <h3 class="text-base font-bold text-text-primary">Family Settings</h3>
              </div>
              <div class="flex items-center justify-between p-4 bg-bg-card-elevated border border-border-default rounded-md">
                <div>
                  <div class="text-sm font-semibold text-text-primary">
                    Allow members to view each other's data
                  </div>
                  <div class="text-xs text-text-muted mt-0.5">
                    When enabled, any family member can view telemetry reports of other members whose sharing is not set to private.
                  </div>
                </div>
                <label class="relative inline-flex items-center cursor-pointer">
                  <input
                    type="checkbox"
                    checked={familySettings()?.members_can_view_others ?? false}
                    onChange={(e) => handleUpdateSettings('members_can_view_others', e.currentTarget.checked)}
                    class="sr-only peer"
                  />
                  <div class="w-11 h-6 bg-white/10 peer-focus:outline-none rounded-full peer peer-checked:after:translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-white after:border-gray-300 after:border after:rounded-full after:h-5 after:w-5 after:transition-all peer-checked:bg-primary"></div>
                </label>
              </div>
            </div>
          </div>
        </Show>
      </Show>

      {/* Rename Family Modal */}
      <Modal
        open={showRenameModal()}
        onClose={() => setShowRenameModal(false)}
        title="Rename Family"
      >
        <form onSubmit={handleRenameFamily} class="flex flex-col gap-4">
          <div>
            <label class="block text-xs font-semibold text-text-muted mb-1">
              Family Name
            </label>
            <input
              type="text"
              required
              value={renameName()}
              onInput={(e) => setRenameName(e.currentTarget.value)}
              placeholder="e.g. The Smiths"
              class="w-full bg-bg-card border border-border-default rounded-sm px-3 py-2 text-sm text-text-primary focus:border-border-focus focus:outline-none"
            />
          </div>

          <div class="flex justify-end gap-2 mt-2">
            <button
              type="button"
              onClick={() => setShowRenameModal(false)}
              class="px-4 py-2 rounded-sm text-xs font-semibold text-text-muted hover:text-text-primary hover:bg-white/5 border border-border-default transition-all cursor-pointer"
            >
              Cancel
            </button>
            <button
              type="submit"
              class="bg-primary hover:bg-primary-hover text-white px-4 py-2 rounded-sm font-semibold text-xs transition-all cursor-pointer shadow-sm"
            >
              Save Name
            </button>
          </div>
        </form>
      </Modal>

      {/* Invite Member Modal */}
      <Modal
        open={showInviteModal()}
        onClose={() => {
          setShowInviteModal(false);
          setInviteLink('');
        }}
        title="Invite Family Member"
      >
        <Show
          when={inviteLink()}
          fallback={
            <form onSubmit={handleInvite} class="flex flex-col gap-4">
              <div>
                <label class="block text-xs font-semibold text-text-muted mb-1">
                  Member Email
                </label>
                <input
                  type="email"
                  required
                  placeholder="family.member@example.com"
                  value={inviteEmail()}
                  onInput={(e) => setInviteEmail(e.currentTarget.value)}
                  class="w-full bg-bg-card border border-border-default rounded-sm px-3 py-2 text-sm text-text-primary focus:border-border-focus focus:outline-none"
                />
              </div>

              <div>
                <label class="block text-xs font-semibold text-text-muted mb-1">
                  Role Permission
                </label>
                <select
                  value={inviteRole()}
                  onChange={(e) => setInviteRole(e.currentTarget.value)}
                  class="w-full bg-bg-card border border-border-default rounded-sm px-3 py-2 text-sm text-text-primary focus:border-border-focus focus:outline-none cursor-pointer"
                >
                  <option value="adult">Adult (Full self-sovereignty)</option>
                  <option value="child">Child (Supervised profile)</option>
                </select>
              </div>

              <button
                type="submit"
                class="w-full bg-primary hover:bg-primary-hover text-white py-2.5 rounded-sm font-semibold text-sm transition-all cursor-pointer shadow-sm mt-2"
              >
                Generate Invitation
              </button>
            </form>
          }
        >
          <div class="flex flex-col gap-3">
            <div class="text-xs text-text-muted">
              Share this secure single-use invitation link with your family member:
            </div>
            <div class="flex items-center gap-2">
              <input
                type="text"
                readonly
                value={inviteLink()}
                class="w-full bg-bg-card border border-border-default rounded-sm px-3 py-2 text-xs font-mono text-text-primary"
              />
              <button
                type="button"
                onClick={copyInviteLink}
                class="bg-primary hover:bg-primary-hover text-white p-2 rounded-sm shrink-0 transition-colors cursor-pointer"
                title="Copy link"
              >
                <Show when={copiedLink()} fallback={<IconCopy size={16} />}>
                  <IconCheck size={16} />
                </Show>
              </button>
            </div>
          </div>
        </Show>
      </Modal>

      {/* Create Child Modal */}
      <Modal
        open={showChildModal()}
        onClose={() => setShowChildModal(false)}
        title="Create Child Account"
      >
        <form onSubmit={handleCreateChild} class="flex flex-col gap-4">
          <div>
            <label class="block text-xs font-semibold text-text-muted mb-1">Child Name</label>
            <input
              type="text"
              required
              placeholder="e.g. Leo"
              value={childName()}
              onInput={(e) => setChildName(e.currentTarget.value)}
              class="w-full bg-bg-card border border-border-default rounded-sm px-3 py-2 text-sm text-text-primary focus:border-border-focus focus:outline-none"
            />
          </div>

          <div>
            <label class="block text-xs font-semibold text-text-muted mb-1">Initial Password / PIN</label>
            <input
              type="password"
              required
              placeholder="Minimum 6 characters"
              value={childPassword()}
              onInput={(e) => setChildPassword(e.currentTarget.value)}
              class="w-full bg-bg-card border border-border-default rounded-sm px-3 py-2 text-sm text-text-primary focus:border-border-focus focus:outline-none"
            />
          </div>

          <button
            type="submit"
            class="w-full bg-primary hover:bg-primary-hover text-white py-2.5 rounded-sm font-semibold text-sm transition-all cursor-pointer shadow-sm mt-2"
          >
            Create Child Profile
          </button>
        </form>
      </Modal>

      {/* Create New Family Modal */}
      <Modal
        open={showCreateFamilyModal()}
        onClose={() => setShowCreateFamilyModal(false)}
        title="Create New Household"
      >
        <form onSubmit={handleCreateFamily} class="flex flex-col gap-4">
          <div>
            <label class="block text-xs font-semibold text-text-muted mb-1">Household Name</label>
            <input
              type="text"
              required
              placeholder="e.g. The Smiths"
              value={newFamilyName()}
              onInput={(e) => setNewFamilyName(e.currentTarget.value)}
              class="w-full bg-bg-card border border-border-default rounded-sm px-3 py-2 text-sm text-text-primary focus:border-border-focus focus:outline-none"
            />
          </div>

          <button
            type="submit"
            class="w-full bg-primary hover:bg-primary-hover text-white py-2.5 rounded-sm font-semibold text-sm transition-all cursor-pointer shadow-sm mt-2"
          >
            Create Household
          </button>
        </form>
      </Modal>
    </div>
  );
}
