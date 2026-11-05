package com.deranjer.cookira.ui.recipes

import android.content.Context
import android.net.Uri
import androidx.activity.compose.rememberLauncherForActivityResult
import androidx.activity.result.PickVisualMediaRequest
import androidx.activity.result.contract.ActivityResultContracts
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ExperimentalLayoutApi
import androidx.compose.foundation.layout.FlowRow
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.imePadding
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.text.KeyboardActions
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.filled.ArrowBack
import androidx.compose.material.icons.filled.Add
import androidx.compose.material.icons.filled.ArrowDownward
import androidx.compose.material.icons.filled.ArrowUpward
import androidx.compose.material.icons.filled.Delete
import androidx.compose.material.icons.filled.PhotoCamera
import androidx.compose.material.icons.filled.PhotoLibrary
import androidx.compose.material3.AssistChip
import androidx.compose.material3.Button
import androidx.compose.material3.Card
import androidx.compose.material3.CardDefaults
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.FilterChip
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.InputChip
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Scaffold
import androidx.compose.material3.SegmentedButton
import androidx.compose.material3.SegmentedButtonDefaults
import androidx.compose.material3.SingleChoiceSegmentedButtonRow
import androidx.compose.material3.SnackbarHost
import androidx.compose.material3.SnackbarHostState
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.material3.TopAppBar
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateListOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.layout.ContentScale
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.text.input.ImeAction
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.unit.dp
import androidx.core.content.FileProvider
import androidx.lifecycle.ViewModel
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import androidx.lifecycle.viewModelScope
import coil.compose.AsyncImage
import com.deranjer.cookira.data.IngredientGroup
import com.deranjer.cookira.data.Recipe
import com.deranjer.cookira.data.RecipeInput
import com.deranjer.cookira.data.Step
import com.deranjer.cookira.di.AppContainer
import com.deranjer.cookira.di.LocalAppContainer
import com.deranjer.cookira.di.appViewModel
import com.deranjer.cookira.ui.common.CenteredLoading
import com.deranjer.cookira.ui.common.ErrorState
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.launch
import java.io.File

data class GroupDraft(val name: String = "", val items: String = "")
data class StepDraft(val text: String = "", val substeps: String = "")

private fun lines(s: String) = s.split('\n').map { it.trim() }.filter { it.isNotEmpty() }

private fun <T> MutableList<T>.move(from: Int, to: Int) {
    if (to < 0 || to >= size) return
    add(to, removeAt(from))
}

/** Editable copy of a recipe. Lives in the ViewModel so rotation doesn't lose unsaved edits. */
class RecipeForm {
    var title by mutableStateOf("")
    var tagline by mutableStateOf("")
    var author by mutableStateOf("")
    var sourceUrl by mutableStateOf("")
    var servings by mutableStateOf("")
    var prep by mutableStateOf("")
    var cook by mutableStateOf("")
    var calories by mutableStateOf("")
    var notes by mutableStateOf("")
    var status by mutableStateOf("draft")
    val tags = mutableStateListOf<String>()
    val groups = mutableStateListOf(GroupDraft())
    val steps = mutableStateListOf(StepDraft())

    fun load(r: Recipe) {
        title = r.title; tagline = r.tagline; author = r.author; sourceUrl = r.sourceUrl
        servings = r.servings.takeIf { it > 0 }?.toString() ?: ""
        prep = r.prepMinutes.takeIf { it > 0 }?.toString() ?: ""
        cook = r.cookMinutes.takeIf { it > 0 }?.toString() ?: ""
        calories = r.calories.takeIf { it > 0 }?.toString() ?: ""
        notes = r.notes; status = r.status
        tags.clear(); tags.addAll(r.tags)
        groups.clear()
        groups.addAll(r.ingredients.map { GroupDraft(it.name, it.items.joinToString("\n")) }.ifEmpty { listOf(GroupDraft()) })
        steps.clear()
        steps.addAll(r.steps.map { StepDraft(it.text, it.substeps.joinToString("\n")) }.ifEmpty { listOf(StepDraft()) })
    }

