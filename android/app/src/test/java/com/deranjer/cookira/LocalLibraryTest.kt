package com.deranjer.cookira

import com.deranjer.cookira.data.IngredientGroup
import com.deranjer.cookira.data.LocalLibrary
import com.deranjer.cookira.data.Recipe
import com.deranjer.cookira.data.RecipeSummary
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNotNull
import org.junit.Assert.assertNull
import org.junit.Test

class LocalLibraryTest {
    private fun recipe(
        id: String,
        title: String,
        updated: String,
        status: String = "published",
        tags: List<String> = emptyList(),
        ingredients: List<String> = emptyList(),
    ) = Recipe(
        id = id, slug = id, title = title, status = status, tags = tags, updatedAt = updated,
        ingredients = listOf(IngredientGroup("", ingredients)),
    )

    private val library = listOf(
        recipe("a", "Tonkotsu Ramen", "2026-11-01T10:00:00-05:00", tags = listOf("soup", "japanese"), ingredients = listOf("2 lb pork bones")),
        recipe("b", "Pancakes", "2026-11-03T10:00:00-05:00", status = "draft", tags = listOf("breakfast"), ingredients = listOf("1 cup flour")),
        recipe("c", "Tomato Soup", "2026-11-02T10:00:00-05:00", tags = listOf("soup"), ingredients = listOf("6 tomatoes")),
    )

    @Test
    fun `newest first`() {
        val (items, total) = LocalLibrary.query(library, null, null, null, 50, 0)
        assertEquals(listOf("b", "c", "a"), items.map { it.id })
        assertEquals(3, total)
    }

    @Test
    fun `search matches title tags and ingredients, all words required`() {
        assertEquals(listOf("a"), LocalLibrary.query(library, "ramen", null, null, 50, 0).first.map { it.id })
        assertEquals(listOf("a"), LocalLibrary.query(library, "PORK bones", null, null, 50, 0).first.map { it.id })
        assertEquals(listOf("c", "a"), LocalLibrary.query(library, "soup", null, null, 50, 0).first.map { it.id })
        assertEquals(emptyList<String>(), LocalLibrary.query(library, "soup flour", null, null, 50, 0).first.map { it.id })
    }

    @Test
    fun `tag and status filters and paging`() {
        assertEquals(listOf("c", "a"), LocalLibrary.query(library, null, "soup", null, 50, 0).first.map { it.id })
        assertEquals(listOf("b"), LocalLibrary.query(library, null, null, "draft", 50, 0).first.map { it.id })
        val (page, total) = LocalLibrary.query(library, null, null, null, 1, 1)
        assertEquals(listOf("c"), page.map { it.id })
        assertEquals(3, total)
    }

    @Test
    fun `tag counts`() {
        assertEquals(listOf("breakfast" to 1, "japanese" to 1, "soup" to 2), LocalLibrary.tags(library).map { it.tag to it.count })
    }

    @Test
    fun `find by id or slug`() {
        assertNotNull(LocalLibrary.find(library, "a"))
        assertNull(LocalLibrary.find(library, "zzz"))
    }

    @Test
    fun `only new or changed recipes are fetched on sync`() {
        val remote = listOf(
            RecipeSummary("a", "a", "Tonkotsu Ramen", updatedAt = "2026-11-01T10:00:00-05:00"), // unchanged
            RecipeSummary("b", "b", "Pancakes", updatedAt = "2026-11-04T09:00:00-05:00"), // edited elsewhere
            RecipeSummary("d", "d", "New one", updatedAt = "2026-11-04T09:00:00-05:00"), // new
            // "c" is gone from the server and so not in the result either
        )
        assertEquals(listOf("b", "d"), LocalLibrary.needsFetch(library, remote).map { it.id })
    }
}
