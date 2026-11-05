package com.deranjer.cookira.ui.auth

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.imePadding
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.safeDrawingPadding
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.text.KeyboardActions
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.Visibility
import androidx.compose.material.icons.filled.VisibilityOff
import androidx.compose.material3.Button
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.text.input.ImeAction
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.text.input.PasswordVisualTransformation
import androidx.compose.ui.text.input.VisualTransformation
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.unit.dp
import androidx.lifecycle.ViewModel
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import androidx.lifecycle.viewModelScope
import com.deranjer.cookira.data.CookiraLink
import com.deranjer.cookira.data.InviteInfoDto
import com.deranjer.cookira.data.ServerUrl
import com.deranjer.cookira.di.AppContainer
import com.deranjer.cookira.di.appViewModel
import com.deranjer.cookira.ui.common.CenteredLoading
import com.deranjer.cookira.ui.common.ErrorState
import com.deranjer.cookira.ui.common.Load
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch

enum class ConnectStep { Server, Login, Setup }

data class ConnectUiState(
    val server: String = "",
    val step: ConnectStep = ConnectStep.Server,
    val serverVersion: String = "",
    val busy: Boolean = false,
    val error: String? = null,
)

class ConnectViewModel(private val c: AppContainer) : ViewModel() {
    private val _ui = MutableStateFlow(ConnectUiState())
    val ui: StateFlow<ConnectUiState> = _ui.asStateFlow()

    init {
        viewModelScope.launch {
            c.sessionStore.lastServerUrl()?.let { last -> _ui.update { if (it.server.isEmpty()) it.copy(server = last) else it } }
        }
    }

    fun setServer(v: String) = _ui.update { it.copy(server = v, error = null) }

    fun changeServer() = _ui.update { it.copy(step = ConnectStep.Server, error = null) }

    fun connect() {
        val server = _ui.value.server
        if (!ServerUrl.isValid(server)) {
            _ui.update { it.copy(error = "Enter your Cookira address, e.g. recipes.example.com") }
            return
        }
        viewModelScope.launch {
            _ui.update { it.copy(busy = true, error = null) }
            try {
                val probe = c.authRepository.probe(server)
                val normalized = ServerUrl.normalize(server)
                c.sessionStore.rememberServerUrl(normalized)
                _ui.update {
                    it.copy(
                        server = normalized,
                        serverVersion = probe.version,
                        step = if (probe.setupRequired) ConnectStep.Setup else ConnectStep.Login,
                        busy = false,
                    )
                }
            } catch (e: Exception) {
                _ui.update { it.copy(busy = false, error = e.message ?: "Couldn't connect") }
            }
        }
    }

    fun login(email: String, password: String) = run {
        c.authRepository.login(_ui.value.server, email, password)
    }

    fun setup(name: String, email: String, password: String) = run {
        c.authRepository.setup(_ui.value.server, name, email, password)
    }

    // On success the persisted session flips the app root to the signed-in graph.
    private fun run(block: suspend () -> com.deranjer.cookira.data.UserDto) {
        viewModelScope.launch {
            _ui.update { it.copy(busy = true, error = null) }
            try {
                c.setUser(block())
            } catch (e: Exception) {
                _ui.update { it.copy(error = e.message ?: "Sign-in failed") }
            }
            _ui.update { it.copy(busy = false) }
        }
    }

    /** "Open a link": a pasted share or invite link goes to the nav host like a shared one. */
    fun openLink(text: String): Boolean {
        val link = CookiraLink.parse(text)
        if (link == null) {
            _ui.update { it.copy(error = "That doesn't look like a Cookira share or invite link.") }
            return false
        }
        c.offerLink(link)
        return true
    }
}

