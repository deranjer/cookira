package com.deranjer.cookira.ui.recipes

/** A time found in step text. [seconds] is the low end of a range ("4–5 hours" starts at 4 hours). */
data class FoundDuration(val text: String, val seconds: Int, val isRange: Boolean)

object DurationParser {
    private const val NUM = "(?:\\d+(?:\\.\\d+)?[½¼¾⅓⅔]?|[½¼¾⅓⅔])"
    private val PATTERN = Regex(
        "(?<![\\d.])($NUM)(?:\\s*(?:-|–|—|to)\\s*($NUM))?\\s*(hours?|hrs?|minutes?|mins?|seconds?|secs?)\\b",
        RegexOption.IGNORE_CASE,
    )

    /** Distinct durations in order of appearance. */
    fun find(text: String): List<FoundDuration> =
        PATTERN.findAll(text).mapNotNull { m ->
            val low = number(m.groupValues[1]) ?: return@mapNotNull null
            val unit = when (m.groupValues[3].lowercase().first()) {
                'h' -> 3600
                'm' -> 60
                else -> 1
            }
            val seconds = (low * unit).toInt()
            if (seconds <= 0) null else FoundDuration(m.value.trim(), seconds, isRange = m.groupValues[2].isNotEmpty())
        }.distinctBy { it.text.lowercase() }.toList()

    private fun number(s: String): Double? {
        val frac = when (s.last()) {
            '½' -> 0.5; '¼' -> 0.25; '¾' -> 0.75; '⅓' -> 1.0 / 3; '⅔' -> 2.0 / 3
            else -> null
        }
        val whole = if (frac != null) s.dropLast(1) else s
        val base = if (whole.isEmpty()) 0.0 else whole.toDoubleOrNull() ?: return null
        return base + (frac ?: 0.0)
    }

    /** 3725 -> "1:02:05", 205 -> "3:25". */
    fun clock(totalSeconds: Long): String {
        val s = totalSeconds.coerceAtLeast(0)
        val h = s / 3600
        val m = (s % 3600) / 60
        val sec = s % 60
        return if (h > 0) "%d:%02d:%02d".format(h, m, sec) else "%d:%02d".format(m, sec)
    }
}
