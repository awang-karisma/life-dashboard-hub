import { createSignal, Show } from 'solid-js';
import { A, useNavigate } from '@solidjs/router';
import { ApiClient } from '../api/client';

export function Register() {
  const navigate = useNavigate();
  const [email, setEmail] = createSignal('');
  const [displayName, setDisplayName] = createSignal('');
  const [password, setPassword] = createSignal('');
  const [error, setError] = createSignal('');
  const [loading, setLoading] = createSignal(false);

  const handleSubmit = async (e: Event) => {
    e.preventDefault();
    setError('');

    if (password().length < 8) {
      setError('Password must be at least 8 characters');
      return;
    }

    setLoading(true);
    try {
      await ApiClient.post('/api/v1/auth/register', {
        email: email(),
        display_name: displayName(),
        password: password(),
      });
      // Automatically log in after registration
      await ApiClient.post('/api/v1/auth/login', {
        email: email(),
        password: password(),
      });
      navigate('/');
    } catch (err: unknown) {
      if (err instanceof Error) {
        setError(err.message);
      } else {
        setError('Registration failed');
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
            Create a new account
          </p>
        </div>

        <Show when={error()}>
          <div style={{ background: '#fee2e2', color: '#b91c1c', padding: '0.75rem', 'border-radius': '0.5rem', 'font-size': '0.875rem', 'margin-bottom': '1rem' }}>
            {error()}
          </div>
        </Show>

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
              Display Name
            </label>
            <input
              type="text"
              class="input"
              placeholder="Your Name"
              value={displayName()}
              onInput={(e) => setDisplayName(e.currentTarget.value)}
            />
          </div>

          <div>
            <label style={{ display: 'block', 'font-size': '0.875rem', 'font-weight': 500, 'margin-bottom': '0.35rem' }}>
              Password (min 8 characters)
            </label>
            <input
              type="password"
              required
              minlength={8}
              class="input"
              placeholder="••••••••"
              value={password()}
              onInput={(e) => setPassword(e.currentTarget.value)}
            />
          </div>

          <button type="submit" disabled={loading()} class="btn btn-primary" style={{ 'width': '100%', 'margin-top': '0.5rem' }}>
            {loading() ? 'Creating account...' : 'Create Account'}
          </button>
        </form>

        <div style={{ 'text-align': 'center', 'margin-top': '1.5rem', 'font-size': '0.875rem', color: 'var(--text-muted)' }}>
          Already have an account? <A href="/login">Sign in</A>
        </div>
      </div>
    </div>
  );
}
