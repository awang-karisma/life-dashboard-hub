import { createSignal, onMount, Show } from 'solid-js';
import { A, useNavigate, useSearchParams } from '@solidjs/router';
import { ApiClient } from '../api/client';

interface AuthInfo {
  local: boolean;
  oidc: {
    enabled: boolean;
    display_name?: string;
    login_url?: string;
  };
}

export function Login() {
  const navigate = useNavigate();
  const [params] = useSearchParams();
  const [email, setEmail] = createSignal('');
  const [password, setPassword] = createSignal('');
  const [error, setError] = createSignal('');
  const [loading, setLoading] = createSignal(false);
  const [authInfo, setAuthInfo] = createSignal<AuthInfo>({
    local: true,
    oidc: { enabled: false },
  });

  onMount(async () => {
    // Check if redirected from OIDC success
    if (params.oidc_login === 'success') {
      navigate('/');
      return;
    }

    try {
      const info = await ApiClient.get<AuthInfo>('/api/v1/auth/info');
      setAuthInfo(info);
    } catch (err) {
      console.error('Failed to load auth info:', err);
    }
  });

  const handleSubmit = async (e: Event) => {
    e.preventDefault();
    setError('');
    setLoading(true);

    try {
      await ApiClient.post('/api/v1/auth/login', {
        email: email(),
        password: password(),
      });
      navigate('/');
    } catch (err: unknown) {
      if (err instanceof Error) {
        setError(err.message);
      } else {
        setError('Login failed');
      }
    } finally {
      setLoading(false);
    }
  };

  return (
    <div style={{ 'min-height': '100vh', display: 'flex', 'align-items': 'center', 'justify-content': 'center', padding: '1.5rem' }}>
      <div class="card" style={{ 'max-width': '420px', width: '100%', padding: '2rem' }}>
        <div style={{ 'text-align': 'center', 'margin-bottom': '1.5rem' }}>
          <h1 style={{ 'font-size': '1.5rem', 'font-weight': 700 }}>Life Dashboard</h1>
          <p style={{ color: 'var(--text-muted)', 'font-size': '0.875rem', 'margin-top': '0.25rem' }}>
            Sign in to access your dashboard
          </p>
        </div>

        <Show when={error()}>
          <div style={{ background: '#fee2e2', color: '#b91c1c', padding: '0.75rem', 'border-radius': '0.5rem', 'font-size': '0.875rem', 'margin-bottom': '1rem' }}>
            {error()}
          </div>
        </Show>

        <Show when={authInfo().local}>
          <form onSubmit={handleSubmit} style={{ display: 'flex', 'flex-direction': 'column', gap: '1rem' }}>
            <div>
              <label style={{ display: 'block', 'font-size': '0.875rem', 'font-weight': 500, 'margin-bottom': '0.35rem' }}>
                Email address
              </label>
              <input
                type="email"
                required
                class="input"
                placeholder="you@example.com"
                value={email()}
                onInput={(e) => setEmail(e.currentTarget.value)}
              />
            </div>

            <div>
              <label style={{ display: 'block', 'font-size': '0.875rem', 'font-weight': 500, 'margin-bottom': '0.35rem' }}>
                Password
              </label>
              <input
                type="password"
                required
                class="input"
                placeholder="••••••••"
                value={password()}
                onInput={(e) => setPassword(e.currentTarget.value)}
              />
            </div>

            <button type="submit" disabled={loading()} class="btn btn-primary" style={{ 'width': '100%', 'margin-top': '0.5rem' }}>
              {loading() ? 'Signing in...' : 'Sign In'}
            </button>
          </form>
        </Show>

        <Show when={authInfo().oidc.enabled}>
          <div style={{ 'margin-top': '1.5rem', 'text-align': 'center' }}>
            <div style={{ display: 'flex', 'align-items': 'center', 'margin-bottom': '1rem' }}>
              <div style={{ flex: 1, height: '1px', background: 'var(--border)' }} />
              <span style={{ padding: '0 0.75rem', 'font-size': '0.75rem', color: 'var(--text-muted)' }}>OR</span>
              <div style={{ flex: 1, height: '1px', background: 'var(--border)' }} />
            </div>

            <a
              href={authInfo().oidc.login_url || '/api/v1/auth/oidc/login'}
              class="btn btn-secondary"
              style={{ width: '100%', 'text-decoration': 'none', display: 'flex' }}
            >
              🔐 Login with {authInfo().oidc.display_name || 'SSO'}
            </a>
          </div>
        </Show>

        <div style={{ 'text-align': 'center', 'margin-top': '1.5rem', 'font-size': '0.875rem', color: 'var(--text-muted)' }}>
          Don't have an account? <A href="/register">Create an account</A>
        </div>
      </div>
    </div>
  );
}
