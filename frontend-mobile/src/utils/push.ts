// Phone notifications: Web Push in the browser / installed PWA, and FCM
// (via the Capacitor plugin) in the native iOS/Android app.
import { PushNotifications } from "@capacitor/push-notifications";
import { Capacitor } from "@capacitor/core";
import { pushAPI } from "../api/client";
import { isNative } from "../platform";

const NATIVE_KEY = "native_push_token";

export function pushSupported(): boolean {
  if (isNative) return true;
  return "serviceWorker" in navigator && "PushManager" in window && "Notification" in window;
}

function keyBytes(b64: string): Uint8Array {
  const pad = "=".repeat((4 - (b64.length % 4)) % 4);
  const raw = atob((b64 + pad).replace(/-/g, "+").replace(/_/g, "/"));
  return Uint8Array.from(raw, (c) => c.charCodeAt(0));
}

async function registration(): Promise<ServiceWorkerRegistration> {
  return (await navigator.serviceWorker.getRegistration("/m/")) || navigator.serviceWorker.register("/m/sw.js", { scope: "/m/" });
}

/** Whether this device currently receives pushes. */
export async function pushEnabled(): Promise<boolean> {
  if (isNative) {
    return (await PushNotifications.checkPermissions()).receive === "granted" && !!localStorage.getItem(NATIVE_KEY);
  }
  if (!pushSupported() || Notification.permission !== "granted") return false;
  const reg = await navigator.serviceWorker.getRegistration("/m/");
  return !!(reg && (await reg.pushManager.getSubscription()));
}

/** Asks for permission and registers this device. Returns false if refused. */
export async function enablePush(): Promise<boolean> {
  if (isNative) return enableNative();
  if (!pushSupported()) return false;
  if ((await Notification.requestPermission()) !== "granted") return false;
  const { public_key } = ((await pushAPI.key()) as { data: { public_key: string } }).data;
  const reg = await registration();
  await navigator.serviceWorker.ready;
  const sub = (await reg.pushManager.getSubscription()) ||
    (await reg.pushManager.subscribe({ userVisibleOnly: true, applicationServerKey: keyBytes(public_key) as BufferSource }));
  await pushAPI.subscribe(sub.toJSON() as Record<string, unknown>);
  return true;
}

export async function disablePush(): Promise<void> {
  if (isNative) {
    const tok = localStorage.getItem(NATIVE_KEY);
    if (tok) await pushAPI.removeNative(tok).catch(() => undefined);
    localStorage.removeItem(NATIVE_KEY);
    await PushNotifications.unregister().catch(() => undefined);
    return;
  }
  const reg = await navigator.serviceWorker.getRegistration("/m/");
  const sub = reg && (await reg.pushManager.getSubscription());
  if (!sub) return;
  await pushAPI.unsubscribe(sub.endpoint).catch(() => undefined);
  await sub.unsubscribe();
}

async function enableNative(): Promise<boolean> {
  let perm = await PushNotifications.checkPermissions();
  if (perm.receive === "prompt" || perm.receive === "prompt-with-rationale") perm = await PushNotifications.requestPermissions();
  if (perm.receive !== "granted") return false;
  const token = await new Promise<string>((resolve, reject) => {
    PushNotifications.addListener("registration", (t) => resolve(t.value));
    PushNotifications.addListener("registrationError", (e) => reject(new Error(String(e.error))));
    PushNotifications.register();
  });
  await pushAPI.registerNative(token, Capacitor.getPlatform() === "ios" ? "ios" : "android");
  localStorage.setItem(NATIVE_KEY, token);
  return true;
}

/** Native app: tapping a notification opens the page it points to. */
export function listenNativePushTaps(open: (path: string) => void): void {
  if (!isNative) return;
  PushNotifications.addListener("pushNotificationActionPerformed", (a) => {
    const url = String((a.notification.data as Record<string, string> | undefined)?.url || "/m/notifications");
    open(url.replace(/^\/m/, "") || "/");
  });
}
