import { onMount, onCleanup, createEffect } from 'solid-js';
import {
  Chart as ChartJS,
  CategoryScale,
  LinearScale,
  BarElement,
  LineElement,
  PointElement,
  ArcElement,
  Title,
  Tooltip,
  Legend,
  ChartConfiguration,
} from 'chart.js';

ChartJS.register(
  CategoryScale,
  LinearScale,
  BarElement,
  LineElement,
  PointElement,
  ArcElement,
  Title,
  Tooltip,
  Legend
);

interface ChartProps {
  config: ChartConfiguration;
  height?: number;
}

export function Chart(props: ChartProps) {
  let canvasRef: HTMLCanvasElement | undefined;
  let chartInstance: ChartJS | undefined;

  const init = () => {
    if (!canvasRef) return;
    if (chartInstance) {
      chartInstance.destroy();
    }
    chartInstance = new ChartJS(canvasRef, props.config);
  };

  onMount(init);

  createEffect(() => {
    // When config changes, re-create chart
    if (props.config) {
      init();
    }
  });

  onCleanup(() => {
    if (chartInstance) {
      chartInstance.destroy();
    }
  });

  return (
    <div style={{ position: 'relative', width: '100%', height: props.height ? `${props.height}px` : '320px' }}>
      <canvas ref={canvasRef} />
    </div>
  );
}
