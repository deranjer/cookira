package com.deranjer.cookira.ui.recipes

import android.Manifest
import android.os.Build
import androidx.activity.compose.BackHandler
import androidx.activity.compose.rememberLauncherForActivityResult
import androidx.activity.result.contract.ActivityResultContracts
import androidx.compose.foundation.layout.ExperimentalLayoutApi
import androidx.compose.foundation.layout.FlowRow
import androidx.compose.material.icons.filled.Timer
import androidx.compose.material3.AssistChip
import androidx.compose.material3.Surface
import androidx.compose.material3.TextButton
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.mutableLongStateOf
import androidx.compose.ui.platform.LocalContext
import com.deranjer.cookira.timers.CookTimers
import kotlinx.coroutines.delay
import androidx.compose.animation.core.animateFloatAsState
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.navigationBarsPadding
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.pager.HorizontalPager
import androidx.compose.foundation.pager.rememberPagerState
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.filled.ArrowBack
import androidx.compose.material.icons.automirrored.filled.ArrowForward
import androidx.compose.material.icons.automirrored.filled.FormatListBulleted
import androidx.compose.material.icons.filled.Check
import androidx.compose.material.icons.filled.Close
import androidx.compose.material3.Button
import androidx.compose.material3.Checkbox
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.LinearProgressIndicator
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.ModalBottomSheet
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Text
import androidx.compose.material3.TopAppBar
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextDecoration
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import com.deranjer.cookira.data.Recipe
import kotlinx.coroutines.launch

/** "Prep: Marinate the chicken." -> ("Prep", "Marinate the chicken."). Only a one-word label is split off. */
internal fun splitPhase(text: String): Pair<String?, String> {
    val m = PHASE.matchEntire(text.trim()) ?: return null to text
    return m.groupValues[1] to m.groupValues[2]
}

private val PHASE = Regex("^(\\p{Lu}\\p{L}{1,11}):\\s+(.+)$", RegexOption.DOT_MATCHES_ALL)

/**
 * Full-screen, one-step-at-a-time view for cooking: big type, big thumb-reach buttons, and under
 * each step just the ingredients it uses (see [IngredientMatcher]). The whole ingredient list is
 * one tap away in a sheet, so you never have to leave the step to find an amount.
 */
