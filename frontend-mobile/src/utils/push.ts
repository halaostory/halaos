// Phone notifications (Web Push) for the installed app.
import { pushAPI } from "../api/client";

export function pushSupported(): boolean {
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
  if (!pushSupported() || Notification.permission !== "granted") return false;
  const reg = await navigator.serviceWorker.getRegistration("/m/");
  return !!(reg && (await reg.pushManager.getSubscription()));
}

/** Asks for permission and registers this device. Returns false if refused. */
export async function enablePush(): Promise<boolean> {
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
  const reg = await navigator.serviceWorker.getRegistration("/m/");
  const sub = reg && (await reg.pushManager.getSubscription());
  if (!sub) return;
  await pushAPI.unsubscribe(sub.endpoint).catch(() => undefined);
  await sub.unsubscribe();
}
