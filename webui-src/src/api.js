// API 封装：自动携带 Bearer 令牌；401 时清除令牌并广播 auth-expired。

const TOKEN_KEY = 'mahiru_dybot_token'

export function getToken() {
  return localStorage.getItem(TOKEN_KEY) || ''
}

export function setToken(t) {
  if (t) localStorage.setItem(TOKEN_KEY, t)
  else localStorage.removeItem(TOKEN_KEY)
}

export function onAuthExpired(fn) {
  window.addEventListener('auth-expired', fn)
  return () => window.removeEventListener('auth-expired', fn)
}

async function request(method, url, body) {
  const headers = {}
  if (body !== undefined) headers['Content-Type'] = 'application/json'
  const t = getToken()
  if (t) headers['Authorization'] = 'Bearer ' + t
  const resp = await fetch(url, {
    method,
    headers,
    body: body !== undefined ? JSON.stringify(body) : undefined
  })
  if (resp.status === 401) {
    setToken('')
    window.dispatchEvent(new Event('auth-expired'))
    throw new Error('未登录或令牌已过期')
  }
  const ct = resp.headers.get('content-type') || ''
  const data = ct.includes('application/json') ? await resp.json() : await resp.text()
  return { status: resp.status, ok: resp.ok, data }
}

export const api = {
  get: (u) => request('GET', u),
  post: (u, b = {}) => request('POST', u, b),
  del: (u) => request('DELETE', u)
}
