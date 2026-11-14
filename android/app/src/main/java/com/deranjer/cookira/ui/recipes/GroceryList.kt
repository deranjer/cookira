package com.deranjer.cookira.ui.recipes

import com.deranjer.cookira.data.IngredientGroup
import java.util.Locale
import java.math.BigDecimal
import java.math.MathContext
import kotlin.math.abs
import kotlin.math.floor
import kotlin.math.round

data class GroceryItem(val text: String, val sources: List<String>, val optional: Boolean = false, val category: String = "")

/** Conservative free-text totals. Keep in sync with web/src/lib/grocery.ts. */
object GroceryList {
    private val fractions = mapOf('½' to 0.5, '¼' to 0.25, '¾' to 0.75, '⅓' to 1.0 / 3, '⅔' to 2.0 / 3,
        '⅛' to 0.125, '⅜' to 0.375, '⅝' to 0.625, '⅞' to 0.875)
    private val quantity = Regex("""^(\d+\s+\d+/\d+|\d+/\d+|\d*\s*[½¼¾⅓⅔⅛⅜⅝⅞]|\d+(?:\.\d+)?)\s+(.+)$""")
    private val unitPattern = Regex("""^(\p{L}+)\.?\s+(.+)$""")
    private val unsafe = Regex("""^to\b|\b(or|plus|divided)\b|[()+]""", RegexOption.IGNORE_CASE)
    private val units = mapOf(
        "tsp" to "tsp teaspoon teaspoons", "tbsp" to "tbsp tablespoon tablespoons", "cup" to "cup cups",
        "lb" to "lb lbs pound pounds", "oz" to "oz ounce ounces", "g" to "g gram grams", "kg" to "kg kilogram kilograms",
        "ml" to "ml milliliter milliliters millilitre millilitres", "l" to "l liter liters litre litres",
    ).flatMap { (unit, aliases) -> aliases.split(' ').map { it to unit } }.toMap()

    private fun number(text: String): Double {
        fractions[text.last()]?.let { return (text.dropLast(1).trim().toDoubleOrNull() ?: 0.0) + it }
        if ('/' in text) {
            val parts = text.split(Regex("""[\s/]+""")).map { it.toDouble() }
            return if (parts.size == 3) parts[0] + parts[1] / parts[2] else parts[0] / parts[1]
        }
        return text.toDouble()
    }

    private fun format(value: Double): String {
        val whole = floor(value).toLong()
        for (denominator in listOf(2, 3, 4, 8, 16)) {
            val numerator = round((value - whole) * denominator).toInt()
            if (numerator in 1 until denominator && abs(value - whole - numerator.toDouble() / denominator) < 1e-8) {
                return (if (whole > 0) "$whole " else "") + "$numerator/$denominator"
            }
        }
        return BigDecimal.valueOf(value).round(MathContext(12)).stripTrailingZeros().toPlainString()
    }

    private fun legacy(groups: List<IngredientGroup>): List<GroceryItem> {
        data class Row(var text: String, val sources: MutableList<String>, var amount: Double = 0.0, val suffix: String = "")
        val rows = mutableListOf<Row>()
        val merged = mutableMapOf<String, Int>()
        for (group in groups) for (raw in group.items) {
            val text = raw.trim().replace(Regex("""\s+"""), " ")
            if (text.isEmpty()) continue
            val source = if (group.name.isNotBlank()) "${group.name.trim()}: $text" else text
            val match = quantity.matchEntire(text)
            val amount = match?.let { number(it.groupValues[1]) } ?: Double.NaN
            var suffix = match?.groupValues?.get(2) ?: ""
            unitPattern.matchEntire(suffix)?.let { unitMatch ->
                units[unitMatch.groupValues[1].lowercase(Locale.ROOT)]?.let { unit -> suffix = "$unit ${unitMatch.groupValues[2]}" }
            }
            if (!amount.isFinite() || amount <= 0 || suffix.firstOrNull()?.isLetter() != true || unsafe.containsMatchIn(suffix)) {
                rows.add(Row(text, mutableListOf(source)))
                continue
            }
            val key = suffix.lowercase(Locale.ROOT)
            val index = merged[key]
            if (index == null) {
                merged[key] = rows.size
                rows.add(Row(text, mutableListOf(source), amount, suffix))
            } else {
                val row = rows[index]
                row.amount += amount
                row.text = "${format(row.amount)} ${row.suffix}"
                row.sources.add(source)
            }
        }
        return rows.map { GroceryItem(it.text, it.sources.toList()) }
    }

    fun build(groups: List<IngredientGroup>): List<GroceryItem> {
        data class Row(var item: GroceryItem, var amount: Double? = null, val unit: String = "", val suffix: String = "", val factor: Double = 1.0)
        val conversion = mapOf(
            "tsp" to ("us-volume" to 1.0), "tbsp" to ("us-volume" to 3.0), "cup" to ("us-volume" to 48.0),
            "g" to ("metric-mass" to 1.0), "kg" to ("metric-mass" to 1000.0),
            "oz" to ("us-mass" to 1.0), "lb" to ("us-mass" to 16.0),
            "ml" to ("metric-volume" to 1.0), "l" to ("metric-volume" to 1000.0),
        )
        val rows = mutableListOf<Row>()
        val merged = mutableMapOf<List<String>, Int>()
        val unstructured = mutableListOf<IngredientGroup>()
        for (group in groups) {
            val entries = group.entries
            if (entries == null) { unstructured.add(group); continue }
            for ((index, e) in entries.withIndex()) {
                if (e.text != null) { unstructured.add(IngredientGroup(group.name, listOf(e.text))); continue }
                val rawUnit = e.unit.lowercase(Locale.ROOT)
                val unit = units[rawUnit] ?: rawUnit
                val suffix = listOf(e.size, e.name).filter { it.isNotBlank() }.joinToString(" ")
                val amount = e.quantity
                val range = e.quantityMax?.let { "–${format(it)}" } ?: ""
                val text = listOf(amount?.let { "${format(it)}$range" } ?: "", unit, suffix).filter { it.isNotBlank() }.joinToString(" ") +
                    (if (e.notes.isNotBlank()) ", ${e.notes}" else "") + (if (e.optional) " (optional)" else "")
                val sourceText = group.items.getOrNull(index) ?: text
                val source = if (group.name.isNotBlank()) "${group.name.trim()}: $sourceText" else sourceText
                val item = GroceryItem(text, listOf(source), e.optional, e.category)
                if (e.ingredientId.isBlank() || amount == null || e.quantityMax != null || e.notes.isNotBlank()) { rows.add(Row(item)); continue }
                val (family, factor) = conversion[unit] ?: (unit to 1.0)
                val key = listOf(e.ingredientId, family, e.size.trim().lowercase(Locale.ROOT), e.optional.toString())
                val found = merged[key]
                if (found == null) {
                    merged[key] = rows.size
                    rows.add(Row(item, amount, unit, suffix, factor))
                } else {
                    val row = rows[found]
                    row.amount = row.amount!! + amount * factor / row.factor
                    row.item = row.item.copy(
                        text = listOf(format(row.amount!!), row.unit, row.suffix).filter { it.isNotBlank() }.joinToString(" ") + (if (e.optional) " (optional)" else ""),
                        sources = row.item.sources + source,
                    )
                }
            }
        }
        return rows.map { it.item } + legacy(unstructured)
    }

    fun text(items: List<GroceryItem>, selected: Set<Int>? = null): String =
        items.filterIndexed { index, _ -> selected == null || index in selected }.joinToString("\n") { it.text }
}