@Composable
fun ConnectScreen() {
    val vm = appViewModel { ConnectViewModel(it) }
    val ui by vm.ui.collectAsStateWithLifecycle()
    var email by rememberSaveable { mutableStateOf("") }
    var password by rememberSaveable { mutableStateOf("") }
    var name by rememberSaveable { mutableStateOf("") }
    var link by rememberSaveable { mutableStateOf("") }

    AuthFrame(subtitle = "Your household recipe book") {
        when (ui.step) {
            ConnectStep.Server -> {
                OutlinedTextField(
                    value = ui.server,
                    onValueChange = vm::setServer,
                    label = { Text("Server address") },
                    placeholder = { Text("recipes.example.com") },
                    singleLine = true,
                    modifier = Modifier.fillMaxWidth(),
                    keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.Uri, imeAction = ImeAction.Go),
                    keyboardActions = KeyboardActions(onGo = { vm.connect() }),
                )
                if (ServerUrl.isCleartext(ui.server)) {
                    Text(
                        "This address uses plain http. Your password and recipes will cross the network unencrypted; only do this on a network you trust.",
                        style = MaterialTheme.typography.bodySmall,
                        color = MaterialTheme.colorScheme.error,
                    )
                }
                ErrorText(ui.error)
                Button(onClick = vm::connect, enabled = !ui.busy && ui.server.isNotBlank(), modifier = Modifier.fillMaxWidth()) {
                    BusyLabel(ui.busy, "Continue")
                }
                Text("Have a share or invite link?", style = MaterialTheme.typography.labelLarge, modifier = Modifier.padding(top = 16.dp))
                OutlinedTextField(
                    value = link,
                    onValueChange = { link = it },
                    label = { Text("Paste link") },
                    singleLine = true,
                    modifier = Modifier.fillMaxWidth(),
                    keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.Uri, imeAction = ImeAction.Go),
                    keyboardActions = KeyboardActions(onGo = { if (vm.openLink(link)) link = "" }),
                )
                TextButton(onClick = { if (vm.openLink(link)) link = "" }, enabled = link.isNotBlank()) { Text("Open link") }
            }

            ConnectStep.Login -> {
                Text(ui.server, style = MaterialTheme.typography.bodyMedium, color = MaterialTheme.colorScheme.onSurfaceVariant)
                EmailField(email, { email = it })
                PasswordField("Password", password, { password = it }, imeAction = ImeAction.Go, onGo = { vm.login(email, password) })
                ErrorText(ui.error)
                Button(
                    onClick = { vm.login(email, password) },
                    enabled = !ui.busy && email.isNotBlank() && password.isNotEmpty(),
                    modifier = Modifier.fillMaxWidth(),
                ) { BusyLabel(ui.busy, "Sign in") }
                TextButton(onClick = vm::changeServer) { Text("Use a different server") }
            }

            ConnectStep.Setup -> {
                Text("Welcome! This server is new. Create the first admin account.", style = MaterialTheme.typography.bodyMedium)
                OutlinedTextField(name, { name = it }, label = { Text("Your name") }, singleLine = true, modifier = Modifier.fillMaxWidth())
                EmailField(email, { email = it })
                PasswordField("Password (8+ characters)", password, { password = it })
                ErrorText(ui.error)
                Button(
                    onClick = { vm.setup(name, email, password) },
                    enabled = !ui.busy && name.isNotBlank() && email.isNotBlank() && password.length >= 8,
                    modifier = Modifier.fillMaxWidth(),
                ) { BusyLabel(ui.busy, "Create admin account") }
                TextButton(onClick = vm::changeServer) { Text("Use a different server") }
            }
        }
    }
}

class AcceptInviteViewModel(private val c: AppContainer, val origin: String, private val token: String) : ViewModel() {
    private val _info = MutableStateFlow<Load<InviteInfoDto>>(Load.Loading)
    val info: StateFlow<Load<InviteInfoDto>> = _info.asStateFlow()
    private val _busy = MutableStateFlow(false)
    val busy: StateFlow<Boolean> = _busy.asStateFlow()
    private val _error = MutableStateFlow<String?>(null)
    val error: StateFlow<String?> = _error.asStateFlow()

    init {
        load()
    }

    fun load() {
        viewModelScope.launch {
            _info.value = Load.Loading
            _info.value = try {
                Load.Ready(c.authRepository.inviteInfo(origin, token))
            } catch (e: Exception) {
                Load.Failed(e.message ?: "This invite link is invalid or has expired.")
            }
        }
    }

    fun accept(name: String, email: String?, password: String) {
        viewModelScope.launch {
            _busy.value = true
            _error.value = null
            try {
                c.setUser(c.authRepository.acceptInvite(origin, token, name, email, password))
            } catch (e: Exception) {
                _error.value = e.message ?: "Couldn't accept the invite"
            }
            _busy.value = false
        }
    }
}

