package com.deranjer.cookira.ui.recipes

import android.widget.Toast
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.filled.ArrowBack
import androidx.compose.material.icons.filled.Delete
import androidx.compose.material.icons.filled.Edit
import androidx.compose.material.icons.filled.History
import androidx.compose.material.icons.filled.MoreVert
import androidx.compose.material.icons.filled.Share
import androidx.compose.material.icons.filled.Visibility
import androidx.compose.material.icons.filled.VisibilityOff
import androidx.compose.material3.AlertDialog
import androidx.compose.material3.DropdownMenu
import androidx.compose.material3.DropdownMenuItem
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.ModalBottomSheet
import androidx.compose.material3.Scaffold
import androidx.compose.material3.SnackbarHost
import androidx.compose.material3.SnackbarHostState
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.material3.TopAppBar
import androidx.compose.material3.FilterChip
import androidx.compose.material3.OutlinedButton
import androidx.compose.runtime.Composable
import androidx.compose.runtime.DisposableEffect
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableIntStateOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.platform.LocalView
import androidx.compose.ui.unit.dp
import androidx.lifecycle.ViewModel
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import androidx.lifecycle.viewModelScope
import com.deranjer.cookira.data.Recipe
import com.deranjer.cookira.data.Revision
import com.deranjer.cookira.data.ShareDto
import com.deranjer.cookira.di.AppContainer
import com.deranjer.cookira.di.LocalAppContainer
import com.deranjer.cookira.di.appViewModel
import com.deranjer.cookira.ui.common.CenteredLoading
import com.deranjer.cookira.ui.common.ErrorState
import com.deranjer.cookira.ui.common.Load
import com.deranjer.cookira.ui.common.OfflineBanner
import com.deranjer.cookira.ui.common.copyToClipboard
import com.deranjer.cookira.ui.common.formatDate
import com.deranjer.cookira.ui.common.formatDateTime
import com.deranjer.cookira.ui.common.shareText
import com.deranjer.cookira.ui.common.toPlainText
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch

class RecipeDetailViewModel(private val c: AppContainer, private val ref: String) : ViewModel() {
    private val _recipe = MutableStateFlow<Load<Recipe>>(Load.Loading)
    val recipe: StateFlow<Load<Recipe>> = _recipe.asStateFlow()

    private val _offline = MutableStateFlow(false)

    /** True when this is the saved copy (server unreachable): read only, no edits. */
    val offline: StateFlow<Boolean> = _offline.asStateFlow()
    private val _syncedAt = MutableStateFlow(0L)
    val syncedAt: StateFlow<Long> = _syncedAt.asStateFlow()

    private val _checked = MutableStateFlow<Set<String>>(emptySet())
    val checked: StateFlow<Set<String>> = _checked.asStateFlow()

    private val _busy = MutableStateFlow(false)
    val busy: StateFlow<Boolean> = _busy.asStateFlow()

    private val _message = MutableStateFlow<String?>(null)
    val message: StateFlow<String?> = _message.asStateFlow()

    private val _shares = MutableStateFlow<Load<List<ShareDto>>>(Load.Loading)
    val shares: StateFlow<Load<List<ShareDto>>> = _shares.asStateFlow()

    private val _revisions = MutableStateFlow<Load<List<Revision>>>(Load.Loading)
    val revisions: StateFlow<Load<List<Revision>>> = _revisions.asStateFlow()

    init {
        load()
    }

    fun load() {
        viewModelScope.launch {
            if (_recipe.value !is Load.Ready) _recipe.value = Load.Loading
            _recipe.value = try {
                val res = c.repository.recipe(ref)
                _offline.value = res.fromCache
                _syncedAt.value = res.syncedAt
                Load.Ready(res.recipe)
            } catch (e: Exception) {
                Load.Failed(e.message ?: "Recipe not found.")
            }
        }
    }

    fun toggleChecked(key: String) = _checked.update { if (key in it) it - key else it + key }
    fun clearMessage() { _message.value = null }

    private fun current(): Recipe? = (_recipe.value as? Load.Ready)?.data

    fun togglePublished() {
        val r = current() ?: return
        mutate {
            val updated = c.repository.updateRecipe(r.id, com.deranjer.cookira.data.RecipeInput(status = if (r.isDraft) "published" else "draft"))
            _recipe.value = Load.Ready(updated)
        }
    }

