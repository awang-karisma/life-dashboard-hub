import { createSignal, onMount, Show, JSX } from 'solid-js';
import { Router, Route } from '@solidjs/router';
import { ApiClient } from './api/client';
import { Nav } from './components/Nav';
import { Login } from './routes/Login';
import { Register } from './routes/Register';
import { Dashboard } from './routes/Dashboard';
import { Devices } from './routes/Devices';
import { DeviceSetup } from './routes/DeviceSetup';
import { ScreenTimeReport } from './routes/ScreenTimeReport';
import { HealthReport } from './routes/HealthReport';
import { Family } from './routes/Family';
import { Settings } from './routes/Settings';

interface CurrentUser {
  id: number;
  email: string;
  display_name: string;
}

function MainLayout(props: { children?: JSX.Element }) {
  const [user, setUser] = createSignal<CurrentUser | null>(null);
  const [loading, setLoading] = createSignal(true);

  onMount(async () => {
    try {
      const u = await ApiClient.get<CurrentUser>('/api/v1/users/me');
      setUser(u);
    } catch {
      // 401 is handled by ApiClient redirecting to /login
    } finally {
      setLoading(false);
    }
  });

  return (
    <div style={{ 'min-height': '100vh', display: 'flex', 'flex-direction': 'column' }}>
      <Nav user={user() || undefined} />
      <main class="container" style={{ flex: 1, padding: '1.5rem 1rem' }}>
        <Show when={!loading()} fallback={<div style={{ padding: '3rem', 'text-align': 'center' }}>Loading dashboard...</div>}>
          {props.children}
        </Show>
      </main>
    </div>
  );
}

export function App() {
  return (
    <Router>
      <Route path="/login" component={Login} />
      <Route path="/register" component={Register} />
      <Route path="/" component={MainLayout}>
        <Route path="/" component={Dashboard} />
        <Route path="/devices" component={Devices} />
        <Route path="/devices/new" component={DeviceSetup} />
        <Route path="/reports/screen-time" component={ScreenTimeReport} />
        <Route path="/reports/health" component={HealthReport} />
        <Route path="/family" component={Family} />
        <Route path="/settings" component={Settings} />
      </Route>
    </Router>
  );
}
