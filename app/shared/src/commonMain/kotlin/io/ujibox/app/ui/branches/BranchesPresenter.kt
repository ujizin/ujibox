package io.ujibox.app.ui.branches

import androidx.compose.runtime.Composable
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.rememberCoroutineScope
import dev.zacsweers.metro.Assisted
import dev.zacsweers.metro.AssistedFactory
import dev.zacsweers.metro.AssistedInject
import io.ujibox.app.data.AppDetail
import io.ujibox.app.data.BuildItem
import io.ujibox.app.data.BuildRepository
import io.ujibox.app.data.SyncState
import io.ujibox.app.install.InstallMessage
import io.ujibox.app.install.Installer
import io.ujibox.app.install.rememberInstallRunner
import io.ujibox.app.presenter.Presenter
import io.ujibox.app.presenter.collectLoaded
import kotlinx.coroutines.launch

data class BranchesModel(
    /** Null if the app is not in the index. */
    val app: AppDetail?,
    /** False until the first read of the database. */
    val loaded: Boolean,
    val state: SyncState,
    val installing: String?,
    val message: InstallMessage?,
    val eventSink: (BranchesEvent) -> Unit,
)

sealed interface BranchesEvent {
    data object Refresh : BranchesEvent

    data object CheckIndex : BranchesEvent

    data class Install(val build: BuildItem) : BranchesEvent

    data object DismissMessage : BranchesEvent
}

/** The builds of one app, grouped by branch. */
@AssistedInject
class BranchesPresenter(
    @Assisted private val appId: String,
    private val repository: BuildRepository,
    private val installer: Installer,
) : Presenter<BranchesModel> {
    @Composable
    override fun present(): BranchesModel {
        val app by collectLoaded(appId) { repository.app(appId) }
        val state by repository.state.collectAsState()
        val runner = rememberInstallRunner(installer, repository)
        val scope = rememberCoroutineScope()
        return BranchesModel(
            app = app?.value,
            loaded = app != null,
            state = state,
            installing = runner.installing,
            message = runner.message,
        ) { event ->
            when (event) {
                BranchesEvent.Refresh -> scope.launch { repository.refresh() }
                BranchesEvent.CheckIndex -> scope.launch { repository.refreshIfChanged() }
                is BranchesEvent.Install -> runner.install(event.build)
                BranchesEvent.DismissMessage -> runner.dismiss()
            }
        }
    }

    @AssistedFactory
    fun interface Factory {
        fun create(appId: String): BranchesPresenter
    }
}
