// @file web/playwright.config.ts
// 职责：Playwright 配置。被测服务由 scripts/verify-p2.sh 起好，这里只负责连上去跑。
// 边界：不在配置里启动 webServer——后端端口是随机的，写死会撞端口。
import { defineConfig } from '@playwright/test'
import fs from 'node:fs'
import path from 'node:path'
import os from 'node:os'

const baseURL = process.env.BASE_URL || 'http://127.0.0.1:8080'

// 浏览器可执行文件探测。
// Playwright 1.55 默认找 chromium_headless_shell-1187，但本机只装了 1178/1217/1247/1248。
// 不传 PW_CHROMIUM 时自动找最新可用的 headless shell，避免"Executable doesn't exist"报错。
function findBrowser(): string | undefined {
  if (process.env.PW_CHROMIUM) return process.env.PW_CHROMIUM
  const cache = path.join(os.homedir(), 'Library/Caches/ms-playwright')
  if (!fs.existsSync(cache)) return undefined
  const dirs = fs.readdirSync(cache)
    .filter((d) => d.startsWith('chromium_headless_shell-') || d.startsWith('chromium-'))
    .sort()
  for (const dir of dirs.reverse()) {
    const cand = path.join(cache, dir, 'chrome-headless-shell-mac-arm64/chrome-headless-shell')
    if (fs.existsSync(cand)) return cand
    const cand2 = path.join(cache, dir, 'chrome-mac/Chromium.app/Contents/MacOS/Chromium')
    if (fs.existsSync(cand2)) return cand2
  }
  return undefined
}

const executablePath = findBrowser()

export default defineConfig({
  testDir: './e2e',
  timeout: 45_000,
  expect: { timeout: 10_000 },
  fullyParallel: false,
  // 单机单实例 + 内存限流，并发跑会互相抢配额，串行更可靠。
  workers: 1,
  retries: 0,
  reporter: [['list']],
  outputDir: process.env.EVIDENCE_DIR || './test-results',
  use: {
    baseURL,
    launchOptions: executablePath ? { executablePath } : {},
    viewport: { width: 1280, height: 880 },
    locale: 'zh-CN',
    // 组件里的测试锚点统一用 data-test 属性。
    testIdAttribute: 'data-test',
    // 失败时留截图与 trace，成功不留，避免证据目录塞满。
    screenshot: 'only-on-failure',
    trace: 'retain-on-failure',
  },
})
