package com.deranjer.cookira

import com.deranjer.cookira.data.IngredientGroup
import com.deranjer.cookira.data.Recipe
import com.deranjer.cookira.data.Step
import com.deranjer.cookira.ui.recipes.IngredientMatcher
import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Test

class IngredientMatcherTest {
    private val recipe = Recipe(
        title = "Chicken rice bowls",
        ingredients = listOf(
            IngredientGroup("Chicken", listOf("1.68 lbs boneless chicken thighs", "1 onion, thickly sliced (for the bottom of the slow cooker)")),
            IngredientGroup(
                "Marinade",
                listOf(
                    "3 tbsp olive oil", "juice of 1 lemon", "4 garlic cloves, minced", "2 tsp cumin", "2 tsp paprika",
                    "½ tsp black pepper", "1 tsp kosher salt",
                ),
            ),
            IngredientGroup(
                "Rice",
                listOf("1½ cups uncooked basmati or jasmine rice", "½ tsp turmeric", "½ tsp cumin", "½ tsp salt", "1 tbsp butter or olive oil"),
            ),
            IngredientGroup("Vegetables", listOf("1 cucumber, diced", "2–3 Roma tomatoes, diced", "½ red onion, thinly sliced", "2 cups shredded romaine lettuce")),
            IngredientGroup(
                "Garlic sauce",
                listOf("¾ cup plain Greek yogurt", "1 tbsp lemon juice", "1 garlic clove, grated", "1 tbsp olive oil", "salt and pepper"),
            ),
        ),
        steps = listOf(
            Step(
                "Prep: Marinate the chicken. Whisk together the marinade ingredients, add the chicken thighs, and toss until evenly coated.",
                listOf("Combine 3 tbsp olive oil, juice of 1 lemon, 4 minced garlic cloves, 2 tsp cumin, 2 tsp paprika, ½ tsp black pepper, 1 tsp kosher salt."),
            ),
            Step("Prep: Make the garlic sauce. Stir together ¾ cup plain Greek yogurt, 1 tbsp lemon juice, 1 grated garlic clove, 1 tbsp olive oil. Season with salt and pepper."),
            Step("Prep: Prepare the vegetables. Dice 1 cucumber and 2–3 Roma tomatoes, thinly slice ½ red onion, and shred 2 cups romaine."),
            Step("Cook: Layer 1 thickly sliced onion in the bottom of the slow cooker, then place the marinated chicken on top."),
            Step("Cook: Cover and cook on LOW approximately 4–5 hours."),
            Step("Cook: Make the rice. Combine 1½ cups uncooked basmati or jasmine rice, ½ tsp turmeric, ½ tsp cumin, ½ tsp salt, and 1 tbsp butter or olive oil."),
            Step("Serve: Divide the rice among bowls and top with the chicken strips, cucumber, tomatoes, red onion, and romaine."),
            Step("Serve: Drizzle with the garlic sauce and serve."),
        ),
    )

    private fun used(step: Int) = IngredientMatcher.match(recipe)[step].map { it.key }.toSet()

    @Test fun `named group brings its whole list plus the chicken`() {
        // Marinade 1:0..1:6, chicken thighs 0:0. No sauce / rice copies.
        assertEquals((0..6).map { "1:$it" }.toSet() + "0:0", used(0))
    }

    @Test fun `sauce step uses sauce lines, not marinade or rice copies`() {
        assertEquals((0..4).map { "4:$it" }.toSet(), used(1))
    }

    @Test fun `vegetable step lists vegetables without the slow cooker onion`() {
        assertEquals((0..3).map { "3:$it" }.toSet(), used(2))
    }

    @Test fun `plain onion step uses the slow cooker onion not the red onion`() {
        assertEquals(setOf("0:1", "0:0"), used(3))
    }

    @Test fun `waiting step uses nothing`() {
        assertTrue(used(4).isEmpty())
    }

    @Test fun `rice step uses rice lines`() {
        assertEquals((0..4).map { "2:$it" }.toSet(), used(5))
    }

    @Test fun `assembly step`() {
        val keys = used(6)
        assertTrue("2:0" in keys) // rice
        assertTrue("0:0" in keys) // chicken
        assertTrue("3:0" in keys && "3:1" in keys && "3:2" in keys && "3:3" in keys)
        assertTrue("0:1" !in keys) // not the slow cooker onion
    }

    @Test fun `serve with the sauce`() {
        assertEquals((0..4).map { "4:$it" }.toSet(), used(7))
    }
}
