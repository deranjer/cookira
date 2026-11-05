package com.deranjer.cookira.data

import android.content.Context
import android.graphics.Bitmap
import android.graphics.BitmapFactory
import android.graphics.Matrix
import android.net.Uri
import androidx.exifinterface.media.ExifInterface
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.TimeoutCancellationException
import kotlinx.coroutines.withTimeout
import kotlinx.coroutines.async
import kotlinx.coroutines.awaitAll
import kotlinx.coroutines.coroutineScope
import kotlinx.coroutines.sync.Semaphore
import kotlinx.coroutines.sync.withPermit
import kotlinx.coroutines.withContext
import okhttp3.MediaType.Companion.toMediaType
import okhttp3.MultipartBody
import okhttp3.RequestBody.Companion.toRequestBody
import java.io.ByteArrayOutputStream
import java.io.IOException
import java.net.URLEncoder

/**
 * Sign-in, token refresh and sign-out. Uses a bare OkHttp client (no auth interceptor or
 * authenticator) so refreshing can never recurse into itself.
 */
class AuthRepository(
    private val authApis: ApiProvider,
    private val store: SessionStore,
    private val holder: TokenHolder,
    private val cache: RecipeCache,
) {
    /** Health + first-run check, used by the connect screen. Throws [ApiException]. */
    suspend fun probe(serverUrl: String): ServerProbe = apiCall {
        val api = authApis.service(serverUrl)
        val health = api.health()
        val setup = api.setupStatus()
        ServerProbe(health.version, setup.required)
    }

    suspend fun login(serverUrl: String, email: String, password: String): UserDto = apiCall {
        val dto = authApis.service(serverUrl).login(LoginRequest(email.trim(), password))
        persist(serverUrl, dto)
    }

    suspend fun setup(serverUrl: String, name: String, email: String, password: String): UserDto = apiCall {
        persist(serverUrl, authApis.service(serverUrl).setup(SetupRequest(name.trim(), email.trim(), password)))
    }

    suspend fun inviteInfo(serverUrl: String, token: String): InviteInfoDto =
        apiCall { authApis.service(serverUrl).lookupInvite(token) }

    suspend fun acceptInvite(serverUrl: String, token: String, name: String, email: String?, password: String): UserDto =
        apiCall {
            val body = AcceptInviteRequest(name.trim(), email?.trim()?.takeIf { it.isNotEmpty() }, password)
            persist(serverUrl, authApis.service(serverUrl).acceptInvite(token, body))
        }

    private suspend fun persist(serverUrl: String, dto: SessionDto): UserDto {
        val t = dto.tokens ?: throw ApiException("The server did not return tokens.")
        val session = Session(ServerUrl.normalize(serverUrl), t.accessToken, t.refreshToken)
        // Publish to memory first so the request that follows is already authenticated.
        holder.session = session
        store.save(session)
        return dto.user()
    }

    /**
     * Exchanges the refresh token for a new pair. Returns null when it can't: if the server
     * rejected the token the session is cleared (the UI falls back to sign-in); a network
     * error leaves the session alone so a flaky connection doesn't sign anyone out.
     */
    suspend fun refresh(): Session? {
        val old = holder.session ?: return null
        return try {
            val dto = authApis.service(old.serverUrl).refresh(RefreshRequest(old.refreshToken))
            val t = dto.tokens ?: return null
            val renewed = Session(old.serverUrl, t.accessToken, t.refreshToken)
            holder.session = renewed
            store.save(renewed)
            renewed
        } catch (e: retrofit2.HttpException) {
            if (e.code() == 401 || e.code() == 403) signOutLocally()
            null
        } catch (e: IOException) {
            null
        }
    }

    /** Revokes the refresh token server-side (best effort), then forgets the session. */
    suspend fun logout() {
        val s = holder.session
        if (s != null) {
            runCatching { authApis.service(s.serverUrl).logout(RefreshRequest(s.refreshToken)) }
        }
        signOutLocally()
    }

    suspend fun signOutLocally() {
        holder.session = null
        store.clear()
        // The offline library belongs to the signed-in account; don't leave it behind.
        cache.clear()
    }
}

