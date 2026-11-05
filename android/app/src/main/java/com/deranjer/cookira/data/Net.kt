package com.deranjer.cookira.data

import kotlinx.coroutines.runBlocking
import okhttp3.Authenticator
import okhttp3.Interceptor
import okhttp3.Request
import okhttp3.Response
import okhttp3.Route

/**
 * Adds `Authorization: Bearer <token>` to requests for the signed-in server only. Requests
 * to any other origin (a share link on a different host, for example) never get the token.
 */
class AuthInterceptor(private val holder: TokenHolder) : Interceptor {
    override fun intercept(chain: Interceptor.Chain): Response {
        val session = holder.session
        val request = chain.request()
        val sameOrigin = session != null && request.url.toString().startsWith(session.serverUrl + "/")
        val authed = if (sameOrigin && request.header("Authorization") == null) {
            request.newBuilder().header("Authorization", "Bearer ${session!!.accessToken}").build()
        } else {
            request
        }
        return chain.proceed(authed)
    }
}

/**
 * Responds to a 401 by exchanging the refresh token for a new access pair and retrying.
 * Access tokens last 15 minutes, so this runs constantly; refresh tokens are single-use,
 * so a lock serialises concurrent attempts and losers simply reuse the winner's token.
 */
class TokenAuthenticator(
    private val holder: TokenHolder,
    private val auth: AuthRepository,
) : Authenticator {
    private val lock = Any()

    override fun authenticate(route: Route?, response: Response): Request? {
        val failedHeader = response.request.header("Authorization") ?: return null
        if (responseCount(response) >= 3) return null

        synchronized(lock) {
            val current = holder.session ?: return null
            if (!response.request.url.toString().startsWith(current.serverUrl + "/")) return null
            if ("Bearer ${current.accessToken}" != failedHeader) {
                // Someone else already refreshed while we waited for the lock.
                return response.request.newBuilder().header("Authorization", "Bearer ${current.accessToken}").build()
            }
            val renewed = runBlocking { auth.refresh() } ?: return null
            return response.request.newBuilder().header("Authorization", "Bearer ${renewed.accessToken}").build()
        }
    }

    private fun responseCount(response: Response): Int {
        var n = 1
        var prior = response.priorResponse
        while (prior != null) {
            n++
            prior = prior.priorResponse
        }
        return n
    }
}
