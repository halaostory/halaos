import type { CapacitorConfig } from "@capacitor/cli";

// Native iOS / Android shell for the employee app. Build the web bundle with
// `npm run build:native`, then `npx cap add ios|android` (once) and
// `npx cap sync`. See docs/mobile-app.md.
const config: CapacitorConfig = {
  appId: "com.halapeople.app",
  appName: "HalaPeople",
  webDir: "dist-native",
  server: { androidScheme: "https" },
  plugins: {
    PushNotifications: { presentationOptions: ["badge", "sound", "alert"] },
  },
};

export default config;
