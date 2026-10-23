package io.ujibox.app.di

import app.cash.sqldelight.db.SqlDriver
import dev.zacsweers.metro.AppScope
import dev.zacsweers.metro.DependencyGraph
import dev.zacsweers.metro.Provides
import dev.zacsweers.metro.SingleIn
import io.ujibox.app.data.SyncFolder
import io.ujibox.app.db.UjiboxDatabase
import io.ujibox.app.install.Installer
import io.ujibox.app.ui.apps.AppsPresenter
import io.ujibox.app.ui.branches.BranchesPresenter
import io.ujibox.app.ui.detail.BuildPresenter

@DependencyGraph(AppScope::class)
interface AppGraph {
    val appsPresenter: AppsPresenter
    val branchesPresenter: BranchesPresenter.Factory
    val buildPresenter: BuildPresenter.Factory

    @Provides
    @SingleIn(AppScope::class)
    fun database(driver: SqlDriver): UjiboxDatabase = UjiboxDatabase(driver)

    /** Each platform gives its own storage, driver and installer. */
    @DependencyGraph.Factory
    fun interface Factory {
        fun create(
            @Provides driver: SqlDriver,
            @Provides folder: SyncFolder,
            @Provides installer: Installer,
        ): AppGraph
    }
}
