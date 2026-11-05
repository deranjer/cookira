package com.deranjer.cookira.ui.settings

import android.widget.Toast
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ExperimentalLayoutApi
import androidx.compose.foundation.layout.FlowRow
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.ContentCopy
import androidx.compose.material.icons.filled.Delete
import androidx.compose.material3.AlertDialog
import androidx.compose.material3.Button
import androidx.compose.material3.Card
import androidx.compose.material3.CardDefaults
import androidx.compose.material3.DropdownMenu
import androidx.compose.material3.DropdownMenuItem
import androidx.compose.material3.FilterChip
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.SegmentedButton
import androidx.compose.material3.SegmentedButtonDefaults
import androidx.compose.material3.SingleChoiceSegmentedButtonRow
import androidx.compose.material3.SnackbarHost
import androidx.compose.material3.SnackbarHostState
import androidx.compose.material3.Switch
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableIntStateOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.unit.dp
import androidx.lifecycle.ViewModel
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import androidx.lifecycle.viewModelScope
import com.deranjer.cookira.BuildConfig
import com.deranjer.cookira.data.InviteDto
import com.deranjer.cookira.data.OAuthConnection
import com.deranjer.cookira.data.ThemeMode
import com.deranjer.cookira.data.UserDto
import com.deranjer.cookira.di.AppContainer
import com.deranjer.cookira.di.LocalAppContainer
import com.deranjer.cookira.di.appViewModel
import com.deranjer.cookira.ui.auth.PasswordField
import com.deranjer.cookira.ui.common.Load
import com.deranjer.cookira.ui.common.copyToClipboard
import com.deranjer.cookira.ui.common.formatDate
import com.deranjer.cookira.ui.common.shareText
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch

class SettingsViewModel(private val c: AppContainer) : ViewModel() {
    val me: StateFlow<UserDto?> = c.currentUser

    private val _users = MutableStateFlow<Load<List<UserDto>>>(Load.Loading)
    val users: StateFlow<Load<List<UserDto>>> = _users.asStateFlow()
    private val _invites = MutableStateFlow<Load<List<InviteDto>>>(Load.Loading)
    val invites: StateFlow<Load<List<InviteDto>>> = _invites.asStateFlow()
    private val _connections = MutableStateFlow<Load<List<OAuthConnection>>>(Load.Loading)
    val connections: StateFlow<Load<List<OAuthConnection>>> = _connections.asStateFlow()
    private val _version = MutableStateFlow<String?>(null)
    val serverVersion: StateFlow<String?> = _version.asStateFlow()

    private val _message = MutableStateFlow<String?>(null)
    val message: StateFlow<String?> = _message.asStateFlow()
    private val _busy = MutableStateFlow(false)
    val busy: StateFlow<Boolean> = _busy.asStateFlow()
    private val _createdInvite = MutableStateFlow<InviteDto?>(null)
    val createdInvite: StateFlow<InviteDto?> = _createdInvite.asStateFlow()

    val serverUrl get() = c.repository.serverUrl.orEmpty()

    init {
        reload()
        viewModelScope.launch { _version.value = c.repository.serverVersion() }
    }

    fun reload() {
        loadConnections()
        viewModelScope.launch {
            // /users and /invites are admin-only; wait until we know who we are.
            val user = c.currentUser.value ?: runCatching { c.repository.me() }.getOrNull()?.also { c.setUser(it) }
            if (user?.isAdmin == true) {
                loadUsers()
                loadInvites()
            }
        }
    }

    fun clearMessage() { _message.value = null }
    fun dismissInvite() { _createdInvite.value = null }

    private suspend fun loadUsers() {
        _users.value = try { Load.Ready(c.repository.users()) } catch (e: Exception) { Load.Failed(e.message ?: "Couldn't load members") }
    }

    private suspend fun loadInvites() {
        _invites.value = try { Load.Ready(c.repository.invites()) } catch (e: Exception) { Load.Failed(e.message ?: "Couldn't load invites") }
    }

    private fun loadConnections() {
        viewModelScope.launch {
            _connections.value = try { Load.Ready(c.repository.oauthConnections()) } catch (e: Exception) { Load.Failed(e.message ?: "Couldn't load connected apps") }
        }
    }

