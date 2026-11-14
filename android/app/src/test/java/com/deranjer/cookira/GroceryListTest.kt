package com.deranjer.cookira

import com.deranjer.cookira.data.IngredientGroup
import com.deranjer.cookira.ui.recipes.GroceryList
import org.junit.Assert.assertEquals
import org.junit.Test

class GroceryListTest {
    private fun texts(vararg items: String) = GroceryList.build(listOf(IngredientGroup(items = items.toList()))).map { it.text }

    @Test fun `totals across groups and retains source amounts`() {
        val items = GroceryList.build(listOf(
            IngredientGroup("Marinade", listOf("3 tbsp olive oil", "2 tsp cumin")),
            IngredientGroup("Rice", listOf("1 tablespoon olive oil", "½ tsp cumin")),
        ))
        assertEquals(listOf("4 tbsp olive oil", "2 1/2 tsp cumin"), items.map { it.text })
        assertEquals(listOf("Marinade: 3 tbsp olive oil", "Rice: 1 tablespoon olive oil"), items[0].sources)
    }

    @Test fun `fractions decimals and count amounts`() {
        assertEquals(listOf("3 cup rice", "2/3 cup milk", "0.3 g spice", "3 onion"),
            texts("1½ cups rice", "1 1/2 cup rice", "1/3 cup milk", "⅓ cup milk", "0.1 g spice", "0.2 g spice", "1 onion", "2 onion"))
    }

    @Test fun `keeps different units names preparations and ambiguous amounts separate`() {
        val raw = listOf("1 tsp salt", "1 tbsp salt", "1 tsp kosher salt", "1 onion, diced", "1 onion, sliced",
            "2–3 tomatoes", "2 to 3 tomatoes", "2 to 3 tomatoes", "1 (14 oz) can tomatoes", "1 (14 oz) can tomatoes",
            "salt to taste", "salt to taste", "1 tbsp butter or olive oil", "1 tbsp butter or olive oil", "1 cup milk, divided", "1 cup milk, divided")
        assertEquals(raw, texts(*raw.toTypedArray()))
    }

    @Test fun `normalizes whitespace case and units and rejects invalid fractions`() {
        assertEquals(listOf("3 tbsp olive oil"), texts(" 1   tbsp. olive oil ", "2 tablespoons Olive Oil", "", "  "))
        assertEquals(listOf("1/0 cup milk", "1/0 cup milk"), texts("1/0 cup milk", "1/0 cup milk"))
    }

    @Test fun `copy all selected and empty`() {
        val items = GroceryList.build(listOf(IngredientGroup(items = listOf("1 onion", "2 cups rice", "1 tsp salt"))))
        assertEquals("1 onion\n2 cups rice\n1 tsp salt", GroceryList.text(items))
        assertEquals("1 onion\n1 tsp salt", GroceryList.text(items, setOf(2, 0)))
        assertEquals("", GroceryList.text(items, emptySet()))
        assertEquals(emptyList<String>(), GroceryList.build(emptyList()))
    }
}
