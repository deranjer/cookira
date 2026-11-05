package com.deranjer.cookira.data

import kotlinx.serialization.decodeFromString
import kotlinx.serialization.json.Json
import okhttp3.MediaType.Companion.toMediaType
import okhttp3.MultipartBody
import okhttp3.OkHttpClient
import retrofit2.HttpException
import retrofit2.Retrofit
import retrofit2.converter.kotlinx.serialization.asConverterFactory
import retrofit2.http.Body
import retrofit2.http.DELETE
import retrofit2.http.GET
import retrofit2.http.Multipart
import retrofit2.http.PATCH
import retrofit2.http.POST
import retrofit2.http.PUT
import retrofit2.http.Part
import retrofit2.http.Path
import retrofit2.http.Query
import java.io.IOException
import java.util.concurrent.ConcurrentHashMap

val AppJson = Json {
    ignoreUnknownKeys = true
    coerceInputValues = true
    // Null PATCH fields must be left out of the body, not sent as null.
    explicitNulls = false
    encodeDefaults = true
}

/** Retrofit surface over Cookira's `/api/v1`. Mirrors web/src/lib/api/client.ts. */
interface ApiService {
    @GET("health") suspend fun health(): HealthDto

    @GET("setup/status") suspend fun setupStatus(): SetupStatusDto
    @POST("setup") suspend fun setup(@Body body: SetupRequest): SessionDto

    @POST("auth/login") suspend fun login(@Body body: LoginRequest): SessionDto
    @POST("auth/refresh") suspend fun refresh(@Body body: RefreshRequest): SessionDto
    @POST("auth/logout") suspend fun logout(@Body body: RefreshRequest): StatusDto
    @GET("auth/me") suspend fun me(): UserDto
    @PATCH("auth/me") suspend fun updateMe(@Body body: UpdateMeRequest): UserDto

    @GET("users") suspend fun users(): List<UserDto>
    @POST("users") suspend fun createUser(@Body body: CreateUserRequest): UserDto
    @PATCH("users/{id}") suspend fun updateUser(@Path("id") id: String, @Body body: UpdateUserRequest): UserDto
    @DELETE("users/{id}") suspend fun deleteUser(@Path("id") id: String): StatusDto

    @GET("invites") suspend fun invites(): List<InviteDto>
    @POST("invites") suspend fun createInvite(@Body body: CreateInviteRequest): InviteDto
    @DELETE("invites/{id}") suspend fun revokeInvite(@Path("id") id: String): StatusDto
    @GET("invites/lookup/{token}") suspend fun lookupInvite(@Path("token") token: String): InviteInfoDto
    @POST("invites/accept/{token}") suspend fun acceptInvite(
        @Path("token") token: String,
        @Body body: AcceptInviteRequest,
    ): SessionDto

    @GET("oauth/connections") suspend fun oauthConnections(): List<OAuthConnection>
    @DELETE("oauth/connections/{id}") suspend fun disconnectOAuth(@Path("id") id: String): StatusDto

    @GET("recipes") suspend fun recipes(
        @Query("q") q: String?,
        @Query("tag") tag: String?,
        @Query("status") status: String?,
        @Query("limit") limit: Int,
        @Query("offset") offset: Int,
    ): RecipeListDto

    @GET("tags") suspend fun tags(): List<TagCount>
    @POST("recipes") suspend fun createRecipe(@Body body: RecipeInput): Recipe
    @GET("recipes/{ref}") suspend fun recipe(@Path("ref") ref: String): Recipe
    @PATCH("recipes/{ref}") suspend fun updateRecipe(@Path("ref") ref: String, @Body body: RecipeInput): Recipe
    @DELETE("recipes/{ref}") suspend fun deleteRecipe(@Path("ref") ref: String): StatusDto

    @GET("recipes/{ref}/revisions") suspend fun revisions(@Path("ref") ref: String): List<Revision>
    @POST("recipes/{ref}/revisions/{revision}/restore") suspend fun restoreRevision(
        @Path("ref") ref: String,
        @Path("revision") revision: Int,
    ): Recipe

    @GET("recipes/{ref}/shares") suspend fun shares(@Path("ref") ref: String): List<ShareDto>
    @POST("recipes/{ref}/shares") suspend fun createShare(
        @Path("ref") ref: String,
        @Body body: CreateShareRequest,
    ): ShareDto
    @DELETE("recipes/{ref}/shares/{shareId}") suspend fun revokeShare(
        @Path("ref") ref: String,
        @Path("shareId") shareId: String,
    ): StatusDto

    @GET("public/{token}") suspend fun sharedRecipe(@Path("token") token: String): Recipe

