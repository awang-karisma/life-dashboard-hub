import { A, useNavigate } from '@solidjs/router';
import { ApiClient } from '../api/client';

interface NavProps {
  user?: {
    email: string;
    display_name: string;
  };
}

export function Nav(props: NavProps) {
  const navigate = useNavigate();

  const handleLogout = async () => {
    try {
      await ApiClient.post('/api/v1/auth/logout');
    } catch {
      // Continue redirect
    }
    navigate('/login');
  };

  return (
    <nav style={{ background: '#fff', 'border-bottom': '1px solid var(--border)', padding: '0.75rem 1.5rem' }}>
      <div style={{ 'max-width': '1200px', margin: '0 auto', display: 'flex', 'align-items': 'center', 'justify-content': 'space-between', 'flex-wrap': 'wrap', gap: '1rem' }}>
        <div style={{ display: 'flex', 'align-items': 'center', gap: '2rem' }}>
          <A href="/" style={{ 'font-weight': 700, 'font-size': '1.125rem', color: 'var(--text-main)', display: 'flex', 'align-items': 'center', gap: '0.5rem', 'text-decoration': 'none' }}>
            <span>📊</span>
            <span>Life Dashboard</span>
          </A>
          <div style={{ display: 'flex', gap: '1.25rem', 'font-size': '0.9rem', 'font-weight': 500 }}>
            <A href="/" end style={{ color: 'var(--text-muted)' }} activeClass="active-nav">Dashboard</A>
            <A href="/devices" style={{ color: 'var(--text-muted)' }} activeClass="active-nav">Devices</A>
            <A href="/reports/screen-time" style={{ color: 'var(--text-muted)' }} activeClass="active-nav">Screen Time</A>
            <A href="/reports/health" style={{ color: 'var(--text-muted)' }} activeClass="active-nav">Health</A>
            <A href="/family" style={{ color: 'var(--text-muted)' }} activeClass="active-nav">Family</A>
            <A href="/settings" style={{ color: 'var(--text-muted)' }} activeClass="active-nav">Settings</A>
          </div>
        </div>

        <div style={{ display: 'flex', 'align-items': 'center', gap: '1rem' }}>
          {props.user && (
            <span style={{ 'font-size': '0.875rem', color: 'var(--text-muted)' }}>
              👤 {props.user.display_name || props.user.email}
            </span>
          )}
          <button onClick={handleLogout} class="btn btn-secondary" style={{ 'padding': '0.35rem 0.75rem', 'font-size': '0.8rem' }}>
            Log out
          </button>
        </div>
      </div>
      <style>{`
        .active-nav {
          color: var(--primary) !important;
          font-weight: 600;
        }
      `}</style>
    </nav>
  );
}
