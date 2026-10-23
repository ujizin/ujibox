package io.ujibox.app.ui.branches

import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.ListItem
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Scaffold
import androidx.compose.material3.SnackbarHost
import androidx.compose.material3.SnackbarHostState
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.material3.TopAppBar
import androidx.compose.runtime.Composable
import androidx.compose.runtime.remember
import androidx.compose.ui.Modifier
import androidx.compose.ui.unit.dp
import io.ujibox.app.data.AppDetail
import io.ujibox.app.data.Install
import io.ujibox.app.resources.Res
import io.ujibox.app.resources.action_back
import io.ujibox.app.resources.app_gone
import io.ujibox.app.resources.ic_back
import io.ujibox.app.ui.Badges
import io.ujibox.app.ui.CenteredMessage
import io.ujibox.app.ui.InstallIconButton
import io.ujibox.app.ui.InstallMessageEffect
import io.ujibox.app.ui.IosWarning
import io.ujibox.app.ui.Loading
import io.ujibox.app.ui.SyncProblem
import io.ujibox.app.ui.WatchIndex
import io.ujibox.app.ui.ago
import io.ujibox.app.ui.formatSize
import io.ujibox.app.ui.versionLabel
import org.jetbrains.compose.resources.painterResource
import org.jetbrains.compose.resources.stringResource

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun BranchesScreen(model: BranchesModel, onBack: () -> Unit, onOpenBuild: (String) -> Unit) {
    WatchIndex(onResume = { model.eventSink(BranchesEvent.Refresh) }, onTick = { model.eventSink(BranchesEvent.CheckIndex) })
    val snackbar = remember { SnackbarHostState() }
    InstallMessageEffect(model.message, snackbar) { model.eventSink(BranchesEvent.DismissMessage) }
    Scaffold(
        topBar = {
            TopAppBar(
                title = { Text(model.app?.name.orEmpty()) },
                navigationIcon = {
                    IconButton(onClick = onBack) {
                        Icon(painterResource(Res.drawable.ic_back), stringResource(Res.string.action_back))
                    }
                },
            )
        },
        snackbarHost = { SnackbarHost(snackbar) },
    ) { padding ->
        Column(Modifier.fillMaxSize().padding(padding)) {
            SyncProblem(model.state, Modifier.padding(horizontal = 16.dp, vertical = 8.dp))
            val app = model.app
            when {
                !model.loaded -> Loading()
                app == null -> CenteredMessage(stringResource(Res.string.app_gone))
                else -> BuildList(app, model, onOpenBuild)
            }
        }
    }
}

@Composable
private fun BuildList(app: AppDetail, model: BranchesModel, onOpenBuild: (String) -> Unit) {
    val commands = app.branches.any { branch -> branch.builds.any { it.install is Install.Command } }
    LazyColumn(contentPadding = PaddingValues(bottom = 16.dp)) {
        if (commands) {
            item(key = "ios-warning") { IosWarning(Modifier.padding(16.dp)) }
        }
        app.branches.forEach { branch ->
            stickyHeader(key = "branch:" + branch.name) {
                Surface(color = MaterialTheme.colorScheme.surfaceContainer, modifier = Modifier.fillMaxWidth()) {
                    Text(
                        branch.name,
                        style = MaterialTheme.typography.titleSmall,
                        modifier = Modifier.padding(horizontal = 16.dp, vertical = 8.dp),
                    )
                }
            }
            items(branch.builds, key = { it.id }) { build ->
                ListItem(
                    headlineContent = { Text(versionLabel(build)) },
                    supportingContent = {
                        Column {
                            Text(listOf(build.shortSha, ago(build.createdAt), formatSize(build.size)).filter { it.isNotEmpty() }.joinToString(" · "))
                            Badges(build)
                        }
                    },
                    trailingContent = {
                        InstallIconButton(build, installing = model.installing == build.id) {
                            model.eventSink(BranchesEvent.Install(build))
                        }
                    },
                    modifier = Modifier.clickable { onOpenBuild(build.id) },
                )
                HorizontalDivider()
            }
        }
    }
}