    fun toInput() = RecipeInput(
        title = title.trim(),
        tagline = tagline.trim(),
        author = author.trim(),
        sourceUrl = sourceUrl.trim(),
        servings = servings.toIntOrNull() ?: 0,
        prepMinutes = prep.toIntOrNull() ?: 0,
        cookMinutes = cook.toIntOrNull() ?: 0,
        calories = calories.toIntOrNull() ?: 0,
        notes = notes,
        status = status,
        tags = tags.toList(),
        ingredients = groups.map { IngredientGroup(it.name.trim(), lines(it.items)) },
        steps = steps.map { Step(it.text.trim(), lines(it.substeps)) },
    )
}

class RecipeEditorViewModel(private val c: AppContainer, private val ref: String?) : ViewModel() {
    val form = RecipeForm()

    private val _loading = MutableStateFlow(ref != null)
    val loading: StateFlow<Boolean> = _loading.asStateFlow()
    private val _loadError = MutableStateFlow<String?>(null)
    val loadError: StateFlow<String?> = _loadError.asStateFlow()
    private val _saving = MutableStateFlow(false)
    val saving: StateFlow<Boolean> = _saving.asStateFlow()
    private val _error = MutableStateFlow<String?>(null)
    val error: StateFlow<String?> = _error.asStateFlow()
    private val _knownTags = MutableStateFlow<List<String>>(emptyList())
    val knownTags: StateFlow<List<String>> = _knownTags.asStateFlow()

    /** The saved recipe, once loaded (editing) - supplies the existing photo. */
    private val _existing = MutableStateFlow<Recipe?>(null)
    val existing: StateFlow<Recipe?> = _existing.asStateFlow()
    private val _pendingPhoto = MutableStateFlow<Uri?>(null)
    val pendingPhoto: StateFlow<Uri?> = _pendingPhoto.asStateFlow()
    private val _removePhoto = MutableStateFlow(false)
    val removePhoto: StateFlow<Boolean> = _removePhoto.asStateFlow()

    val isNew get() = ref == null

    init {
        viewModelScope.launch { _knownTags.value = runCatching { c.repository.tags() }.getOrNull()?.map { it.tag } ?: emptyList() }
        load()
    }

    fun load() {
        if (ref == null) return
        viewModelScope.launch {
            _loading.value = true
            _loadError.value = null
            try {
                val r = c.repository.recipe(ref).recipe
                _existing.value = r
                form.load(r)
            } catch (e: Exception) {
                _loadError.value = e.message ?: "Recipe not found."
            }
            _loading.value = false
        }
    }

    fun setPhoto(uri: Uri?) {
        _pendingPhoto.value = uri
        if (uri != null) _removePhoto.value = false
    }

    fun removePhoto() {
        _pendingPhoto.value = null
        _removePhoto.value = true
    }

    /** Creates or updates, then applies the photo change. [onSaved] gets the recipe's slug. */
    fun save(onSaved: (slug: String) -> Unit) {
        if (form.title.isBlank() || _saving.value) return
        viewModelScope.launch {
            _saving.value = true
            _error.value = null
            try {
                val saved = if (ref == null) c.repository.createRecipe(form.toInput()) else c.repository.updateRecipe(ref, form.toInput())
                // The recipe exists now; a photo failure must not lose the edit, so report it and stay put.
                val photoError = try {
                    when {
                        _pendingPhoto.value != null -> c.repository.uploadImage(saved.id, _pendingPhoto.value!!)
                        _removePhoto.value && _existing.value?.hasImage == true -> c.repository.removeImage(saved.id)
                    }
                    null
                } catch (e: Exception) {
                    e.message ?: "unknown error"
                }
                c.recipesChanged()
                if (photoError != null) {
                    _error.value = "Recipe saved, but the photo change failed: $photoError"
                    _pendingPhoto.value = null
                    _existing.value = saved
                } else {
                    onSaved(saved.slug.ifBlank { saved.id })
                }
            } catch (e: Exception) {
                _error.value = e.message ?: "Save failed"
            }
            _saving.value = false
        }
    }

