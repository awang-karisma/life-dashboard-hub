import { formatMinutes } from '../lib/format';

interface AppUsageBarProps {
  appName: string;
  packageName: string;
  minutes: number;
  maxMinutes: number;
  onRename?: (packageName: string, currentName: string) => void;
}

export function AppUsageBar(props: AppUsageBarProps) {
  const percent = () => {
    if (!props.maxMinutes || props.maxMinutes <= 0) return 0;
    return Math.min(100, Math.round((props.minutes / props.maxMinutes) * 100));
  };

  return (
    <div style={{ 'margin-bottom': '0.75rem' }}>
      <div style={{ display: 'flex', 'justify-content': 'space-between', 'align-items': 'center', 'font-size': '0.875rem', 'margin-bottom': '0.25rem' }}>
        <div style={{ display: 'flex', 'align-items': 'center', gap: '0.5rem' }}>
          <span style={{ 'font-weight': 600 }}>{props.appName}</span>
          <span style={{ 'font-size': '0.75rem', color: 'var(--text-muted)' }}>({props.packageName})</span>
          {props.onRename && (
            <button
              onClick={() => props.onRename?.(props.packageName, props.appName)}
              style={{ background: 'none', border: 'none', cursor: 'pointer', 'font-size': '0.75rem', color: 'var(--primary)', padding: 0 }}
              title="Rename app"
            >
              ✏️
            </button>
          )}
        </div>
        <span style={{ color: 'var(--text-muted)' }}>{formatMinutes(props.minutes)}</span>
      </div>
      <div style={{ background: '#e2e8f0', 'border-radius': '9999px', height: '8px', overflow: 'hidden' }}>
        <div
          style={{
            background: 'var(--primary)',
            height: '100%',
            width: `${percent()}%`,
            'border-radius': '9999px',
            transition: 'width 0.3s ease',
          }}
        />
      </div>
    </div>
  );
}
