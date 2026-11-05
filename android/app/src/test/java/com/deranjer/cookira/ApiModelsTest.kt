package com.deranjer.cookira

import com.deranjer.cookira.data.AppJson
import com.deranjer.cookira.data.CookiraLink
import com.deranjer.cookira.data.IngredientGroup
import com.deranjer.cookira.data.LoginRequest
import com.deranjer.cookira.data.ProblemDto
import com.deranjer.cookira.data.Recipe
import com.deranjer.cookira.data.RecipeInput
import com.deranjer.cookira.data.ServerUrl
import com.deranjer.cookira.data.SessionDto
import com.deranjer.cookira.data.Step
import com.deranjer.cookira.ui.common.MdBlock
import com.deranjer.cookira.ui.common.formatMinutes
import com.deranjer.cookira.ui.common.parseMarkdown
import com.deranjer.cookira.ui.common.toPlainText
import kotlinx.serialization.encodeToString
import kotlinx.serialization.json.Json
import kotlinx.serialization.json.JsonObject
import kotlinx.serialization.json.boolean
import kotlinx.serialization.json.jsonArray
import kotlinx.serialization.json.jsonObject
import kotlinx.serialization.json.jsonPrimitive
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test

class ApiModelsTest {
    @Test
    fun `server url is reduced to its origin`() {
        assertEquals("https://recipes.example.com", ServerUrl.normalize("recipes.example.com"))
        assertEquals("https://recipes.example.com", ServerUrl.normalize("  https://Recipes.Example.com/login  "))
        assertEquals("http://10.0.0.5:8080", ServerUrl.normalize("http://10.0.0.5:8080/"))
        assertEquals("http://10.0.0.5:8080", ServerUrl.normalize("http://10.0.0.5:8080/api/v1"))
        assertEquals("", ServerUrl.normalize("   "))
        assertTrue(ServerUrl.isValid("recipes.example.com"))
        assertFalse(ServerUrl.isValid("https://"))
        assertTrue(ServerUrl.isCleartext("http://nas.local:8080"))
        assertFalse(ServerUrl.isCleartext("nas.local"))
    }

    @Test
    fun `share and invite links are recognised inside shared text`() {
        val share = CookiraLink.parse("Check this out https://recipes.example.com/s/AbC-123_xyz thanks!")
        assertEquals(CookiraLink.Share("https://recipes.example.com", "AbC-123_xyz"), share)
        val invite = CookiraLink.parse("http://192.168.1.5:8080/invite/tok_en-1")
        assertEquals(CookiraLink.Invite("http://192.168.1.5:8080", "tok_en-1"), invite)
        assertNull(CookiraLink.parse("https://recipes.example.com/recipes/pancakes"))
        assertNull(CookiraLink.parse(null))
        assertNull(CookiraLink.parse("hello"))
    }

    @Test
    fun `login request asks for tokens in the body`() {
        val json = AppJson.encodeToString(LoginRequest("a@b.c", "pw"))
        val obj = Json.parseToJsonElement(json).jsonObject
        assertTrue(obj["tokens"]!!.jsonPrimitive.boolean)
        assertEquals("a@b.c", obj["email"]!!.jsonPrimitive.content)
    }

    @Test
    fun `session response with tokens decodes`() {
        val body = """{"id":"u1","name":"Sam","email":"s@x.y","role":"admin",
            "tokens":{"accessToken":"A","accessExpiresAt":"2026-01-01T00:00:00Z","refreshToken":"R","refreshExpiresAt":"2026-04-01T00:00:00Z"},
            "unknown":1}""".trimIndent()
        val s = AppJson.decodeFromString<SessionDto>(body)
        assertEquals("A", s.tokens!!.accessToken)
        assertEquals("R", s.tokens!!.refreshToken)
        assertTrue(s.user().isAdmin)
    }

    @Test
    fun `recipe tolerates null lists from Go nil slices`() {
        val body = """{"id":"r1","slug":"s","title":"T","tagline":"","author":"","servings":0,"prepMinutes":0,
            "cookMinutes":0,"calories":0,"sourceUrl":"","notes":"","status":"draft","tags":null,
            "ingredients":[{"name":"","items":null}],"steps":[{"text":"x"}],"hasImage":false,"createdAt":"","updatedAt":""}"""
        val r = AppJson.decodeFromString<Recipe>(body)
        assertTrue(r.tags.isEmpty())
        assertTrue(r.ingredients.single().items.isEmpty())
        assertTrue(r.steps.single().substeps.isEmpty())
        assertTrue(r.isDraft)
    }

    @Test
    fun `patch body omits unset fields but keeps explicit empty lists`() {
        val patch = AppJson.encodeToString(RecipeInput(status = "published", tags = emptyList()))
        val obj: JsonObject = Json.parseToJsonElement(patch).jsonObject
        assertEquals(setOf("status", "tags"), obj.keys)
        assertEquals(0, obj["tags"]!!.jsonArray.size)
    }

    @Test
    fun `huma problem bodies decode`() {
        val p = AppJson.decodeFromString<ProblemDto>(
            """{"title":"Unprocessable Entity","status":422,"detail":"validation failed","errors":[{"message":"expected length >= 8","location":"body.password"}]}""",
        )
        assertEquals("validation failed", p.detail)
        assertEquals("expected length >= 8", p.errors.first().message)
    }

    @Test
    fun `minutes are formatted like the web ui`() {
        assertEquals("", formatMinutes(0))
        assertEquals("45 min", formatMinutes(45))
        assertEquals("2 hr", formatMinutes(120))
        assertEquals("1 hr 5 min", formatMinutes(65))
    }

    @Test
    fun `markdown blocks`() {
        val blocks = parseMarkdown("## Tips\n\nUse **cold** butter.\nKeep it chilled.\n\n- one\n- two\n1. first\n> quoted")
        assertEquals(MdBlock.Heading(2, "Tips"), blocks[0])
        assertEquals(MdBlock.Paragraph("Use **cold** butter. Keep it chilled."), blocks[1])
        assertEquals(MdBlock.Bullet("one"), blocks[2])
        assertEquals(MdBlock.Bullet("two"), blocks[3])
        assertEquals(MdBlock.Numbered(1, "first"), blocks[4])
        assertEquals(MdBlock.Quote("quoted"), blocks[5])
    }

    @Test
    fun `plain text export lists ingredients and lettered substeps`() {
        val r = Recipe(
            title = "Ramen", servings = 2, prepMinutes = 10, cookMinutes = 20,
            ingredients = listOf(IngredientGroup("Broth", listOf("2 cups stock"))),
            steps = listOf(Step("Simmer", listOf("Add soy", "Add mirin"))),
        )
        val text = r.toPlainText()
        assertTrue(text.contains("Broth:\n- 2 cups stock"))
        assertTrue(text.contains("1. Simmer\n   a. Add soy\n   b. Add mirin"))
        assertTrue(text.contains("Serves 2 · Prep 10 min · Cook 20 min"))
    }
}
