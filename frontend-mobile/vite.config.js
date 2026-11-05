import { defineConfig } from "vite";
import vue from "@vitejs/plugin-vue";
import Components from "unplugin-vue-components/vite";
import { VantResolver } from "@vant/auto-import-resolver";
// `vite build --mode native` builds the Capacitor (iOS/Android) bundle:
// relative paths, its own output folder.
export default defineConfig(({ mode }) => ({
    base: mode === "native" ? "./" : "/m/",
    plugins: [
        vue(),
        Components({
            resolvers: [VantResolver()],
        }),
    ],
    server: {
        port: 3002,
        proxy: {
            "/api": {
                target: "http://127.0.0.1:8080",
                changeOrigin: true,
            },
        },
    },
    build: {
        outDir: mode === "native" ? "dist-native" : "dist",
        rollupOptions: {
            output: {
                manualChunks: {
                    vant: ["vant"],
                },
            },
        },
    },
}));
