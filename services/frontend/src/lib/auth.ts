import { createSignal } from "solid-js";

export interface User {
  id: number;
  username: string;
  role: string;
}

const [token, setTokenRaw] = createSignal<string | null>(
  typeof localStorage !== "undefined" ? localStorage.getItem("token") : null
);
const [user, setUser] = createSignal<User | null>(null);

export function setToken(t: string | null) {
  if (t) {
    localStorage.setItem("token", t);
  } else {
    localStorage.removeItem("token");
  }
  setTokenRaw(t);
}

export function getToken() {
  return token();
}

export function getUser() {
  return user();
}

export function setUserInfo(u: User | null) {
  setUser(u);
}

export function logout() {
  setToken(null);
  setUser(null);
  window.location.href = "/login";
}

export function isLoggedIn() {
  return !!token();
}

// Add auth header to fetch requests
export function authHeaders(): Record<string, string> {
  const t = token();
  if (t) {
    return { Authorization: `Bearer ${t}` };
  }
  return {};
}
