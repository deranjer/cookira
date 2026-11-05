package com.deranjer.cookira.ui

import android.net.Uri
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.widthIn
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.MenuBook
import androidx.compose.material.icons.filled.Settings
import androidx.compose.material3.AlertDialog
import androidx.compose.material3.CenterAlignedTopAppBar
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.NavigationBar
import androidx.compose.material3.NavigationBarItem
import androidx.compose.material3.Scaffold
import androidx.compose.material3.SnackbarHost
import androidx.compose.material3.SnackbarHostState
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.remember
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.unit.dp
import androidx.compose.foundation.layout.PaddingValues
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import androidx.navigation.NavHostController
import androidx.navigation.NavType
import androidx.navigation.compose.NavHost
import androidx.navigation.compose.composable
import androidx.navigation.compose.currentBackStackEntryAsState
import androidx.navigation.compose.rememberNavController
import androidx.navigation.navArgument
import com.deranjer.cookira.data.CookiraLink
import com.deranjer.cookira.data.ThemeMode
import com.deranjer.cookira.di.LocalAppContainer
import com.deranjer.cookira.di.SessionState
import com.deranjer.cookira.ui.auth.AcceptInviteScreen
import com.deranjer.cookira.ui.auth.ConnectScreen
import com.deranjer.cookira.ui.recipes.RecipeDetailScreen
import com.deranjer.cookira.ui.recipes.RecipeEditorScreen
import com.deranjer.cookira.ui.recipes.RecipesScreen
import com.deranjer.cookira.ui.recipes.SharedRecipeScreen
import com.deranjer.cookira.ui.settings.SettingsScreen
import com.deranjer.cookira.ui.theme.CookiraTheme

private object Routes {
    const val RECIPES = "recipes"
    const val SETTINGS = "settings"
    const val CONNECT = "connect"
    const val DETAIL = "recipe/{ref}"
    const val EDIT = "edit?ref={ref}"
    const val SHARED = "shared/{origin}/{token}"
    const val INVITE = "invite/{origin}/{token}"

    fun detail(ref: String) = "recipe/${Uri.encode(ref)}"
    fun edit(ref: String?) = if (ref == null) "edit" else "edit?ref=${Uri.encode(ref)}"
    fun shared(origin: String, token: String) = "shared/${Uri.encode(origin)}/${Uri.encode(token)}"
    fun invite(origin: String, token: String) = "invite/${Uri.encode(origin)}/${Uri.encode(token)}"
}

@Composable
fun AppRoot() {
    val container = LocalAppContainer.current
    val theme by container.prefs.themeMode.collectAsStateWithLifecycle(initialValue = ThemeMode.System)
    val state by container.sessionState.collectAsStateWithLifecycle()

    CookiraTheme(theme) {
        Surface(Modifier.fillMaxSize(), color = MaterialTheme.colorScheme.background) {
            // Cap the width so phones-in-landscape and tablets don't get edge-to-edge forms.
            Box(Modifier.fillMaxSize(), contentAlignment = Alignment.TopCenter) {
                Box(Modifier.widthIn(max = 720.dp).fillMaxSize()) {
                    when (state) {
                        SessionState.Loading -> Box(Modifier.fillMaxSize(), contentAlignment = Alignment.Center) { CircularProgressIndicator() }
                        SessionState.SignedOut -> SignedOutNav()
                        is SessionState.SignedIn -> SignedInNav()
                    }
                }
            }
        }
    }
}

@Composable
private fun SignedOutNav() {
    val container = LocalAppContainer.current
    val nav = rememberNavController()
    val pending by container.pendingLink.collectAsStateWithLifecycle()

    LaunchedEffect(pending) {
        when (val link = pending) {
            is CookiraLink.Share -> nav.navigate(Routes.shared(link.origin, link.token))
            is CookiraLink.Invite -> nav.navigate(Routes.invite(link.origin, link.token))
            null -> return@LaunchedEffect
        }
        container.consumeLink()
    }

    NavHost(nav, startDestination = Routes.CONNECT) {
        composable(Routes.CONNECT) { ConnectScreen() }
        composable(Routes.SHARED, args("origin", "token")) { e ->
            SharedRecipeScreen(e.arg("origin"), e.arg("token"), onBack = { nav.popBackStack() })
        }
        composable(Routes.INVITE, args("origin", "token")) { e ->
            AcceptInviteScreen(e.arg("origin"), e.arg("token"), onCancel = { nav.popBackStack() })
        }
    }
}

