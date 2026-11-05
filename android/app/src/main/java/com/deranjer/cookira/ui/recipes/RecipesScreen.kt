package com.deranjer.cookira.ui.recipes

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.LazyRow
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.lazy.rememberLazyListState
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.Add
import androidx.compose.material.icons.filled.Close
import androidx.compose.material.icons.filled.Schedule
import androidx.compose.material.icons.filled.Search
import androidx.compose.material3.Card
import androidx.compose.material3.CardDefaults
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.FilterChip
import androidx.compose.material3.FloatingActionButton
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.SegmentedButton
import androidx.compose.material3.SegmentedButtonDefaults
import androidx.compose.material3.SingleChoiceSegmentedButtonRow
import androidx.compose.material3.Text
import androidx.compose.material3.pulltorefresh.PullToRefreshBox
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.snapshotFlow
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.layout.ContentScale
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.lifecycle.ViewModel
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import androidx.lifecycle.viewModelScope
import coil.compose.AsyncImage
import com.deranjer.cookira.data.RecipeSummary
import com.deranjer.cookira.data.TagCount
import com.deranjer.cookira.di.AppContainer
import com.deranjer.cookira.di.LocalAppContainer
import com.deranjer.cookira.di.appViewModel
import com.deranjer.cookira.ui.common.DraftBadge
import com.deranjer.cookira.ui.common.OfflineBanner
import com.deranjer.cookira.ui.common.formatMinutes
import kotlinx.coroutines.FlowPreview
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.combine
import kotlinx.coroutines.flow.debounce
import kotlinx.coroutines.flow.distinctUntilChanged
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch

data class RecipesUiState(
    val items: List<RecipeSummary> = emptyList(),
    val total: Int = 0,
    val tags: List<TagCount> = emptyList(),
    val loading: Boolean = true,
    val refreshing: Boolean = false,
    val loadingMore: Boolean = false,
    val error: String? = null,
    /** True while showing the saved copy because the server cannot be reached. */
    val offline: Boolean = false,
    val syncedAt: Long = 0,
) {
    val canLoadMore get() = items.size < total
}

@OptIn(FlowPreview::class)
class RecipesViewModel(private val c: AppContainer) : ViewModel() {
    private val _ui = MutableStateFlow(RecipesUiState())
    val ui: StateFlow<RecipesUiState> = _ui.asStateFlow()

    val query = MutableStateFlow("")
    val tag = MutableStateFlow<String?>(null)
    val status = MutableStateFlow("")

    private data class Filters(val q: String, val tag: String?, val status: String, val version: Int)

    init {
        // Search is debounced; the other filters and "a recipe changed elsewhere" apply right away.
        val debouncedQuery = query.debounce(250).distinctUntilChanged()
        viewModelScope.launch {
            combine(debouncedQuery, tag, status, c.recipesVersion) { q, t, s, v -> Filters(q, t, s, v) }
                .distinctUntilChanged()
                .collect { reload(it, showSpinner = true) }
        }
    }

    private var current = Filters("", null, "", 0)

    private suspend fun reload(f: Filters, showSpinner: Boolean) {
        current = f
        _ui.update { it.copy(loading = showSpinner && it.items.isEmpty(), error = null) }
        try {
            val page = c.repository.recipes(f.q, f.tag, f.status, PAGE, 0)
            val tags = runCatching { c.repository.tags() }.getOrNull()
            _ui.update {
                it.copy(
                    items = page.items, total = page.total, tags = tags ?: it.tags, loading = false, refreshing = false,
                    offline = page.fromCache, syncedAt = page.syncedAt,
                )
            }
            // Keep the saved library fresh whenever the server is reachable and it has gone stale.
            if (!page.fromCache && System.currentTimeMillis() - page.syncedAt > STALE_MS) c.syncNow()
        } catch (e: Exception) {
            _ui.update { it.copy(loading = false, refreshing = false, error = e.message ?: "Couldn't load recipes") }
        }
    }

