package com.deranjer.cookira.di

import android.content.Context
import androidx.compose.runtime.Composable
import androidx.compose.runtime.staticCompositionLocalOf
import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewmodel.compose.viewModel
import androidx.lifecycle.viewmodel.initializer
import androidx.lifecycle.viewmodel.viewModelFactory
import android.net.ConnectivityManager
import coil.ImageLoader
import coil.disk.DiskCache
import coil.request.ImageRequest
import com.deranjer.cookira.data.ApiProvider
import com.deranjer.cookira.data.AuthInterceptor
import com.deranjer.cookira.data.AuthRepository
import com.deranjer.cookira.data.CookiraLink
import com.deranjer.cookira.data.CookiraRepository
import com.deranjer.cookira.data.RecipeCache
import com.deranjer.cookira.data.PrefsStore
import com.deranjer.cookira.data.Session
import com.deranjer.cookira.data.SessionStore
import com.deranjer.cookira.data.TokenAuthenticator
import com.deranjer.cookira.data.TokenHolder
import com.deranjer.cookira.data.UserDto
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.sync.Mutex
import kotlinx.coroutines.flow.SharingStarted
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.launchIn
import kotlinx.coroutines.flow.map
import kotlinx.coroutines.flow.onEach
import kotlinx.coroutines.flow.stateIn
import kotlinx.coroutines.launch
import okhttp3.OkHttpClient
import java.io.File
import java.util.concurrent.TimeUnit

sealed interface SessionState {
    data object Loading : SessionState
    data object SignedOut : SessionState
    data class SignedIn(val session: Session) : SessionState
}

/**
 * Manual dependency container (no Hilt/KSP, keeping the build free of annotation
 * processors, which also keeps F-Droid builds simple). Created once by CookiraApp.
 */
class AppContainer(context: Context, private val appScope: CoroutineScope) {
    private val appContext = context.applicationContext

    val sessionStore = SessionStore(appContext)
    val prefs = PrefsStore(appContext)
    private val tokenHolder = TokenHolder()

    // Bare client: no auth header, no 401 handling. Used for login / refresh / logout, which
    // must never recurse through the authenticator.
    private val bareClient: OkHttpClient = OkHttpClient.Builder()
        .connectTimeout(20, TimeUnit.SECONDS)
        .readTimeout(30, TimeUnit.SECONDS)
        .writeTimeout(60, TimeUnit.SECONDS)
        .build()

    private val recipeCache = RecipeCache(appContext)

    val authRepository = AuthRepository(ApiProvider(bareClient), sessionStore, tokenHolder, recipeCache)

    private val client: OkHttpClient = bareClient.newBuilder()
        .addInterceptor(AuthInterceptor(tokenHolder))
        .authenticator(TokenAuthenticator(tokenHolder, authRepository))
        .build()

    val repository = CookiraRepository(ApiProvider(client), tokenHolder, appContext, recipeCache)

    /** Coil loader that reuses the authenticated client so recipe photos load with the token.
     *  Image URLs carry `?v=<updatedAt>`, so cache headers (the server says no-cache) are ignored. */
    val imageLoader: ImageLoader = ImageLoader.Builder(appContext)
        .okHttpClient(client)
        // Photos viewed (or prefetched) stay on disk so they still show offline. Kept in app-private
        // files rather than cacheDir, which Android may clear whenever it likes.
        .diskCache {
            DiskCache.Builder().directory(File(appContext.filesDir, "image_cache")).maxSizeBytes(256L * 1024 * 1024).build()
        }
        .respectCacheHeaders(false)
        .crossfade(true)
        .build()

    /** Drives the auth gate. The token holder is updated before this publishes, so the first
     *  request after a state change is already authenticated. */
    val sessionState: StateFlow<SessionState> = sessionStore.session
        .onEach { tokenHolder.session = it }
        .map { if (it == null) SessionState.SignedOut else SessionState.SignedIn(it) }
        .stateIn(appScope, SharingStarted.Eagerly, SessionState.Loading)

    private val _currentUser = MutableStateFlow<UserDto?>(null)
    val currentUser: StateFlow<UserDto?> = _currentUser.asStateFlow()

