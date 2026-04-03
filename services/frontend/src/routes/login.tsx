import { createSignal, onMount, Show } from "solid-js";
import { useNavigate } from "@solidjs/router";
import { authAPI } from "~/lib/api";
import { setToken, setUserInfo, isLoggedIn } from "~/lib/auth";
import "~/app.css";

export default function LoginPage() {
  const navigate = useNavigate();
  const [username, setUsername] = createSignal("");
  const [password, setPassword] = createSignal("");
  const [error, setError] = createSignal("");
  const [loading, setLoading] = createSignal(false);
  const [setupMode, setSetupMode] = createSignal(false);

  onMount(async () => {
    if (isLoggedIn()) {
      navigate("/", { replace: true });
      return;
    }
    try {
      const check = await authAPI.check();
      setSetupMode(check.setup_needed);
    } catch {}
  });

  const handleSubmit = async (e: Event) => {
    e.preventDefault();
    setError("");
    setLoading(true);

    try {
      if (setupMode()) {
        // Register first admin user
        await authAPI.register(username(), password());
      }

      // Login
      const data = await authAPI.login(username(), password());
      setToken(data.token);
      setUserInfo(data.user);
      navigate("/", { replace: true });
    } catch (err: any) {
      setError(err.message || "Authentication failed");
    } finally {
      setLoading(false);
    }
  };

  return (
    <div class="min-h-screen flex items-center justify-center bg-slate-950 px-4">
      <div class="w-full max-w-md">
        {/* Logo */}
        <div class="text-center mb-8">
          <div class="w-16 h-16 bg-blue-600 rounded-2xl flex items-center justify-center mx-auto mb-4">
            <svg class="w-9 h-9 text-white" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M21 12a9 9 0 01-9 9m9-9a9 9 0 00-9-9m9 9H3m9 9a9 9 0 01-9-9m9 9c1.657 0 3-4.03 3-9s-1.343-9-3-9m0 18c-1.657 0-3-4.03-3-9s1.343-9 3-9m-9 9a9 9 0 019-9" />
            </svg>
          </div>
          <h1 class="text-2xl font-bold text-white">Knot DNS Monitor</h1>
          <p class="text-sm text-slate-500 mt-1">
            {setupMode() ? "Create your admin account" : "Sign in to dashboard"}
          </p>
        </div>

        {/* Login Form */}
        <form onSubmit={handleSubmit} class="bg-slate-900 rounded-2xl p-8 border border-slate-800 shadow-xl">
          <Show when={setupMode()}>
            <div class="mb-6 p-3 bg-blue-500/10 border border-blue-500/20 rounded-lg">
              <p class="text-sm text-blue-400">
                First time setup — create an admin account to secure your dashboard.
              </p>
            </div>
          </Show>

          <Show when={error()}>
            <div class="mb-4 p-3 bg-red-500/10 border border-red-500/20 rounded-lg">
              <p class="text-sm text-red-400">{error()}</p>
            </div>
          </Show>

          <div class="space-y-4">
            <div>
              <label class="block text-sm text-slate-400 mb-1.5">Username</label>
              <input
                type="text"
                required
                autofocus
                class="w-full bg-slate-800 border border-slate-700 rounded-lg px-4 py-2.5 text-white placeholder:text-slate-600 focus:outline-none focus:border-blue-500 focus:ring-1 focus:ring-blue-500 transition"
                placeholder="admin"
                value={username()}
                onInput={(e) => setUsername(e.target.value)}
              />
            </div>
            <div>
              <label class="block text-sm text-slate-400 mb-1.5">Password</label>
              <input
                type="password"
                required
                class="w-full bg-slate-800 border border-slate-700 rounded-lg px-4 py-2.5 text-white placeholder:text-slate-600 focus:outline-none focus:border-blue-500 focus:ring-1 focus:ring-blue-500 transition"
                placeholder={setupMode() ? "Min 8 characters" : "Enter password"}
                value={password()}
                onInput={(e) => setPassword(e.target.value)}
              />
            </div>
          </div>

          <button
            type="submit"
            disabled={loading()}
            class="w-full mt-6 py-2.5 bg-blue-600 hover:bg-blue-700 disabled:bg-blue-800 disabled:cursor-not-allowed text-white font-medium rounded-lg transition-colors"
          >
            {loading() ? "Please wait..." : setupMode() ? "Create Account & Login" : "Sign In"}
          </button>
        </form>

        <p class="text-center text-xs text-slate-600 mt-6">
          Knot DNS Monitor v1.0 — Powered by Knot Resolver 6.2
        </p>
      </div>
    </div>
  );
}
