package com.deranjer.cookira.ui.recipes

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ExperimentalLayoutApi
import androidx.compose.foundation.layout.FlowRow
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.clickable
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.Card
import androidx.compose.material3.CardDefaults
import androidx.compose.material3.Checkbox
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.layout.ContentScale
import androidx.compose.ui.platform.LocalUriHandler
import androidx.compose.ui.text.style.TextDecoration
import androidx.compose.ui.unit.dp
import coil.compose.AsyncImage
import com.deranjer.cookira.data.Recipe
import com.deranjer.cookira.di.LocalAppContainer
import com.deranjer.cookira.ui.common.DraftBadge
import com.deranjer.cookira.ui.common.MarkdownText
import com.deranjer.cookira.ui.common.TagRow
import com.deranjer.cookira.ui.common.formatMinutes

/**
 * The read-only rendering of a recipe, shared by the signed-in detail screen and the public
 * share view (same as the web's RecipeView). Ingredients get tick boxes when [onToggle] is set.
 */
@OptIn(ExperimentalLayoutApi::class)
@Composable
fun RecipeContent(
    recipe: Recipe,
    imageUrl: String?,
    modifier: Modifier = Modifier,
    checked: Set<String> = emptySet(),
    onToggle: ((String) -> Unit)? = null,
) {
    val imageLoader = LocalAppContainer.current.imageLoader
    val uriHandler = LocalUriHandler.current
    Column(modifier, verticalArrangement = Arrangement.spacedBy(20.dp)) {
        if (imageUrl != null) {
            AsyncImage(
                model = imageUrl,
                imageLoader = imageLoader,
                contentDescription = null,
                contentScale = ContentScale.Crop,
                modifier = Modifier.fillMaxWidth().height(220.dp).clip(RoundedCornerShape(12.dp)),
            )
        }

        Column(verticalArrangement = Arrangement.spacedBy(6.dp)) {
            Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                Text(recipe.title, style = MaterialTheme.typography.headlineMedium, modifier = Modifier.weight(1f, fill = false))
                if (recipe.isDraft) DraftBadge()
            }
            if (recipe.tagline.isNotBlank()) {
                Text(recipe.tagline, style = MaterialTheme.typography.titleMedium, color = MaterialTheme.colorScheme.onSurfaceVariant)
            }
            if (recipe.author.isNotBlank() || recipe.sourceUrl.isNotBlank()) {
                Row {
                    if (recipe.author.isNotBlank()) {
                        Text("By ${recipe.author}", style = MaterialTheme.typography.bodyMedium, color = MaterialTheme.colorScheme.onSurfaceVariant)
                    }
                    if (recipe.author.isNotBlank() && recipe.sourceUrl.isNotBlank()) {
                        Text(" · ", style = MaterialTheme.typography.bodyMedium, color = MaterialTheme.colorScheme.onSurfaceVariant)
                    }
                    if (recipe.sourceUrl.isNotBlank()) {
                        Text(
                            "Source",
                            style = MaterialTheme.typography.bodyMedium,
                            color = MaterialTheme.colorScheme.primary,
                            textDecoration = TextDecoration.Underline,
                            modifier = Modifier.clickable {
                                // Only web links are opened; the field is free text.
                                if (recipe.sourceUrl.startsWith("http://") || recipe.sourceUrl.startsWith("https://")) {
                                    runCatching { uriHandler.openUri(recipe.sourceUrl) }
                                }
                            },
                        )
                    }
                }
            }
            TagRow(recipe.tags)
        }

        Stats(recipe)

        Column(verticalArrangement = Arrangement.spacedBy(12.dp)) {
            Text("Ingredients", style = MaterialTheme.typography.titleLarge)
            if (recipe.ingredients.isEmpty()) {
                Text("No ingredients yet.", color = MaterialTheme.colorScheme.onSurfaceVariant)
            }
            recipe.ingredients.forEachIndexed { gi, group ->
                if (group.name.isNotBlank()) Text(group.name, style = MaterialTheme.typography.titleMedium)
                group.items.forEachIndexed { ii, item ->
                    val key = "$gi:$ii"
                    if (onToggle != null) {
                        Row(
                            Modifier.fillMaxWidth().clickable { onToggle(key) },
                            verticalAlignment = Alignment.CenterVertically,
                        ) {
                            Checkbox(checked = key in checked, onCheckedChange = { onToggle(key) })
                            Text(
                                item,
                                style = MaterialTheme.typography.bodyLarge,
                                textDecoration = if (key in checked) TextDecoration.LineThrough else null,
                                color = if (key in checked) MaterialTheme.colorScheme.onSurfaceVariant else MaterialTheme.colorScheme.onSurface,
                            )
                        }
                    } else {
                        Row(Modifier.padding(start = 4.dp)) {
                            Text("•", Modifier.padding(end = 8.dp))
                            Text(item, style = MaterialTheme.typography.bodyLarge)
                        }
                    }
                }
            }
        }

        HorizontalDivider()

        Column(verticalArrangement = Arrangement.spacedBy(12.dp)) {
            Text("Directions", style = MaterialTheme.typography.titleLarge)
            if (recipe.steps.isEmpty()) {
                Text("No steps yet.", color = MaterialTheme.colorScheme.onSurfaceVariant)
            }
            recipe.steps.forEachIndexed { i, step ->
                Row {
                    Text("${i + 1}.", style = MaterialTheme.typography.bodyLarge, modifier = Modifier.width(28.dp))
                    Column(verticalArrangement = Arrangement.spacedBy(4.dp)) {
                        Text(step.text, style = MaterialTheme.typography.bodyLarge)
                        step.substeps.forEachIndexed { j, sub ->
                            Row {
                                Text("${'a' + j % 26}.", style = MaterialTheme.typography.bodyMedium, modifier = Modifier.width(24.dp))
                                Text(sub, style = MaterialTheme.typography.bodyMedium)
                            }
                        }
                    }
                }
            }
        }

        if (recipe.notes.isNotBlank()) {
            HorizontalDivider()
            Column(verticalArrangement = Arrangement.spacedBy(8.dp)) {
                Text("Notes", style = MaterialTheme.typography.titleLarge)
                MarkdownText(recipe.notes)
            }
        }
        Spacer(Modifier.height(24.dp))
    }
}

@OptIn(ExperimentalLayoutApi::class)
@Composable
private fun Stats(recipe: Recipe) {
    val items = listOfNotNull(
        recipe.servings.takeIf { it > 0 }?.let { "Servings" to "$it" },
        formatMinutes(recipe.prepMinutes).takeIf { it.isNotEmpty() }?.let { "Prep" to it },
        formatMinutes(recipe.cookMinutes).takeIf { it.isNotEmpty() }?.let { "Cook" to it },
        recipe.calories.takeIf { it > 0 }?.let { "Calories" to "$it kcal" },
    )
    if (items.isEmpty()) return
    val total = recipe.prepMinutes + recipe.cookMinutes
    Card(colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.surfaceContainer)) {
        Column(Modifier.padding(16.dp), verticalArrangement = Arrangement.spacedBy(8.dp)) {
            FlowRow(horizontalArrangement = Arrangement.spacedBy(28.dp), verticalArrangement = Arrangement.spacedBy(12.dp)) {
                for ((label, value) in items) {
                    Column {
                        Text(label.uppercase(), style = MaterialTheme.typography.labelSmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
                        Text(value, style = MaterialTheme.typography.titleMedium)
                    }
                }
            }
            if (recipe.prepMinutes > 0 && recipe.cookMinutes > 0) {
                Text("Total ${formatMinutes(total)}", style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
            }
        }
    }
}
