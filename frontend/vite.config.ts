import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

export default defineConfig({
  plugins: [react()],
  build: {
    chunkSizeWarningLimit: 550,
    rolldownOptions: {
      output: {
        manualChunks(id) {
          const moduleID = id.replaceAll("\\", "/");
          if (moduleID.includes("/node_modules/hls.js/")) return "hls";
          if (moduleID.includes("/node_modules/lucide-react/")) return "icons";
          if (moduleID.includes("/node_modules/react/") || moduleID.includes("/node_modules/react-dom/") || moduleID.includes("/node_modules/react-router") || moduleID.includes("/node_modules/scheduler/")) return "react";
        }
      }
    }
  },
  server: {
    host: "127.0.0.1",
    port: 5173,
    proxy: {
      "/api": "http://127.0.0.1:8080",
      "/media": "http://127.0.0.1:8080",
      "/healthz": "http://127.0.0.1:8080"
    }
  }
});