    private fun act(success: String? = null, block: suspend () -> Unit) {
        viewModelScope.launch {
            _busy.value = true
            try {
                block()
                if (success != null) _message.value = success
            } catch (e: Exception) {
                _message.value = e.message ?: "Something went wrong"
            }
            _busy.value = false
        }
    }

    fun saveName(name: String) = act("Name updated") { c.setUser(c.repository.updateName(name)) }

    fun changePassword(current: String, next: String) = act("Password changed. Other devices were signed out.") {
        c.repository.changePassword(current, next)
    }

    fun disconnect(id: String) = act("App disconnected") {
        c.repository.disconnectOAuth(id)
        loadConnections()
    }

    fun setRole(user: UserDto, role: String) = act { c.repository.updateUser(user.id, role = role); loadUsers() }

    fun resetPassword(user: UserDto, password: String) = act("Password reset for ${user.name}") {
        c.repository.updateUser(user.id, password = password)
    }

    fun removeUser(user: UserDto) = act("${user.name} removed") { c.repository.deleteUser(user.id); loadUsers() }

    fun addUser(name: String, email: String, password: String, role: String) = act("Member added") {
        c.repository.createUser(name, email, password, role)
        loadUsers()
    }

    fun invite(email: String, role: String, days: Int) = act {
        _createdInvite.value = c.repository.createInvite(email, role, days)
        loadInvites()
    }

    fun revokeInvite(id: String) = act("Invite revoked") { c.repository.revokeInvite(id); loadInvites() }

    fun signOut() = c.logout()
}

