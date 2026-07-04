import react from "@vitejs/plugin-react";
import { defineConfig } from "vite";

export default defineConfig({
  plugins: [react()],
  build: {
    rolldownOptions: {
      output: {
        codeSplitting: {
          minSize: 20 * 1024,
          groups: [
            {
              name: "react-vendor",
              test: /node_modules[\\/](react|react-dom|scheduler)[\\/]/,
              priority: 30,
            },
            {
              name: "antd-vendor",
              test: /node_modules[\\/](antd|@ant-design|rc-[^\\/]+|@rc-component)[\\/]/,
              priority: 20,
              maxSize: 320 * 1024,
            },
            {
              name: "vendor",
              test: /node_modules[\\/]/,
              priority: 10,
              maxSize: 320 * 1024,
            },
          ],
        },
      },
    },
  },
  server: {
    host: "127.0.0.1",
    port: 9202,
    proxy: {
      "/api": "http://127.0.0.1:9200",
    },
  },
});
