import type { Component } from "solid-js";
import { For, createMemo } from "solid-js";

interface DonutChartProps {
  data: { label: string; value: number; color: string }[];
  size?: number;
  title?: string;
}

const DonutChart: Component<DonutChartProps> = (props) => {
  const size = () => props.size || 200;
  const radius = () => size() / 2 - 10;
  const innerRadius = () => radius() * 0.6;
  const center = () => size() / 2;

  const total = createMemo(() => props.data.reduce((sum, d) => sum + d.value, 0));

  const segments = createMemo(() => {
    let startAngle = -Math.PI / 2;
    return props.data.map((d) => {
      const angle = total() > 0 ? (d.value / total()) * Math.PI * 2 : 0;
      const endAngle = startAngle + angle;
      const r = radius();
      const ir = innerRadius();
      const cx = center();
      const cy = center();

      const x1 = cx + r * Math.cos(startAngle);
      const y1 = cy + r * Math.sin(startAngle);
      const x2 = cx + r * Math.cos(endAngle);
      const y2 = cy + r * Math.sin(endAngle);
      const x3 = cx + ir * Math.cos(endAngle);
      const y3 = cy + ir * Math.sin(endAngle);
      const x4 = cx + ir * Math.cos(startAngle);
      const y4 = cy + ir * Math.sin(startAngle);

      const largeArc = angle > Math.PI ? 1 : 0;
      const path = `M ${x1} ${y1} A ${r} ${r} 0 ${largeArc} 1 ${x2} ${y2} L ${x3} ${y3} A ${ir} ${ir} 0 ${largeArc} 0 ${x4} ${y4} Z`;

      const pct = total() > 0 ? ((d.value / total()) * 100).toFixed(1) : "0";
      startAngle = endAngle;

      return { ...d, path, pct };
    });
  });

  return (
    <div class="flex flex-col items-center gap-3">
      {props.title && <h3 class="text-sm font-medium text-slate-400">{props.title}</h3>}
      <svg width={size()} height={size()} viewBox={`0 0 ${size()} ${size()}`}>
        <For each={segments()}>
          {(seg) => (
            <path d={seg.path} fill={seg.color} stroke="#1e293b" stroke-width="2">
              <title>{seg.label}: {seg.pct}%</title>
            </path>
          )}
        </For>
        <text x={center()} y={center()} text-anchor="middle" dominant-baseline="middle"
              fill="#e2e8f0" font-size="14" font-weight="bold">
          {total().toLocaleString()}
        </text>
      </svg>
      <div class="flex flex-wrap gap-3 justify-center text-xs">
        <For each={segments()}>
          {(seg) => (
            <div class="flex items-center gap-1">
              <span class="w-2.5 h-2.5 rounded-full" style={{ background: seg.color }} />
              <span class="text-slate-400">{seg.label}</span>
              <span class="text-slate-300 font-medium">{seg.pct}%</span>
            </div>
          )}
        </For>
      </div>
    </div>
  );
};

export default DonutChart;
