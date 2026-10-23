package io.ujibox.app

import androidx.compose.ui.window.ComposeUIViewController
import app.cash.sqldelight.driver.native.NativeSqliteDriver
import dev.zacsweers.metro.createGraphFactory
import io.ujibox.app.data.BuildItem
import io.ujibox.app.data.Install
import io.ujibox.app.data.SyncFolder
import io.ujibox.app.db.UjiboxDatabase
import io.ujibox.app.di.AppGraph
import io.ujibox.app.install.InstallResult
import io.ujibox.app.install.Installer
import io.ujibox.app.ui.App
import kotlinx.cinterop.ExperimentalForeignApi
import platform.Foundation.NSDocumentDirectory
import platform.Foundation.NSFileManager
import platform.Foundation.NSString
import platform.Foundation.NSURL
import platform.Foundation.NSUTF8StringEncoding
import platform.Foundation.NSUserDomainMask
import platform.Foundation.stringWithContentsOfFile
import platform.UIKit.UIPasteboard
import platform.UIKit.UIViewController

private val graph: AppGraph by lazy {
    createGraphFactory<AppGraph.Factory>().create(
        driver = NativeSqliteDriver(UjiboxDatabase.Schema, "ujibox.db"),
        folder = IosSyncFolder(),
        installer = IosInstaller(),
    )
}

/** The entry point for the Swift app. */
@Suppress("unused", "FunctionName")
fun MainViewController(): UIViewController = ComposeUIViewController { App(graph) }

/** <data container>/Documents/ujibox. `uji sync` writes there on a simulator. */
private class IosSyncFolder : SyncFolder {
    override val path: String by lazy {
        val documents = NSFileManager.defaultManager
            .URLsForDirectory(NSDocumentDirectory, NSUserDomainMask)
            .firstOrNull() as? NSURL
        (documents?.path ?: NSHomeDirectoryPath()) + "/ujibox"
    }

    @OptIn(ExperimentalForeignApi::class)
    override fun readIndex(): String? {
        val file = "$path/index.json"
        if (!NSFileManager.defaultManager.fileExistsAtPath(file)) return null
        return NSString.stringWithContentsOfFile(file, NSUTF8StringEncoding, null)
            ?: throw IllegalStateException("cannot read $file")
    }
}

private fun NSHomeDirectoryPath(): String = platform.Foundation.NSHomeDirectory() + "/Documents"

/** iOS apps cannot install other apps, so the app copies the simctl command. */
private class IosInstaller : Installer {
    override suspend fun install(build: BuildItem): InstallResult {
        val command = (build.install as? Install.Command)?.command
            ?: return InstallResult.Failed("unsupported install kind")
        UIPasteboard.generalPasteboard.string = command
        return InstallResult.Copied
    }
}
