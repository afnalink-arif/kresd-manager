import { createSignal, onMount, Show, For } from "solid-js";
import Layout from "~/components/Layout";
import KPICard from "~/components/KPICard";
import { authHeaders } from "~/lib/auth";
import { filterAPI, blockpageAPI, type FilterRule, type BlockPageConfig } from "~/lib/api";

type Tab = "rules" | "blockpage";

export default function FilteringPage() {
  const [activeTab, setActiveTab] = createSignal<Tab>("rules");
  const [rules, setRules] = createSignal<FilterRule[]>([]);
  const [stats, setStats] = createSignal<any>(null);
  const [loading, setLoading] = createSignal(true);
  const [applying, setApplying] = createSignal(false);
  const [msg, setMsg] = createSignal("");
  const [msgError, setMsgError] = createSignal(false);

  // Block page config
  const [bpConfig, setBpConfig] = createSignal<BlockPageConfig>({
    title: "Akses Diblokir",
    subtitle: "Domain ini telah diblokir oleh administrator jaringan melalui DNS filtering.",
    message: "Jika Anda merasa ini adalah kesalahan, silakan hubungi administrator.",
    contact: "",
    bg_color: "#0f172a",
    accent_color: "#ef4444",
    show_domain: true,
    show_logo: true,
    footer_text: "DNS Filter — Knot DNS Monitor",
  });
  const [bpSaving, setBpSaving] = createSignal(false);

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

  onMount(() => { loadData(); loadBlockPageConfig(); });

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

  const loadBlockPageConfig = async () => {
    try { setBpConfig(await blockpageAPI.getConfig()); } catch {}
  };

  const handleSaveBlockPage = async () => {
    setBpSaving(true);
    try {
      await blockpageAPI.updateConfig(bpConfig());
      showMsg("Block page disimpan");
    } catch (err: any) { showMsg(err.message, true); }
    setBpSaving(false);
  };

  const updateBp = (key: keyof BlockPageConfig, value: any) => {
    setBpConfig((prev) => ({ ...prev, [key]: value }));
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

        {/* Rules Tab Content */}
        <Show when={activeTab() === "rules"}>

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

        </Show>

        {/* Tabs */}
        <div class="flex gap-1 bg-slate-800/50 rounded-lg p-1">
          <button onClick={() => setActiveTab("rules")}
            class={`flex-1 px-3 py-2 rounded-md text-xs font-medium transition-all ${activeTab() === "rules" ? "bg-slate-700 text-white shadow-sm" : "text-slate-400 hover:text-slate-300"}`}>
            Filter Rules
          </button>
          <button onClick={() => setActiveTab("blockpage")}
            class={`flex-1 px-3 py-2 rounded-md text-xs font-medium transition-all ${activeTab() === "blockpage" ? "bg-slate-700 text-white shadow-sm" : "text-slate-400 hover:text-slate-300"}`}>
            Block Page Design
          </button>
        </div>

        {/* Block Page Settings Tab */}
        <Show when={activeTab() === "blockpage"}>
          <div class="grid grid-cols-1 lg:grid-cols-2 gap-5">
            {/* Settings Form */}
            <div class="bg-slate-800 rounded-xl p-5 border border-slate-700 space-y-4">
              <h3 class="text-sm font-medium text-white mb-3">Kustomisasi Block Page</h3>

              <div>
                <label class="block text-xs text-slate-500 mb-1">Judul</label>
                <input type="text" class="w-full bg-slate-900/50 border border-slate-700 rounded-lg px-3 py-2 text-sm text-white focus:outline-none focus:border-blue-500 transition"
                  value={bpConfig().title} onInput={(e) => updateBp("title", e.target.value)} />
              </div>

              <div>
                <label class="block text-xs text-slate-500 mb-1">Subjudul</label>
                <textarea rows={2} class="w-full bg-slate-900/50 border border-slate-700 rounded-lg px-3 py-2 text-sm text-white focus:outline-none focus:border-blue-500 transition"
                  value={bpConfig().subtitle} onInput={(e) => updateBp("subtitle", e.target.value)} />
              </div>

              <div>
                <label class="block text-xs text-slate-500 mb-1">Pesan Tambahan</label>
                <textarea rows={2} class="w-full bg-slate-900/50 border border-slate-700 rounded-lg px-3 py-2 text-sm text-white focus:outline-none focus:border-blue-500 transition"
                  value={bpConfig().message} onInput={(e) => updateBp("message", e.target.value)} />
              </div>

              <div>
                <label class="block text-xs text-slate-500 mb-1">Info Kontak (opsional)</label>
                <input type="text" placeholder="Email: admin@example.com" class="w-full bg-slate-900/50 border border-slate-700 rounded-lg px-3 py-2 text-sm text-white placeholder-slate-600 focus:outline-none focus:border-blue-500 transition"
                  value={bpConfig().contact} onInput={(e) => updateBp("contact", e.target.value)} />
              </div>

              <div>
                <label class="block text-xs text-slate-500 mb-1">Footer</label>
                <input type="text" class="w-full bg-slate-900/50 border border-slate-700 rounded-lg px-3 py-2 text-sm text-white focus:outline-none focus:border-blue-500 transition"
                  value={bpConfig().footer_text} onInput={(e) => updateBp("footer_text", e.target.value)} />
              </div>

              <div class="grid grid-cols-2 gap-3">
                <div>
                  <label class="block text-xs text-slate-500 mb-1">Background</label>
                  <div class="flex items-center gap-2">
                    <input type="color" class="w-8 h-8 rounded border-0 cursor-pointer"
                      value={bpConfig().bg_color} onInput={(e) => updateBp("bg_color", e.target.value)} />
                    <input type="text" class="flex-1 bg-slate-900/50 border border-slate-700 rounded-lg px-3 py-2 text-xs text-white font-mono focus:outline-none focus:border-blue-500 transition"
                      value={bpConfig().bg_color} onInput={(e) => updateBp("bg_color", e.target.value)} />
                  </div>
                </div>
                <div>
                  <label class="block text-xs text-slate-500 mb-1">Accent</label>
                  <div class="flex items-center gap-2">
                    <input type="color" class="w-8 h-8 rounded border-0 cursor-pointer"
                      value={bpConfig().accent_color} onInput={(e) => updateBp("accent_color", e.target.value)} />
                    <input type="text" class="flex-1 bg-slate-900/50 border border-slate-700 rounded-lg px-3 py-2 text-xs text-white font-mono focus:outline-none focus:border-blue-500 transition"
                      value={bpConfig().accent_color} onInput={(e) => updateBp("accent_color", e.target.value)} />
                  </div>
                </div>
              </div>

              <div class="flex items-center gap-4">
                <label class="flex items-center gap-2 cursor-pointer">
                  <input type="checkbox" class="w-4 h-4 rounded bg-slate-900 border-slate-700 text-blue-600 focus:ring-blue-500"
                    checked={bpConfig().show_domain} onChange={(e) => updateBp("show_domain", e.target.checked)} />
                  <span class="text-xs text-slate-400">Tampilkan domain</span>
                </label>
                <label class="flex items-center gap-2 cursor-pointer">
                  <input type="checkbox" class="w-4 h-4 rounded bg-slate-900 border-slate-700 text-blue-600 focus:ring-blue-500"
                    checked={bpConfig().show_logo} onChange={(e) => updateBp("show_logo", e.target.checked)} />
                  <span class="text-xs text-slate-400">Tampilkan ikon</span>
                </label>
              </div>

              <button onClick={handleSaveBlockPage} disabled={bpSaving()}
                class="w-full px-4 py-2.5 bg-blue-600 hover:bg-blue-700 disabled:opacity-50 text-white text-sm font-medium rounded-lg transition-colors">
                {bpSaving() ? "Menyimpan..." : "Simpan Block Page"}
              </button>
            </div>

            {/* Live Preview */}
            <div class="bg-slate-800 rounded-xl border border-slate-700 overflow-hidden">
              <div class="px-4 py-2.5 border-b border-slate-700 flex items-center justify-between">
                <span class="text-xs text-slate-400">Preview</span>
                <a href="/blockpage" target="_blank" class="text-xs text-blue-400 hover:text-blue-300 transition-colors">
                  Buka di tab baru
                </a>
              </div>
              <div class="aspect-[4/3] relative overflow-hidden" style={`background:${bpConfig().bg_color}`}>
                {/* Mini preview */}
                <div class="absolute inset-0 flex items-center justify-center p-6">
                  <div class="w-full max-w-[280px] text-center p-5 rounded-xl border border-white/10" style="background:rgba(255,255,255,0.04);backdrop-filter:blur(20px)">
                    <Show when={bpConfig().show_logo}>
                      <div class="w-10 h-10 mx-auto mb-3 rounded-full flex items-center justify-center" style={`background:${bpConfig().accent_color}`}>
                        <svg class="w-5 h-5 text-white" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                          <path stroke-linecap="round" stroke-linejoin="round" d="M18.364 18.364A9 9 0 005.636 5.636m12.728 12.728A9 9 0 015.636 5.636m12.728 12.728L5.636 5.636"/>
                        </svg>
                      </div>
                    </Show>
                    <h3 class="text-sm font-bold text-white mb-1">{bpConfig().title}</h3>
                    <Show when={bpConfig().show_domain}>
                      <div class="text-[10px] font-mono px-2 py-1 rounded my-2" style={`color:${bpConfig().accent_color};background:rgba(0,0,0,0.3)`}>
                        blocked-domain.com
                      </div>
                    </Show>
                    <p class="text-[10px] text-slate-400 leading-relaxed">{bpConfig().subtitle}</p>
                    <p class="text-[10px] text-slate-500 mt-1">{bpConfig().message}</p>
                    <Show when={bpConfig().contact}>
                      <div class="mt-2 text-[9px] text-blue-300 bg-blue-500/10 rounded px-2 py-1">{bpConfig().contact}</div>
                    </Show>
                    <div class="mt-3 text-[8px] text-slate-600">{bpConfig().footer_text}</div>
                  </div>
                </div>
              </div>
            </div>
          </div>
        </Show>

        {/* Info */}
        <Show when={activeTab() === "rules"}>
          <div class="bg-slate-800/50 rounded-xl p-4 border border-slate-700/50">
            <p class="text-xs text-slate-500">
              Setelah menambah/menghapus domain, klik <strong class="text-emerald-400">Terapkan Filter</strong> untuk
              mengupdate konfigurasi DNS resolver. Domain yang diblokir akan diarahkan ke halaman block page.
            </p>
          </div>
        </Show>
      </div>
    </Layout>
  );
}
