import type {
  ApiKey,
  OAuthAuthorizeInfo,
  OAuthConnection,
  NewApiKey,
  Recipe,
  RecipeInput,
  RecipeList,
  Revision,
  Invite,
  InviteInfo,
  Share,
  TagCount,
  User,
} from './types'

export class ApiError extends Error {
  status: number
  constructor(status: number, message: string) {
    super(message)
    this.status = status
  }
}

// Access tokens are short-lived (15 min) and live in an httpOnly cookie. When a
// request comes back 401 we silently redeem the refresh cookie once - shared
// by all concurrent requests, since refresh tokens are single-use - and retry.
let refreshing: Promise<boolean> | null = null

function refreshSession(): Promise<boolean> {
  refreshing ??= fetch('/api/v1/auth/refresh', {
    method: 'POST',
    credentials: 'include',
    headers: { 'Content-Type': 'application/json' },
    body: '{}',
  })
    .then((res) => res.ok)
    .catch(() => false)
    .finally(() => {
      refreshing = null
    })
  return refreshing
}

// Endpoints where a 401 means "wrong credentials", not "expired access token".
const noRefreshPaths = ['/auth/login', '/auth/refresh', '/auth/logout', '/setup', '/invites/']

// fetchWithRefresh is fetch with credentials and one transparent refresh+retry.
export async function fetchWithRefresh(url: string, init: RequestInit, canRefresh = true): Promise<Response> {
  const res = await fetch(url, { credentials: 'include', ...init })
  if (res.status === 401 && canRefresh && (await refreshSession())) {
    return fetch(url, { credentials: 'include', ...init })
  }
  return res
}

async function request<T>(path: string, init?: RequestInit, prefix = '/api/v1'): Promise<T> {
  const canRefresh = !noRefreshPaths.some((p) => path.startsWith(p))
  const res = await fetchWithRefresh(
    `${prefix}${path}`,
    { headers: { 'Content-Type': 'application/json' }, ...init },
    canRefresh,
  )
  if (!res.ok) {
    const body = await res.json().catch(() => ({}))
    throw new ApiError(res.status, body.detail || body.error_description || body.title || `request failed: ${res.status}`)
  }
  if (res.status === 204) return undefined as T
  return res.json()
}

const json = (body: unknown) => JSON.stringify(body)

export const authApi = {
  me: () => request<User>('/auth/me'),
  login: (email: string, password: string) =>
    request<User>('/auth/login', { method: 'POST', body: json({ email, password }) }),
  logout: () => request<{ status: string }>('/auth/logout', { method: 'POST' }),
  update: (input: { name?: string; currentPassword?: string; password?: string }) =>
    request<User>('/auth/me', { method: 'PATCH', body: json(input) }),
}

export const setupApi = {
  status: () => request<{ required: boolean }>('/setup/status'),
  complete: (name: string, email: string, password: string) =>
    request<User>('/setup', { method: 'POST', body: json({ name, email, password }) }),
}

export const usersApi = {
  list: () => request<User[]>('/users'),
  create: (input: { name: string; email: string; password: string; role: string }) =>
    request<User>('/users', { method: 'POST', body: json(input) }),
  update: (id: string, input: { name?: string; role?: string; password?: string }) =>
    request<User>(`/users/${id}`, { method: 'PATCH', body: json(input) }),
  remove: (id: string) => request<{ status: string }>(`/users/${id}`, { method: 'DELETE' }),
}

export const invitesApi = {
  list: () => request<Invite[]>('/invites'),
  create: (input: { email?: string; role: string; expiresInDays?: number }) =>
    request<Invite>('/invites', { method: 'POST', body: json(input) }),
  revoke: (id: string) => request<{ status: string }>(`/invites/${id}`, { method: 'DELETE' }),
  lookup: (token: string) => request<InviteInfo>(`/invites/lookup/${encodeURIComponent(token)}`),
  accept: (token: string, input: { name: string; email?: string; password: string }) =>
    request<User>(`/invites/accept/${encodeURIComponent(token)}`, { method: 'POST', body: json(input) }),
}

