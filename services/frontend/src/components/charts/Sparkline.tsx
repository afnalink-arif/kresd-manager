import type { Component } from "solid-js";
import { createMemo } from "solid-js";

interface SparklineProps {
  data: number[];
  width?: number;
  height?: number;
  color?: string;
}

const Sparkline: Component<SparklineProps> = (props) => {
  const w = () => props.width || 120;
  const h = () => props.height || 32;
  const color = () => props.color || "#3b82f6";

  const path = createMemo(() => {
    const d = props.data;
    if (!d || d.length < 2) return "";

    const min = Math.min(...d);
    const max = Math.max(...d);
    const range = max - min || 1;
    const stepX = w() / (d.length - 1);

    const points = d.map((v, i) => {
      const x = i * stepX;
      const y = h() - ((v - min) / range) * (h() - 4) - 2;
      return `${x},${y}`;
    });

    return `M ${points.join(" L ")}`;
  });

  const areaPath = createMemo(() => {
    if (!path()) return "";
    return `${path()} L ${w()},${h()} L 0,${h()} Z`;
  });

  return (
    <svg width={w()} height={h()} viewBox={`0 0 ${w()} ${h()}`}>
      <path d={areaPath()} fill={color()} opacity="0.15" />
      <path d={path()} fill="none" stroke={color()} stroke-width="1.5" />
    </svg>
  );
};

export default Sparkline;
