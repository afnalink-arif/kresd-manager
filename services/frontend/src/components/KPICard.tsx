import type { Component, JSX } from "solid-js";
import Sparkline from "./charts/Sparkline";

interface KPICardProps {
  title: string;
  value: string;
  subtitle?: string;
  sparklineData?: number[];
  color?: string;
  icon?: JSX.Element;
}

const KPICard: Component<KPICardProps> = (props) => {
  return (
    <div class="bg-slate-800 rounded-xl p-5 border border-slate-700 hover:border-slate-600 transition-colors">
      <div class="flex items-start justify-between">
        <div>
          <p class="text-sm text-slate-400 mb-1">{props.title}</p>
          <p class="text-2xl font-bold text-white">{props.value}</p>
          {props.subtitle && (
            <p class="text-xs text-slate-500 mt-1">{props.subtitle}</p>
          )}
        </div>
        <div class="flex flex-col items-end gap-2">
          {props.icon && <div class="text-slate-500">{props.icon}</div>}
          {props.sparklineData && (
            <Sparkline data={props.sparklineData} color={props.color || "#3b82f6"} />
          )}
        </div>
      </div>
    </div>
  );
};

export default KPICard;
