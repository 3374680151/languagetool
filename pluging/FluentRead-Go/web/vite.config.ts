// @file web/vite.config.ts
// 职责：前端构建配置。dev 时把 /api 代理到 Go 服务，build 时输出到 web/dist，
// 由 Go 的 go:embed 打进二进制（一个容器、一个进程，不额外加 nginx）。
// 边界：只做构建与代理配置，不写业务逻辑。
import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

// vite.config.ts 在 Node 里执行，但项目没有装 @types/node（刻意不加依赖），
// 这里只声明用到的那一点类型。
declare const process: { env: Record<string, string | undefined> }

// 后端地址。默认 8080；本地验证时后端跑在随机端口，用 FR_BACKEND 覆盖。
const backend = process.env.FR_BACKEND || 'http://127.0.0.1:8787'

export default defineConfig({
  plugins: [vue()],
  server: {
    port: 5173,
    strictPort: false,
    proxy: {
      '/api': { target: backend, changeOrigin: false },
    },
  },
  build: {
    outDir: 'dist',
    emptyOutDir: true,
    // 单页应用只有一个入口，CSS 合并成一个文件更省请求数。
    cssCodeSplit: false,
    chunkSizeWarningLimit: 200,
    rollupOptions: {
      output: {
        entryFileNames: 'assets/[name]-[hash].js',
        chunkFileNames: 'assets/[name]-[hash].js',
        assetFileNames: 'assets/[name]-[hash][extname]',
      },
    },
  },
})