    fun refresh() {
        _ui.update { it.copy(refreshing = true) }
        c.repository.forgetOffline()
        viewModelScope.launch {
            reload(current, showSpinner = false)
            if (!_ui.value.offline) c.syncNow()
        }
    }

    fun loadMore() {
        val s = _ui.value
        if (s.loadingMore || s.loading || !s.canLoadMore) return
        _ui.update { it.copy(loadingMore = true) }
        viewModelScope.launch {
            try {
                val f = current
                val page = c.repository.recipes(f.q, f.tag, f.status, PAGE, s.items.size)
                _ui.update { it.copy(items = it.items + page.items, total = page.total, loadingMore = false) }
            } catch (e: Exception) {
                _ui.update { it.copy(loadingMore = false, error = e.message) }
            }
        }
    }

    private companion object {
        const val PAGE = 30
        const val STALE_MS = 30 * 60 * 1000L
    }
}

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun RecipesScreen(onOpen: (String) -> Unit, onNew: () -> Unit) {
    val vm = appViewModel { RecipesViewModel(it) }
    val ui by vm.ui.collectAsStateWithLifecycle()
    val query by vm.query.collectAsStateWithLifecycle()
    val tag by vm.tag.collectAsStateWithLifecycle()
    val status by vm.status.collectAsStateWithLifecycle()
    val listState = rememberLazyListState()

    // Infinite scroll: ask for the next page when the last few rows come into view.
    LaunchedEffect(listState, ui.items.size) {
        snapshotFlow { listState.layoutInfo.visibleItemsInfo.lastOrNull()?.index ?: 0 }
            .collect { last -> if (last >= ui.items.size - 4) vm.loadMore() }
    }

    Box(Modifier.fillMaxSize()) {
        Column(Modifier.fillMaxSize()) {
            if (ui.offline) OfflineBanner(ui.syncedAt)
            Column(Modifier.padding(horizontal = 16.dp, vertical = 8.dp), verticalArrangement = Arrangement.spacedBy(8.dp)) {
                OutlinedTextField(
                    value = query,
                    onValueChange = { vm.query.value = it },
                    modifier = Modifier.fillMaxWidth(),
                    placeholder = { Text("Search recipes, ingredients, tags…") },
                    leadingIcon = { Icon(Icons.Default.Search, null) },
                    trailingIcon = {
                        if (query.isNotEmpty()) {
                            IconButton(onClick = { vm.query.value = "" }) { Icon(Icons.Default.Close, "Clear search") }
                        }
                    },
                    singleLine = true,
                )
                val statuses = listOf("" to "All", "published" to "Published", "draft" to "Drafts")
                SingleChoiceSegmentedButtonRow(Modifier.fillMaxWidth()) {
                    statuses.forEachIndexed { i, (value, label) ->
                        SegmentedButton(
                            selected = status == value,
                            onClick = { vm.status.value = value },
                            shape = SegmentedButtonDefaults.itemShape(i, statuses.size),
                        ) { Text(label) }
                    }
                }
            }
            if (ui.tags.isNotEmpty()) {
                LazyRow(
                    contentPadding = PaddingValues(horizontal = 16.dp),
                    horizontalArrangement = Arrangement.spacedBy(8.dp),
                ) {
                    items(ui.tags, key = { it.tag }) { t ->
                        FilterChip(
                            selected = tag == t.tag,
                            onClick = { vm.tag.value = if (tag == t.tag) null else t.tag },
                            label = { Text("${t.tag} (${t.count})") },
                        )
                    }
                }
            }

            PullToRefreshBox(isRefreshing = ui.refreshing, onRefresh = vm::refresh, modifier = Modifier.weight(1f)) {
                when {
                    ui.loading -> Box(Modifier.fillMaxSize(), contentAlignment = Alignment.Center) { CircularProgressIndicator() }
                    ui.error != null && ui.items.isEmpty() -> Box(Modifier.fillMaxSize().verticalScroll(rememberScrollState()).padding(24.dp), contentAlignment = Alignment.Center) {
                        Text(ui.error ?: "", color = MaterialTheme.colorScheme.error)
                    }
                    ui.items.isEmpty() -> Box(Modifier.fillMaxSize().verticalScroll(rememberScrollState()).padding(24.dp), contentAlignment = Alignment.Center) {
                        Text(
                            "No recipes found. Add one with +, or ask an agent connected over MCP.",
                            color = MaterialTheme.colorScheme.onSurfaceVariant,
                        )
                    }
                    else -> LazyColumn(
                        state = listState,
                        contentPadding = PaddingValues(start = 16.dp, end = 16.dp, top = 12.dp, bottom = 96.dp),
                        verticalArrangement = Arrangement.spacedBy(12.dp),
                    ) {
                        items(ui.items, key = { it.id }) { r -> RecipeCard(r, onClick = { onOpen(r.slug) }) }
                        if (ui.loadingMore) {
                            item { Box(Modifier.fillMaxWidth().padding(8.dp), contentAlignment = Alignment.Center) { CircularProgressIndicator() } }
                        }
                    }
                }
            }
        }
        // Creating needs the server; the saved copy is read-only.
        if (!ui.offline) FloatingActionButton(
            onClick = onNew,
            modifier = Modifier.align(Alignment.BottomEnd).padding(16.dp),
        ) { Icon(Icons.Default.Add, contentDescription = "New recipe") }
    }
}

