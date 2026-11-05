package com.deranjer.cookira.data

import kotlinx.serialization.Serializable

// DTOs mirror internal/recipes/model.go and internal/api/*.go. Field names are the
// JSON names the server uses; unknown keys are ignored (see AppJson).

@Serializable
data class UserDto(
    val id: String,
    val name: String,
    val email: String,
    val role: String,
) {
    val isAdmin get() = role == "admin"
}

@Serializable
data class TokensDto(
    val accessToken: String,
    val accessExpiresAt: String = "",
    val refreshToken: String,
    val refreshExpiresAt: String = "",
)

/** The user plus (for `tokens: true` requests) their credentials. */
@Serializable
data class SessionDto(
    val id: String,
    val name: String,
    val email: String,
    val role: String,
    val tokens: TokensDto? = null,
) {
    fun user() = UserDto(id, name, email, role)
}

@Serializable
data class LoginRequest(val email: String, val password: String, val tokens: Boolean = true)

@Serializable
data class SetupRequest(val name: String, val email: String, val password: String, val tokens: Boolean = true)

@Serializable
data class RefreshRequest(val refreshToken: String)

@Serializable
data class SetupStatusDto(val required: Boolean)

@Serializable
data class HealthDto(val status: String = "", val version: String = "", val dbConnected: Boolean = false)

@Serializable
data class StatusDto(val status: String = "")

@Serializable
data class UpdateMeRequest(
    val name: String? = null,
    val currentPassword: String? = null,
    val password: String? = null,
    /** Keeps this device signed in when the password changes (other devices are signed out). */
    val refreshToken: String? = null,
)

@Serializable
data class CreateUserRequest(val name: String, val email: String, val password: String, val role: String)

@Serializable
data class UpdateUserRequest(val name: String? = null, val role: String? = null, val password: String? = null)

@Serializable
data class InviteDto(
    val id: String,
    val email: String? = null,
    val role: String,
    val createdAt: String = "",
    val expiresAt: String = "",
    val expired: Boolean = false,
    /** Only present in the response that creates the invite. */
    val link: String? = null,
)

@Serializable
data class CreateInviteRequest(val email: String? = null, val role: String, val expiresInDays: Int? = null)

@Serializable
data class InviteInfoDto(val email: String? = null, val role: String)

@Serializable
data class AcceptInviteRequest(val name: String, val email: String? = null, val password: String, val tokens: Boolean = true)

@Serializable
data class IngredientGroup(val name: String = "", val items: List<String> = emptyList())

@Serializable
data class Step(val text: String = "", val substeps: List<String> = emptyList())

@Serializable
data class Recipe(
    val id: String = "",
    val slug: String = "",
    val title: String,
    val tagline: String = "",
    val author: String = "",
    val servings: Int = 0,
    val prepMinutes: Int = 0,
    val cookMinutes: Int = 0,
    val calories: Int = 0,
    val sourceUrl: String = "",
    val notes: String = "",
    val status: String = "draft",
    val tags: List<String> = emptyList(),
    val ingredients: List<IngredientGroup> = emptyList(),
    val steps: List<Step> = emptyList(),
    val hasImage: Boolean = false,
    val createdAt: String = "",
    val updatedAt: String = "",
) {
    val isDraft get() = status == "draft"
}

@Serializable
data class RecipeSummary(
    val id: String,
    val slug: String,
    val title: String,
    val tagline: String = "",
    val author: String = "",
    val servings: Int = 0,
    val prepMinutes: Int = 0,
    val cookMinutes: Int = 0,
    val calories: Int = 0,
    val status: String = "draft",
    val tags: List<String> = emptyList(),
    val hasImage: Boolean = false,
    val updatedAt: String = "",
) {
    val isDraft get() = status == "draft"
}

@Serializable
data class RecipeListDto(val items: List<RecipeSummary> = emptyList(), val total: Int = 0)

/**
 * Create / PATCH body. Null fields are omitted from the JSON (see AppJson), which on PATCH
 * means "leave unchanged". An empty list (not null) clears tags / ingredients / steps.
 */
@Serializable
data class RecipeInput(
    val title: String? = null,
    val tagline: String? = null,
    val author: String? = null,
    val servings: Int? = null,
    val prepMinutes: Int? = null,
    val cookMinutes: Int? = null,
    val calories: Int? = null,
    val sourceUrl: String? = null,
    val notes: String? = null,
    val status: String? = null,
    val tags: List<String>? = null,
    val ingredients: List<IngredientGroup>? = null,
    val steps: List<Step>? = null,
)

@Serializable
data class TagCount(val tag: String, val count: Int = 0)

@Serializable
data class Revision(
    val revision: Int,
    val action: String = "",
    val actor: String = "",
    val createdAt: String = "",
)

@Serializable
data class ShareDto(
    val id: String,
    val token: String = "",
    val url: String,
    val createdAt: String = "",
    val expiresAt: String? = null,
)

@Serializable
data class CreateShareRequest(val expiresInDays: Int)

@Serializable
data class OAuthConnection(
    val id: String,
    val clientName: String,
    val scopes: List<String> = emptyList(),
    val createdAt: String = "",
    val lastUsedAt: String? = null,
)

/** RFC 9457 problem body used by huma, plus the OAuth `error_description` variant. */
@Serializable
data class ProblemDto(
    val title: String? = null,
    val detail: String? = null,
    val status: Int? = null,
    val error_description: String? = null,
    val errors: List<ProblemDetail> = emptyList(),
)

@Serializable
data class ProblemDetail(val message: String? = null, val location: String? = null)