@OptIn(ExperimentalLayoutApi::class)
@Composable
fun SettingsScreen(snackbar: SnackbarHostState) {
    val container = LocalAppContainer.current
    val vm = appViewModel { SettingsViewModel(it) }
    val me by vm.me.collectAsStateWithLifecycle()
    val users by vm.users.collectAsStateWithLifecycle()
    val invites by vm.invites.collectAsStateWithLifecycle()
    val connections by vm.connections.collectAsStateWithLifecycle()
    val message by vm.message.collectAsStateWithLifecycle()
    val busy by vm.busy.collectAsStateWithLifecycle()
    val created by vm.createdInvite.collectAsStateWithLifecycle()
    val serverVersion by vm.serverVersion.collectAsStateWithLifecycle()
    val sync by container.syncState.collectAsStateWithLifecycle()
    val theme by container.prefs.themeMode.collectAsStateWithLifecycle(initialValue = ThemeMode.System)
    val keepAwake by container.prefs.keepAwake.collectAsStateWithLifecycle(initialValue = true)
    val scope = rememberCoroutineScope()
    val context = LocalContext.current

    LaunchedEffect(message) {
        message?.let {
            snackbar.showSnackbar(it)
            vm.clearMessage()
        }
    }
    // Admin status can arrive after first composition (the user is fetched async).
    LaunchedEffect(me?.id) { if (me != null && users is Load.Loading) vm.reload() }

    var clearOffline by remember { mutableStateOf(false) }
    var addMember by remember { mutableStateOf(false) }
    var inviting by remember { mutableStateOf(false) }
    var signOutConfirm by remember { mutableStateOf(false) }

    Column(
        Modifier.fillMaxWidth().verticalScroll(rememberScrollState()).padding(16.dp),
        verticalArrangement = Arrangement.spacedBy(24.dp),
    ) {
        // --- Account ---
        Section("Your account") {
            var name by rememberSaveable(me?.name) { mutableStateOf(me?.name ?: "") }
            Text(me?.email ?: "", style = MaterialTheme.typography.bodyMedium, color = MaterialTheme.colorScheme.onSurfaceVariant)
            OutlinedTextField(name, { name = it }, label = { Text("Name") }, singleLine = true, modifier = Modifier.fillMaxWidth())
            Button(onClick = { vm.saveName(name) }, enabled = !busy && name.isNotBlank() && name.trim() != me?.name) { Text("Save name") }

            HorizontalDivider()
            Text("Change password", style = MaterialTheme.typography.titleMedium)
            var current by rememberSaveable { mutableStateOf("") }
            var next by rememberSaveable { mutableStateOf("") }
            var confirm by rememberSaveable { mutableStateOf("") }
            PasswordField("Current password", current, { current = it }, imeAction = androidx.compose.ui.text.input.ImeAction.Next)
            PasswordField("New password (8+ characters)", next, { next = it }, imeAction = androidx.compose.ui.text.input.ImeAction.Next)
            PasswordField("Confirm new password", confirm, { confirm = it })
            if (confirm.isNotEmpty() && confirm != next) {
                Text("Passwords don't match.", color = MaterialTheme.colorScheme.error, style = MaterialTheme.typography.bodySmall)
            }
            Button(
                onClick = { vm.changePassword(current, next); current = ""; next = ""; confirm = "" },
                enabled = !busy && current.isNotEmpty() && next.length >= 8 && next == confirm,
            ) { Text("Change password") }
        }

        // --- Appearance ---
        Section("Appearance") {
            val modes = ThemeMode.entries
            SingleChoiceSegmentedButtonRow(Modifier.fillMaxWidth()) {
                modes.forEachIndexed { i, m ->
                    SegmentedButton(
                        selected = theme == m,
                        onClick = { scope.launch { container.prefs.setTheme(m) } },
                        shape = SegmentedButtonDefaults.itemShape(i, modes.size),
                    ) { Text(m.name) }
                }
            }
            Row(Modifier.fillMaxWidth(), verticalAlignment = Alignment.CenterVertically) {
                Column(Modifier.weight(1f)) {
                    Text("Keep screen on while cooking", style = MaterialTheme.typography.bodyLarge)
                    Text("Stops the screen sleeping while a recipe is open.", style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
                }
                Switch(checked = keepAwake, onCheckedChange = { scope.launch { container.prefs.setKeepAwake(it) } })
            }
        }

        // --- Offline library ---
        Section("Offline library") {
            Text(
                "Your recipes are saved on this phone so you can read and cook from them without a connection. " +
                    "Editing needs a connection.",
                style = MaterialTheme.typography.bodyMedium,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
            )
            Text(
                if (sync.syncing) "Syncing…"
                else "${sync.count} recipes saved · last synced ${com.deranjer.cookira.ui.common.formatAgo(sync.syncedAt)}",
                style = MaterialTheme.typography.bodyLarge,
            )
            sync.error?.let { Text(it, color = MaterialTheme.colorScheme.error, style = MaterialTheme.typography.bodySmall) }
            Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                Button(onClick = { container.syncNow() }, enabled = !sync.syncing) { Text("Sync now") }
                OutlinedButton(onClick = { container.syncNow(allPhotos = true) }, enabled = !sync.syncing) { Text("Download all photos") }
            }
            Text(
                "Photos you open are kept automatically, and small previews download on Wi-Fi. \"Download all photos\" also fetches the full-size pictures.",
                style = MaterialTheme.typography.bodySmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
            )
            TextButton(onClick = { clearOffline = true }) { Text("Clear saved recipes and photos", color = MaterialTheme.colorScheme.error) }
        }

        // --- Household (admin) ---
        if (me?.isAdmin == true) {
            Section("Household") {
                Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                    Button(onClick = { inviting = true }) { Text("Invite") }
                    OutlinedButton(onClick = { addMember = true }) { Text("Add with password") }
                }
                when (val u = users) {
                    Load.Loading -> Text("Loading members…")
                    is Load.Failed -> Text(u.message, color = MaterialTheme.colorScheme.error)
                    is Load.Ready -> u.data.forEach { MemberRow(it, isSelf = it.id == me?.id, busy = busy, vm = vm) }
                }
                Text("Pending invites", style = MaterialTheme.typography.titleMedium)
                when (val i = invites) {
                    Load.Loading -> Text("Loading…")
                    is Load.Failed -> Text(i.message, color = MaterialTheme.colorScheme.error)
                    is Load.Ready -> if (i.data.isEmpty()) {
                        Text("No pending invites.", color = MaterialTheme.colorScheme.onSurfaceVariant)
                    } else i.data.forEach { inv ->
                        Row(Modifier.fillMaxWidth(), verticalAlignment = Alignment.CenterVertically) {
                            Column(Modifier.weight(1f)) {
                                Text(inv.email ?: "Anyone with the link", style = MaterialTheme.typography.bodyLarge)
                                Text(
                                    "${inv.role} · " + if (inv.expired) "expired" else "expires ${formatDate(inv.expiresAt)}",
                                    style = MaterialTheme.typography.bodySmall,
                                    color = if (inv.expired) MaterialTheme.colorScheme.error else MaterialTheme.colorScheme.onSurfaceVariant,
                                )
                            }
                            IconButton(onClick = { vm.revokeInvite(inv.id) }, enabled = !busy) {
                                Icon(Icons.Default.Delete, "Revoke invite", tint = MaterialTheme.colorScheme.error)
                            }
                        }
                    }
                }
            }
        }

        // --- Connected apps ---
        Section("Connected apps") {
            Text(
                "Agents such as Claude and ChatGPT that you approved through the sign-in page. Disconnecting revokes their access immediately.",
                style = MaterialTheme.typography.bodyMedium,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
            )
            when (val c = connections) {
                Load.Loading -> Text("Loading…")
                is Load.Failed -> Text(c.message, color = MaterialTheme.colorScheme.error)
                is Load.Ready -> if (c.data.isEmpty()) {
                    Text("No apps connected yet.", color = MaterialTheme.colorScheme.onSurfaceVariant)
                } else c.data.forEach { conn ->
                    Row(Modifier.fillMaxWidth(), verticalAlignment = Alignment.CenterVertically) {
                        Column(Modifier.weight(1f)) {
                            Text(conn.clientName, style = MaterialTheme.typography.bodyLarge)
                            val access = if ("write" in conn.scopes) "Read & write" else "Read-only"
                            Text(
                                "$access · connected ${formatDate(conn.createdAt)} · last used ${formatDate(conn.lastUsedAt)}",
                                style = MaterialTheme.typography.bodySmall,
                                color = MaterialTheme.colorScheme.onSurfaceVariant,
                            )
                        }
                        IconButton(onClick = { vm.disconnect(conn.id) }, enabled = !busy) {
                            Icon(Icons.Default.Delete, "Disconnect ${conn.clientName}", tint = MaterialTheme.colorScheme.error)
                        }
                    }
                }
            }
        }

        // --- Connect an agent ---
        Section("Connect an agent") {
            val mcpUrl = "${vm.serverUrl}/mcp"
            Text(
                "Agents connect with OAuth, so there are no keys to copy around. Add this address to your client, sign in when it asks, and choose read-only or read-write.",
                style = MaterialTheme.typography.bodyMedium,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
            )
            CopyRow(mcpUrl) { context.copyToClipboard("Cookira MCP URL", mcpUrl); Toast.makeText(context, "Copied", Toast.LENGTH_SHORT).show() }
            Text("Claude Code", style = MaterialTheme.typography.titleSmall)
            val claudeCode = "claude mcp add --transport http cookira $mcpUrl"
            CopyRow(claudeCode) { context.copyToClipboard("Claude Code command", claudeCode); Toast.makeText(context, "Copied", Toast.LENGTH_SHORT).show() }
            Text(
                "claude.ai, Claude desktop and ChatGPT: add a custom connector with the address above, choose OAuth, and leave client ID and secret empty. " +
                    "Hosted apps need Cookira on public HTTPS.",
                style = MaterialTheme.typography.bodySmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
            )
            if (vm.serverUrl.startsWith("http://")) {
                Text(
                    "This server is on plain http, so hosted apps can't reach it.",
                    style = MaterialTheme.typography.bodySmall,
                    color = MaterialTheme.colorScheme.error,
                )
            }
        }

        // --- About / sign out ---
        Section("About") {
            Text("Server: ${vm.serverUrl}", style = MaterialTheme.typography.bodyMedium)
            Text("Server version: ${serverVersion ?: "…"}", style = MaterialTheme.typography.bodyMedium)
            Text("App version: ${BuildConfig.VERSION_NAME}", style = MaterialTheme.typography.bodyMedium)
            OutlinedButton(onClick = { signOutConfirm = true }) { Text("Sign out") }
        }
    }

    if (clearOffline) {
        AlertDialog(
            onDismissRequest = { clearOffline = false },
            title = { Text("Clear saved recipes?") },
            text = { Text("Removes the offline copy and cached photos from this phone. Your recipes stay on the server and are saved again on the next sync.") },
            confirmButton = { TextButton(onClick = { clearOffline = false; container.clearOfflineLibrary() }) { Text("Clear") } },
            dismissButton = { TextButton(onClick = { clearOffline = false }) { Text("Cancel") } },
        )
    }
    if (signOutConfirm) {
        AlertDialog(
            onDismissRequest = { signOutConfirm = false },
            title = { Text("Sign out?") },
            text = { Text("You'll need to sign in again on this device.") },
            confirmButton = { TextButton(onClick = { signOutConfirm = false; vm.signOut() }) { Text("Sign out") } },
            dismissButton = { TextButton(onClick = { signOutConfirm = false }) { Text("Cancel") } },
        )
    }
    if (addMember) {
        AddMemberDialog(onDismiss = { addMember = false }, onAdd = { n, e, p, r -> vm.addUser(n, e, p, r); addMember = false })
    }
    if (inviting) {
        InviteDialog(onDismiss = { inviting = false }, onCreate = { e, r, d -> vm.invite(e, r, d); inviting = false })
    }
    created?.let { inv ->
        AlertDialog(
            onDismissRequest = vm::dismissInvite,
            title = { Text("Invite created") },
            text = {
                Column(verticalArrangement = Arrangement.spacedBy(8.dp)) {
                    Text("Send this link to the person you're inviting. It is shown only once and works a single time.")
                    Text(inv.link ?: "", style = MaterialTheme.typography.bodySmall)
                }
            },
            confirmButton = {
                TextButton(onClick = { inv.link?.let { context.shareText("Join Cookira", it) } }) { Text("Send") }
            },
            dismissButton = {
                Row {
                    TextButton(onClick = {
                        inv.link?.let { context.copyToClipboard("Invite link", it) }
                        Toast.makeText(context, "Copied", Toast.LENGTH_SHORT).show()
                    }) { Text("Copy") }
                    TextButton(onClick = vm::dismissInvite) { Text("Done") }
                }
            },
        )
    }
}

