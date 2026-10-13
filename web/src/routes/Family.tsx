import { createSignal, onMount, For, Show } from 'solid-js';
import {
  IconUsers,
  IconShield,
  IconUserPlus,
  IconBabyCarriage,
  IconMail,
  IconClock,
  IconPlus,
  IconCopy,
  IconCheck,
} from '@tabler/icons-solidjs';
import { ApiClient } from '../api/client';
import { Modal } from '../components/Modal';
import { showToast } from '../components/Toast';
import { formatMinutes } from '../lib/format';

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

export function Family() {
  const [families, setFamilies] = createSignal<FamilyItem[]>([]);
  const [selectedFamily, setSelectedFamily] = createSignal<FamilyItem | null>(null);
  const [newFamilyName, setNewFamilyName] = createSignal('');
  const [inviteEmail, setInviteEmail] = createSignal('');
  const [inviteRole, setInviteRole] = createSignal('adult');
  const [inviteLink, setInviteLink] = createSignal('');
  const [copiedLink, setCopiedLink] = createSignal(false);
  const [childName, setChildName] = createSignal('');
  const [childPassword, setChildPassword] = createSignal('');
  const [loading, setLoading] = createSignal(true);
  const [memberScreenTime, setMemberScreenTime] = createSignal<{ [userId: number]: number }>({});

  const [showInviteModal, setShowInviteModal] = createSignal(false);
  const [showChildModal, setShowChildModal] = createSignal(false);
  const [showCreateFamilyModal, setShowCreateFamilyModal] = createSignal(false);

  const loadFamilies = async () => {
    try {
      const list = await ApiClient.get<FamilyItem[]>('/api/v1/families');
      setFamilies(list || []);
      if (list && list.length > 0) {
        await selectFamily(list[0].id);
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

  onMount(loadFamilies);

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
          <button
            type="button"
            onClick={() => setShowCreateFamilyModal(true)}
            class="px-3 py-1.5 rounded-sm bg-white/5 hover:bg-white/10 text-text-muted hover:text-text-primary text-xs font-semibold border border-border-default transition-all flex items-center gap-1.5 cursor-pointer"
          >
            <IconPlus size={15} />
            <span>New Family</span>
          </button>

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
        </div>
      </div>

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
            when={selectedFamily()?.members && selectedFamily()!.members!.length > 0}
            fallback={
              <div class="text-text-muted text-xs py-12 text-center flex flex-col items-center justify-center gap-3">
                <IconUsers size={32} class="text-text-dim" />
                <span>No members in this family yet.</span>
                <button
                  type="button"
                  onClick={() => setShowInviteModal(true)}
                  class="text-primary-light hover:text-primary font-semibold text-xs cursor-pointer"
                >
                  Invite your first member &rarr;
                </button>
              </div>
            }
          >
            <div class="flex flex-col gap-3">
              <For each={selectedFamily()?.members}>
                {(m) => {
                  const displayName = m.display_name || m.email;
                  const isChild = m.role === 'child';
                  const isOrganizer = m.role === 'organizer';
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

                      <div class="flex items-center gap-6">
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
                            <option value="full">Full Details</option>
                          </select>
                        </div>
                      </div>
                    </div>
                  );
                }}
              </For>
            </div>
          </Show>
        </div>
      </div>

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