    fun delete(onDone: () -> Unit) {
        val r = current() ?: return
        mutate {
            c.repository.deleteRecipe(r.id)
            onDone()
        }
    }

    // --- sharing ---
    fun loadShares() {
        val r = current() ?: return
        viewModelScope.launch {
            _shares.value = try {
                Load.Ready(c.repository.shares(r.id))
            } catch (e: Exception) {
                Load.Failed(e.message ?: "Couldn't load share links")
            }
        }
    }

    fun createShare(expiresInDays: Int, onCreated: (ShareDto) -> Unit) {
        val r = current() ?: return
        mutate {
            val share = c.repository.createShare(r.id, expiresInDays)
            onCreated(share)
            loadShares()
        }
    }

    fun revokeShare(id: String) {
        val r = current() ?: return
        mutate {
            c.repository.revokeShare(r.id, id)
            loadShares()
        }
    }

    // --- history ---
    fun loadRevisions() {
        val r = current() ?: return
        viewModelScope.launch {
            _revisions.value = try {
                Load.Ready(c.repository.revisions(r.id).sortedByDescending { it.revision })
            } catch (e: Exception) {
                Load.Failed(e.message ?: "Couldn't load history")
            }
        }
    }

    fun restore(revision: Int) {
        val r = current() ?: return
        mutate {
            _recipe.value = Load.Ready(c.repository.restoreRevision(r.id, revision))
            loadRevisions()
            _message.value = "Restored revision $revision"
        }
    }

