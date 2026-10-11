import { createSignal, onMount, Show } from 'solid-js';
import { ApiClient } from '../api/client';

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
  const [profileSuccess, setProfileSuccess] = createSignal('');
  const [pwSuccess, setPwSuccess] = createSignal('');
  const [error, setError] = createSignal('');
  const [exportFrom, setExportFrom] = createSignal('');
  const [exportTo, setExportTo] = createSignal('');

  onMount(async () => {
    try {
      const u = await ApiClient.get<UserProfile>('/api/v1/users/me');
      setProfile(u);
      setDisplayName(u.display_name);
      setTimezone(u.timezone || 'UTC');
    } catch (err) {
      console.error(err);
    }
  });

  const handleUpdateProfile = async (e: Event) => {
    e.preventDefault();
    setProfileSuccess('');
    setError('');

    try {
      const updated = await ApiClient.patch<UserProfile>('/api/v1/users/me', {
        display_name: displayName(),
        timezone: timezone(),
      });
      setProfile(updated);
      setProfileSuccess('Profile updated successfully.');
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : 'Failed to update profile');
    }
  };

  const handleChangePassword = async (e: Event) => {
    e.preventDefault();
    setPwSuccess('');
    setError('');

    if (newPassword().length < 8) {
      setError('New password must be at least 8 characters');
      return;
    }

    try {
      await ApiClient.patch('/api/v1/users/me', {
        current_password: currentPassword(),
        new_password: newPassword(),
      });
      setCurrentPassword('');
      setNewPassword('');
      setPwSuccess('Password changed successfully.');
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : 'Failed to change password');
    }
  };

  const handleExport = (format: 'csv' | 'json') => {
    let url = `/api/v1/data/export?format=${format}`;
    if (exportFrom()) url += `&from=${exportFrom()}`;
    if (exportTo()) url += `&to=${exportTo()}`;
    window.open(url, '_blank');
  };

  const handleDeleteData = async () => {
    if (!confirm('DANGER: This will permanently erase your usage data records in the selected range. Continue?')) {
      return;
    }

    try {
      let url = '/api/v1/data';
      const params = [];
      if (exportFrom()) params.push(`from=${exportFrom()}`);
      if (exportTo()) params.push(`to=${exportTo()}`);
      if (params.length) url += `?${params.join('&')}`;

      await ApiClient.delete(url);
      alert('Data deleted successfully.');
    } catch (err: unknown) {
      alert(err instanceof Error ? err.message : 'Failed to delete data');
    }
  };

  return (
    <div style={{ 'max-width': '760px', margin: '0 auto' }}>
      <div style={{ 'margin-bottom': '1.5rem' }}>
        <h1 style={{ 'font-size': '1.75rem', 'font-weight': 700 }}>Account & Privacy Settings</h1>
        <p style={{ color: 'var(--text-muted)', 'font-size': '0.875rem' }}>
          Manage your personal profile, authentication, and data ownership
        </p>
      </div>

      <Show when={error()}>
        <div style={{ background: '#fee2e2', color: '#b91c1c', padding: '0.75rem', 'border-radius': '0.5rem', 'margin-bottom': '1rem' }}>
          {error()}
        </div>
      </Show>

      {/* Profile Form */}
      <div class="card">
        <h2 style={{ 'font-size': '1.125rem', 'font-weight': 600, 'margin-bottom': '1rem' }}>Profile Information</h2>
        <Show when={profileSuccess()}>
          <div style={{ background: '#dcfce7', color: '#15803d', padding: '0.5rem', 'border-radius': '0.375rem', 'margin-bottom': '1rem', 'font-size': '0.875rem' }}>
            {profileSuccess()}
          </div>
        </Show>
        <form onSubmit={handleUpdateProfile} style={{ display: 'flex', 'flex-direction': 'column', gap: '1rem' }}>
          <div>
            <label style={{ display: 'block', 'font-size': '0.875rem', 'font-weight': 500, 'margin-bottom': '0.25rem' }}>
              Email address
            </label>
            <input type="text" disabled class="input" style={{ background: '#f8fafc', color: 'var(--text-muted)' }} value={profile()?.email || ''} />
          </div>

          <div>
            <label style={{ display: 'block', 'font-size': '0.875rem', 'font-weight': 500, 'margin-bottom': '0.25rem' }}>
              Display Name
            </label>
            <input
              type="text"
              class="input"
              value={displayName()}
              onInput={(e) => setDisplayName(e.currentTarget.value)}
            />
          </div>

          <div>
            <label style={{ display: 'block', 'font-size': '0.875rem', 'font-weight': 500, 'margin-bottom': '0.25rem' }}>
              Timezone
            </label>
            <input
              type="text"
              class="input"
              placeholder="e.g. UTC or America/New_York"
              value={timezone()}
              onInput={(e) => setTimezone(e.currentTarget.value)}
            />
          </div>

          <button type="submit" class="btn btn-primary" style={{ 'align-self': 'flex-start' }}>
            Save Changes
          </button>
        </form>
      </div>

      {/* Password Change / SSO Banner */}
      <div class="card">
        <h2 style={{ 'font-size': '1.125rem', 'font-weight': 600, 'margin-bottom': '1rem' }}>Security & Password</h2>
        <Show
          when={profile()?.auth_method === 'local'}
          fallback={
            <div style={{ color: 'var(--text-muted)', 'font-size': '0.875rem' }}>
              Your account is authenticated via single sign-on (SSO/OIDC). Passwords are managed by your identity provider.
            </div>
          }
        >
          <Show when={pwSuccess()}>
            <div style={{ background: '#dcfce7', color: '#15803d', padding: '0.5rem', 'border-radius': '0.375rem', 'margin-bottom': '1rem', 'font-size': '0.875rem' }}>
              {pwSuccess()}
            </div>
          </Show>
          <form onSubmit={handleChangePassword} style={{ display: 'flex', 'flex-direction': 'column', gap: '1rem' }}>
            <div>
              <label style={{ display: 'block', 'font-size': '0.875rem', 'font-weight': 500, 'margin-bottom': '0.25rem' }}>
                Current Password
              </label>
              <input
                type="password"
                required
                class="input"
                value={currentPassword()}
                onInput={(e) => setCurrentPassword(e.currentTarget.value)}
              />
            </div>
            <div>
              <label style={{ display: 'block', 'font-size': '0.875rem', 'font-weight': 500, 'margin-bottom': '0.25rem' }}>
                New Password (min 8 chars)
              </label>
              <input
                type="password"
                required
                minlength={8}
                class="input"
                value={newPassword()}
                onInput={(e) => setNewPassword(e.currentTarget.value)}
              />
            </div>
            <button type="submit" class="btn btn-secondary" style={{ 'align-self': 'flex-start' }}>
              Update Password
            </button>
          </form>
        </Show>
      </div>

      {/* Data Export & Deletion */}
      <div class="card">
        <h2 style={{ 'font-size': '1.125rem', 'font-weight': 600, 'margin-bottom': '0.5rem' }}>Data Ownership & Export</h2>
        <p style={{ color: 'var(--text-muted)', 'font-size': '0.875rem', 'margin-bottom': '1rem' }}>
          Download a complete archive of your screen time and health records.
        </p>

        <div style={{ display: 'flex', gap: '0.75rem', 'margin-bottom': '1rem', 'flex-wrap': 'wrap' }}>
          <div>
            <label style={{ 'font-size': '0.75rem', color: 'var(--text-muted)', display: 'block' }}>From (optional)</label>
            <input type="date" class="input" value={exportFrom()} onInput={(e) => setExportFrom(e.currentTarget.value)} />
          </div>
          <div>
            <label style={{ 'font-size': '0.75rem', color: 'var(--text-muted)', display: 'block' }}>To (optional)</label>
            <input type="date" class="input" value={exportTo()} onInput={(e) => setExportTo(e.currentTarget.value)} />
          </div>
        </div>

        <div style={{ display: 'flex', gap: '0.75rem' }}>
          <button onClick={() => handleExport('csv')} class="btn btn-secondary">
            📥 Download CSV
          </button>
          <button onClick={() => handleExport('json')} class="btn btn-secondary">
            📥 Download JSON
          </button>
        </div>

        <div style={{ 'margin-top': '2rem', 'border-top': '1px solid var(--border)', 'padding-top': '1.5rem' }}>
          <h3 style={{ 'font-size': '1rem', 'font-weight': 600, color: 'var(--danger)', 'margin-bottom': '0.25rem' }}>
            Delete Data
          </h3>
          <p style={{ color: 'var(--text-muted)', 'font-size': '0.875rem', 'margin-bottom': '0.75rem' }}>
            Permanently remove your records within the selected date range.
          </p>
          <button onClick={handleDeleteData} class="btn btn-danger">
            Delete Records
          </button>
        </div>
      </div>
    </div>
  );
}
