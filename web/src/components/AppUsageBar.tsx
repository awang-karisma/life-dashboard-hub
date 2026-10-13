import { IconEdit } from '@tabler/icons-solidjs';
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
    <div class="mb-3">
      <div class="flex justify-between items-center text-sm mb-1">
        <div class="flex items-center gap-2">
          <span class="font-semibold text-text-primary">{props.appName}</span>
          <span class="text-xs text-text-muted font-mono">({props.packageName})</span>
          {props.onRename && (
            <button
              type="button"
              onClick={() => props.onRename?.(props.packageName, props.appName)}
              class="text-primary hover:text-primary-light transition-colors cursor-pointer p-0.5"
              title="Rename app"
            >
              <IconEdit size={14} />
            </button>
          )}
        </div>
        <span class="text-text-muted text-xs font-medium">{formatMinutes(props.minutes)}</span>
      </div>
      <div class="bg-primary/15 rounded-full h-2 overflow-hidden">
        <div
          class="bg-primary h-full rounded-full transition-all duration-300"
          style={{ width: `${percent()}%` }}
        />
      </div>
    </div>
  );
}
