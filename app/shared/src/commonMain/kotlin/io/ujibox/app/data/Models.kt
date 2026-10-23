package io.ujibox.app.data

import kotlin.time.Instant

data class SyncInfo(
    val updatedAt: Instant?,
    val host: String,
    val platform: String,
)

data class AppSummary(
    val id: String,
    val name: String,
    val builds: Int,
    val branches: Int,
    val latest: Instant?,
)

data class AppDetail(
    val id: String,
    val name: String,
    val branches: List<BranchGroup>,
)

/** The builds of one branch. A detached HEAD has the name "@<worktree>". */
data class BranchGroup(
    val name: String,
    val detached: Boolean,
    val builds: List<BuildItem>,
)

data class BuildItem(
    val id: String,
    val appId: String,
    val appName: String,
    val branch: String,
    val packageId: String,
    val versionName: String,
    val versionCode: String,
    val sha: String,
    val dirty: Boolean,
    val pinned: Boolean,
    val createdAt: Instant?,
    val size: Long,
    val install: Install,
    /** True when this app installed this build last for its package. */
    val installed: Boolean,
) {
    val shortSha: String get() = sha.take(8)
}

/** How the app installs a build. */
sealed interface Install {
    /** An APK, relative to the sync folder. The app installs it. */
    data class Apk(val file: String) : Install

    /** iOS apps cannot install other apps. The app copies this command. */
    data class Command(val command: String) : Install

    /** A kind that this version of the app does not know. */
    data object Unsupported : Install
}

/** What the app knows about the sync folder. */
sealed interface SyncState {
    data object Loading : SyncState

    data object Ready : SyncState

    /** The sync folder has no index.json: `uji sync` did not run yet. */
    data object NoIndex : SyncState

    /** A newer uji wrote the index. */
    data class NewerSchema(val schema: Int) : SyncState

    data class Failed(val reason: String) : SyncState
}