data class ServerProbe(val version: String, val setupRequired: Boolean)

/** Everything else: recipes, sharing, household, connected apps. Requires a session. */
class CookiraRepository(
    private val apis: ApiProvider,
    private val holder: TokenHolder,
    private val context: Context,
    private val cache: RecipeCache,
) {
    private fun api(): ApiService {
        val s = holder.session ?: throw ApiException("Not signed in.", status = 401, unauthorized = true)
        return apis.service(s.serverUrl)
    }

    val serverUrl: String? get() = holder.session?.serverUrl

    // --- URLs for Coil (loaded through the authenticated client) ---

    fun imageUrl(recipeId: String, version: String, thumb: Boolean = false): String? {
        val base = serverUrl ?: return null
        val v = URLEncoder.encode(version, "UTF-8")
        return "$base/api/v1/recipes/$recipeId/image?v=$v" + if (thumb) "&size=thumb" else ""
    }

    fun sharedImageUrl(origin: String, token: String) = "${ServerUrl.normalize(origin)}/api/v1/public/$token/image"

    // --- account ---
    suspend fun me() = apiCall { api().me() }

    suspend fun updateName(name: String) = apiCall { api().updateMe(UpdateMeRequest(name = name.trim())) }

    suspend fun changePassword(current: String, next: String) = apiCall {
        api().updateMe(
            UpdateMeRequest(currentPassword = current, password = next, refreshToken = holder.session?.refreshToken),
        )
    }

    // --- recipes (network first; the saved copy answers when the server can't be reached) ---

    private val server get() = serverUrl.orEmpty()

    @Volatile
    private var offlineUntil = 0L

    /** Call when the user explicitly asks for fresh data (pull to refresh, Sync now). */
    fun forgetOffline() {
        offlineUntil = 0
    }

    /**
     * A read that may be answered from the saved copy. When there is a saved copy the network
     * gets only a few seconds (a dead connection would otherwise hang for the full connect
     * timeout), and after a failure we skip the network for a short while so each screen
     * doesn't wait again. With no saved copy it behaves like a normal call.
     */
    private suspend fun <T> readOrFallBack(block: suspend () -> T): T {
        val saved = cache.hasData(server)
        if (saved && System.currentTimeMillis() < offlineUntil) {
            throw ApiException("Can't reach the server.", network = true)
        }
        try {
            val result = if (saved) withTimeout(FAST_READ_MS) { apiCall(block) } else apiCall(block)
            offlineUntil = 0
            return result
        } catch (e: TimeoutCancellationException) {
            offlineUntil = System.currentTimeMillis() + OFFLINE_MEMO_MS
            throw ApiException("Can't reach the server.", network = true, cause = e)
        } catch (e: ApiException) {
            if (e.network) offlineUntil = System.currentTimeMillis() + OFFLINE_MEMO_MS
            throw e
        }
    }

    suspend fun recipes(q: String?, tag: String?, status: String?, limit: Int, offset: Int): RecipePage = try {
        val dto = readOrFallBack {
            api().recipes(q?.takeIf { it.isNotBlank() }, tag, status?.takeIf { it.isNotEmpty() }, limit, offset)
        }
        RecipePage(dto.items, dto.total, fromCache = false, syncedAt = cache.syncedAt(server))
    } catch (e: ApiException) {
        if (!e.network || !cache.hasData(server)) throw e
        val (items, total) = LocalLibrary.query(cache.all(server), q, tag, status?.takeIf { it.isNotEmpty() }, limit, offset)
        RecipePage(items, total, fromCache = true, syncedAt = cache.syncedAt(server))
    }

    suspend fun tags(): List<TagCount> = try {
        readOrFallBack { api().tags() }
    } catch (e: ApiException) {
        if (!e.network || !cache.hasData(server)) throw e
        LocalLibrary.tags(cache.all(server))
    }

    suspend fun recipe(ref: String): RecipeResult = try {
        val r = readOrFallBack { api().recipe(ref) }
        cache.upsert(server, r)
        RecipeResult(r, fromCache = false, syncedAt = cache.syncedAt(server))
    } catch (e: ApiException) {
        if (!e.network) throw e
        val saved = LocalLibrary.find(cache.all(server), ref) ?: throw ApiException(
            "You're offline and this recipe isn't saved on this phone yet.", network = true, cause = e,
        )
        RecipeResult(saved, fromCache = true, syncedAt = cache.syncedAt(server))
    }

    suspend fun createRecipe(input: RecipeInput) = apiCall { api().createRecipe(input) }.also { cache.upsert(server, it) }
    suspend fun updateRecipe(ref: String, input: RecipeInput) = apiCall { api().updateRecipe(ref, input) }.also { cache.upsert(server, it) }
    suspend fun deleteRecipe(ref: String) = apiCall { api().deleteRecipe(ref) }.also { cache.remove(server, ref) }
    suspend fun revisions(ref: String) = apiCall { api().revisions(ref) }
    suspend fun restoreRevision(ref: String, revision: Int) =
        apiCall { api().restoreRevision(ref, revision) }.also { cache.upsert(server, it) }

    // --- offline library ---

    suspend fun cachedCount(): Int = cache.count(server)
    suspend fun cachedSyncedAt(): Long = cache.syncedAt(server)
    suspend fun cachedRecipes(): List<Recipe> = cache.all(server)
    suspend fun clearOfflineLibrary() = cache.clear()

    /**
     * Brings the saved copy in line with the server: lists every recipe, then fetches the full
     * body of only those that are new or changed, and drops ones deleted elsewhere. Throws if
     * the server can't be reached, leaving the previous copy intact.
     */
    suspend fun syncLibrary(): Int {
        val remote = buildList {
            var offset = 0
            while (true) {
                val page = apiCall { api().recipes(null, null, null, SYNC_PAGE, offset) }
                addAll(page.items)
                offset += page.items.size
                if (page.items.isEmpty() || offset >= page.total) break
            }
        }
        val cached = cache.all(server)
        val stale = LocalLibrary.needsFetch(cached, remote)
        val gate = Semaphore(4)
        val fetched = coroutineScope {
            stale.map { s -> async { gate.withPermit { apiCall { api().recipe(s.id) } } } }.awaitAll()
        }.associateBy { it.id }
        val byId = cached.associateBy { it.id }
        cache.replaceAll(server, remote.mapNotNull { fetched[it.id] ?: byId[it.id] })
        return remote.size
    }

    // --- sharing ---
    suspend fun shares(ref: String) = apiCall { api().shares(ref) }
    suspend fun createShare(ref: String, expiresInDays: Int) = apiCall { api().createShare(ref, CreateShareRequest(expiresInDays)) }
    suspend fun revokeShare(ref: String, shareId: String) = apiCall { api().revokeShare(ref, shareId) }

    /** Public, token-gated; goes to [origin] (the link's own host), not necessarily our server. */
    suspend fun sharedRecipe(origin: String, token: String) = apiCall { apis.service(origin).sharedRecipe(token) }

    // --- photos ---
    suspend fun uploadImage(ref: String, uri: Uri) {
        val bytes = withContext(Dispatchers.IO) { readImageBytes(uri) }
        val part = MultipartBody.Part.createFormData(
            "file",
            "photo.jpg",
            bytes.toRequestBody("image/jpeg".toMediaType()),
        )
        apiCall { api().uploadImage(ref, part) }
        refreshCached(ref)
    }

    suspend fun removeImage(ref: String) {
        apiCall { api().removeImage(ref) }
        refreshCached(ref)
    }

    /** Photo changes alter hasImage / updatedAt; re-read the recipe so the saved copy matches. */
    private suspend fun refreshCached(ref: String) {
        runCatching { cache.upsert(server, api().recipe(ref)) }
    }

    /**
     * The server accepts up to 10 MB and resizes to 1600px itself, so downscaling on the
     * phone just saves upload time on mobile data. Re-encoding drops EXIF, so the
     * orientation is applied to the pixels here.
     */
    private fun readImageBytes(uri: Uri): ByteArray {
        val resolver = context.contentResolver
        val bounds = BitmapFactory.Options().apply { inJustDecodeBounds = true }
        resolver.openInputStream(uri)?.use { BitmapFactory.decodeStream(it, null, bounds) }
            ?: throw ApiException("Couldn't read that photo.")
        var sample = 1
        val longest = maxOf(bounds.outWidth, bounds.outHeight)
        while (longest / (sample * 2) >= MAX_UPLOAD_EDGE) sample *= 2
        val opts = BitmapFactory.Options().apply { inSampleSize = sample }
        val decoded = resolver.openInputStream(uri)?.use { BitmapFactory.decodeStream(it, null, opts) }
            ?: throw ApiException("Couldn't read that photo.")
        val orientation = runCatching {
            resolver.openInputStream(uri)?.use { ExifInterface(it).getAttributeInt(ExifInterface.TAG_ORIENTATION, 1) }
        }.getOrNull() ?: ExifInterface.ORIENTATION_NORMAL
        val rotated = decoded.rotated(orientation)
        val out = ByteArrayOutputStream()
        rotated.compress(Bitmap.CompressFormat.JPEG, 90, out)
        return out.toByteArray()
    }

    private fun Bitmap.rotated(orientation: Int): Bitmap {
        val m = Matrix()
        when (orientation) {
            ExifInterface.ORIENTATION_ROTATE_90 -> m.postRotate(90f)
            ExifInterface.ORIENTATION_ROTATE_180 -> m.postRotate(180f)
            ExifInterface.ORIENTATION_ROTATE_270 -> m.postRotate(270f)
            ExifInterface.ORIENTATION_FLIP_HORIZONTAL -> m.postScale(-1f, 1f)
            ExifInterface.ORIENTATION_FLIP_VERTICAL -> m.postScale(1f, -1f)
            else -> return this
        }
        return Bitmap.createBitmap(this, 0, 0, width, height, m, true)
    }

    // --- household (admin) ---
    suspend fun users() = apiCall { api().users() }
    suspend fun createUser(name: String, email: String, password: String, role: String) =
        apiCall { api().createUser(CreateUserRequest(name.trim(), email.trim(), password, role)) }

    suspend fun updateUser(id: String, name: String? = null, role: String? = null, password: String? = null) =
        apiCall { api().updateUser(id, UpdateUserRequest(name, role, password)) }

    suspend fun deleteUser(id: String) = apiCall { api().deleteUser(id) }
    suspend fun invites() = apiCall { api().invites() }
    suspend fun createInvite(email: String?, role: String, expiresInDays: Int) =
        apiCall { api().createInvite(CreateInviteRequest(email?.trim()?.takeIf { it.isNotEmpty() }, role, expiresInDays)) }

    suspend fun revokeInvite(id: String) = apiCall { api().revokeInvite(id) }

    // --- connected apps (MCP / OAuth) ---
    suspend fun oauthConnections() = apiCall { api().oauthConnections() }
    suspend fun disconnectOAuth(id: String) = apiCall { api().disconnectOAuth(id) }

    suspend fun serverVersion(): String? = runCatching { apiCall { api().health().version } }.getOrNull()

    private companion object {
        const val MAX_UPLOAD_EDGE = 2400
        const val SYNC_PAGE = 200
        const val FAST_READ_MS = 5_000L
        const val OFFLINE_MEMO_MS = 15_000L
    }
}
