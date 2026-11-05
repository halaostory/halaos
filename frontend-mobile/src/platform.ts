// Where the app runs and which server it talks to.
//
// The same app serves every company: in the browser it talks to the server
// it was loaded from (/api); the native iOS/Android app (Capacitor) talks to
// HalaPeople cloud by default (VITE_API_URL at build time) or to a
// company's own server entered on the sign-in screen (self-hosted).
import { Capacitor } from "@capacitor/core";

export const isNative = Capacitor.isNativePlatform();

const KEY = "server_url";

/** The server the person chose (native app only), without /api. */
export function serverURL(): string {
  try {
    return localStorage.getItem(KEY) || "";
  } catch {
    return "";
  }
}

/** Saves or clears the server; returns the normalised URL. */
export function setServerURL(url: string): string {
  let u = url.trim().replace(/\/+$/, "").replace(/\/api$/, "");
  if (u && !/^https?:\/\//i.test(u)) u = "https://" + u;
  try {
    if (u) localStorage.setItem(KEY, u);
    else localStorage.removeItem(KEY);
  } catch {
    /* storage blocked */
  }
  return u;
}

/** Base URL of the API, e.g. /api or https://people.example.com/api. */
export function apiBase(): string {
  const chosen = serverURL();
  if (chosen) return chosen + "/api";
  return import.meta.env.VITE_API_URL || "/api";
}
