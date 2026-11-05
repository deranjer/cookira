package com.deranjer.cookira.ui.recipes

import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.filled.ArrowBack
import androidx.compose.material.icons.filled.Share
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Text
import androidx.compose.material3.TopAppBar
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.unit.dp
import androidx.lifecycle.ViewModel
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import androidx.lifecycle.viewModelScope
import com.deranjer.cookira.data.Recipe
import com.deranjer.cookira.di.AppContainer
import com.deranjer.cookira.di.LocalAppContainer
import com.deranjer.cookira.di.appViewModel
import com.deranjer.cookira.ui.common.CenteredLoading
import com.deranjer.cookira.ui.common.ErrorState
import com.deranjer.cookira.ui.common.Load
import com.deranjer.cookira.ui.common.shareText
import com.deranjer.cookira.ui.common.toPlainText
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.launch

class SharedRecipeViewModel(private val c: AppContainer, private val origin: String, private val token: String) : ViewModel() {
    private val _recipe = MutableStateFlow<Load<Recipe>>(Load.Loading)
    val recipe: StateFlow<Load<Recipe>> = _recipe.asStateFlow()

    init {
        load()
    }

    fun load() {
        viewModelScope.launch {
            _recipe.value = Load.Loading
            _recipe.value = try {
                Load.Ready(c.repository.sharedRecipe(origin, token))
            } catch (e: Exception) {
                Load.Failed(if (e.message.isNullOrBlank()) "This link is invalid or has expired." else e.message!!)
            }
        }
    }
}

/** The public, read-only view a share link opens (no login needed). */
@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun SharedRecipeScreen(origin: String, token: String, onBack: () -> Unit) {
    val container = LocalAppContainer.current
    val vm = appViewModel(key = "shared-$origin-$token") { SharedRecipeViewModel(it, origin, token) }
    val state by vm.recipe.collectAsStateWithLifecycle()
    val context = LocalContext.current

    Scaffold(
        topBar = {
            TopAppBar(
                title = { Text("Shared recipe") },
                navigationIcon = { IconButton(onClick = onBack) { Icon(Icons.AutoMirrored.Filled.ArrowBack, "Back") } },
                actions = {
                    (state as? Load.Ready)?.data?.let { r ->
                        IconButton(onClick = { context.shareText(r.title, r.toPlainText()) }) { Icon(Icons.Default.Share, "Share as text") }
                    }
                },
            )
        },
    ) { padding ->
        when (val s = state) {
            Load.Loading -> CenteredLoading(Modifier.padding(padding))
            is Load.Failed -> ErrorState(s.message, onRetry = vm::load, modifier = Modifier.padding(padding))
            is Load.Ready -> RecipeContent(
                recipe = s.data,
                imageUrl = if (s.data.hasImage) container.repository.sharedImageUrl(origin, token) else null,
                modifier = Modifier.padding(padding).verticalScroll(rememberScrollState()).padding(horizontal = 16.dp),
            )
        }
    }
}