    @Multipart
    @PUT("recipes/{ref}/image")
    suspend fun uploadImage(@Path("ref") ref: String, @Part file: MultipartBody.Part): StatusDto

    @DELETE("recipes/{ref}/image") suspend fun removeImage(@Path("ref") ref: String): StatusDto
}

/** Builds and caches an [ApiService] per server origin; the URL is chosen at login. */
class ApiProvider(private val client: OkHttpClient) {
    private val cache = ConcurrentHashMap<String, ApiService>()
    private val contentType = "application/json".toMediaType()

    fun service(serverUrl: String): ApiService {
        val base = ServerUrl.normalize(serverUrl)
        return cache.getOrPut(base) {
            Retrofit.Builder()
                .baseUrl("$base/api/v1/")
                .client(client)
                .addConverterFactory(AppJson.asConverterFactory(contentType))
                .build()
                .create(ApiService::class.java)
        }
    }
}

class ApiException(
    message: String,
    val status: Int = 0,
    val unauthorized: Boolean = false,
    /** True when the server couldn't be reached at all (no signal, DNS, timeout), as opposed to answering with an error. */
    val network: Boolean = false,
    cause: Throwable? = null,
) : Exception(message, cause)

object ServerUrl {
    /**
     * Reduces whatever the user typed (`recipes.example.com`, `http://10.0.0.5:8080/login`,
     * a pasted share link…) to `scheme://host[:port]`. A bare host defaults to https.
     */
    fun normalize(input: String): String {
        var s = input.trim()
        if (s.isEmpty()) return ""
        if (!s.contains("://")) s = "https://$s"
        val schemeEnd = s.indexOf("://") + 3
        val pathStart = s.indexOf('/', schemeEnd)
        return (if (pathStart == -1) s else s.substring(0, pathStart)).trimEnd('/').lowercase()
    }

    fun isValid(input: String): Boolean {
        val n = normalize(input)
        return (n.startsWith("http://") || n.startsWith("https://")) && n.length > n.indexOf("://") + 3
    }

    fun isCleartext(input: String) = normalize(input).startsWith("http://")
}

/** A recognised Cookira link pasted or shared into the app. */
sealed interface CookiraLink {
    val origin: String
    val token: String

    data class Share(override val origin: String, override val token: String) : CookiraLink
    data class Invite(override val origin: String, override val token: String) : CookiraLink

    companion object {
        private val share = Regex("""^(https?://[^/\s]+)/s/([A-Za-z0-9_\-]+)/?(?:[?#].*)?$""")
        private val invite = Regex("""^(https?://[^/\s]+)/invite/([A-Za-z0-9_\-%.~]+)/?(?:[?#].*)?$""")

        /** Finds the first share or invite link inside arbitrary text. */
        fun parse(text: String?): CookiraLink? {
            if (text.isNullOrBlank()) return null
            for (word in text.split(Regex("\\s+"))) {
                val w = word.trim()
                share.matchEntire(w)?.let { return Share(it.groupValues[1].lowercase(), it.groupValues[2]) }
                invite.matchEntire(w)?.let { return Invite(it.groupValues[1].lowercase(), it.groupValues[2]) }
            }
            return null
        }
    }
}

/** Runs a Retrofit call, translating HTTP / network failures into [ApiException]. */
suspend fun <T> apiCall(block: suspend () -> T): T = try {
    block()
} catch (e: HttpException) {
    throw e.toApiException()
} catch (e: ApiException) {
    throw e
} catch (e: IOException) {
    throw ApiException("Can't reach the server. Check the address and your connection.", network = true, cause = e)
} catch (e: kotlinx.coroutines.CancellationException) {
    throw e
} catch (e: kotlinx.serialization.SerializationException) {
    throw ApiException("Unexpected response from the server. Is this a Cookira server?", cause = e)
}

fun HttpException.toApiException(): ApiException {
    val raw = runCatching { response()?.errorBody()?.string() }.getOrNull()
    val problem = raw?.let { runCatching { AppJson.decodeFromString<ProblemDto>(it) }.getOrNull() }
    // huma puts the useful part of a 422 in errors[] ("validation failed" alone says nothing).
    val fieldErrors = problem?.errors?.mapNotNull { e ->
        e.message?.let { m -> e.location?.removePrefix("body.")?.takeIf { it.isNotBlank() }?.let { "$it: $m" } ?: m }
    }.orEmpty()
    val message = (if (fieldErrors.isNotEmpty()) fieldErrors.joinToString("; ") else problem?.detail)
        ?: problem?.error_description
        ?: problem?.title
        ?: "Request failed (${code()})"
    return ApiException(message, status = code(), unauthorized = code() == 401, cause = this)
}