@Composable
private fun RecipeCard(recipe: RecipeSummary, onClick: () -> Unit) {
    val container = LocalAppContainer.current
    val time = formatMinutes(recipe.prepMinutes + recipe.cookMinutes)
    Card(
        onClick = onClick,
        colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.surfaceContainer),
        modifier = Modifier.fillMaxWidth(),
    ) {
        if (recipe.hasImage) {
            AsyncImage(
                model = container.repository.imageUrl(recipe.id, recipe.updatedAt, thumb = true),
                imageLoader = container.imageLoader,
                contentDescription = null,
                contentScale = ContentScale.Crop,
                modifier = Modifier.fillMaxWidth().height(160.dp),
            )
        } else {
            // Same placeholder idea as the web card: a plain tinted band.
            Box(Modifier.fillMaxWidth().height(56.dp).background(MaterialTheme.colorScheme.primaryContainer))
        }
        Column(Modifier.padding(16.dp), verticalArrangement = Arrangement.spacedBy(6.dp)) {
            Row(horizontalArrangement = Arrangement.spacedBy(8.dp), verticalAlignment = Alignment.Top) {
                Text(
                    recipe.title,
                    style = MaterialTheme.typography.titleLarge,
                    maxLines = 2,
                    overflow = TextOverflow.Ellipsis,
                    modifier = Modifier.weight(1f),
                )
                if (recipe.isDraft) DraftBadge()
            }
            if (recipe.tagline.isNotBlank()) {
                Text(
                    recipe.tagline,
                    style = MaterialTheme.typography.bodyMedium,
                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                    maxLines = 2,
                    overflow = TextOverflow.Ellipsis,
                )
            }
            Row(horizontalArrangement = Arrangement.spacedBy(10.dp), verticalAlignment = Alignment.CenterVertically) {
                if (time.isNotEmpty()) {
                    Icon(Icons.Default.Schedule, null, Modifier.height(16.dp))
                    Text(time, style = MaterialTheme.typography.labelMedium)
                }
                Text(
                    recipe.tags.take(3).joinToString("  ·  "),
                    style = MaterialTheme.typography.labelMedium,
                    color = MaterialTheme.colorScheme.primary,
                    maxLines = 1,
                    overflow = TextOverflow.Ellipsis,
                )
            }
        }
    }
}
