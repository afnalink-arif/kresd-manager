import { onMount, onCleanup, createEffect } from "solid-js";
import type { Component } from "solid-js";
import uPlot from "uplot";
import "uplot/dist/uPlot.min.css";

interface TimeSeriesProps {
  data: uPlot.AlignedData;
  series: uPlot.Series[];
  title?: string;
  width?: number;
  height?: number;
  yLabel?: string;
}

const TimeSeriesChart: Component<TimeSeriesProps> = (props) => {
  let container!: HTMLDivElement;
  let chart: uPlot | null = null;

  const colors = {
    grid: "#334155",
    tick: "#94a3b8",
    text: "#94a3b8",
  };

  onMount(() => {
    const opts: uPlot.Options = {
      width: props.width || container.clientWidth,
      height: props.height || 280,
      title: props.title,
      cursor: {
        drag: { x: true, y: false },
      },
      scales: {
        x: { time: true },
      },
      axes: [
        {
          stroke: colors.text,
          grid: { stroke: colors.grid, width: 1 },
          ticks: { stroke: colors.tick, width: 1 },
        },
        {
          stroke: colors.text,
          grid: { stroke: colors.grid, width: 1 },
          ticks: { stroke: colors.tick, width: 1 },
          label: props.yLabel,
        },
      ],
      series: [
        { label: "Time" },
        ...props.series,
      ],
    };

    chart = new uPlot(opts, props.data, container);

    const ro = new ResizeObserver(() => {
      if (chart && container) {
        chart.setSize({ width: container.clientWidth, height: props.height || 280 });
      }
    });
    ro.observe(container);

    onCleanup(() => {
      ro.disconnect();
      chart?.destroy();
    });
  });

  createEffect(() => {
    if (chart && props.data) {
      chart.setData(props.data);
    }
  });

  return <div ref={container} class="w-full" />;
};

export default TimeSeriesChart;
