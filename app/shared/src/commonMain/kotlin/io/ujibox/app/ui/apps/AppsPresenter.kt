package io.ujibox.app.ui.apps

import androidx.compose.runtime.Composable
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import dev.zacsweers.metro.Inject
import io.ujibox.app.data.AppSummary
import io.ujibox.app.data.BuildRepository
import io.ujibox.app.data.SyncInfo
import io.ujibox.app.data.SyncState
import io.ujibox.app.presenter.Presenter
import kotlinx.coroutines.launch

data class AppsModel(
    /** Null until the first read of the database. */
    val apps: List<AppSummary>?,
    val sync: SyncInfo?,
    val state: SyncState,
    val folder: String,
    val eventSink: (AppsEvent) -> Unit,
)

sealed interface AppsEvent {
    data object Refresh : AppsEvent

    /** Reads the index only if it changed. */
    data object CheckIndex : AppsEvent
}

@Inject
class AppsPresenter(private val repository: BuildRepository) : Presenter<AppsModel> {
    @Composable
    override fun present(): AppsModel {
        val apps by remember { repository.apps() }.collectAsState(null)
        val sync by remember { repository.sync() }.collectAsState(null)
        val state by repository.state.collectAsState()
        val scope = rememberCoroutineScope()
        return AppsModel(apps, sync, state, repository.folderPath) { event ->
            when (event) {
                AppsEvent.Refresh -> scope.launch { repository.refresh() }
                AppsEvent.CheckIndex -> scope.launch { repository.refreshIfChanged() }
            }
        }
    }
}
