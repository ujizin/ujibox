package io.ujibox.app.ui

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.SnackbarHostState
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.unit.dp
import io.ujibox.app.data.BuildItem
import io.ujibox.app.data.Install
import io.ujibox.app.data.SyncState
import io.ujibox.app.install.InstallMessage
import io.ujibox.app.install.InstallResult
import io.ujibox.app.resources.Res
import io.ujibox.app.resources.action_copy_command
import io.ujibox.app.resources.action_install
import io.ujibox.app.resources.badge_dirty
import io.ujibox.app.resources.badge_installed
import io.ujibox.app.resources.badge_pinned
import io.ujibox.app.resources.ic_copy
import io.ujibox.app.resources.ic_install
import io.ujibox.app.resources.install_cancelled
import io.ujibox.app.resources.install_copied
import io.ujibox.app.resources.install_done
import io.ujibox.app.resources.install_failed
import io.ujibox.app.resources.install_missing
import io.ujibox.app.resources.install_permission
import io.ujibox.app.resources.ios_warning
import io.ujibox.app.resources.state_failed
import io.ujibox.app.resources.state_newer_schema
import io.ujibox.app.resources.version
import org.jetbrains.compose.resources.getString
import org.jetbrains.compose.resources.painterResource
import org.jetbrains.compose.resources.stringResource

/** The icon button of a build row: install the APK, or copy the command. */
@Composable
fun InstallIconButton(build: BuildItem, installing: Boolean, onInstall: () -> Unit) {
    if (installing) {
        CircularProgressIndicator(modifier = Modifier.padding(12.dp).size(24.dp), strokeWidth = 2.dp)
        return
    }
    val command = build.install is Install.Command
    IconButton(onClick = onInstall, enabled = build.install !is Install.Unsupported) {
        Icon(
            painter = painterResource(if (command) Res.drawable.ic_copy else Res.drawable.ic_install),
            contentDescription = stringResource(if (command) Res.string.action_copy_command else Res.string.action_install),
        )
    }
}

@Composable
fun Badges(build: BuildItem) {
    Row(horizontalArrangement = Arrangement.spacedBy(6.dp)) {
        if (build.installed) Badge(stringResource(Res.string.badge_installed), MaterialTheme.colorScheme.primary)
        if (build.pinned) Badge(stringResource(Res.string.badge_pinned), MaterialTheme.colorScheme.tertiary)
        if (build.dirty) Badge(stringResource(Res.string.badge_dirty), MaterialTheme.colorScheme.error)
    }
}

@Composable
private fun Badge(text: String, color: Color) {
    Surface(color = color.copy(alpha = 0.12f), contentColor = color, shape = MaterialTheme.shapes.small) {
        Text(text, style = MaterialTheme.typography.labelSmall, modifier = Modifier.padding(horizontal = 6.dp, vertical = 2.dp))
    }
}

/** A line for a problem with the sync folder. Shows nothing when there is no problem. */
@Composable
fun SyncProblem(state: SyncState, modifier: Modifier = Modifier) {
    val text = when (state) {
        is SyncState.Failed -> stringResource(Res.string.state_failed, state.reason)
        is SyncState.NewerSchema -> stringResource(Res.string.state_newer_schema, state.schema)
        else -> return
    }
    Notice(text, MaterialTheme.colorScheme.errorContainer, MaterialTheme.colorScheme.onErrorContainer, modifier)
}

/** iOS apps cannot install apps. Tell the user what to do with the command. */
@Composable
fun IosWarning(modifier: Modifier = Modifier) {
    Notice(
        stringResource(Res.string.ios_warning),
        MaterialTheme.colorScheme.tertiaryContainer,
        MaterialTheme.colorScheme.onTertiaryContainer,
        modifier,
    )
}

@Composable
private fun Notice(text: String, container: Color, content: Color, modifier: Modifier) {
    Surface(color = container, contentColor = content, shape = MaterialTheme.shapes.medium, modifier = modifier.fillMaxWidth()) {
        Text(text, style = MaterialTheme.typography.bodyMedium, modifier = Modifier.padding(12.dp))
    }
}

@Composable
fun CenteredMessage(title: String, body: String? = null, detail: String? = null) {
    Column(
        modifier = Modifier.fillMaxSize().padding(32.dp),
        verticalArrangement = Arrangement.Center,
        horizontalAlignment = Alignment.CenterHorizontally,
    ) {
        Text(title, style = MaterialTheme.typography.titleMedium, textAlign = TextAlign.Center)
        if (body != null) {
            Spacer(Modifier.height(8.dp))
            Text(body, style = MaterialTheme.typography.bodyMedium, textAlign = TextAlign.Center)
        }
        if (detail != null) {
            Spacer(Modifier.height(16.dp))
            Text(
                detail,
                style = MaterialTheme.typography.bodySmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
                textAlign = TextAlign.Center,
            )
        }
    }
}

@Composable
fun Loading() {
    Column(Modifier.fillMaxSize(), verticalArrangement = Arrangement.Center, horizontalAlignment = Alignment.CenterHorizontally) {
        CircularProgressIndicator()
    }
}

/** Shows [message] in the snackbar, then calls [onShown]. */
@Composable
fun InstallMessageEffect(message: InstallMessage?, snackbar: SnackbarHostState, onShown: () -> Unit) {
    LaunchedEffect(message) {
        if (message == null) return@LaunchedEffect
        val text = when (val r = message.result) {
            InstallResult.Installed -> getString(
                Res.string.install_done,
                getString(Res.string.version, message.build.versionName, message.build.versionCode),
            )
            InstallResult.Copied -> getString(Res.string.install_copied)
            InstallResult.NeedsPermission -> getString(Res.string.install_permission)
            InstallResult.Cancelled -> getString(Res.string.install_cancelled)
            InstallResult.Missing -> getString(Res.string.install_missing)
            is InstallResult.Failed -> getString(Res.string.install_failed, r.reason)
        }
        snackbar.showSnackbar(text)
        onShown()
    }
}
