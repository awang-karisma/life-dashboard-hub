import { formatMinutes } from '../lib/format';

interface AppUsageBarProps {
  appName: string;
  packageName: string;
  minutes: number;
  maxMinutes: number;
}

export function AppUsageBar(props: AppUsageBarProps) {
  const percent = () => {
    if (!props.maxMinutes || props.maxMinutes <= 0) return 0;
    return Math.min(100, Math.round((props.minutes / props.maxMinutes) * 100));
  };

  return (
    <div style={{ 'margin-bottom': '0.75rem' }}>
      <div style={{ display: 'flex', 'justify-content': 'space-between', 'font-size': '0.875rem', 'margin-bottom': '0.25rem' }}>
        <span style={{ 'font-weight': 600 }}>{props.appName}</span>
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