    private fun mutate(block: suspend () -> Unit) {
        viewModelScope.launch {
            _busy.value = true
            try {
                block()
                c.recipesChanged()
            } catch (e: Exception) {
                _message.value = e.message ?: "Something went wrong"
            } finally {
                _busy.value = false
            }
        }
    }
}

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun RecipeDetailScreen(ref: String, onBack: () -> Unit, onEdit: (String) -> Unit) {
    val container = LocalAppContainer.current
    val vm = appViewModel(key = "detail-$ref") { RecipeDetailViewModel(it, ref) }
    val state by vm.recipe.collectAsStateWithLifecycle()
    val checked by vm.checked.collectAsStateWithLifecycle()
    val busy by vm.busy.collectAsStateWithLifecycle()
    val offline by vm.offline.collectAsStateWithLifecycle()
    val syncedAt by vm.syncedAt.collectAsStateWithLifecycle()
    val message by vm.message.collectAsStateWithLifecycle()
    val keepAwake by container.prefs.keepAwake.collectAsStateWithLifecycle(initialValue = true)
    val context = LocalContext.current
    val view = LocalView.current
    val snackbar = remember { SnackbarHostState() }
    var menu by remember { mutableStateOf(false) }
    var showShare by remember { mutableStateOf(false) }
    var showHistory by remember { mutableStateOf(false) }
    var confirmDelete by remember { mutableStateOf(false) }

    // Cooking mode: keep the screen on while a recipe is open, unless turned off in Settings.
    DisposableEffect(keepAwake) {
        view.keepScreenOn = keepAwake
        onDispose { view.keepScreenOn = false }
    }
    LaunchedEffect(message) {
        message?.let {
            snackbar.showSnackbar(it)
            vm.clearMessage()
        }
    }
    // A recipe may have been edited on the editor screen above us in the back stack.
    val version by container.recipesVersion.collectAsStateWithLifecycle()
    LaunchedEffect(version) { if (state is Load.Ready) vm.load() }

    Scaffold(
        snackbarHost = { SnackbarHost(snackbar) },
        topBar = {
            TopAppBar(
                title = {},
                navigationIcon = { IconButton(onClick = onBack) { Icon(Icons.AutoMirrored.Filled.ArrowBack, "Back") } },
                actions = {
                    val recipe = (state as? Load.Ready)?.data
                    if (recipe != null) {
                        IconButton(onClick = { vm.togglePublished() }, enabled = !busy && !offline) {
                            Icon(
                                if (recipe.isDraft) Icons.Default.Visibility else Icons.Default.VisibilityOff,
                                contentDescription = if (recipe.isDraft) "Publish" else "Unpublish",
                            )
                        }
                        IconButton(onClick = { onEdit(recipe.id) }, enabled = !offline) { Icon(Icons.Default.Edit, "Edit") }
                        IconButton(onClick = { showShare = true; vm.loadShares() }, enabled = !offline) { Icon(Icons.Default.Share, "Share link") }
                        IconButton(onClick = { menu = true }) { Icon(Icons.Default.MoreVert, "More") }
                        DropdownMenu(expanded = menu, onDismissRequest = { menu = false }) {
                            DropdownMenuItem(
                                text = { Text("Share as text") },
                                leadingIcon = { Icon(Icons.Default.Share, null) },
                                onClick = { menu = false; context.shareText(recipe.title, recipe.toPlainText()) },
                            )
                            DropdownMenuItem(
                                enabled = !offline,
                                text = { Text("History") },
                                leadingIcon = { Icon(Icons.Default.History, null) },
                                onClick = { menu = false; showHistory = true; vm.loadRevisions() },
                            )
                            DropdownMenuItem(
                                enabled = !offline,
                                text = { Text("Delete") },
                                leadingIcon = { Icon(Icons.Default.Delete, null, tint = MaterialTheme.colorScheme.error) },
                                onClick = { menu = false; confirmDelete = true },
                            )
                        }
                    }
                },
            )
        },
    ) { padding ->
        when (val s = state) {
            Load.Loading -> CenteredLoading(Modifier.padding(padding))
            is Load.Failed -> ErrorState(s.message, onRetry = vm::load, modifier = Modifier.padding(padding))
            is Load.Ready -> Column(Modifier.padding(padding)) {
                if (offline) OfflineBanner(syncedAt)
                RecipeContent(
                    recipe = s.data,
                    imageUrl = if (s.data.hasImage) container.repository.imageUrl(s.data.id, s.data.updatedAt) else null,
                    checked = checked,
                    onToggle = vm::toggleChecked,
                    modifier = Modifier.verticalScroll(rememberScrollState()).padding(horizontal = 16.dp),
                )
            }
        }
    }

    val recipe = (state as? Load.Ready)?.data
    if (recipe != null && showShare) {
        ShareSheet(vm, onDismiss = { showShare = false })
    }
    if (recipe != null && showHistory) {
        HistorySheet(vm, onDismiss = { showHistory = false })
    }
    if (confirmDelete && recipe != null) {
        AlertDialog(
            onDismissRequest = { confirmDelete = false },
            title = { Text("Delete recipe?") },
            text = { Text("\"${recipe.title}\" will be deleted. You can still restore it from its history using its id.") },
            confirmButton = {
                TextButton(onClick = { confirmDelete = false; vm.delete(onDone = onBack) }) {
                    Text("Delete", color = MaterialTheme.colorScheme.error)
                }
            },
            dismissButton = { TextButton(onClick = { confirmDelete = false }) { Text("Cancel") } },
        )
    }
}

private val EXPIRY = listOf(0 to "Never expires", 1 to "1 day", 7 to "7 days", 30 to "30 days")

