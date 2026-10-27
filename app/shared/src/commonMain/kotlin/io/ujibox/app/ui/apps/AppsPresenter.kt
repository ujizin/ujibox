package io.ujibox.app.ui.apps

import androidx.compose.runtime.Composable
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import dev.zacsweers.metro.Inject
import io.ujibox.app.data.AppSummary
import io.ujibox.app.data.BuildRepository
import io.ujibox.app.data.SyncInfo
import io.ujibox.app.data.SyncState
import io.ujibox.app.data.matching
import io.ujibox.app.presenter.Presenter
import kotlinx.coroutines.launch

data class AppsModel(
    /** Null until the first read of the database. */
    val apps: List<AppSummary>?,
    /** False when the database has no app, whatever the query. */
    val hasApps: Boolean,
    val query: String,
    val sync: SyncInfo?,
    val state: SyncState,
    val folder: String,
    val eventSink: (AppsEvent) -> Unit,
)

sealed interface AppsEvent {
    data object Refresh : AppsEvent

    /** Reads the index only if it changed. */
    data object CheckIndex : AppsEvent

    data class Search(val query: String) : AppsEvent
}

@Inject
class AppsPresenter(private val repository: BuildRepository) : Presenter<AppsModel> {
    @Composable
    override fun present(): AppsModel {
        val apps by remember { repository.apps() }.collectAsState(null)
        val sync by remember { repository.sync() }.collectAsState(null)
        val state by repository.state.collectAsState()
        var query by remember { mutableStateOf("") }
        val scope = rememberCoroutineScope()
        return AppsModel(
            apps = apps?.matching(query),
            hasApps = !apps.isNullOrEmpty(),
            query = query,
            sync = sync,
            state = state,
            folder = repository.folderPath,
        ) { event ->
            when (event) {
                AppsEvent.Refresh -> scope.launch { repository.refresh() }
                AppsEvent.CheckIndex -> scope.launch { repository.refreshIfChanged() }
                is AppsEvent.Search -> query = event.query
            }
        }
    }
}
