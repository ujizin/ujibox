package io.ujibox.app.install

import androidx.compose.runtime.Composable
import androidx.compose.runtime.Stable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import io.ujibox.app.data.BuildItem
import io.ujibox.app.data.BuildRepository
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.launch

/** The result of an install, for the message that the screen shows. */
data class InstallMessage(val build: BuildItem, val result: InstallResult)

/** Runs one install at a time for a presenter. */
@Stable
class InstallRunner internal constructor(
    private val scope: CoroutineScope,
    private val installer: Installer,
    private val repository: BuildRepository,
) {
    /** The id of the build that is installing, or null. */
    var installing: String? by mutableStateOf(null)
        private set

    var message: InstallMessage? by mutableStateOf(null)
        private set

    fun install(build: BuildItem) {
        if (installing != null) return
        installing = build.id
        message = null
        scope.launch {
            try {
                val result = installer.install(build)
                if (result == InstallResult.Installed) repository.recordInstall(build)
                message = InstallMessage(build, result)
            } finally {
                installing = null
            }
        }
    }

    fun dismiss() {
        message = null
    }
}

@Composable
fun rememberInstallRunner(installer: Installer, repository: BuildRepository): InstallRunner {
    val scope = rememberCoroutineScope()
    return remember(installer, repository) { InstallRunner(scope, installer, repository) }
}
