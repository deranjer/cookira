package com.deranjer.cookira.data

import android.content.Context
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.sync.Mutex
import kotlinx.coroutines.sync.withLock
import kotlinx.coroutines.withContext
import kotlinx.serialization.Serializable
import java.io.File
import java.time.OffsetDateTime

/** What is stored on disk: the whole library as last seen from one server. */
@Serializable
data class CacheSnapshot(
    val serverUrl: String = "",
    val syncedAt: Long = 0,
    val recipes: List<Recipe> = emptyList(),
)

/** One page of recipes, from the server or (when offline) from the saved copy. */
data class RecipePage(
    val items: List<RecipeSummary>,
    val total: Int,
    val fromCache: Boolean = false,
    val syncedAt: Long = 0,
)

/** A full recipe, from the server or from the saved copy. */
data class RecipeResult(val recipe: Recipe, val fromCache: Boolean = false, val syncedAt: Long = 0)

/**
 * Read-only offline copy of the recipe library. A JSON file in app-private storage (no Room,
 * so the build stays free of annotation processors). It holds only the signed-in server's data
 * and is wiped on sign-out. Recipe photos are cached separately by Coil's disk cache.
 */
class RecipeCache(context: Context) {
    private val file = File(context.filesDir, "offline/recipes.json")
    private val mutex = Mutex()
    private var mem: CacheSnapshot? = null

    private suspend fun load(serverUrl: String): CacheSnapshot = mutex.withLock { loadLocked(serverUrl) }

    private suspend fun loadLocked(serverUrl: String): CacheSnapshot {
        mem?.let { return if (it.serverUrl == serverUrl) it else CacheSnapshot() }
        val read = withContext(Dispatchers.IO) {
            runCatching { if (file.exists()) AppJson.decodeFromString<CacheSnapshot>(file.readText()) else null }.getOrNull()
        } ?: CacheSnapshot()
        mem = read
        return if (read.serverUrl == serverUrl) read else CacheSnapshot()
    }

    private suspend fun store(snapshot: CacheSnapshot) {
        mem = snapshot
        withContext(Dispatchers.IO) {
            file.parentFile?.mkdirs()
            // Write then rename so a crash mid-write can't leave a truncated cache.
            val tmp = File(file.parentFile, "recipes.json.tmp")
            tmp.writeText(AppJson.encodeToString(CacheSnapshot.serializer(), snapshot))
            if (!tmp.renameTo(file)) {
                file.delete()
                tmp.renameTo(file)
            }
        }
    }

    suspend fun all(serverUrl: String): List<Recipe> = load(serverUrl).recipes
    suspend fun syncedAt(serverUrl: String): Long = load(serverUrl).syncedAt
    suspend fun count(serverUrl: String): Int = load(serverUrl).recipes.size
    suspend fun hasData(serverUrl: String): Boolean = load(serverUrl).syncedAt > 0

    /** Replaces the library after a full sync. */
    suspend fun replaceAll(serverUrl: String, recipes: List<Recipe>, now: Long = System.currentTimeMillis()) =
        mutex.withLock { store(CacheSnapshot(serverUrl, now, recipes)) }

    /** Write-through after an edit made in the app. Keeps the sync time: other recipes aren't any fresher. */
    suspend fun upsert(serverUrl: String, recipe: Recipe) = mutex.withLock {
        val cur = loadLocked(serverUrl)
        if (cur.syncedAt == 0L) return@withLock // nothing synced yet; the first sync gets it
        store(cur.copy(serverUrl = serverUrl, recipes = cur.recipes.filterNot { it.id == recipe.id } + recipe))
    }

    suspend fun remove(serverUrl: String, id: String) = mutex.withLock {
        val cur = loadLocked(serverUrl)
        if (cur.syncedAt == 0L) return@withLock
        store(cur.copy(recipes = cur.recipes.filterNot { it.id == id }))
    }

    suspend fun clear() = mutex.withLock {
        mem = null
        withContext(Dispatchers.IO) { file.parentFile?.deleteRecursively() }
    }
}

/** Pure offline equivalents of the server's list / search / tags. */
object LocalLibrary {
    private fun Recipe.summary() = RecipeSummary(
        id = id, slug = slug, title = title, tagline = tagline, author = author, servings = servings,
        prepMinutes = prepMinutes, cookMinutes = cookMinutes, calories = calories, status = status,
        tags = tags, hasImage = hasImage, updatedAt = updatedAt,
    )

    private fun instant(iso: String): Long = runCatching { OffsetDateTime.parse(iso).toInstant().toEpochMilli() }.getOrDefault(0L)

    /** Every search word must appear in the title, tagline, author, a tag or an ingredient. */
    private fun Recipe.matches(words: List<String>): Boolean {
        if (words.isEmpty()) return true
        val haystack = buildString {
            append(title).append(' ').append(tagline).append(' ').append(author).append(' ')
            tags.forEach { append(it).append(' ') }
            ingredients.forEach { g -> append(g.name).append(' '); g.items.forEach { append(it).append(' ') } }
        }.lowercase()
        return words.all { it in haystack }
    }

    fun query(all: List<Recipe>, q: String?, tag: String?, status: String?, limit: Int, offset: Int): Pair<List<RecipeSummary>, Int> {
        val words = q.orEmpty().lowercase().split(Regex("\\s+")).filter { it.isNotEmpty() }
        val hits = all
            .filter { tag == null || tag in it.tags }
            .filter { status.isNullOrEmpty() || it.status == status }
            .filter { it.matches(words) }
            .sortedByDescending { instant(it.updatedAt) }
        return hits.drop(offset).take(limit).map { it.summary() } to hits.size
    }

    fun tags(all: List<Recipe>): List<TagCount> =
        all.flatMap { it.tags }.groupingBy { it }.eachCount().map { TagCount(it.key, it.value) }.sortedBy { it.tag }

    fun find(all: List<Recipe>, ref: String): Recipe? = all.firstOrNull { it.id == ref || it.slug == ref }

    /** Recipes that are new or changed on the server and so need their full body fetched. */
    fun needsFetch(cached: List<Recipe>, remote: List<RecipeSummary>): List<RecipeSummary> {
        val have = cached.associateBy { it.id }
        return remote.filter { have[it.id]?.updatedAt != it.updatedAt }
    }
}