    fun imageUrl(): String? {
        val r = _existing.value ?: return null
        if (!r.hasImage || _removePhoto.value) return null
        return c.repository.imageUrl(r.id, r.updatedAt)
    }
}

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun RecipeEditorScreen(ref: String?, onBack: () -> Unit, onSaved: (String) -> Unit) {
    val vm = appViewModel(key = "editor-${ref ?: "new"}") { RecipeEditorViewModel(it, ref) }
    val loading by vm.loading.collectAsStateWithLifecycle()
    val loadError by vm.loadError.collectAsStateWithLifecycle()
    val saving by vm.saving.collectAsStateWithLifecycle()
    val error by vm.error.collectAsStateWithLifecycle()
    val snackbar = remember { SnackbarHostState() }
    LaunchedEffect(error) { error?.let { snackbar.showSnackbar(it) } }

    Scaffold(
        snackbarHost = { SnackbarHost(snackbar) },
        topBar = {
            TopAppBar(
                title = { Text(if (vm.isNew) "New recipe" else "Edit recipe") },
                navigationIcon = { IconButton(onClick = onBack) { Icon(Icons.AutoMirrored.Filled.ArrowBack, "Cancel") } },
                actions = {
                    TextButton(onClick = { vm.save(onSaved) }, enabled = !saving && vm.form.title.isNotBlank() && !loading) {
                        if (saving) CircularProgressIndicator(Modifier.height(18.dp)) else Text(if (vm.isNew) "Create" else "Save")
                    }
                },
            )
        },
    ) { padding ->
        when {
            loading -> CenteredLoading(Modifier.padding(padding))
            loadError != null -> ErrorState(loadError ?: "", onRetry = vm::load, modifier = Modifier.padding(padding))
            else -> EditorForm(vm, Modifier.padding(padding))
        }
    }
}