@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun CookModeScreen(
    recipe: Recipe,
    checked: Set<String>,
    onToggle: (String) -> Unit,
    onExit: () -> Unit,
) {
    BackHandler(onBack = onExit)
    val steps = recipe.steps
    val used = remember(recipe) { IngredientMatcher.match(recipe) }
    val pager = rememberPagerState { steps.size + 1 } // last page is "all done"
    val scope = rememberCoroutineScope()
    var showAll by remember { mutableStateOf(false) }
    val context = LocalContext.current
    val askNotifications = rememberLauncherForActivityResult(ActivityResultContracts.RequestPermission()) {}
    val startTimer = { label: String, seconds: Int ->
        // The timer runs either way; without notification permission it just can't ring when the app is closed.
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.TIRAMISU) askNotifications.launch(Manifest.permission.POST_NOTIFICATIONS)
        CookTimers.start(context, label, recipe.title, seconds)
    }
    val page = pager.currentPage
    val onDone = page >= steps.size

    fun go(to: Int) = scope.launch { pager.animateScrollToPage(to.coerceIn(0, steps.size)) }

    Scaffold(
        topBar = {
            Column {
                TopAppBar(
                    title = {
                        Text(
                            if (onDone) "Done" else "Step ${page + 1} of ${steps.size}",
                            style = MaterialTheme.typography.titleMedium,
                        )
                    },
                    navigationIcon = { IconButton(onClick = onExit) { Icon(Icons.Default.Close, "Exit cook mode") } },
                    actions = {
                        IconButton(onClick = { showAll = true }) { Icon(Icons.AutoMirrored.Filled.FormatListBulleted, "All ingredients") }
                    },
                )
                val progress by animateFloatAsState(
                    if (steps.isEmpty()) 1f else (page.coerceAtMost(steps.size)).toFloat() / steps.size,
                    label = "progress",
                )
                LinearProgressIndicator(progress = { progress }, modifier = Modifier.fillMaxWidth())
                TimerBar()
            }
        },
        bottomBar = {
            Row(
                Modifier.fillMaxWidth().navigationBarsPadding().padding(horizontal = 16.dp, vertical = 12.dp),
                horizontalArrangement = Arrangement.spacedBy(12.dp),
            ) {
                OutlinedButton(
                    onClick = { go(page - 1) },
                    enabled = page > 0,
                    modifier = Modifier.height(64.dp).weight(1f),
                ) {
                    Icon(Icons.AutoMirrored.Filled.ArrowBack, null)
                    Spacer(Modifier.width(8.dp))
                    Text("Back", fontSize = 18.sp)
                }
                Button(
                    onClick = { if (onDone) onExit() else go(page + 1) },
                    modifier = Modifier.height(64.dp).weight(2f),
                ) {
                    if (onDone) {
                        Icon(Icons.Default.Check, null)
                        Spacer(Modifier.width(8.dp))
                        Text("Finish", fontSize = 20.sp)
                    } else {
                        Text(if (page == steps.size - 1) "Last step done" else "Next step", fontSize = 20.sp)
                        Spacer(Modifier.width(8.dp))
                        Icon(Icons.AutoMirrored.Filled.ArrowForward, null)
                    }
                }
            }
        },
    ) { padding ->
        HorizontalPager(state = pager, modifier = Modifier.padding(padding).fillMaxSize()) { p ->
            if (p >= steps.size) {
                Column(Modifier.fillMaxSize().padding(24.dp), verticalArrangement = Arrangement.Center) {
                    Text("Enjoy!", style = MaterialTheme.typography.displaySmall)
                    if (recipe.notes.isNotBlank()) {
                        Spacer(Modifier.height(16.dp))
                        Text(recipe.notes, style = MaterialTheme.typography.bodyLarge, color = MaterialTheme.colorScheme.onSurfaceVariant)
                    }
                }
            } else {
                StepPage(
                    index = p,
                    recipe = recipe,
                    ingredients = used[p],
                    checked = checked,
                    onToggle = onToggle,
                    onStartTimer = startTimer,
                )
            }
        }
    }

    if (showAll) {
        ModalBottomSheet(onDismissRequest = { showAll = false }) {
            Column(
                Modifier.verticalScroll(rememberScrollState()).padding(horizontal = 16.dp).padding(bottom = 24.dp),
                verticalArrangement = Arrangement.spacedBy(4.dp),
            ) {
                Text("Ingredients", style = MaterialTheme.typography.titleLarge)
                recipe.ingredients.forEachIndexed { gi, group ->
                    if (group.name.isNotBlank()) {
                        Text(group.name, style = MaterialTheme.typography.titleMedium, modifier = Modifier.padding(top = 12.dp))
                    }
                    group.items.forEachIndexed { ii, item ->
                        val key = "$gi:$ii"
                        IngredientRow(item, key in checked) { onToggle(key) }
                    }
                }
            }
        }
    }
}