@Composable
private fun Section(title: String, content: @Composable () -> Unit) {
    Column(verticalArrangement = Arrangement.spacedBy(12.dp)) {
        Text(title, style = MaterialTheme.typography.titleLarge)
        content()
    }
}

@Composable
private fun CopyRow(text: String, onCopy: () -> Unit) {
    Card(colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.surfaceContainerHigh)) {
        Row(Modifier.padding(start = 12.dp), verticalAlignment = Alignment.CenterVertically) {
            Text(text, Modifier.weight(1f).padding(vertical = 10.dp), style = MaterialTheme.typography.bodySmall, fontFamily = androidx.compose.ui.text.font.FontFamily.Monospace)
            IconButton(onClick = onCopy) { Icon(Icons.Default.ContentCopy, "Copy") }
        }
    }
}

@Composable
private fun MemberRow(user: UserDto, isSelf: Boolean, busy: Boolean, vm: SettingsViewModel) {
    var menu by remember { mutableStateOf(false) }
    var reset by remember { mutableStateOf(false) }
    var remove by remember { mutableStateOf(false) }
    Row(Modifier.fillMaxWidth(), verticalAlignment = Alignment.CenterVertically) {
        Column(Modifier.weight(1f)) {
            Text(user.name + if (isSelf) " (you)" else "", style = MaterialTheme.typography.bodyLarge)
            Text(user.email, style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
        }
        Column {
            FilterChip(selected = true, onClick = { menu = true }, label = { Text(user.role) }, enabled = !busy)
            DropdownMenu(expanded = menu, onDismissRequest = { menu = false }) {
                listOf("member", "admin").forEach { role ->
                    DropdownMenuItem(text = { Text(role) }, onClick = { menu = false; if (role != user.role) vm.setRole(user, role) })
                }
                DropdownMenuItem(text = { Text("Reset password") }, onClick = { menu = false; reset = true })
                if (!isSelf) DropdownMenuItem(text = { Text("Remove", color = MaterialTheme.colorScheme.error) }, onClick = { menu = false; remove = true })
            }
        }
    }
    if (reset) {
        var pw by remember { mutableStateOf("") }
        AlertDialog(
            onDismissRequest = { reset = false },
            title = { Text("Reset password for ${user.name}") },
            text = {
                Column(verticalArrangement = Arrangement.spacedBy(8.dp)) {
                    Text("They will be signed out everywhere.")
                    PasswordField("New password (8+ characters)", pw, { pw = it })
                }
            },
            confirmButton = { TextButton(onClick = { vm.resetPassword(user, pw); reset = false }, enabled = pw.length >= 8) { Text("Reset") } },
            dismissButton = { TextButton(onClick = { reset = false }) { Text("Cancel") } },
        )
    }
    if (remove) {
        AlertDialog(
            onDismissRequest = { remove = false },
            title = { Text("Remove ${user.name}?") },
            text = { Text("They will be signed out and lose access.") },
            confirmButton = { TextButton(onClick = { vm.removeUser(user); remove = false }) { Text("Remove", color = MaterialTheme.colorScheme.error) } },
            dismissButton = { TextButton(onClick = { remove = false }) { Text("Cancel") } },
        )
    }
}

@Composable
private fun AddMemberDialog(onDismiss: () -> Unit, onAdd: (String, String, String, String) -> Unit) {
    var name by remember { mutableStateOf("") }
    var email by remember { mutableStateOf("") }
    var password by remember { mutableStateOf("") }
    var role by remember { mutableStateOf("member") }
    AlertDialog(
        onDismissRequest = onDismiss,
        title = { Text("Add a member") },
        text = {
            Column(verticalArrangement = Arrangement.spacedBy(8.dp)) {
                OutlinedTextField(name, { name = it }, label = { Text("Name") }, singleLine = true)
                OutlinedTextField(
                    email, { email = it }, label = { Text("Email") }, singleLine = true,
                    keyboardOptions = androidx.compose.foundation.text.KeyboardOptions(keyboardType = KeyboardType.Email),
                )
                PasswordField("Password (8+ characters)", password, { password = it })
                RoleChips(role) { role = it }
            }
        },
        confirmButton = {
            TextButton(onClick = { onAdd(name, email, password, role) }, enabled = name.isNotBlank() && email.isNotBlank() && password.length >= 8) { Text("Add") }
        },
        dismissButton = { TextButton(onClick = onDismiss) { Text("Cancel") } },
    )
}

@Composable
private fun InviteDialog(onDismiss: () -> Unit, onCreate: (String, String, Int) -> Unit) {
    var email by remember { mutableStateOf("") }
    var role by remember { mutableStateOf("member") }
    var days by remember { mutableIntStateOf(7) }
    AlertDialog(
        onDismissRequest = onDismiss,
        title = { Text("Invite someone") },
        text = {
            Column(verticalArrangement = Arrangement.spacedBy(8.dp)) {
                Text("Creates a single-use link where they choose their own name and password.")
                OutlinedTextField(
                    email, { email = it }, label = { Text("Email (optional)") }, singleLine = true,
                    supportingText = { Text("Lock the invite to this address") },
                    keyboardOptions = androidx.compose.foundation.text.KeyboardOptions(keyboardType = KeyboardType.Email),
                )
                RoleChips(role) { role = it }
                Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                    listOf(1, 7, 30).forEach { d -> FilterChip(selected = days == d, onClick = { days = d }, label = { Text("$d d") }) }
                }
            }
        },
        confirmButton = { TextButton(onClick = { onCreate(email, role, days) }) { Text("Create link") } },
        dismissButton = { TextButton(onClick = onDismiss) { Text("Cancel") } },
    )
}

@Composable
private fun RoleChips(role: String, onChange: (String) -> Unit) {
    Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
        listOf("member", "admin").forEach { r -> FilterChip(selected = role == r, onClick = { onChange(r) }, label = { Text(r) }) }
    }
}