export const oauthApi = {
  connections: () => request<OAuthConnection[]>('/oauth/connections'),
  disconnect: (id: string) => request<{ status: string }>(`/oauth/connections/${id}`, { method: 'DELETE' }),
  // The consent endpoints live outside /api/v1 (they are part of the OAuth spec surface).
  authorizeInfo: (query: string) => request<OAuthAuthorizeInfo>(`/oauth/authorize/info?${query}`, undefined, ''),
  decide: (body: Record<string, unknown>) =>
    request<{ redirectUrl: string }>('/oauth/authorize/decision', { method: 'POST', body: json(body) }, ''),
}

export const apiKeysApi = {
  list: () => request<ApiKey[]>('/api-keys'),
  create: (name: string, readOnly: boolean) =>
    request<NewApiKey>('/api-keys', { method: 'POST', body: json({ name, readOnly }) }),
  delete: (id: string) => request<{ status: string }>(`/api-keys/${id}`, { method: 'DELETE' }),
}

export interface RecipeFilters {
  q?: string
  tag?: string
  status?: string
  limit?: number
  offset?: number
}

export const recipesApi = {
  list: (f: RecipeFilters) => {
    const params = new URLSearchParams()
    if (f.q) params.set('q', f.q)
    if (f.tag) params.set('tag', f.tag)
    if (f.status) params.set('status', f.status)
    params.set('limit', String(f.limit ?? 60))
    params.set('offset', String(f.offset ?? 0))
    return request<RecipeList>(`/recipes?${params.toString()}`)
  },
  get: (ref: string) => request<Recipe>(`/recipes/${encodeURIComponent(ref)}`),
  create: (input: RecipeInput) => request<Recipe>('/recipes', { method: 'POST', body: json(input) }),
  update: (ref: string, input: RecipeInput) =>
    request<Recipe>(`/recipes/${encodeURIComponent(ref)}`, { method: 'PATCH', body: json(input) }),
  delete: (ref: string) => request<{ status: string }>(`/recipes/${encodeURIComponent(ref)}`, { method: 'DELETE' }),
  tags: () => request<TagCount[]>('/tags'),
  revisions: (ref: string) => request<Revision[]>(`/recipes/${encodeURIComponent(ref)}/revisions`),
  restore: (ref: string, revision: number) =>
    request<Recipe>(`/recipes/${encodeURIComponent(ref)}/revisions/${revision}/restore`, { method: 'POST' }),
  imageUrl: (ref: string, version: string, thumb = false) =>
    `/api/v1/recipes/${encodeURIComponent(ref)}/image?v=${encodeURIComponent(version)}${thumb ? '&size=thumb' : ''}`,
  removeImage: (ref: string) => request<{ status: string }>(`/recipes/${encodeURIComponent(ref)}/image`, { method: 'DELETE' }),
  uploadImage: async (ref: string, file: File) => {
    const form = new FormData()
    form.append('file', file)
    // Raw fetch: the upload route is a multipart handler, not a JSON operation.
    const res = await fetchWithRefresh(`/api/v1/recipes/${encodeURIComponent(ref)}/image`, {
      method: 'PUT',
      body: form,
    })
    if (!res.ok) {
      const body = await res.json().catch(() => ({}))
      throw new ApiError(res.status, body.detail || `upload failed: ${res.status}`)
    }
  },
}

export const sharesApi = {
  list: (ref: string) => request<Share[]>(`/recipes/${encodeURIComponent(ref)}/shares`),
  create: (ref: string, expiresInDays: number) =>
    request<Share>(`/recipes/${encodeURIComponent(ref)}/shares`, { method: 'POST', body: json({ expiresInDays }) }),
  revoke: (ref: string, shareId: string) =>
    request<{ status: string }>(`/recipes/${encodeURIComponent(ref)}/shares/${shareId}`, { method: 'DELETE' }),
  getPublic: (token: string) => request<Recipe>(`/public/${encodeURIComponent(token)}`),
  publicImageUrl: (token: string) => `/api/v1/public/${encodeURIComponent(token)}/image`,
}
