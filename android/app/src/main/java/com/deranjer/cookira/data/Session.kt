package com.deranjer.cookira.data

import android.content.Context
import androidx.datastore.preferences.core.edit
import androidx.datastore.preferences.core.stringPreferencesKey
import androidx.datastore.preferences.preferencesDataStore
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.first
import kotlinx.coroutines.flow.map

/** The signed-in session: which server, the 15-minute access token, and the rotating refresh token. */
data class Session(val serverUrl: String, val accessToken: String, val refreshToken: String)

/** In-memory mirror of the persisted session, read by the OkHttp interceptor / authenticator. */
class TokenHolder {
    @Volatile
    var session: Session? = null
}

private val Context.sessionDataStore by preferencesDataStore(name = "cookira_session")

/**
 * Persists the session in DataStore so the user stays signed in across restarts, and the
 * last-used server URL separately so it survives sign-out and pre-fills the connect screen.
 */
class SessionStore(private val context: Context) {
    private val keyServer = stringPreferencesKey("server_url")
    private val keyAccess = stringPreferencesKey("access_token")
    private val keyRefresh = stringPreferencesKey("refresh_token")
    private val keyLastServer = stringPreferencesKey("last_server_url")

    val session: Flow<Session?> = context.sessionDataStore.data.map { p ->
        val url = p[keyServer]
        val access = p[keyAccess]
        val refresh = p[keyRefresh]
        if (url != null && access != null && refresh != null) Session(url, access, refresh) else null
    }

    suspend fun current(): Session? = session.first()
    suspend fun lastServerUrl(): String? = context.sessionDataStore.data.first()[keyLastServer]

    suspend fun save(session: Session) {
        context.sessionDataStore.edit { p ->
            p[keyServer] = session.serverUrl
            p[keyAccess] = session.accessToken
            p[keyRefresh] = session.refreshToken
            p[keyLastServer] = session.serverUrl
        }
    }

    suspend fun rememberServerUrl(url: String) {
        context.sessionDataStore.edit { it[keyLastServer] = url }
    }

    suspend fun clear() {
        context.sessionDataStore.edit { p ->
            p.remove(keyServer)
            p.remove(keyAccess)
            p.remove(keyRefresh)
        }
    }
}

enum class ThemeMode { System, Light, Dark }

private val Context.prefsDataStore by preferencesDataStore(name = "cookira_prefs")

/** Per-device UI preferences (not tied to the account). */
class PrefsStore(private val context: Context) {
    private val keyTheme = stringPreferencesKey("theme")
    private val keyKeepAwake = stringPreferencesKey("keep_awake")

    val themeMode: Flow<ThemeMode> = context.prefsDataStore.data.map { p ->
        p[keyTheme]?.let { runCatching { ThemeMode.valueOf(it) }.getOrNull() } ?: ThemeMode.System
    }

    /** Keep the screen on while a recipe is open (handy with floury hands). */
    val keepAwake: Flow<Boolean> = context.prefsDataStore.data.map { it[keyKeepAwake] != "false" }

    suspend fun setTheme(mode: ThemeMode) {
        context.prefsDataStore.edit { it[keyTheme] = mode.name }
    }

    suspend fun setKeepAwake(on: Boolean) {
        context.prefsDataStore.edit { it[keyKeepAwake] = on.toString() }
    }
}
