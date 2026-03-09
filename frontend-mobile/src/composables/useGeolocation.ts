import { ref } from "vue";

export type GeoStatus = "idle" | "acquiring" | "acquired" | "denied" | "error";

export function useGeolocation() {
  const status = ref<GeoStatus>("idle");
  const lat = ref<number | null>(null);
  const lng = ref<number | null>(null);

  function acquire(): Promise<void> {
    return new Promise((resolve) => {
      if (!navigator.geolocation) {
        status.value = "error";
        resolve();
        return;
      }

      status.value = "acquiring";

      navigator.geolocation.getCurrentPosition(
        (pos) => {
          lat.value = pos.coords.latitude;
          lng.value = pos.coords.longitude;
          status.value = "acquired";
          resolve();
        },
        (err) => {
          if (err.code === err.PERMISSION_DENIED) {
            status.value = "denied";
          } else {
            status.value = "error";
          }
          resolve();
        },
        {
          enableHighAccuracy: true,
          timeout: 10000,
          maximumAge: 0,
        },
      );
    });
  }

  return { status, lat, lng, acquire };
}
