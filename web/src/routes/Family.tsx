import { createSignal, onMount, For, Show } from 'solid-js';
import { ApiClient } from '../api/client';
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
  const [childName, setChildName] = createSignal('');
  const [childPassword, setChildPassword] = createSignal('');
  const [loading, setLoading] = createSignal(true);
  const [memberScreenTime, setMemberScreenTime] = createSignal<{ [userId: number]: number }>({});

  const loadFamilies = async () => {
    try {
      const list = await ApiClient.get<FamilyItem[]>('/api/v1/families');
      setFamilies(list || []);
      if (list && list.length > 0) {
        await selectFamily(list[0].id);
      }
    } catch (err) {
      console.error(err);
    } finally {
      setLoading(false);
    }
  };

  const selectFamily = async (id: number) => {
    try {
      const fam = await ApiClient.get<FamilyItem>(`/api/v1/families/${id}`);
      setSelectedFamily(fam);

      // Pre-fetch screen time for visible members if permitted
      if (fam.members) {
        for (const m of fam.members) {
          if (m.sharing !== 'none' || m.role === 'child') {
            try {
              const st = await ApiClient.get<any>(`/api/v1/families/${fam.id}/members/${m.user_id}/reports/screen-time/daily`);
              if (st && st.days && st.days.length > 0) {
                const latest = st.days[st.days.length - 1];
                setMemberScreenTime((prev) => ({ ...prev, [m.user_id]: latest.total_screen_time_minutes }));
              }
            } catch {
              // Not permitted or no data
            }
          }
        }
      }
    } catch (err) {
      console.error(err);
    }
  };

  onMount(loadFamilies);

  const handleCreateFamily = async (e: Event) => {
    e.preventDefault();
    if (!newFamilyName()) return;
    try {
      await ApiClient.post('/api/v1/families', { name: newFamilyName() });
      setNewFamilyName('');
      await loadFamilies();
    } catch (err: unknown) {
      alert(err instanceof Error ? err.message : 'Failed to create family');
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
      setInviteEmail('');
    } catch (err: unknown) {
      alert(err instanceof Error ? err.message : 'Failed to create invitation');
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
      setChildName('');
      setChildPassword('');
      await selectFamily(fam.id);
    } catch (err: unknown) {
      alert(err instanceof Error ? err.message : 'Failed to create child account');
    }
  };

  const handleUpdateSharing = async (userId: number, newSharing: string) => {
    const fam = selectedFamily();
    if (!fam) return;

    try {
      await ApiClient.patch(`/api/v1/families/${fam.id}/members/${userId}/sharing`, {
        sharing: newSharing,
      });
      await selectFamily(fam.id);
    } catch (err: unknown) {
      alert(err instanceof Error ? err.message : 'Failed to update sharing');
    }
  };

  return (
    <div>
      <div style={{ 'margin-bottom': '1.5rem', display: 'flex', 'justify-content': 'space-between', 'align-items': 'center', 'flex-wrap': 'wrap', gap: '1rem' }}>
        <div>
          <h1 style={{ 'font-size': '1.75rem', 'font-weight': 700 }}>Family Hub</h1>
          <p style={{ color: 'var(--text-muted)', 'font-size': '0.875rem' }}>
            Coordinate family accounts, configure sharing consent, and view activity
          </p>
        </div>
      </div>

      <Show
        when={families().length > 0}
        fallback={
          <div class="card" style={{ 'max-width': '500px', margin: '0 auto', 'text-align': 'center', padding: '2rem' }}>
            <h2 style={{ 'font-size': '1.25rem', 'font-weight': 600, 'margin-bottom': '0.5rem' }}>Create a Family Group</h2>
            <p style={{ color: 'var(--text-muted)', 'font-size': '0.875rem', 'margin-bottom': '1.5rem' }}>
              Connect multiple accounts and monitor children's screen time with privacy controls.
            </p>
            <form onSubmit={handleCreateFamily} style={{ display: 'flex', gap: '0.5rem' }}>
              <input
                type="text"
                required
                class="input"
                placeholder="Family name (e.g. Smith Family)"
                value={newFamilyName()}
                onInput={(e) => setNewFamilyName(e.currentTarget.value)}
              />
              <button type="submit" class="btn btn-primary" style={{ 'white-space': 'nowrap' }}>
                Create Family
              </button>
            </form>
          </div>
        }
      >
        <Show when={selectedFamily()}>
          {(fam) => (
            <div>
              <div class="card" style={{ 'margin-bottom': '1.5rem' }}>
                <div style={{ display: 'flex', 'justify-content': 'space-between', 'align-items': 'center', 'margin-bottom': '1.5rem' }}>
                  <div>
                    <h2 style={{ 'font-size': '1.25rem', 'font-weight': 700 }}>{fam().name}</h2>
                    <span class="badge badge-gray" style={{ 'margin-top': '0.25rem' }}>Your role: {fam().role}</span>
                  </div>
                  <div style={{ display: 'flex', gap: '0.5rem' }}>
                    <button
                      onClick={() => {
                        const token = prompt('Enter Invitation Token to accept:');
                        if (token) {
                          ApiClient.post(`/api/v1/invitations/${token}/accept`)
                            .then(() => loadFamilies())
                            .catch((err) => alert(err.message));
                        }
                      }}
                      class="btn btn-secondary"
                      style={{ 'font-size': '0.8rem' }}
                    >
                      Join Another Family
                    </button>
                  </div>
                </div>

                <h3 style={{ 'font-size': '1rem', 'font-weight': 600, 'margin-bottom': '0.75rem' }}>Family Members</h3>
                <table class="table">
                  <thead>
                    <tr>
                      <th>Member</th>
                      <th>Email / ID</th>
                      <th>Role</th>
                      <th>Sharing Consent</th>
                      <th>Today's Usage</th>
                      <th style={{ 'text-align': 'right' }}>Actions</th>
                    </tr>
                  </thead>
                  <tbody>
                    <For each={fam().members}>
                      {(m) => (
                        <tr>
                          <td style={{ 'font-weight': 600 }}>{m.display_name}</td>
                          <td style={{ color: 'var(--text-muted)' }}>{m.email}</td>
                          <td>
                            <span class={`badge ${m.role === 'admin' ? 'badge-yellow' : m.role === 'child' ? 'badge-green' : 'badge-gray'}`}>
                              {m.role}
                            </span>
                          </td>
                          <td>
                            <span class={`badge ${m.sharing === 'detailed' ? 'badge-green' : m.sharing === 'summary' ? 'badge-yellow' : 'badge-red'}`}>
                              {m.sharing}
                            </span>
                          </td>
                          <td style={{ 'font-weight': 600 }}>
                            {memberScreenTime()[m.user_id] !== undefined
                              ? formatMinutes(memberScreenTime()[m.user_id])
                              : <span style={{ color: 'var(--text-muted)', 'font-size': '0.8rem' }}>Private</span>}
                          </td>
                          <td style={{ 'text-align': 'right' }}>
                            <select
                              class="select"
                              style={{ width: 'auto', padding: '0.2rem 0.5rem', 'font-size': '0.75rem' }}
                              value={m.sharing}
                              onChange={(e) => handleUpdateSharing(m.user_id, e.currentTarget.value)}
                            >
                              <option value="none">Sharing: None</option>
                              <option value="summary">Sharing: Summary</option>
                              <option value="detailed">Sharing: Detailed</option>
                            </select>
                          </td>
                        </tr>
                      )}
                    </For>
                  </tbody>
                </table>
              </div>

              {/* Management tools */}
              <div class="grid-cols-2">
                <div class="card">
                  <h3 style={{ 'font-size': '1rem', 'font-weight': 600, 'margin-bottom': '0.75rem' }}>Invite Member</h3>
                  <form onSubmit={handleInvite} style={{ display: 'flex', 'flex-direction': 'column', gap: '0.75rem' }}>
                    <input
                      type="email"
                      required
                      class="input"
                      placeholder="member@example.com"
                      value={inviteEmail()}
                      onInput={(e) => setInviteEmail(e.currentTarget.value)}
                    />
                    <select class="select" value={inviteRole()} onChange={(e) => setInviteRole(e.currentTarget.value)}>
                      <option value="parent">Role: Parent</option>
                      <option value="adult">Role: Adult</option>
                      <option value="child">Role: Child</option>
                    </select>
                    <button type="submit" class="btn btn-primary">Send Invitation</button>
                  </form>
                  <Show when={inviteLink()}>
                    <div style={{ 'margin-top': '1rem', background: '#f8fafc', padding: '0.75rem', 'border-radius': '0.5rem', 'border': '1px solid var(--border)' }}>
                      <div style={{ 'font-size': '0.75rem', color: 'var(--text-muted)' }}>Shareable Invitation Link:</div>
                      <code style={{ 'word-break': 'break-all', 'font-size': '0.8rem' }}>{inviteLink()}</code>
                    </div>
                  </Show>
                </div>

                <div class="card">
                  <h3 style={{ 'font-size': '1rem', 'font-weight': 600, 'margin-bottom': '0.75rem' }}>Create Child Account</h3>
                  <p style={{ 'font-size': '0.75rem', color: 'var(--text-muted)', 'margin-bottom': '0.75rem' }}>
                    Children do not need an email address. You can log them into companion app or web directly.
                  </p>
                  <form onSubmit={handleCreateChild} style={{ display: 'flex', 'flex-direction': 'column', gap: '0.75rem' }}>
                    <input
                      type="text"
                      required
                      class="input"
                      placeholder="Child Display Name (e.g. Leo)"
                      value={childName()}
                      onInput={(e) => setChildName(e.currentTarget.value)}
                    />
                    <input
                      type="password"
                      required
                      minlength={8}
                      class="input"
                      placeholder="Password (min 8 chars)"
                      value={childPassword()}
                      onInput={(e) => setChildPassword(e.currentTarget.value)}
                    />
                    <button type="submit" class="btn btn-secondary">Create Child Account</button>
                  </form>
                </div>
              </div>
            </div>
          )}
        </Show>
      </Show>
    </div>
  );
}
