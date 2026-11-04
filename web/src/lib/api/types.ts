export interface User {
  id: string
  name: string
  email: string
  role: 'admin' | 'member'
}

export interface Invite {
  id: string
  email?: string
  role: 'admin' | 'member'
  createdAt: string
  expiresAt: string
  expired: boolean
  /** Only present on the response that creates the invite. */
  link?: string
}

export interface InviteInfo {
  email?: string
  role: 'admin' | 'member'
}

export interface IngredientGroup {
  name: string
  items: string[]
}

export interface Step {
  text: string
  substeps?: string[]
}

export type RecipeStatus = 'draft' | 'published'

export interface Recipe {
  id: string
  slug: string
  title: string
  tagline: string
  author: string
  servings: number
  prepMinutes: number
  cookMinutes: number
  calories: number
  sourceUrl: string
  notes: string
  status: RecipeStatus
  tags: string[]
  ingredients: IngredientGroup[]
  steps: Step[]
  hasImage: boolean
  createdAt: string
  updatedAt: string
}

export type RecipeSummary = Omit<Recipe, 'sourceUrl' | 'notes' | 'ingredients' | 'steps' | 'createdAt'>

export interface RecipeList {
  items: RecipeSummary[]
  total: number
}

/** Editable fields; the same shape is used for create and for PATCH. */
export type RecipeInput = Partial<
  Pick<
    Recipe,
    | 'slug'
    | 'title'
    | 'tagline'
    | 'author'
    | 'servings'
    | 'prepMinutes'
    | 'cookMinutes'
    | 'calories'
    | 'sourceUrl'
    | 'notes'
    | 'status'
    | 'tags'
    | 'ingredients'
    | 'steps'
  >
>

export interface TagCount {
  tag: string
  count: number
}

export interface Revision {
  revision: number
  action: string
  actor: string
  createdAt: string
}

export interface Share {
  id: string
  token: string
  url: string
  createdAt: string
  expiresAt: string | null
}

export interface OAuthConnection {
  id: string
  clientName: string
  scopes: string[]
  createdAt: string
  lastUsedAt: string | null
}

export interface OAuthAuthorizeInfo {
  clientName: string
  redirectHost: string
  canWrite: boolean
}