@Composable
private fun StepPage(
    index: Int,
    recipe: Recipe,
    ingredients: List<IngredientRef>,
    checked: Set<String>,
    onToggle: (String) -> Unit,
    onStartTimer: (label: String, seconds: Int) -> Unit,
) {
    val step = recipe.steps[index]
    val (phase, text) = splitPhase(step.text)
    val durations = remember(step) { DurationParser.find(step.text + " " + step.substeps.joinToString(" ")) }
    Column(
        Modifier.fillMaxSize().verticalScroll(rememberScrollState()).padding(horizontal = 20.dp, vertical = 16.dp),
        verticalArrangement = Arrangement.spacedBy(16.dp),
    ) {
        if (phase != null) {
            Text(phase.uppercase(), style = MaterialTheme.typography.labelLarge, color = MaterialTheme.colorScheme.primary)
        }
        Text(text, fontSize = 26.sp, lineHeight = 36.sp, fontWeight = FontWeight.Medium)

        step.substeps.forEachIndexed { j, sub ->
            Row {
                Text("${'a' + j % 26}.", fontSize = 20.sp, lineHeight = 28.sp, modifier = Modifier.width(32.dp))
                Text(sub, fontSize = 20.sp, lineHeight = 28.sp)
            }
        }

        if (durations.isNotEmpty()) {
            FlowRow(horizontalArrangement = Arrangement.spacedBy(8.dp), verticalArrangement = Arrangement.spacedBy(8.dp)) {
                durations.forEach { d ->
                    AssistChip(
                        onClick = {
                            onStartTimer(if (d.isRange) "${DurationParser.clock(d.seconds.toLong())} (low end of ${d.text})" else d.text, d.seconds)
                        },
                        leadingIcon = { Icon(Icons.Default.Timer, null) },
                        label = { Text(if (d.isRange) "Start timer: ${d.text} (checks at the low end)" else "Start timer: ${d.text}", fontSize = 16.sp) },
                        modifier = Modifier.heightIn(min = 48.dp),
                    )
                }
            }
        }

        if (ingredients.isNotEmpty()) {
            HorizontalDivider()
            Text("You'll need", style = MaterialTheme.typography.titleMedium, color = MaterialTheme.colorScheme.onSurfaceVariant)
            ingredients.forEach { ref ->
                IngredientRow(ref.text, ref.key in checked) { onToggle(ref.key) }
            }
        }
    }
}

/** A tall tap target: wet, floury hands are not precise. */
@Composable
private fun IngredientRow(text: String, done: Boolean, onClick: () -> Unit) {
    Row(
        Modifier.fillMaxWidth().clickable(onClick = onClick).padding(vertical = 4.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Checkbox(
            checked = done,
            onCheckedChange = { onClick() },
            modifier = Modifier.semantics { contentDescription = if (done) "Done" else "Not done" },
        )
        Text(
            text,
            fontSize = 20.sp,
            lineHeight = 28.sp,
            textDecoration = if (done) TextDecoration.LineThrough else null,
            color = if (done) MaterialTheme.colorScheme.onSurfaceVariant else MaterialTheme.colorScheme.onSurface,
        )
    }
}

/** Running timers, ticking once a second, shown on every step so none gets lost. */
@Composable
private fun TimerBar() {
    val context = LocalContext.current
    val timers by CookTimers.timers.collectAsState()
    if (timers.isEmpty()) return
    var now by remember { mutableLongStateOf(System.currentTimeMillis()) }
    LaunchedEffect(Unit) {
        while (true) {
            now = System.currentTimeMillis()
            delay(1000)
        }
    }
    Column(Modifier.fillMaxWidth().padding(horizontal = 12.dp, vertical = 6.dp), verticalArrangement = Arrangement.spacedBy(6.dp)) {
        timers.forEach { t ->
            val left = (t.endAt - now + 999) / 1000
            val done = left <= 0
            Surface(
                shape = MaterialTheme.shapes.large,
                color = if (done) MaterialTheme.colorScheme.errorContainer else MaterialTheme.colorScheme.secondaryContainer,
            ) {
                Row(Modifier.fillMaxWidth().padding(start = 16.dp, end = 4.dp), verticalAlignment = Alignment.CenterVertically) {
                    Icon(Icons.Default.Timer, null)
                    Column(Modifier.weight(1f).padding(horizontal = 12.dp, vertical = 8.dp)) {
                        Text(if (done) "Time's up" else DurationParser.clock(left), fontSize = 24.sp, fontWeight = FontWeight.SemiBold)
                        Text(t.label, style = MaterialTheme.typography.bodyMedium)
                    }
                    TextButton(onClick = { CookTimers.cancel(context, t.id) }) { Text(if (done) "Dismiss" else "Cancel") }
                }
            }
        }
    }
}