@OptIn(ExperimentalMaterial3Api::class)
@Composable
private fun SignedInNav() {
    val container = LocalAppContainer.current
    val nav = rememberNavController()
    val entry by nav.currentBackStackEntryAsState()
    val route = entry?.destination?.route
    val topLevel = route == Routes.RECIPES || route == Routes.SETTINGS
    val snackbar = remember { SnackbarHostState() }
    val pending by container.pendingLink.collectAsStateWithLifecycle()

    LaunchedEffect(pending) {
        val link = pending as? CookiraLink.Share ?: return@LaunchedEffect
        nav.navigate(Routes.shared(link.origin, link.token))
        container.consumeLink()
    }

    // Signed in already, so an invite link has nothing to offer.
    (pending as? CookiraLink.Invite)?.let {
        AlertDialog(
            onDismissRequest = container::consumeLink,
            title = { Text("Invite link") },
            text = { Text("You're already signed in. To join another household with this link, sign out first (Settings → Sign out), then open it again.") },
            confirmButton = { TextButton(onClick = container::consumeLink) { Text("OK") } },
        )
    }

    Scaffold(
        snackbarHost = { SnackbarHost(snackbar) },
        topBar = {
            if (topLevel) {
                CenterAlignedTopAppBar(title = { Text(if (route == Routes.SETTINGS) "Settings" else "Recipes", style = MaterialTheme.typography.titleLarge) })
            }
        },
        bottomBar = {
            if (topLevel) {
                NavigationBar {
                    NavigationBarItem(
                        selected = route == Routes.RECIPES,
                        onClick = { nav.goTop(Routes.RECIPES) },
                        icon = { Icon(Icons.Default.MenuBook, null) },
                        label = { Text("Recipes") },
                    )
                    NavigationBarItem(
                        selected = route == Routes.SETTINGS,
                        onClick = { nav.goTop(Routes.SETTINGS) },
                        icon = { Icon(Icons.Default.Settings, null) },
                        label = { Text("Settings") },
                    )
                }
            }
        },
    ) { padding ->
        NavHost(
            nav,
            startDestination = Routes.RECIPES,
            // Detail / editor screens bring their own Scaffold, so they only get insets when top-level.
            modifier = Modifier.padding(if (topLevel) padding else PaddingValues(0.dp)),
        ) {
            composable(Routes.RECIPES) {
                RecipesScreen(onOpen = { nav.navigate(Routes.detail(it)) }, onNew = { nav.navigate(Routes.edit(null)) })
            }
            composable(Routes.SETTINGS) { SettingsScreen(snackbar) }
            composable(Routes.DETAIL, args("ref")) { e ->
                RecipeDetailScreen(
                    ref = e.arg("ref"),
                    onBack = { nav.popBackStack() },
                    onEdit = { nav.navigate(Routes.edit(it)) },
                )
            }
            composable(
                Routes.EDIT,
                arguments = listOf(navArgument("ref") { type = NavType.StringType; nullable = true; defaultValue = null }),
            ) { e ->
                val ref = e.arguments?.getString("ref")
                RecipeEditorScreen(
                    ref = ref,
                    onBack = { nav.popBackStack() },
                    onSaved = { slug ->
                        // New recipe: replace the editor with its detail page. Edit: just go back to it.
                        if (ref == null) {
                            nav.popBackStack()
                            nav.navigate(Routes.detail(slug))
                        } else {
                            nav.popBackStack()
                        }
                    },
                )
            }
            composable(Routes.SHARED, args("origin", "token")) { e ->
                SharedRecipeScreen(e.arg("origin"), e.arg("token"), onBack = { nav.popBackStack() })
            }
        }
    }
}

private fun args(vararg names: String) = names.map { navArgument(it) { type = NavType.StringType } }

private fun androidx.navigation.NavBackStackEntry.arg(name: String) = arguments?.getString(name).orEmpty()

private fun NavHostController.goTop(route: String) = navigate(route) {
    popUpTo(Routes.RECIPES) { saveState = true }
    launchSingleTop = true
    restoreState = true
}