@OptIn(ExperimentalLayoutApi::class)
@Composable
private fun EditorForm(vm: RecipeEditorViewModel, modifier: Modifier = Modifier) {
    val f = vm.form
    val known by vm.knownTags.collectAsStateWithLifecycle()
    var tagInput by remember { mutableStateOf("") }

    fun addTag(raw: String) {
        val t = raw.trim().lowercase().trim(',')
        if (t.isNotEmpty() && t !in f.tags) f.tags.add(t)
        tagInput = ""
    }

    Column(
        modifier.fillMaxWidth().verticalScroll(rememberScrollState()).imePadding().padding(horizontal = 16.dp, vertical = 8.dp),
        verticalArrangement = Arrangement.spacedBy(14.dp),
    ) {
        PhotoField(vm)

        OutlinedTextField(f.title, { f.title = it }, label = { Text("Title *") }, singleLine = true, modifier = Modifier.fillMaxWidth())
        OutlinedTextField(f.tagline, { f.tagline = it }, label = { Text("Tagline") }, modifier = Modifier.fillMaxWidth())
        OutlinedTextField(f.author, { f.author = it }, label = { Text("Author") }, singleLine = true, modifier = Modifier.fillMaxWidth())
        OutlinedTextField(
            f.sourceUrl, { f.sourceUrl = it }, label = { Text("Source URL") }, placeholder = { Text("https://") },
            singleLine = true, modifier = Modifier.fillMaxWidth(),
            keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.Uri),
        )
        Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
            NumberField("Servings", f.servings, { f.servings = it }, Modifier.weight(1f))
            NumberField("Calories", f.calories, { f.calories = it }, Modifier.weight(1f))
        }
        Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
            NumberField("Prep (min)", f.prep, { f.prep = it }, Modifier.weight(1f))
            NumberField("Cook (min)", f.cook, { f.cook = it }, Modifier.weight(1f))
        }

        // Tags
        Text("Tags", style = MaterialTheme.typography.titleSmall)
        FlowRow(horizontalArrangement = Arrangement.spacedBy(6.dp), verticalArrangement = Arrangement.spacedBy(4.dp)) {
            f.tags.toList().forEach { t ->
                InputChip(
                    selected = true,
                    onClick = { f.tags.remove(t) },
                    label = { Text(t) },
                    trailingIcon = { Icon(Icons.Default.Delete, "Remove $t", Modifier.height(16.dp)) },
                )
            }
        }
        OutlinedTextField(
            value = tagInput,
            onValueChange = { v -> if (v.endsWith(",")) addTag(v) else tagInput = v },
            label = { Text("Add a tag") },
            singleLine = true,
            modifier = Modifier.fillMaxWidth(),
            keyboardOptions = KeyboardOptions(imeAction = ImeAction.Done),
            keyboardActions = KeyboardActions(onDone = { addTag(tagInput) }),
            trailingIcon = { if (tagInput.isNotBlank()) IconButton(onClick = { addTag(tagInput) }) { Icon(Icons.Default.Add, "Add tag") } },
        )
        val suggestions = known.filter { it !in f.tags && (tagInput.isBlank() || it.contains(tagInput.trim().lowercase())) }.take(10)
        if (suggestions.isNotEmpty()) {
            FlowRow(horizontalArrangement = Arrangement.spacedBy(6.dp), verticalArrangement = Arrangement.spacedBy(4.dp)) {
                suggestions.forEach { t -> AssistChip(onClick = { addTag(t) }, label = { Text(t) }) }
            }
        }

        // Ingredients
        Text("Ingredients", style = MaterialTheme.typography.titleLarge)
        f.groups.forEachIndexed { i, g ->
            Card(colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.surfaceContainer)) {
                Column(Modifier.padding(12.dp), verticalArrangement = Arrangement.spacedBy(8.dp)) {
                    OutlinedTextField(
                        g.name, { f.groups[i] = g.copy(name = it) },
                        placeholder = { Text("Group name (optional), e.g. Broth") },
                        singleLine = true, modifier = Modifier.fillMaxWidth(),
                    )
                    OutlinedTextField(
                        g.items, { f.groups[i] = g.copy(items = it) },
                        placeholder = { Text("One ingredient per line") },
                        minLines = 3, modifier = Modifier.fillMaxWidth(),
                    )
                    ReorderRow(
                        canUp = i > 0, canDown = i < f.groups.lastIndex,
                        onUp = { f.groups.move(i, i - 1) }, onDown = { f.groups.move(i, i + 1) },
                        onDelete = { f.groups.removeAt(i) }, deleteLabel = "Remove group",
                    )
                }
            }
        }
        OutlinedButton(onClick = { f.groups.add(GroupDraft()) }) {
            Icon(Icons.Default.Add, null); Spacer(Modifier.padding(2.dp)); Text("Add ingredient group")
        }

        // Directions
        Text("Directions", style = MaterialTheme.typography.titleLarge)
        f.steps.forEachIndexed { i, s ->
            Card(colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.surfaceContainer)) {
                Column(Modifier.padding(12.dp), verticalArrangement = Arrangement.spacedBy(8.dp)) {
                    Text("Step ${i + 1}", style = MaterialTheme.typography.labelLarge)
                    OutlinedTextField(s.text, { f.steps[i] = s.copy(text = it) }, minLines = 2, modifier = Modifier.fillMaxWidth())
                    OutlinedTextField(
                        s.substeps, { f.steps[i] = s.copy(substeps = it) },
                        placeholder = { Text("Sub-steps, one per line (optional)") },
                        modifier = Modifier.fillMaxWidth(),
                    )
                    ReorderRow(
                        canUp = i > 0, canDown = i < f.steps.lastIndex,
                        onUp = { f.steps.move(i, i - 1) }, onDown = { f.steps.move(i, i + 1) },
                        onDelete = { f.steps.removeAt(i) }, deleteLabel = "Remove step",
                    )
                }
            }
        }
        OutlinedButton(onClick = { f.steps.add(StepDraft()) }) {
            Icon(Icons.Default.Add, null); Spacer(Modifier.padding(2.dp)); Text("Add step")
        }

        OutlinedTextField(
            f.notes, { f.notes = it }, label = { Text("Notes (markdown)") },
            minLines = 3, modifier = Modifier.fillMaxWidth(),
        )

        Text("Status", style = MaterialTheme.typography.titleSmall)
        val options = listOf("draft" to "Draft", "published" to "Published")
        SingleChoiceSegmentedButtonRow(Modifier.fillMaxWidth()) {
            options.forEachIndexed { i, (value, label) ->
                SegmentedButton(
                    selected = f.status == value,
                    onClick = { f.status = value },
                    shape = SegmentedButtonDefaults.itemShape(i, options.size),
                ) { Text(label) }
            }
        }
        Spacer(Modifier.height(32.dp))
    }
}

