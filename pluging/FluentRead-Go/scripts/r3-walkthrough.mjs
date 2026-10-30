// @file scripts/r3-walkthrough.mjs
// 职责：R3 走查的环境准备——Node 脚本，不是 bash。
// 只做三件事：起假网关 → 起后端 → 跑 Playwright。不做 curl 断言。
// 边界：测试逻辑全在 `web/e2e/r3-walkthrough.spec.ts` 里，这里只负责生命周期。

import { spawn } from 'node:child_process'
import { createServer } from 'node:net'
import { fileURLToPath } from 'node:url'
import path from 'node:path'
import fs from 'node:fs'

const __dirname = path.dirname(fileURLToPath(import.meta.url))
const ROOT = path.resolve(__dirname, '..')

function freePort() {
  return new Promise((resolve) => {
    const s = createServer()
    s.listen(0, '127.0.0.1', () => {
      const p = s.address().port
      s.close(() => resolve(p))
    })
  })
}

function run(cmd, args, opts = {}) {
  return new Promise((resolve, reject) => {
    const cp = spawn(cmd, args, {
      cwd: opts.cwd || ROOT,
      env: { ...process.env, ...(opts.env || {}) },
      stdio: 'pipe',
    })
    let out = ''
    cp.stdout.on('data', (d) => { out += d })
    cp.stderr.on('data', (d) => { out += d })
    cp.on('error', reject)
    cp.on('close', (code) => resolve({ code, out }))
  })
}

function sleep(ms) {
  return new Promise((r) => setTimeout(r, ms))
}

async function waitFor(url, maxSec = 20) {
  for (let i = 0; i < maxSec * 5; i++) {
    try {
      const res = await fetch(url, { signal: AbortSignal.timeout(2000) })
      if (res.ok) return true
    } catch { /* ignore */ }
    await sleep(200)
  }
  return false
}

async function main() {
  const gwPort = await freePort()
  const frPort = await freePort()
  const dbPath = path.join(ROOT, 'data', 'r3.db')
  fs.mkdirSync(path.dirname(dbPath), { recursive: true })

  // 必须先构建前端、再编译后端：go:embed 在编译时把 web/dist 打进二进制，
  // 顺序反了就会嵌入空目录，页面打开是"404 page not found"。
  console.log('=== 构建前端 ===')
  const webDir = path.join(ROOT, 'web')
  const distDir = path.join(webDir, 'dist')
  fs.readdirSync(distDir).forEach((f) => {
    const p = path.join(distDir, f)
    if (fs.lstatSync(p).isDirectory()) fs.rmSync(p, { recursive: true })
    else fs.unlinkSync(p)
  })
  const buildRes = await run('npm', ['run', 'build'], { cwd: webDir })
  if (buildRes.code !== 0) {
    console.error('!! 前端构建失败:\n' + buildRes.out)
    process.exit(1)
  }

  console.log('=== 编译后端 ===')
  const goRes = await run('go', ['build', '-o', 'bin/fluentread', './cmd/server'])
  if (goRes.code !== 0) {
    console.error('!! 后端编译失败:\n' + goRes.out)
    process.exit(1)
  }

  console.log(`=== 起假网关 (:${gwPort}) ===`)
  const gw = spawn('./bin/fake-gateway', ['-addr', `127.0.0.1:${gwPort}`, '-models', 'deploy/models.json'], {
    cwd: ROOT, env: process.env, stdio: 'ignore',
  })

  if (!await waitFor(`http://127.0.0.1:${gwPort}/v1/models`)) {
    console.error('!! 假网关未就绪')
    gw.kill()
    process.exit(1)
  }

  console.log(`=== 起后端 (:${frPort}) ===`)
  const fr = spawn('./bin/fluentread', [], {
    cwd: ROOT,
    env: {
      ...process.env,
      FR_ADDR: `127.0.0.1:${frPort}`,
      FR_DB_PATH: dbPath,
      FR_GATEWAY_BASE: `http://127.0.0.1:${gwPort}`,
      FR_GATEWAY_KEY: 'test-key',
      FR_DEFAULT_MODEL: 'deepseek-chat',
      FR_MYMEMORY_BASE: `http://127.0.0.1:${gwPort}/get`,
      FR_QUOTA_DAILY: '1000',
      FR_RATE_LIMIT: '1000',
      // 超时用例要靠它：假网关故意睡 30 秒，客户端若还等默认 60 秒，
      // 一个用例就把整场的 45 秒预算吃光，后面的长文和窄屏根本跑不到。
      FR_AI_TIMEOUT: '4s',
    },
    stdio: 'ignore',
  })

  if (!await waitFor(`http://127.0.0.1:${frPort}/api/health`)) {
    console.error('!! 后端未就绪')
    fr.kill()
    gw.kill()
    process.exit(1)
  }

  const shotsDir = path.join(ROOT, 'docs', 'evidence', 'R3', 'shots')
  fs.mkdirSync(shotsDir, { recursive: true })

  console.log('=== 跑 R3 走查 ===')
  // 环境变量必须放进 env 里。早先直接写在 opts 顶层，run() 只读 opts.env，
  // 于是 BASE_URL 被静默丢掉，Playwright 回落到默认 :8080——
  // 那是本机另一个服务的端口，页面打开是 "404 page not found"，
  // 看起来像前端没打进去，其实是跑错了地方。
  const testRes = await run('npx', ['playwright', 'test', 'e2e/r3-walkthrough.spec.ts'], {
    cwd: webDir,
    env: {
      BASE_URL: `http://127.0.0.1:${frPort}`,
      FR_GATEWAY_BASE: `http://127.0.0.1:${gwPort}`,
      EVIDENCE_DIR: shotsDir,
    },
  })
  console.log(testRes.out)

  fr.kill()
  gw.kill()
  await sleep(300)

  if (testRes.code !== 0) {
    console.error('!! R3 走查失败')
    process.exit(1)
  }
  console.log('\n=== R3 走查通过 ===')
  console.log('截图：docs/evidence/R3/shots/')
}

main().catch((e) => { console.error(e); process.exit(1) })
