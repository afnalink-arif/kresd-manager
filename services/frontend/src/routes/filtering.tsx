import { createSignal, onMount, Show, For } from "solid-js";
import Layout from "~/components/Layout";
import KPICard from "~/components/KPICard";
import { authHeaders } from "~/lib/auth";
import { filterAPI, type FilterRule } from "~/lib/api";

export default function FilteringPage() {
  const [rules, setRules] = createSignal<FilterRule[]>([]);
  const [stats, setStats] = createSignal<any>(null);
  const [loading, setLoading] = createSignal(true);
  const [applying, setApplying] = createSignal(false);
  const [msg, setMsg] = createSignal("");
  const [msgError, setMsgError] = createSignal(false);

  // Add domain
  const [newDomain, setNewDomain] = createSignal("");
  const [newCategory, setNewCategory] = createSignal("custom");

  // Import
  const [importMode, setImportMode] = createSignal(false);
  const [importUrl, setImportUrl] = createSignal("");
  const [importBulk, setImportBulk] = createSignal("");
  const [importCategory, setImportCategory] = createSignal("imported");
  const [importing, setImporting] = createSignal(false);

  // Search
  const [search, setSearch] = createSignal("");

  onMount(() => { loadData(); });

  const loadData = async () => {
    setLoading(true);
    try {
      const [filterData, statsData] = await Promise.all([
        filterAPI.list(),
        filterAPI.stats(),
      ]);
      setRules(filterData.rules || []);
      setStats(statsData);
    } catch {}
    setLoading(false);
  };

  const showMsg = (text: string, error = false) => {
    setMsg(text);
    setMsgError(error);
    setTimeout(() => setMsg(""), 4000);
  };

  const handleAdd = async (e: Event) => {
    e.preventDefault();
    if (!newDomain()) return;
    try {
      await filterAPI.add({ domain: newDomain(), category: newCategory() });
      setNewDomain("");
      showMsg("Domain ditambahkan");
      loadData();
    } catch (err: any) { showMsg(err.message, true); }
  };

  const handleDelete = async (id: number) => {
    try {
      await filterAPI.delete(id);
      setRules((prev) => prev.filter((r) => r.id !== id));
    } catch {}
  };

  const handleToggle = async (id: number) => {
    try {
      await filterAPI.toggle(id);
      setRules((prev) => prev.map((r) => r.id === id ? { ...r, enabled: !r.enabled } : r));
    } catch {}
  };

  const handleImport = async (e: Event) => {
    e.preventDefault();
    setImporting(true);
    try {
      const result = await filterAPI.import({
        url: importUrl(),
        domains: importBulk(),
        category: importCategory(),
      });
      showMsg(`${result.imported} domain diimport`);
      setImportUrl("");
      setImportBulk("");
      setImportMode(false);
      loadData();
    } catch (err: any) { showMsg(err.message, true); }
    setImporting(false);
  };

  const handleApply = async () => {
    setApplying(true);
    try {
      const result = await filterAPI.apply();
      showMsg(`Filter diterapkan: ${result.domains_blocked} domain diblokir`);
    } catch (err: any) { showMsg(err.message, true); }
    setApplying(false);
  };

  const filteredRules = () => {
    const q = search().toLowerCase();
    if (!q) return rules();
    return rules().filter((r) => r.domain.includes(q) || r.category.includes(q));
  };

  const categories = () => {
    const cats = new Set(rules().map((r) => r.category));
    return Array.from(cats).sort();
  };

  const categoryColors: Record<string, string> = {
    custom: "#3b82f6",
    ads: "#f59e0b",
    malware: "#ef4444",
    tracking: "#8b5cf6",
    adult: "#ec4899",
    gambling: "#f97316",
    social: "#06b6d4",
    imported: "#6b7280",
  };

  return (
    <Layout>
      <div class="space-y-5">
        <div class="flex items-center justify-between">
          <div>
            <h1 class="text-xl font-bold text-white">DNS Filtering</h1>
            <p class="text-xs text-slate-500 mt-0.5">Kelola domain yang diblokir</p>
          </div>
          <button
            onClick={handleApply}
            disabled={applying()}
            class="px-4 py-2 bg-emerald-600 hover:bg-emerald-700 disabled:opacity-50 text-white text-sm font-medium rounded-lg transition-colors flex items-center gap-2"
          >
            <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
              <path stroke-linecap="round" stroke-linejoin="round" d="M5 13l4 4L19 7" />
            </svg>
            {applying() ? "Menerapkan..." : "Terapkan Filter"}
          </button>
        </div>

        {/* Alert */}
        <Show when={msg()}>
          <div class={`p-2.5 rounded-lg text-xs ${msgError() ? "bg-red-500/10 text-red-400" : "bg-emerald-500/10 text-emerald-400"}`}>
            {msg()}
          </div>
        </Show>

        {/* KPI */}
        <div class="grid grid-cols-1 md:grid-cols-4 gap-4">
          <KPICard
            title="Total Rules"
            value={String(stats()?.total_rules || 0)}
            subtitle="Semua aturan filter"
            color="#3b82f6"
          />
          <KPICard
            title="Aktif"
            value={String(stats()?.enabled_rules || 0)}
            subtitle="Domain diblokir"
            color="#22c55e"
          />
          <KPICard
            title="Kategori"
            value={String(stats()?.categories?.length || 0)}
            subtitle="Jenis filter"
            color="#a855f7"
          />
          <KPICard
            title="Status"
            value={applying() ? "Updating..." : "Ready"}
            subtitle="Filter engine"
            color="#eab308"
          />
        </div>

        {/* Add domain + Import */}
        <div class="bg-slate-800 rounded-xl p-4 border border-slate-700">
          <div class="flex items-center justify-between mb-3">
            <h3 class="text-sm font-medium text-white">Tambah Domain</h3>
            <button
              onClick={() => setImportMode(!importMode())}
              class="text-xs text-blue-400 hover:text-blue-300 transition-colors"
            >
              {importMode() ? "Tutup Import" : "Import List"}
            </button>
          </div>

          {/* Quick add */}
          <form onSubmit={handleAdd} class="flex gap-2 items-end">
            <div class="flex-1">
              <input
                type="text"
                placeholder="contoh: ads.example.com"
                class="w-full bg-slate-900/50 border border-slate-700 rounded-lg px-3 py-2 text-sm text-white placeholder-slate-600 focus:outline-none focus:border-blue-500 transition"
                value={newDomain()}
                onInput={(e) => setNewDomain(e.target.value)}
              />
            </div>
            <select
              class="bg-slate-900/50 border border-slate-700 rounded-lg px-3 py-2 text-sm text-white focus:outline-none focus:border-blue-500 transition"
              value={newCategory()}
              onChange={(e) => setNewCategory(e.target.value)}
            >
              <option value="custom">Custom</option>
              <option value="ads">Ads</option>
              <option value="malware">Malware</option>
              <option value="tracking">Tracking</option>
              <option value="adult">Adult</option>
              <option value="gambling">Gambling</option>
              <option value="social">Social</option>
            </select>
            <button type="submit" class="px-4 py-2 bg-blue-600 hover:bg-blue-700 text-white text-sm rounded-lg transition-colors flex-shrink-0">
              + Blokir
            </button>
          </form>

          {/* Import panel */}
          <Show when={importMode()}>
            <form onSubmit={handleImport} class="mt-4 pt-4 border-t border-slate-700 space-y-3">
              <div class="grid grid-cols-2 gap-3">
                <div>
                  <label class="block text-xs text-slate-500 mb-1">URL Blocklist (format hosts)</label>
                  <input
                    type="url"
                    placeholder="https://raw.githubusercontent.com/..."
                    class="w-full bg-slate-900/50 border border-slate-700 rounded-lg px-3 py-2 text-sm text-white placeholder-slate-600 focus:outline-none focus:border-blue-500 transition"
                    value={importUrl()}
                    onInput={(e) => setImportUrl(e.target.value)}
                  />
                </div>
                <div>
                  <label class="block text-xs text-slate-500 mb-1">Kategori</label>
                  <select
                    class="w-full bg-slate-900/50 border border-slate-700 rounded-lg px-3 py-2 text-sm text-white focus:outline-none focus:border-blue-500 transition"
                    value={importCategory()}
                    onChange={(e) => setImportCategory(e.target.value)}
                  >
                    <option value="ads">Ads</option>
                    <option value="malware">Malware</option>
                    <option value="tracking">Tracking</option>
                    <option value="adult">Adult</option>
                    <option value="gambling">Gambling</option>
                    <option value="imported">Other</option>
                  </select>
                </div>
              </div>
              <div>
                <label class="block text-xs text-slate-500 mb-1">Atau paste domain (satu per baris)</label>
                <textarea
                  placeholder={"ads.example.com\ntracker.example.com\n..."}
                  rows={3}
                  class="w-full bg-slate-900/50 border border-slate-700 rounded-lg px-3 py-2 text-sm text-white placeholder-slate-600 focus:outline-none focus:border-blue-500 transition font-mono"
                  value={importBulk()}
                  onInput={(e) => setImportBulk(e.target.value)}
                />
              </div>
              <button
                type="submit"
                disabled={importing() || (!importUrl() && !importBulk())}
                class="px-4 py-2 bg-purple-600 hover:bg-purple-700 disabled:opacity-50 text-white text-sm rounded-lg transition-colors"
              >
                {importing() ? "Importing..." : "Import"}
              </button>
            </form>
          </Show>
        </div>

        {/* Rules list */}
        <div class="bg-slate-800 rounded-xl border border-slate-700 overflow-hidden">
          <div class="p-4 border-b border-slate-700 flex items-center justify-between">
            <div class="flex items-center gap-3">
              <h3 class="text-sm font-medium text-white">Blocked Domains</h3>
              <span class="text-xs text-slate-500">{filteredRules().length} rules</span>
            </div>
            <input
              type="text"
              placeholder="Cari domain..."
              class="bg-slate-900/50 border border-slate-700 rounded-lg px-3 py-1.5 text-xs text-white placeholder-slate-600 focus:outline-none focus:border-blue-500 transition w-48"
              value={search()}
              onInput={(e) => setSearch(e.target.value)}
            />
          </div>

          <Show when={!loading()} fallback={<div class="p-8 text-center text-slate-500 text-sm">Loading...</div>}>
            <Show
              when={filteredRules().length > 0}
              fallback={<div class="p-8 text-center text-slate-500 text-sm">Belum ada domain yang diblokir</div>}
            >
              <div class="max-h-[500px] overflow-y-auto">
                <For each={filteredRules()}>
                  {(rule) => (
                    <div class="flex items-center justify-between px-4 py-2.5 border-b border-slate-700/50 hover:bg-slate-700/20 group">
                      <div class="flex items-center gap-3 min-w-0">
                        <button
                          onClick={() => handleToggle(rule.id)}
                          class={`w-8 h-4 rounded-full transition-colors relative flex-shrink-0 ${
                            rule.enabled ? "bg-emerald-600" : "bg-slate-600"
                          }`}
                        >
                          <span class={`absolute top-0.5 w-3 h-3 bg-white rounded-full transition-transform ${
                            rule.enabled ? "left-4" : "left-0.5"
                          }`} />
                        </button>
                        <span class={`text-sm font-mono truncate ${rule.enabled ? "text-white" : "text-slate-500 line-through"}`}>
                          {rule.domain}
                        </span>
                        <span
                          class="text-[10px] px-1.5 py-0.5 rounded-full flex-shrink-0"
                          style={{ background: (categoryColors[rule.category] || "#6b7280") + "20", color: categoryColors[rule.category] || "#6b7280" }}
                        >
                          {rule.category}
                        </span>
                      </div>
                      <button
                        onClick={() => handleDelete(rule.id)}
                        class="text-red-400 opacity-0 group-hover:opacity-100 transition-opacity px-2 py-1 text-xs hover:bg-red-500/10 rounded"
                      >
                        Hapus
                      </button>
                    </div>
                  )}
                </For>
              </div>
            </Show>
          </Show>
        </div>

        {/* Info */}
        <div class="bg-slate-800/50 rounded-xl p-4 border border-slate-700/50">
          <p class="text-xs text-slate-500">
            Setelah menambah/menghapus domain, klik <strong class="text-emerald-400">Terapkan Filter</strong> untuk
            mengupdate konfigurasi DNS resolver. Domain yang diblokir akan diarahkan ke halaman block page.
          </p>
        </div>
      </div>
    </Layout>
  );
}