@Composable
fun AcceptInviteScreen(origin: String, token: String, onCancel: () -> Unit) {
    val vm = appViewModel(key = "invite-$origin-$token") { AcceptInviteViewModel(it, origin, token) }
    val info by vm.info.collectAsStateWithLifecycle()
    val busy by vm.busy.collectAsStateWithLifecycle()
    val error by vm.error.collectAsStateWithLifecycle()
    var name by rememberSaveable { mutableStateOf("") }
    var email by rememberSaveable { mutableStateOf("") }
    var password by rememberSaveable { mutableStateOf("") }

    when (val s = info) {
        Load.Loading -> CenteredLoading()
        is Load.Failed -> Column {
            ErrorState(s.message, onRetry = vm::load, modifier = Modifier.weight(1f))
            TextButton(onClick = onCancel, modifier = Modifier.padding(16.dp)) { Text("Back") }
        }
        is Load.Ready -> AuthFrame(subtitle = "You're invited to join this household") {
            Text(origin, style = MaterialTheme.typography.bodyMedium, color = MaterialTheme.colorScheme.onSurfaceVariant)
            OutlinedTextField(name, { name = it }, label = { Text("Your name") }, singleLine = true, modifier = Modifier.fillMaxWidth())
            if (s.data.email == null) {
                EmailField(email, { email = it })
            } else {
                Text("Joining as ${s.data.email}", style = MaterialTheme.typography.bodyMedium)
            }
            PasswordField("Choose a password (8+ characters)", password, { password = it })
            ErrorText(error)
            val needEmail = s.data.email == null
            Button(
                onClick = { vm.accept(name, if (needEmail) email else null, password) },
                enabled = !busy && name.isNotBlank() && password.length >= 8 && (!needEmail || email.isNotBlank()),
                modifier = Modifier.fillMaxWidth(),
            ) { BusyLabel(busy, "Join") }
            TextButton(onClick = onCancel) { Text("Cancel") }
        }
    }
}

// --- shared bits ---

@Composable
private fun AuthFrame(subtitle: String, content: @Composable () -> Unit) {
    Column(
        Modifier.fillMaxSize().safeDrawingPadding().imePadding().verticalScroll(rememberScrollState()).padding(24.dp),
        horizontalAlignment = Alignment.CenterHorizontally,
        verticalArrangement = Arrangement.spacedBy(12.dp, Alignment.CenterVertically),
    ) {
        Text("Cookira", style = MaterialTheme.typography.displaySmall, color = MaterialTheme.colorScheme.primary)
        Text(subtitle, style = MaterialTheme.typography.bodyLarge, color = MaterialTheme.colorScheme.onSurfaceVariant, textAlign = TextAlign.Center)
        Column(Modifier.fillMaxWidth().padding(top = 8.dp), verticalArrangement = Arrangement.spacedBy(12.dp)) { content() }
    }
}

@Composable
internal fun ErrorText(message: String?) {
    if (message != null) Text(message, color = MaterialTheme.colorScheme.error, style = MaterialTheme.typography.bodyMedium)
}

@Composable
internal fun BusyLabel(busy: Boolean, label: String) {
    if (busy) CircularProgressIndicator(Modifier.padding(2.dp), strokeWidth = 2.dp) else Text(label)
}

@Composable
internal fun EmailField(value: String, onChange: (String) -> Unit) {
    OutlinedTextField(
        value, onChange, label = { Text("Email") }, singleLine = true, modifier = Modifier.fillMaxWidth(),
        keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.Email, imeAction = ImeAction.Next),
    )
}

@Composable
internal fun PasswordField(
    label: String,
    value: String,
    onChange: (String) -> Unit,
    imeAction: ImeAction = ImeAction.Done,
    onGo: (() -> Unit)? = null,
) {
    var visible by remember { mutableStateOf(false) }
    OutlinedTextField(
        value = value,
        onValueChange = onChange,
        label = { Text(label) },
        singleLine = true,
        modifier = Modifier.fillMaxWidth(),
        visualTransformation = if (visible) VisualTransformation.None else PasswordVisualTransformation(),
        keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.Password, imeAction = imeAction),
        keyboardActions = KeyboardActions(onGo = { onGo?.invoke() }, onDone = { onGo?.invoke() }),
        trailingIcon = {
            IconButton(onClick = { visible = !visible }) {
                Icon(if (visible) Icons.Default.VisibilityOff else Icons.Default.Visibility, if (visible) "Hide password" else "Show password")
            }
        },
    )
}
