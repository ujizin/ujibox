package io.ujibox.app.ui.apps

import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.fillMaxSize
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
import androidx.compose.material3.Text
import androidx.compose.material3.TopAppBar
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.unit.dp
import io.ujibox.app.data.AppSummary
import io.ujibox.app.data.SyncState
import io.ujibox.app.resources.Res
import io.ujibox.app.resources.action_refresh
import io.ujibox.app.resources.app_title
import io.ujibox.app.resources.branches
import io.ujibox.app.resources.builds
import io.ujibox.app.resources.empty_body
import io.ujibox.app.resources.empty_title
import io.ujibox.app.resources.ic_refresh
import io.ujibox.app.resources.search_no_results
import io.ujibox.app.resources.sync_folder
import io.ujibox.app.resources.synced_from
import io.ujibox.app.ui.CenteredMessage
import io.ujibox.app.ui.Loading
import io.ujibox.app.ui.SearchField
import io.ujibox.app.ui.SyncProblem
import io.ujibox.app.ui.WatchIndex
import io.ujibox.app.ui.ago
import org.jetbrains.compose.resources.painterResource
import org.jetbrains.compose.resources.pluralStringResource
import org.jetbrains.compose.resources.stringResource

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun AppsScreen(model: AppsModel, onOpenApp: (String) -> Unit) {
    WatchIndex(onResume = { model.eventSink(AppsEvent.Refresh) }, onTick = { model.eventSink(AppsEvent.CheckIndex) })
    Scaffold(
        topBar = {
            TopAppBar(
                title = { Text(stringResource(Res.string.app_title)) },
                actions = {
                    IconButton(onClick = { model.eventSink(AppsEvent.Refresh) }) {
                        Icon(painterResource(Res.drawable.ic_refresh), stringResource(Res.string.action_refresh))
                    }
                },
            )
        },
    ) { padding ->
        Column(Modifier.fillMaxSize().padding(padding)) {
            SyncProblem(model.state, Modifier.padding(horizontal = 16.dp, vertical = 8.dp))
            val apps = model.apps
            if (model.hasApps) {
                SearchField(
                    query = model.query,
                    onQueryChange = { model.eventSink(AppsEvent.Search(it)) },
                    modifier = Modifier.padding(horizontal = 16.dp, vertical = 8.dp),
                )
            }
            when {
                apps == null || (!model.hasApps && model.state == SyncState.Loading) -> Loading()
                !model.hasApps -> CenteredMessage(
                    title = stringResource(Res.string.empty_title),
                    body = stringResource(Res.string.empty_body),
                    detail = stringResource(Res.string.sync_folder, model.folder),
                )
                apps.isEmpty() -> CenteredMessage(stringResource(Res.string.search_no_results, model.query.trim()))
                else -> AppList(model, apps, onOpenApp)
            }
        }
    }
}

@Composable
private fun AppList(model: AppsModel, apps: List<AppSummary>, onOpenApp: (String) -> Unit) {
    LazyColumn(contentPadding = PaddingValues(bottom = 16.dp)) {
        model.sync?.let { sync ->
            item(key = "sync") {
                Text(
                    stringResource(Res.string.synced_from, sync.host, ago(sync.updatedAt)),
                    style = MaterialTheme.typography.bodySmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                    modifier = Modifier.padding(horizontal = 16.dp, vertical = 8.dp),
                )
            }
        }
        items(apps, key = { it.id }) { app ->
            ListItem(
                headlineContent = { Text(app.name) },
                supportingContent = {
                    Text(
                        pluralStringResource(Res.plurals.branches, app.branches, app.branches) + " · " +
                            pluralStringResource(Res.plurals.builds, app.builds, app.builds),
                    )
                },
                trailingContent = { Text(ago(app.latest), style = MaterialTheme.typography.bodySmall) },
                modifier = Modifier.clickable { onOpenApp(app.id) },
            )
            HorizontalDivider()
        }
    }
}
