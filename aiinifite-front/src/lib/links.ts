/**
 * 链接一律基于当前访问域名动态拼接，不写死任何域名。
 * 无论前端部署在 https://cn.aiinfinite.online 还是 https://api.aiinfinite.online，
 * 登录/控制台等链接都会自动拼在当前域名后面。
 */
const SITE_HOSTS = new Set(['api.aiinfinite.online', 'cn.aiinfinite.online'])

function siteOrigin() {
  const currentUrl = new URL(window.location.href)
  // When embedded by Sub2API, the referrer is the outer api/cn site.
  try {
    const referrerUrl = new URL(document.referrer)
    if (SITE_HOSTS.has(referrerUrl.hostname)) {
      return referrerUrl.origin
    }
  } catch {
    // Use the current origin when there is no usable referrer.
  }

  return SITE_HOSTS.has(currentUrl.hostname) ? currentUrl.origin : window.location.origin
}

export function apiPath(path: string) {
  const clean = path.startsWith('/') ? path : `/${path}`
  return `${siteOrigin()}${clean}`
}

export const loginUrl = () => apiPath('/login')
export const registerUrl = () => apiPath('/register')
export const consoleUrl = () => apiPath('/')
