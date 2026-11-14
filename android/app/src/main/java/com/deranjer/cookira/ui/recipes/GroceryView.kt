package com.deranjer.cookira.ui.recipes

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.itemsIndexed
import androidx.compose.foundation.selection.toggleable
import androidx.compose.material3.Checkbox
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.ModalBottomSheet
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.material3.rememberModalBottomSheetState
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.Alignment
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.unit.dp
import com.deranjer.cookira.data.IngredientGroup
import com.deranjer.cookira.ui.common.copyToClipboard

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun GroceryView(ingredients: List<IngredientGroup>) {
    var opened by remember(ingredients) { mutableStateOf(false) }
    OutlinedButton(onClick = { opened = true }) { Text("Grocery view") }
    if (opened) {
        val items = remember(ingredients) { GroceryList.build(ingredients) }
        var selected by remember(items) { mutableStateOf(items.indices.toSet()) }
        var status by remember { mutableStateOf("") }
        val context = LocalContext.current
        ModalBottomSheet(
            onDismissRequest = { opened = false },
            sheetState = rememberModalBottomSheetState(skipPartiallyExpanded = true),
        ) {
            Column(Modifier.padding(horizontal = 16.dp), verticalArrangement = Arrangement.spacedBy(8.dp)) {
                Text("Grocery list", style = MaterialTheme.typography.headlineSmall)
                Text("Matching ingredient names and units are totaled across groups. Other entries stay separate. Select what you need to buy.", style = MaterialTheme.typography.bodyMedium)
                if (items.isEmpty()) Text("No ingredients yet.")
                if (items.isNotEmpty()) {
                    Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                        OutlinedButton(onClick = {
                            context.copyToClipboard("Grocery list", GroceryList.text(items))
                            status = "Copied ${items.size} items."
                        }) { Text("Copy all") }
                        OutlinedButton(enabled = selected.isNotEmpty(), onClick = {
                            context.copyToClipboard("Grocery list", GroceryList.text(items, selected))
                            status = "Copied ${selected.size} items."
                        }) { Text("Copy selected (${selected.size})") }
                    }
                    Row {
                        TextButton(onClick = { selected = items.indices.toSet() }) { Text("Select all") }
                        TextButton(onClick = { selected = emptySet() }) { Text("Clear selection") }
                    }
                }
                if (status.isNotEmpty()) Text(status, color = MaterialTheme.colorScheme.primary)
                LazyColumn(Modifier.weight(1f, fill = false).fillMaxWidth()) {
                    itemsIndexed(items) { index, item ->
                        Column(Modifier.padding(bottom = 12.dp)) {
                            Row(
                                Modifier.fillMaxWidth().toggleable(
                                    value = index in selected,
                                    role = Role.Checkbox,
                                    onValueChange = { selected = if (it) selected + index else selected - index },
                                ),
                                verticalAlignment = Alignment.CenterVertically,
                            ) {
                                Checkbox(checked = index in selected, onCheckedChange = null)
                                Text(item.text, Modifier.weight(1f), style = MaterialTheme.typography.bodyLarge)
                            }
                            if (item.sources.size > 1) Text(
                                item.sources.joinToString(" + "),
                                Modifier.padding(start = 48.dp),
                                style = MaterialTheme.typography.bodySmall,
                                color = MaterialTheme.colorScheme.onSurfaceVariant,
                            )
                        }
                    }
                }
            }
        }
    }
}
