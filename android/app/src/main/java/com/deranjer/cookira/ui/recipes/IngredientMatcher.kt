package com.deranjer.cookira.ui.recipes

import com.deranjer.cookira.data.Recipe

/** An ingredient line, addressed the same way the recipe screen's tick boxes are ("group:item"). */
data class IngredientRef(val key: String, val text: String)

/**
 * Works out which ingredients each step uses, from the text alone (no schema support needed).
 *
 * An ingredient matches when its meaningful words (quantities, units and prep words removed)
 * show up in the step. Ingredient *groups* add context: when a step names a group
 * ("whisk together the marinade", "make the garlic sauce") that group is "in focus", and
 * same-named lines from other groups (every group has its own salt and olive oil) are dropped.
 */
object IngredientMatcher {

    fun match(recipe: Recipe): List<List<IngredientRef>> {
        val groups = recipe.ingredients.mapIndexed { gi, g ->
            Group(
                nameTokens = tokens(g.name),
                items = g.items.mapIndexed { ii, text -> Item("$gi:$ii", text, tokens(text.substringBefore(','))) },
            )
        }
        return recipe.steps.map { step -> matchStep(groups, tokenSet(step.text + " " + step.substeps.joinToString(" "))) }
    }

    private class Item(val key: String, val text: String, val tokens: Set<String>)

    private class Group(val nameTokens: Set<String>, val items: List<Item>) {
        /** A name like "Chicken" or "Rice" is also an ingredient word; "Marinade" is only a label. */
        val isLabel = nameTokens.isNotEmpty() && items.none { it.tokens.containsAll(nameTokens) }
    }

    private class Candidate(val item: Item, val group: Int, val full: Boolean, val viaGroup: Boolean)

    private fun matchStep(groups: List<Group>, step: Set<String>): List<IngredientRef> {
        val focus = groups.indices.filter { groups[it].nameTokens.isNotEmpty() && step.containsAll(groups[it].nameTokens) }.toSet()
        val wholeGroups = focus.filter { groups[it].isLabel }.toSet()

        val found = ArrayList<Candidate>()
        groups.forEachIndexed { gi, g ->
            for (item in g.items) {
                if (item.tokens.isEmpty()) continue
                if (gi in wholeGroups) {
                    found += Candidate(item, gi, full = true, viaGroup = true)
                } else if (item.tokens.any { it in step }) {
                    found += Candidate(item, gi, full = step.containsAll(item.tokens), viaGroup = false)
                }
            }
        }

        // "onion" vs "red onion": the more specific line wins when the step is that specific,
        // and the plain line wins when the step only says "onion".
        val specific = found.filter { c ->
            c.viaGroup || found.none { o ->
                o !== c && o.item.tokens.size != c.item.tokens.size && when {
                    o.item.tokens.size > c.item.tokens.size -> o.full && o.item.tokens.containsAll(c.item.tokens)
                    else -> o.full && !c.full && c.item.tokens.containsAll(o.item.tokens)
                }
            }
        }

        // Another group's copy of something the focused group already lists (every group has
        // its own salt and olive oil) is a duplicate, not a second use.
        val focusTokens = specific.filter { it.group in focus }.map { it.item.tokens }
        val kept = specific.filter { c ->
            c.viaGroup || c.group in focus || focusTokens.none { f -> f.any { it in c.item.tokens } }
        }
        return kept.map { IngredientRef(it.item.key, it.item.text) }
    }

    // --- tokenising ---

    private fun tokenSet(text: String): Set<String> = tokens(text, dropFillers = false)

    internal fun tokens(text: String, dropFillers: Boolean = true): Set<String> =
        text.replace(PARENS, " ")
            .lowercase()
            .split(NON_LETTER)
            .map(::singular)
            .filter { it.length >= 2 && (!dropFillers || it !in FILLERS) }
            .toSet()

    private fun singular(w: String): String = when {
        w.length > 4 && w.endsWith("ies") -> w.dropLast(3) + "y"
        w.length > 4 && w.endsWith("oes") -> w.dropLast(2)
        w.length > 3 && w.endsWith("s") && !w.endsWith("ss") -> w.dropLast(1)
        else -> w
    }

    private val PARENS = Regex("\\([^)]*\\)")
    private val NON_LETTER = Regex("[^\\p{L}]+")

    /** Units, prep words, and glue that say how much or how prepared, not what the ingredient is. */
    private val FILLERS: Set<String> = (
        "tsp teaspoon tbsp tablespoon cup lb pound oz ounce gram kg ml liter litre pinch dash clove can " +
            "slice stick bunch package pkg piece handful " +
            "a an the of and or to for with in on at into plus about more taste optional " +
            "diced minced chopped sliced grated shredded crushed mashed melted softened cubed halved peeled " +
            "thinly thickly finely roughly freshly fresh uncooked cooked raw large small medium extra " +
            "boneless skinless kosher plain whole ground"
        ).split(' ').map(::singular).toSet()
}
