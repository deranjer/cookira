package com.deranjer.cookira.ui.common

import android.content.ClipData
import android.content.ClipboardManager
import android.content.Context
import android.content.Intent
import com.deranjer.cookira.data.Recipe
import java.time.OffsetDateTime
import java.time.ZoneId
import java.time.format.DateTimeFormatter
import java.time.format.FormatStyle

/** Same wording as the web UI's formatMinutes(). */
fun formatMinutes(total: Int): String {
    if (total <= 0) return ""
    val h = total / 60
    val m = total % 60
    return when {
        h == 0 -> "$m min"
        m == 0 -> "$h hr"
        else -> "$h hr $m min"
    }
}

fun formatDate(iso: String?, fallback: String = "Never"): String {
    if (iso.isNullOrBlank()) return fallback
    return runCatching {
        // The server emits RFC 3339 timestamps with an offset.
        DateTimeFormatter.ofLocalizedDate(FormatStyle.MEDIUM)
            .withZone(ZoneId.systemDefault())
            .format(OffsetDateTime.parse(iso).toInstant())
    }.getOrDefault(iso.take(10).ifBlank { fallback })
}

/** "just now", "5 min ago", "3 h ago", or the date for older syncs. */
fun formatAgo(epochMs: Long, now: Long = System.currentTimeMillis()): String {
    if (epochMs <= 0) return "never"
    val mins = (now - epochMs) / 60_000
    return when {
        mins < 1 -> "just now"
        mins < 60 -> "$mins min ago"
        mins < 24 * 60 -> "${mins / 60} h ago"
        else -> DateTimeFormatter.ofLocalizedDate(FormatStyle.MEDIUM)
            .withZone(ZoneId.systemDefault())
            .format(java.time.Instant.ofEpochMilli(epochMs))
    }
}

fun formatDateTime(iso: String?): String {
    if (iso.isNullOrBlank()) return ""
    return runCatching {
        DateTimeFormatter.ofLocalizedDateTime(FormatStyle.MEDIUM, FormatStyle.SHORT)
            .withZone(ZoneId.systemDefault())
            .format(OffsetDateTime.parse(iso))
    }.getOrDefault(iso)
}

/** Plain-text rendering of a recipe, for the system share sheet (the phone's "print"). */
fun Recipe.toPlainText(): String = buildString {
    appendLine(title)
    if (tagline.isNotBlank()) appendLine(tagline)
    val meta = listOfNotNull(
        author.takeIf { it.isNotBlank() }?.let { "By $it" },
        servings.takeIf { it > 0 }?.let { "Serves $it" },
        formatMinutes(prepMinutes).takeIf { it.isNotEmpty() }?.let { "Prep $it" },
        formatMinutes(cookMinutes).takeIf { it.isNotEmpty() }?.let { "Cook $it" },
        calories.takeIf { it > 0 }?.let { "$it kcal" },
    )
    if (meta.isNotEmpty()) appendLine(meta.joinToString(" · "))
    if (sourceUrl.isNotBlank()) appendLine(sourceUrl)
    appendLine()
    appendLine("INGREDIENTS")
    for (g in ingredients) {
        if (g.name.isNotBlank()) appendLine("${g.name}:")
        g.items.forEach { appendLine("- $it") }
    }
    appendLine()
    appendLine("DIRECTIONS")
    steps.forEachIndexed { i, s ->
        appendLine("${i + 1}. ${s.text}")
        s.substeps.forEachIndexed { j, sub -> appendLine("   ${'a' + j % 26}. $sub") }
    }
    if (notes.isNotBlank()) {
        appendLine()
        appendLine("NOTES")
        appendLine(notes)
    }
}.trimEnd()

fun Context.shareText(subject: String, text: String) {
    val send = Intent(Intent.ACTION_SEND).apply {
        type = "text/plain"
        putExtra(Intent.EXTRA_SUBJECT, subject)
        putExtra(Intent.EXTRA_TEXT, text)
    }
    startActivity(Intent.createChooser(send, null).addFlags(Intent.FLAG_ACTIVITY_NEW_TASK))
}

fun Context.copyToClipboard(label: String, text: String) {
    val cm = getSystemService(Context.CLIPBOARD_SERVICE) as ClipboardManager
    cm.setPrimaryClip(ClipData.newPlainText(label, text))
}