@Composable
private fun NumberField(label: String, value: String, onChange: (String) -> Unit, modifier: Modifier = Modifier) {
    OutlinedTextField(
        value = value,
        onValueChange = { v -> onChange(v.filter { it.isDigit() }.take(6)) },
        label = { Text(label) },
        singleLine = true,
        modifier = modifier,
        keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.Number),
    )
}

@Composable
private fun ReorderRow(
    canUp: Boolean,
    canDown: Boolean,
    onUp: () -> Unit,
    onDown: () -> Unit,
    onDelete: () -> Unit,
    deleteLabel: String,
) {
    Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.End) {
        IconButton(onClick = onUp, enabled = canUp) { Icon(Icons.Default.ArrowUpward, "Move up") }
        IconButton(onClick = onDown, enabled = canDown) { Icon(Icons.Default.ArrowDownward, "Move down") }
        IconButton(onClick = onDelete) { Icon(Icons.Default.Delete, deleteLabel, tint = MaterialTheme.colorScheme.error) }
    }
}

/** Header photo: shows the saved or newly chosen picture, with gallery / camera / remove. */
@Composable
private fun PhotoField(vm: RecipeEditorViewModel) {
    val context = LocalContext.current
    val container = LocalAppContainer.current
    val pending by vm.pendingPhoto.collectAsStateWithLifecycle()
    val existing by vm.existing.collectAsStateWithLifecycle()
    val removed by vm.removePhoto.collectAsStateWithLifecycle()
    var cameraUri by rememberSaveable { mutableStateOf<Uri?>(null) }

    val pick = rememberLauncherForActivityResult(ActivityResultContracts.PickVisualMedia()) { uri -> if (uri != null) vm.setPhoto(uri) }
    val capture = rememberLauncherForActivityResult(ActivityResultContracts.TakePicture()) { ok ->
        if (ok) cameraUri?.let { vm.setPhoto(it) }
    }

    val model: Any? = pending ?: if (!removed && existing?.hasImage == true) vm.imageUrl() else null

    Column(verticalArrangement = Arrangement.spacedBy(8.dp)) {
        Box(
            Modifier.fillMaxWidth().height(180.dp).clip(RoundedCornerShape(12.dp)),
            contentAlignment = Alignment.Center,
        ) {
            if (model != null) {
                AsyncImage(
                    model = model,
                    imageLoader = container.imageLoader,
                    contentDescription = "Recipe photo",
                    contentScale = ContentScale.Crop,
                    modifier = Modifier.fillMaxWidth().height(180.dp),
                )
            } else {
                Card(
                    colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.primaryContainer),
                    modifier = Modifier.fillMaxWidth().height(180.dp),
                ) {
                    Box(Modifier.fillMaxWidth().height(180.dp), contentAlignment = Alignment.Center) {
                        Text("No photo", color = MaterialTheme.colorScheme.onPrimaryContainer)
                    }
                }
            }
        }
        Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
            OutlinedButton(onClick = { pick.launch(PickVisualMediaRequest(ActivityResultContracts.PickVisualMedia.ImageOnly)) }) {
                Icon(Icons.Default.PhotoLibrary, null); Spacer(Modifier.padding(2.dp)); Text("Choose")
            }
            OutlinedButton(onClick = {
                val uri = newCameraUri(context)
                cameraUri = uri
                capture.launch(uri)
            }) {
                Icon(Icons.Default.PhotoCamera, null); Spacer(Modifier.padding(2.dp)); Text("Camera")
            }
            if (model != null) {
                TextButton(onClick = { vm.removePhoto() }) { Text("Remove", color = MaterialTheme.colorScheme.error) }
            }
        }
        if (pending != null) {
            Text(
                "The photo is uploaded when you save.",
                style = MaterialTheme.typography.bodySmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
            )
        }
    }
}

private fun newCameraUri(context: Context): Uri {
    val dir = File(context.cacheDir, "images").apply { mkdirs() }
    val file = File.createTempFile("capture_", ".jpg", dir)
    return FileProvider.getUriForFile(context, "${context.packageName}.fileprovider", file)
}
