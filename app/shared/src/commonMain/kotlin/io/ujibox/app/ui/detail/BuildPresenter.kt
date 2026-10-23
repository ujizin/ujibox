package io.ujibox.app.ui.detail

import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.rememberCoroutineScope
import dev.zacsweers.metro.Assisted
import dev.zacsweers.metro.AssistedFactory
import dev.zacsweers.metro.AssistedInject
import io.ujibox.app.data.BuildItem
import io.ujibox.app.data.BuildRepository
import io.ujibox.app.install.InstallMessage
import io.ujibox.app.install.Installer
import io.ujibox.app.install.rememberInstallRunner
import io.ujibox.app.presenter.Presenter
import io.ujibox.app.presenter.collectLoaded
import kotlinx.coroutines.launch

data class BuildModel(
    /** Null if the build is not in the index. */
    val build: BuildItem?,
    /** False until the first read of the database. */
    val loaded: Boolean,
    val installing: Boolean,
    val message: InstallMessage?,
    val eventSink: (BuildEvent) -> Unit,
)

sealed interface BuildEvent {
    data object Install : BuildEvent

    data object CheckIndex : BuildEvent

    data object DismissMessage : BuildEvent
}

@AssistedInject
class BuildPresenter(
    @Assisted private val buildId: String,
    private val repository: BuildRepository,
    private val installer: Installer,
) : Presenter<BuildModel> {
    @Composable
    override fun present(): BuildModel {
        val loaded by collectLoaded(buildId) { repository.build(buildId) }
        val build = loaded?.value
        val runner = rememberInstallRunner(installer, repository)
        val scope = rememberCoroutineScope()
        return BuildModel(
            build = build,
            loaded = loaded != null,
            installing = runner.installing != null,
            message = runner.message,
        ) { event ->
            when (event) {
                BuildEvent.Install -> build?.let(runner::install)
                BuildEvent.DismissMessage -> runner.dismiss()
                BuildEvent.CheckIndex -> scope.launch { repository.refreshIfChanged() }
            }
        }
    }

    @AssistedFactory
    fun interface Factory {
        fun create(buildId: String): BuildPresenter
    }
}
