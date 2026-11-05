package com.deranjer.cookira.ui.common

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.padding
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.text.AnnotatedString
import androidx.compose.ui.text.LinkAnnotation
import androidx.compose.ui.text.SpanStyle
import androidx.compose.ui.text.TextLinkStyles
import androidx.compose.ui.text.buildAnnotatedString
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontStyle
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextDecoration
import androidx.compose.ui.text.withLink
import androidx.compose.ui.text.withStyle
import androidx.compose.ui.unit.dp

sealed interface MdBlock {
    data class Heading(val level: Int, val text: String) : MdBlock
    data class Paragraph(val text: String) : MdBlock
    data class Bullet(val text: String) : MdBlock
    data class Numbered(val n: Int, val text: String) : MdBlock
    data class Quote(val text: String) : MdBlock
}

/** Block-level parse of the small markdown subset recipe notes use. */
fun parseMarkdown(src: String): List<MdBlock> {
    val blocks = mutableListOf<MdBlock>()
    val para = StringBuilder()
    fun flush() {
        if (para.isNotEmpty()) {
            blocks += MdBlock.Paragraph(para.toString().trim())
            para.clear()
        }
    }
    for (raw in src.replace("\r\n", "\n").split('\n')) {
        val line = raw.trimEnd()
        val t = line.trimStart()
        when {
            t.isEmpty() -> flush()
            t.startsWith("#") -> {
                flush()
                val level = t.takeWhile { it == '#' }.length.coerceIn(1, 6)
                blocks += MdBlock.Heading(level, t.drop(level).trim())
            }
            t.startsWith("- ") || t.startsWith("* ") || t.startsWith("+ ") -> {
                flush()
                blocks += MdBlock.Bullet(t.drop(2).trim())
            }
            Regex("""^\d+[.)]\s""").containsMatchIn(t) -> {
                flush()
                val n = t.takeWhile { it.isDigit() }.toInt()
                blocks += MdBlock.Numbered(n, t.dropWhile { it.isDigit() }.drop(2).trim())
            }
            t.startsWith(">") -> {
                flush()
                blocks += MdBlock.Quote(t.drop(1).trim())
            }
            else -> {
                if (para.isNotEmpty()) para.append(' ')
                para.append(t)
            }
        }
    }
    flush()
    return blocks
}

private val inlineToken = Regex("""\*\*(.+?)\*\*|__(.+?)__|\*(.+?)\*|_(.+?)_|`(.+?)`|\[(.+?)]\((\S+?)\)""")

/** Inline spans: bold, italic, code and links. */
fun inlineMarkdown(text: String, linkColor: androidx.compose.ui.graphics.Color): AnnotatedString =
    buildAnnotatedString {
        var last = 0
        for (m in inlineToken.findAll(text)) {
            append(text.substring(last, m.range.first))
            val g = m.groupValues
            when {
                g[1].isNotEmpty() -> withStyle(SpanStyle(fontWeight = FontWeight.Bold)) { append(g[1]) }
                g[2].isNotEmpty() -> withStyle(SpanStyle(fontWeight = FontWeight.Bold)) { append(g[2]) }
                g[3].isNotEmpty() -> withStyle(SpanStyle(fontStyle = FontStyle.Italic)) { append(g[3]) }
                g[4].isNotEmpty() -> withStyle(SpanStyle(fontStyle = FontStyle.Italic)) { append(g[4]) }
                g[5].isNotEmpty() -> withStyle(SpanStyle(fontFamily = FontFamily.Monospace)) { append(g[5]) }
                else -> {
                    val url = g[7]
                    if (url.startsWith("http://") || url.startsWith("https://")) {
                        withLink(
                            LinkAnnotation.Url(
                                url,
                                TextLinkStyles(SpanStyle(color = linkColor, textDecoration = TextDecoration.Underline)),
                            ),
                        ) { append(g[6]) }
                    } else {
                        append(g[6])
                    }
                }
            }
            last = m.range.last + 1
        }
        append(text.substring(last))
    }

@Composable
fun MarkdownText(markdown: String, modifier: Modifier = Modifier) {
    val link = MaterialTheme.colorScheme.primary
    Column(modifier, verticalArrangement = Arrangement.spacedBy(8.dp)) {
        for (b in parseMarkdown(markdown)) {
            when (b) {
                is MdBlock.Heading -> Text(
                    inlineMarkdown(b.text, link),
                    style = if (b.level <= 2) MaterialTheme.typography.titleLarge else MaterialTheme.typography.titleMedium,
                )
                is MdBlock.Paragraph -> Text(inlineMarkdown(b.text, link), style = MaterialTheme.typography.bodyLarge)
                is MdBlock.Bullet -> Row {
                    Text("•", Modifier.padding(end = 8.dp, start = 4.dp))
                    Text(inlineMarkdown(b.text, link), style = MaterialTheme.typography.bodyLarge)
                }
                is MdBlock.Numbered -> Row {
                    Text("${b.n}.", Modifier.padding(end = 8.dp, start = 4.dp))
                    Text(inlineMarkdown(b.text, link), style = MaterialTheme.typography.bodyLarge)
                }
                is MdBlock.Quote -> Text(
                    inlineMarkdown(b.text, link),
                    style = MaterialTheme.typography.bodyLarge.copy(fontStyle = FontStyle.Italic),
                    modifier = Modifier.padding(start = 12.dp),
                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                )
            }
        }
    }
}
