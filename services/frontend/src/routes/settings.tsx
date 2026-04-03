import { createSignal, onMount } from "solid-js";
import Layout from "~/components/Layout";
import { authHeaders, getToken, logout } from "~/lib/auth";

export default function SettingsPage() {
  const [username, setUsername] = createSignal("");
  const [role, setRole] = createSignal("");

  // Change password
  const [oldPass, setOldPass] = createSignal("");
  const [newPass, setNewPass] = createSignal("");
  const [confirmPass, setConfirmPass] = createSignal("");
  const [passMsg, setPassMsg] = createSignal("");
  const [passError, setPassError] = createSignal(false);
  const [passLoading, setPassLoading] = createSignal(false);

  // Add user (admin only)
  const [newUser, setNewUser] = createSignal("");
  const [newUserPass, setNewUserPass] = createSignal("");
  const [newUserRole, setNewUserRole] = createSignal("viewer");
  const [userMsg, setUserMsg] = createSignal("");
  const [userError, setUserError] = createSignal(false);

  onMount(async () => {
    try {
      const res = await fetch("/api/auth/me", { headers: authHeaders() });
      if (res.ok) {
        const data = await res.json();
        setUsername(data.username);
        setRole(data.role);
      }
    } catch {}
  });

  const handleChangePassword = async (e: Event) => {
    e.preventDefault();
    setPassMsg("");
    setPassError(false);

    if (newPass() !== confirmPass()) {
      setPassMsg("Password baru tidak cocok");
      setPassError(true);
      return;
    }
    if (newPass().length < 8) {
      setPassMsg("Password minimal 8 karakter");
      setPassError(true);
      return;
    }

    setPassLoading(true);
    try {
      const res = await fetch("/api/auth/change-password", {
        method: "POST",
        headers: { "Content-Type": "application/json", ...authHeaders() },
        body: JSON.stringify({ old_password: oldPass(), new_password: newPass() }),
      });
      const data = await res.json();
      if (!res.ok) {
        setPassMsg(data.error || "Gagal ganti password");
        setPassError(true);
      } else {
        setPassMsg("Password berhasil diganti! Silakan login ulang.");
        setPassError(false);
        setOldPass("");
        setNewPass("");
        setConfirmPass("");
        setTimeout(() => logout(), 2000);
      }
    } catch (err: any) {
      setPassMsg(err.message);
      setPassError(true);
    } finally {
      setPassLoading(false);
    }
  };

  const handleAddUser = async (e: Event) => {
    e.preventDefault();
    setUserMsg("");
    setUserError(false);

    if (newUserPass().length < 8) {
      setUserMsg("Password minimal 8 karakter");
      setUserError(true);
      return;
    }

    try {
      const res = await fetch("/api/auth/register", {
        method: "POST",
        headers: { "Content-Type": "application/json", ...authHeaders() },
        body: JSON.stringify({ username: newUser(), password: newUserPass(), role: newUserRole() }),
      });
      const data = await res.json();
      if (!res.ok) {
        setUserMsg(data.error || "Gagal membuat user");
        setUserError(true);
      } else {
        setUserMsg(`User "${data.username}" (${data.role}) berhasil dibuat`);
        setUserError(false);
        setNewUser("");
        setNewUserPass("");
      }
    } catch (err: any) {
      setUserMsg(err.message);
      setUserError(true);
    }
  };

  return (
    <Layout>
      <div class="space-y-6 max-w-2xl">
        <div>
          <h1 class="text-2xl font-bold text-white">Settings</h1>
          <p class="text-sm text-slate-400 mt-1">Account and user management</p>
        </div>

        {/* Current User Info */}
        <div class="bg-slate-800 rounded-xl p-5 border border-slate-700">
          <h3 class="text-sm font-medium text-slate-400 mb-4">Current User</h3>
          <div class="flex items-center gap-4">
            <div class="w-12 h-12 bg-blue-600 rounded-full flex items-center justify-center text-white text-lg font-bold">
              {username() ? username()[0].toUpperCase() : "?"}
            </div>
            <div>
              <p class="text-white font-medium">{username()}</p>
              <p class="text-sm text-slate-400 capitalize">{role()}</p>
            </div>
          </div>
        </div>

        {/* Change Password */}
        <div class="bg-slate-800 rounded-xl p-5 border border-slate-700">
          <h3 class="text-sm font-medium text-slate-400 mb-4">Change Password</h3>

          {passMsg() && (
            <div class={`mb-4 p-3 rounded-lg border text-sm ${
              passError()
                ? "bg-red-500/10 border-red-500/20 text-red-400"
                : "bg-emerald-500/10 border-emerald-500/20 text-emerald-400"
            }`}>
              {passMsg()}
            </div>
          )}

          <form onSubmit={handleChangePassword} class="space-y-4">
            <div>
              <label class="block text-sm text-slate-400 mb-1">Password Lama</label>
              <input
                type="password"
                required
                class="w-full bg-slate-900 border border-slate-700 rounded-lg px-4 py-2.5 text-white focus:outline-none focus:border-blue-500 transition"
                value={oldPass()}
                onInput={(e) => setOldPass(e.target.value)}
              />
            </div>
            <div>
              <label class="block text-sm text-slate-400 mb-1">Password Baru</label>
              <input
                type="password"
                required
                class="w-full bg-slate-900 border border-slate-700 rounded-lg px-4 py-2.5 text-white focus:outline-none focus:border-blue-500 transition"
                placeholder="Minimal 8 karakter"
                value={newPass()}
                onInput={(e) => setNewPass(e.target.value)}
              />
            </div>
            <div>
              <label class="block text-sm text-slate-400 mb-1">Konfirmasi Password Baru</label>
              <input
                type="password"
                required
                class="w-full bg-slate-900 border border-slate-700 rounded-lg px-4 py-2.5 text-white focus:outline-none focus:border-blue-500 transition"
                value={confirmPass()}
                onInput={(e) => setConfirmPass(e.target.value)}
              />
            </div>
            <button
              type="submit"
              disabled={passLoading()}
              class="px-6 py-2.5 bg-blue-600 hover:bg-blue-700 disabled:bg-blue-800 text-white text-sm font-medium rounded-lg transition-colors"
            >
              {passLoading() ? "Menyimpan..." : "Ganti Password"}
            </button>
          </form>
        </div>

        {/* Add User (Admin Only) */}
        {role() === "admin" && (
          <div class="bg-slate-800 rounded-xl p-5 border border-slate-700">
            <h3 class="text-sm font-medium text-slate-400 mb-4">Tambah User Baru</h3>

            {userMsg() && (
              <div class={`mb-4 p-3 rounded-lg border text-sm ${
                userError()
                  ? "bg-red-500/10 border-red-500/20 text-red-400"
                  : "bg-emerald-500/10 border-emerald-500/20 text-emerald-400"
              }`}>
                {userMsg()}
              </div>
            )}

            <form onSubmit={handleAddUser} class="space-y-4">
              <div class="grid grid-cols-2 gap-4">
                <div>
                  <label class="block text-sm text-slate-400 mb-1">Username</label>
                  <input
                    type="text"
                    required
                    class="w-full bg-slate-900 border border-slate-700 rounded-lg px-4 py-2.5 text-white focus:outline-none focus:border-blue-500 transition"
                    value={newUser()}
                    onInput={(e) => setNewUser(e.target.value)}
                  />
                </div>
                <div>
                  <label class="block text-sm text-slate-400 mb-1">Role</label>
                  <select
                    class="w-full bg-slate-900 border border-slate-700 rounded-lg px-4 py-2.5 text-white focus:outline-none focus:border-blue-500 transition"
                    value={newUserRole()}
                    onChange={(e) => setNewUserRole(e.target.value)}
                  >
                    <option value="viewer">Viewer</option>
                    <option value="admin">Admin</option>
                  </select>
                </div>
              </div>
              <div>
                <label class="block text-sm text-slate-400 mb-1">Password</label>
                <input
                  type="password"
                  required
                  class="w-full bg-slate-900 border border-slate-700 rounded-lg px-4 py-2.5 text-white focus:outline-none focus:border-blue-500 transition"
                  placeholder="Minimal 8 karakter"
                  value={newUserPass()}
                  onInput={(e) => setNewUserPass(e.target.value)}
                />
              </div>
              <button
                type="submit"
                class="px-6 py-2.5 bg-emerald-600 hover:bg-emerald-700 text-white text-sm font-medium rounded-lg transition-colors"
              >
                Tambah User
              </button>
            </form>
          </div>
        )}

        {/* Logout */}
        <div class="bg-slate-800 rounded-xl p-5 border border-red-500/20">
          <div class="flex items-center justify-between">
            <div>
              <h3 class="text-sm font-medium text-slate-400">Session</h3>
              <p class="text-xs text-slate-500 mt-1">Logout dari dashboard</p>
            </div>
            <button
              onClick={() => logout()}
              class="px-4 py-2 bg-red-600/20 hover:bg-red-600/30 text-red-400 text-sm rounded-lg border border-red-500/20 transition-colors"
            >
              Logout
            </button>
          </div>
        </div>
      </div>
    </Layout>
  );
}
