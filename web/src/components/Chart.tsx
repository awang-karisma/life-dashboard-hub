import { onMount, onCleanup, createEffect } from 'solid-js';
import { Chart as ChartJS, registerables, ChartConfiguration } from 'chart.js';

ChartJS.register(...registerables);

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
    <div
      class="relative w-full"
      style={{ height: props.height ? `${props.height}px` : '320px' }}
    >
      <canvas ref={canvasRef} />
    </div>
  );
}