@OptIn(ExperimentalMaterial3Api::class, androidx.compose.foundation.layout.ExperimentalLayoutApi::class)
@Composable
private fun ShareSheet(vm: RecipeDetailViewModel, onDismiss: () -> Unit) {
    val shares by vm.shares.collectAsStateWithLifecycle()
    val busy by vm.busy.collectAsStateWithLifecycle()
    val context = LocalContext.current
    var expiry by remember { mutableIntStateOf(0) }

    ModalBottomSheet(onDismissRequest = onDismiss) {
        Column(Modifier.padding(horizontal = 16.dp).padding(bottom = 24.dp), verticalArrangement = Arrangement.spacedBy(12.dp)) {
            Text("Share recipe", style = MaterialTheme.typography.titleLarge)
            Text(
                "Anyone with the link can view this recipe without signing in. Revoke a link to stop access.",
                style = MaterialTheme.typography.bodyMedium,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
            )
            Text("Link lifetime", style = MaterialTheme.typography.labelLarge)
            androidx.compose.foundation.layout.FlowRow(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                EXPIRY.forEach { (days, label) ->
                    FilterChip(selected = expiry == days, onClick = { expiry = days }, label = { Text(label) })
                }
            }
            OutlinedButton(
                onClick = {
                    vm.createShare(expiry) { share ->
                        context.copyToClipboard("Share link", share.url)
                        Toast.makeText(context, "Link copied", Toast.LENGTH_SHORT).show()
                    }
                },
                enabled = !busy,
                modifier = Modifier.fillMaxWidth(),
            ) { Text("Create link") }

            when (val s = shares) {
                Load.Loading -> CenteredLoadingInline()
                is Load.Failed -> Text(s.message, color = MaterialTheme.colorScheme.error)
                is Load.Ready -> if (s.data.isEmpty()) {
                    Text("No active share links.", color = MaterialTheme.colorScheme.onSurfaceVariant)
                } else {
                    s.data.forEach { sh ->
                        Column(verticalArrangement = Arrangement.spacedBy(2.dp)) {
                            Text(sh.url, style = MaterialTheme.typography.bodyMedium)
                            Text(
                                if (sh.expiresAt != null) "Expires ${formatDate(sh.expiresAt)}" else "Never expires",
                                style = MaterialTheme.typography.bodySmall,
                                color = MaterialTheme.colorScheme.onSurfaceVariant,
                            )
                            Row(horizontalArrangement = Arrangement.spacedBy(4.dp)) {
                                TextButton(onClick = {
                                    context.copyToClipboard("Share link", sh.url)
                                    Toast.makeText(context, "Link copied", Toast.LENGTH_SHORT).show()
                                }) { Text("Copy") }
                                TextButton(onClick = { context.shareText("Recipe link", sh.url) }) { Text("Send") }
                                TextButton(onClick = { vm.revokeShare(sh.id) }, enabled = !busy) {
                                    Text("Revoke", color = MaterialTheme.colorScheme.error)
                                }
                            }
                        }
                    }
                }
            }
        }
    }
}

@OptIn(ExperimentalMaterial3Api::class)
@Composable
private fun HistorySheet(vm: RecipeDetailViewModel, onDismiss: () -> Unit) {
    val revisions by vm.revisions.collectAsStateWithLifecycle()
    val busy by vm.busy.collectAsStateWithLifecycle()
    var confirm by remember { mutableStateOf<Revision?>(null) }

    ModalBottomSheet(onDismissRequest = onDismiss) {
        Column(
            Modifier.padding(horizontal = 16.dp).padding(bottom = 24.dp).verticalScroll(rememberScrollState()),
            verticalArrangement = Arrangement.spacedBy(10.dp),
        ) {
            Text("History", style = MaterialTheme.typography.titleLarge)
            Text(
                "Every change, from the app or a connected agent, is recorded. Restoring a revision brings back that version of the recipe.",
                style = MaterialTheme.typography.bodyMedium,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
            )
            when (val s = revisions) {
                Load.Loading -> CenteredLoadingInline()
                is Load.Failed -> Text(s.message, color = MaterialTheme.colorScheme.error)
                is Load.Ready -> s.data.forEach { rev ->
                    Row(Modifier.fillMaxWidth(), verticalAlignment = Alignment.CenterVertically) {
                        Column(Modifier.weight(1f)) {
                            Text("#${rev.revision} · ${rev.action}", style = MaterialTheme.typography.titleSmall)
                            Text(
                                "${rev.actor} · ${formatDateTime(rev.createdAt)}",
                                style = MaterialTheme.typography.bodySmall,
                                color = MaterialTheme.colorScheme.onSurfaceVariant,
                            )
                        }
                        TextButton(onClick = { confirm = rev }, enabled = !busy) { Text("Restore") }
                    }
                }
            }
        }
    }
    confirm?.let { rev ->
        AlertDialog(
            onDismissRequest = { confirm = null },
            title = { Text("Restore revision ${rev.revision}?") },
            text = { Text("The recipe returns to how it was then. The current version stays in history.") },
            confirmButton = { TextButton(onClick = { vm.restore(rev.revision); confirm = null }) { Text("Restore") } },
            dismissButton = { TextButton(onClick = { confirm = null }) { Text("Cancel") } },
        )
    }
}

@Composable
private fun CenteredLoadingInline() {
    androidx.compose.foundation.layout.Box(Modifier.fillMaxWidth().padding(12.dp), contentAlignment = Alignment.Center) {
        androidx.compose.material3.CircularProgressIndicator()
    }
}
