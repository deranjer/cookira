package com.deranjer.cookira.ui.recipes

import com.deranjer.cookira.data.IngredientGroup
import java.util.Locale
import java.math.BigDecimal
import java.math.MathContext
import kotlin.math.abs
import kotlin.math.floor
import kotlin.math.round

data class GroceryItem(val text: String, val sources: List<String>)

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

    fun build(groups: List<IngredientGroup>): List<GroceryItem> {
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

    fun text(items: List<GroceryItem>, selected: Set<Int>? = null): String =
        items.filterIndexed { index, _ -> selected == null || index in selected }.joinToString("\n") { it.text }
}
