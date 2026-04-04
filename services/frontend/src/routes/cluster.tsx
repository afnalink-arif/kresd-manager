import { createResource, Show, For } from "solid-js";
import Layout from "~/components/Layout";
import KPICard from "~/components/KPICard";
import { clusterAPI } from "~/lib/api";
import { extractValue, fmt } from "~/lib/prometheus";

export default function ClusterPage() {
  const [overview, { refetch }] = createResource(() => clusterAPI.getOverview());

  const onlineCount = () => {
    const o = overview();
    if (!o) return 0;
    return o.nodes.filter((n) => n.status === "online" || n.status === "degraded").length;
  };

  const totalNodes = () => overview()?.node_count || 0;

  const aggregateMetric = (key: string): number | null => {
    const o = overview();
    if (!o || o.nodes.length === 0) return null;
    let sum = 0;
    let count = 0;
    for (const n of o.nodes) {
      if (!n.metrics) continue;
      const v = extractValue(n.metrics[key]);
      if (v !== null && !isNaN(v)) {
        sum += v;
        count++;
      }
    }
    return count > 0 ? sum : null;
  };

  const avgMetric = (key: string): number | null => {
    const o = overview();
    if (!o || o.nodes.length === 0) return null;
    let sum = 0;
    let count = 0;
    for (const n of o.nodes) {
      if (!n.metrics) continue;
      const v = extractValue(n.metrics[key]);
      if (v !== null && !isNaN(v)) {
        sum += v;
        count++;
      }
    }
    return count > 0 ? sum / count : null;
  };

  const statusColor = (status: string) => {
    switch (status) {
      case "online": return "bg-emerald-500";
      case "degraded": return "bg-amber-500";
      case "offline": return "bg-red-500";
      default: return "bg-slate-500";
    }
  };

  const timeSince = (date: string | null) => {
    if (!date) return "Never";
    const diff = Date.now() - new Date(date).getTime();
    if (diff < 60000) return `${Math.floor(diff / 1000)}s ago`;
    if (diff < 3600000) return `${Math.floor(diff / 60000)}m ago`;
    return `${Math.floor(diff / 3600000)}h ago`;
  };

  // Auto-refresh every 15s
  setInterval(() => refetch(), 15000);

  return (
    <Layout>
      <div class="space-y-6">
        <div class="flex items-center justify-between">
          <div>
            <h1 class="text-2xl font-bold text-white">Cluster Overview</h1>
            <p class="text-sm text-slate-400 mt-1">Monitoring all registered DNS nodes</p>
          </div>
          <span class="text-sm text-slate-500">{totalNodes()} nodes registered</span>
        </div>

        {/* Aggregated KPIs */}
        <div class="grid grid-cols-1 md:grid-cols-4 gap-4">
          <KPICard
            title="Nodes Online"
            value={`${onlineCount()} / ${totalNodes()}`}
            subtitle="Active nodes in cluster"
            color={onlineCount() === totalNodes() ? "#22c55e" : "#eab308"}
          />
          <KPICard
            title="Total QPS"
            value={aggregateMetric("qps") !== null ? fmt(aggregateMetric("qps")!, 1) : "--"}
            subtitle="Queries/sec across all nodes"
            color="#3b82f6"
          />
          <KPICard
            title="Avg Cache Hit"
            value={avgMetric("cache_hit_ratio") !== null ? fmt(avgMetric("cache_hit_ratio")! * 100, 1) + "%" : "--"}
            subtitle="Average cache hit ratio"
            color="#22c55e"
          />
          <KPICard
            title="Avg Latency"
            value={avgMetric("avg_latency_ms") !== null ? fmt(avgMetric("avg_latency_ms")!, 1) + "ms" : "--"}
            subtitle="Average query latency"
            color="#a855f7"
          />
        </div>

        {/* Node Grid */}
        <Show
          when={overview() && overview()!.nodes.length > 0}
          fallback={
            <div class="bg-slate-800 rounded-xl p-8 border border-slate-700 text-center">
              <p class="text-slate-400">No nodes registered yet.</p>
              <p class="text-sm text-slate-500 mt-1">Go to Settings to add agent nodes.</p>
            </div>
          }
        >
          <div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
            <For each={overview()!.nodes}>
              {(node) => {
                const nodeQps = () => node.metrics ? extractValue(node.metrics.qps) : null;
                const nodeCacheHit = () => node.metrics ? extractValue(node.metrics.cache_hit_ratio) : null;
                const nodeLatency = () => node.metrics ? extractValue(node.metrics.avg_latency_ms) : null;

                return (
                  <div class="bg-slate-800 rounded-xl p-5 border border-slate-700 hover:border-slate-600 transition-colors">
                    <div class="flex items-center justify-between mb-4">
                      <div class="flex items-center gap-3">
                        <span class={`w-2.5 h-2.5 rounded-full ${statusColor(node.status)} ${node.status === "online" ? "animate-pulse" : ""}`} />
                        <div>
                          <h3 class="text-white font-medium text-sm">{node.name || node.domain}</h3>
                          <p class="text-xs text-slate-500">{node.domain}</p>
                        </div>
                      </div>
                      <Show when={node.version}>
                        <span class="text-xs font-mono text-slate-500">{node.version}</span>
                      </Show>
                    </div>

                    <div class="grid grid-cols-3 gap-3">
                      <div>
                        <p class="text-xs text-slate-500">QPS</p>
                        <p class="text-lg font-medium text-white">{nodeQps() !== null ? fmt(nodeQps()!, 1) : "--"}</p>
                      </div>
                      <div>
                        <p class="text-xs text-slate-500">Cache</p>
                        <p class="text-lg font-medium text-emerald-400">
                          {nodeCacheHit() !== null ? fmt(nodeCacheHit()! * 100, 1) + "%" : "--"}
                        </p>
                      </div>
                      <div>
                        <p class="text-xs text-slate-500">Latency</p>
                        <p class="text-lg font-medium text-purple-400">
                          {nodeLatency() !== null ? fmt(nodeLatency()!, 1) + "ms" : "--"}
                        </p>
                      </div>
                    </div>

                    <div class="mt-3 pt-3 border-t border-slate-700 flex items-center justify-between">
                      <span class={`text-xs capitalize ${
                        node.status === "online" ? "text-emerald-400" :
                        node.status === "degraded" ? "text-amber-400" :
                        node.status === "offline" ? "text-red-400" : "text-slate-400"
                      }`}>{node.status}</span>
                      <span class="text-xs text-slate-500">{timeSince(node.last_seen_at)}</span>
                    </div>

                    <Show when={node.last_error}>
                      <div class="mt-2 p-2 bg-red-500/10 rounded text-xs text-red-400 truncate" title={node.last_error}>
                        {node.last_error}
                      </div>
                    </Show>
                  </div>
                );
              }}
            </For>
          </div>
        </Show>
      </div>
    </Layout>
  );
}
