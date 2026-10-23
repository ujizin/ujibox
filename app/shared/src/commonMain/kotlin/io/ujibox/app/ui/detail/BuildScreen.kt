package io.ujibox.app.ui.detail

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.text.selection.SelectionContainer
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.Button
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Scaffold
import androidx.compose.material3.SnackbarHost
import androidx.compose.material3.SnackbarHostState
import androidx.compose.material3.Text
import androidx.compose.material3.TopAppBar
import androidx.compose.runtime.Composable
import androidx.compose.runtime.remember
import androidx.compose.ui.Modifier
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.unit.dp
import io.ujibox.app.data.BuildItem
import io.ujibox.app.data.Install
import io.ujibox.app.resources.Res
import io.ujibox.app.resources.action_back
import io.ujibox.app.resources.action_copy_command
import io.ujibox.app.resources.action_install
import io.ujibox.app.resources.build_gone
import io.ujibox.app.resources.commit_dirty
import io.ujibox.app.resources.ic_back
import io.ujibox.app.resources.label_app
import io.ujibox.app.resources.label_branch
import io.ujibox.app.resources.label_built
import io.ujibox.app.resources.label_command
import io.ujibox.app.resources.label_commit
import io.ujibox.app.resources.label_package
import io.ujibox.app.resources.label_size
import io.ujibox.app.resources.label_version
import io.ujibox.app.resources.unsupported_install
import io.ujibox.app.ui.Badges
import io.ujibox.app.ui.CenteredMessage
import io.ujibox.app.ui.InstallMessageEffect
import io.ujibox.app.ui.IosWarning
import io.ujibox.app.ui.Loading
import io.ujibox.app.ui.ago
import io.ujibox.app.ui.formatSize
import io.ujibox.app.ui.versionLabel
import org.jetbrains.compose.resources.StringResource
import org.jetbrains.compose.resources.painterResource
import org.jetbrains.compose.resources.stringResource

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun BuildScreen(model: BuildModel, onBack: () -> Unit) {
    val snackbar = remember { SnackbarHostState() }
    InstallMessageEffect(model.message, snackbar) { model.eventSink(BuildEvent.DismissMessage) }
    Scaffold(
        topBar = {
            TopAppBar(
                title = { Text(model.build?.let { versionLabel(it) }.orEmpty()) },
                navigationIcon = {
                    IconButton(onClick = onBack) {
                        Icon(painterResource(Res.drawable.ic_back), stringResource(Res.string.action_back))
                    }
                },
            )
        },
        snackbarHost = { SnackbarHost(snackbar) },
    ) { padding ->
        val build = model.build
        Column(Modifier.fillMaxSize().padding(padding)) {
            when {
                !model.loaded -> Loading()
                build == null -> CenteredMessage(stringResource(Res.string.build_gone))
                else -> Details(build, model)
            }
        }
    }
}

@Composable
private fun Details(build: BuildItem, model: BuildModel) {
    Column(
        modifier = Modifier.fillMaxSize().verticalScroll(rememberScrollState()).padding(16.dp),
        verticalArrangement = Arrangement.spacedBy(16.dp),
    ) {
        Badges(build)
        Field(Res.string.label_app, build.appName)
        Field(Res.string.label_branch, build.branch)
        Field(Res.string.label_package, build.packageId)
        Field(Res.string.label_version, versionLabel(build))
        Field(
            Res.string.label_commit,
            if (build.dirty) stringResource(Res.string.commit_dirty, build.sha) else build.sha,
        )
        Field(Res.string.label_built, ago(build.createdAt))
        Field(Res.string.label_size, formatSize(build.size))

        when (val install = build.install) {
            is Install.Apk -> InstallButton(Res.string.action_install, model)
            is Install.Command -> {
                IosWarning()
                Field(Res.string.label_command, install.command, monospace = true)
                InstallButton(Res.string.action_copy_command, model)
            }
            Install.Unsupported -> Text(stringResource(Res.string.unsupported_install), color = MaterialTheme.colorScheme.error)
        }
    }
}

@Composable
private fun InstallButton(label: StringResource, model: BuildModel) {
    Button(
        onClick = { model.eventSink(BuildEvent.Install) },
        enabled = !model.installing,
        modifier = Modifier.fillMaxWidth(),
    ) {
        if (model.installing) {
            CircularProgressIndicator(Modifier.size(18.dp), strokeWidth = 2.dp)
        } else {
            Text(stringResource(label))
        }
    }
}

@Composable
private fun Field(label: StringResource, value: String, monospace: Boolean = false) {
    if (value.isEmpty()) return
    Column {
        Text(stringResource(label), style = MaterialTheme.typography.labelMedium, color = MaterialTheme.colorScheme.onSurfaceVariant)
        SelectionContainer {
            Text(
                value,
                style = MaterialTheme.typography.bodyLarge,
                fontFamily = if (monospace) FontFamily.Monospace else null,
            )
        }
    }
}
