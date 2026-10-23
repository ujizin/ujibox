package io.ujibox.app.install

import io.ujibox.app.data.BuildItem

/** Installs a build on this device, or copies its install command. */
interface Installer {
    suspend fun install(build: BuildItem): InstallResult
}

sealed interface InstallResult {
    data object Installed : InstallResult

    /** The install command is on the pasteboard. */
    data object Copied : InstallResult

    /** The system settings are open, so the user can let the app install apps. */
    data object NeedsPermission : InstallResult

    data object Cancelled : InstallResult

    /** The file of the build is not in the sync folder. */
    data object Missing : InstallResult

    data class Failed(val reason: String) : InstallResult
}