    /** Bumped after any recipe mutation so list screens reload when they come back into view. */
    private val _recipesVersion = MutableStateFlow(0)
    val recipesVersion: StateFlow<Int> = _recipesVersion.asStateFlow()

    fun recipesChanged() {
        _recipesVersion.value += 1
    }

    /** A share / invite link handed to the app (shared text or deep link), consumed by the nav host. */
    private val _pendingLink = MutableStateFlow<CookiraLink?>(null)
    val pendingLink: StateFlow<CookiraLink?> = _pendingLink.asStateFlow()

    fun offerLink(link: CookiraLink?) {
        if (link != null) _pendingLink.value = link
    }

    fun consumeLink() {
        _pendingLink.value = null
    }

    suspend fun refreshUser() {
        runCatching { repository.me() }.getOrNull()?.let { _currentUser.value = it }
    }

    fun setUser(user: UserDto) {
        _currentUser.value = user
    }

    /** Signs out on the app scope so it completes even as the authenticated UI is torn down. */
    fun logout() {
        appScope.launch {
            authRepository.logout()
            imageLoader.diskCache?.clear()
            imageLoader.memoryCache?.clear()
            _currentUser.value = null
        }
    }


    // --- offline library ---

    data class SyncState(
        val syncing: Boolean = false,
        val syncedAt: Long = 0,
        val count: Int = 0,
        val error: String? = null,
    )

    private val _sync = MutableStateFlow(SyncState())
    val syncState: StateFlow<SyncState> = _sync.asStateFlow()
    private val syncLock = Mutex()

    private fun isMetered(): Boolean =
        (appContext.getSystemService(Context.CONNECTIVITY_SERVICE) as? ConnectivityManager)?.isActiveNetworkMetered ?: true

    /** Re-reads what's saved on the phone into [syncState] (after sign-in, clear, etc.). */
    suspend fun loadSyncState() {
        _sync.update { it.copy(syncedAt = repository.cachedSyncedAt(), count = repository.cachedCount()) }
    }

    /**
     * Updates the saved library from the server. No-op if a sync is already running. Thumbnails
     * are prefetched too unless the connection is metered; [allPhotos] also fetches full-size
     * photos (the Settings button).
     */
    fun syncNow(allPhotos: Boolean = false) {
        appScope.launch {
            if (allPhotos) repository.forgetOffline()
            if (!syncLock.tryLock()) return@launch
            try {
                _sync.update { it.copy(syncing = true, error = null) }
                repository.syncLibrary()
                loadSyncState()
                if (allPhotos || !isMetered()) prefetchPhotos(full = allPhotos)
            } catch (e: Exception) {
                _sync.update { it.copy(error = e.message ?: "Sync failed") }
            } finally {
                _sync.update { it.copy(syncing = false) }
                syncLock.unlock()
            }
        }
    }

    private suspend fun prefetchPhotos(full: Boolean) {
        for (r in repository.cachedRecipes()) {
            if (!r.hasImage) continue
            repository.imageUrl(r.id, r.updatedAt, thumb = true)?.let { enqueueImage(it) }
            if (full) repository.imageUrl(r.id, r.updatedAt, thumb = false)?.let { enqueueImage(it) }
        }
    }

    private fun enqueueImage(url: String) {
        imageLoader.enqueue(ImageRequest.Builder(appContext).data(url).build())
    }

    fun clearOfflineLibrary() {
        appScope.launch {
            repository.clearOfflineLibrary()
            imageLoader.diskCache?.clear()
            imageLoader.memoryCache?.clear()
            loadSyncState()
        }
    }

    init {
        sessionState.onEach { state ->
            if (state is SessionState.SignedIn) {
                refreshUser()
                loadSyncState()
                syncNow()
            } else {
                _currentUser.value = null
                _sync.value = SyncState()
            }
        }.launchIn(appScope)
    }
}

val LocalAppContainer = staticCompositionLocalOf<AppContainer> { error("AppContainer not provided") }

/** Creates a ViewModel from the [AppContainer] without a Factory class per ViewModel. */
@Composable
inline fun <reified VM : ViewModel> appViewModel(
    key: String? = null,
    crossinline create: (AppContainer) -> VM,
): VM {
    val container = LocalAppContainer.current
    return viewModel(key = key, factory = viewModelFactory { initializer { create(container) } })
}
