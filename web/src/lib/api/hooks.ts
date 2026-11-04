import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import {
  apiKeysApi,
  authApi,
  invitesApi,
  oauthApi,
  recipesApi,
  setupApi,
  sharesApi,
  usersApi,
  type RecipeFilters,
} from './client'
import type { RecipeInput } from './types'

export function useMe() {
  return useQuery({ queryKey: ['me'], queryFn: authApi.me, retry: false })
}

export function useLogin() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ email, password }: { email: string; password: string }) => authApi.login(email, password),
    onSuccess: (user) => qc.setQueryData(['me'], user),
  })
}

export function useLogout() {
  const qc = useQueryClient()
  return useMutation({ mutationFn: authApi.logout, onSuccess: () => qc.clear() })
}

export function useSetupStatus() {
  return useQuery({ queryKey: ['setup-status'], queryFn: setupApi.status, retry: false })
}

export function useCompleteSetup() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (v: { name: string; email: string; password: string }) =>
      setupApi.complete(v.name, v.email, v.password),
    onSuccess: (user) => {
      qc.setQueryData(['me'], user)
      qc.setQueryData(['setup-status'], { required: false })
    },
  })
}

export function useUsers() {
  return useQuery({ queryKey: ['users'], queryFn: usersApi.list })
}

export function useCreateUser() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: usersApi.create,
    onSuccess: () => qc.invalidateQueries({ queryKey: ['users'] }),
  })
}

export function useUpdateMe() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: authApi.update,
    onSuccess: (user) => qc.setQueryData(['me'], user),
  })
}

export function useUpdateUser() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ id, ...input }: { id: string; name?: string; role?: string; password?: string }) =>
      usersApi.update(id, input),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['users'] }),
  })
}

export function useRemoveUser() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: usersApi.remove,
    onSuccess: () => qc.invalidateQueries({ queryKey: ['users'] }),
  })
}

export function useInvites(enabled: boolean) {
  return useQuery({ queryKey: ['invites'], queryFn: invitesApi.list, enabled })
}

export function useCreateInvite() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: invitesApi.create,
    onSuccess: () => qc.invalidateQueries({ queryKey: ['invites'] }),
  })
}

export function useRevokeInvite() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: invitesApi.revoke,
    onSuccess: () => qc.invalidateQueries({ queryKey: ['invites'] }),
  })
}

export function useInviteInfo(token: string) {
  return useQuery({ queryKey: ['invite', token], queryFn: () => invitesApi.lookup(token), retry: false })
}

export function useAcceptInvite(token: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (v: { name: string; email?: string; password: string }) => invitesApi.accept(token, v),
    onSuccess: (user) => qc.setQueryData(['me'], user),
  })
}

export function useApiKeys() {
  return useQuery({ queryKey: ['api-keys'], queryFn: apiKeysApi.list })
}

export function useCreateApiKey() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ name, readOnly }: { name: string; readOnly: boolean }) => apiKeysApi.create(name, readOnly),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['api-keys'] }),
  })
}

export function useDeleteApiKey() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: apiKeysApi.delete,
    onSuccess: () => qc.invalidateQueries({ queryKey: ['api-keys'] }),
  })
}

export function useOAuthConnections() {
  return useQuery({ queryKey: ['oauth-connections'], queryFn: oauthApi.connections })
}

export function useDisconnectOAuth() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: oauthApi.disconnect,
    onSuccess: () => qc.invalidateQueries({ queryKey: ['oauth-connections'] }),
  })
}

export function useOAuthAuthorizeInfo(query: string) {
  return useQuery({ queryKey: ['oauth-authorize', query], queryFn: () => oauthApi.authorizeInfo(query), retry: false })
}

export function useOAuthDecision() {
  return useMutation({ mutationFn: oauthApi.decide })
}

export function useRecipes(filters: RecipeFilters) {
  return useQuery({ queryKey: ['recipes', filters], queryFn: () => recipesApi.list(filters) })
}

export function useRecipe(ref: string | undefined) {
  return useQuery({
    queryKey: ['recipe', ref],
    queryFn: () => recipesApi.get(ref!),
    enabled: !!ref,
  })
}

export function useTags() {
  return useQuery({ queryKey: ['tags'], queryFn: recipesApi.tags })
}

function useInvalidateRecipes() {
  const qc = useQueryClient()
  return () => {
    qc.invalidateQueries({ queryKey: ['recipes'] })
    qc.invalidateQueries({ queryKey: ['recipe'] })
    qc.invalidateQueries({ queryKey: ['tags'] })
  }
}

export function useCreateRecipe() {
  const invalidate = useInvalidateRecipes()
  return useMutation({ mutationFn: (input: RecipeInput) => recipesApi.create(input), onSuccess: invalidate })
}

export function useUpdateRecipe(ref: string) {
  const invalidate = useInvalidateRecipes()
  return useMutation({ mutationFn: (input: RecipeInput) => recipesApi.update(ref, input), onSuccess: invalidate })
}

export function useDeleteRecipe() {
  const invalidate = useInvalidateRecipes()
  return useMutation({ mutationFn: (ref: string) => recipesApi.delete(ref), onSuccess: invalidate })
}

export function useUploadImage(ref: string) {
  const invalidate = useInvalidateRecipes()
  return useMutation({ mutationFn: (file: File) => recipesApi.uploadImage(ref, file), onSuccess: invalidate })
}

export function useRemoveImage(ref: string) {
  const invalidate = useInvalidateRecipes()
  return useMutation({ mutationFn: () => recipesApi.removeImage(ref), onSuccess: invalidate })
}

export function useRevisions(ref: string, enabled: boolean) {
  return useQuery({
    queryKey: ['revisions', ref],
    queryFn: () => recipesApi.revisions(ref),
    enabled,
  })
}

export function useRestoreRevision(ref: string) {
  const invalidate = useInvalidateRecipes()
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (revision: number) => recipesApi.restore(ref, revision),
    onSuccess: () => {
      invalidate()
      qc.invalidateQueries({ queryKey: ['revisions', ref] })
    },
  })
}

export function useShares(ref: string, enabled: boolean) {
  return useQuery({ queryKey: ['shares', ref], queryFn: () => sharesApi.list(ref), enabled })
}

export function useCreateShare(ref: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (expiresInDays: number) => sharesApi.create(ref, expiresInDays),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['shares', ref] }),
  })
}

export function useRevokeShare(ref: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (shareId: string) => sharesApi.revoke(ref, shareId),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['shares', ref] }),
  })
}

export function useSharedRecipe(token: string | undefined) {
  return useQuery({
    queryKey: ['shared', token],
    queryFn: () => sharesApi.getPublic(token!),
    enabled: !!token,
    retry: false,
  })
}
