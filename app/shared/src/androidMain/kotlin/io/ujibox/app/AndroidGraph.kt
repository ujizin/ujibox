package io.ujibox.app

import android.app.Application
import android.content.Context
import app.cash.sqldelight.driver.android.AndroidSqliteDriver
import dev.zacsweers.metro.createGraphFactory
import io.ujibox.app.db.UjiboxDatabase
import io.ujibox.app.di.AppGraph
import java.io.File

/** Creates the graph of the app. Call it once, from the Application. */
fun createAppGraph(app: Application): AppGraph {
    val folder = AndroidSyncFolder(app)
    return createGraphFactory<AppGraph.Factory>().create(
        driver = AndroidSqliteDriver(UjiboxDatabase.Schema, app, "ujibox.db"),
        folder = folder,
        installer = AndroidInstaller(app, folder, ResumedActivity(app)),
    )
}

/** /sdcard/Android/data/<package>/files/ujibox: adb can write there without root. */
internal class AndroidSyncFolder(context: Context) : io.ujibox.app.data.SyncFolder {
    // getExternalFilesDir also creates the folder, so `uji sync` finds it.
    private val dir = File(context.getExternalFilesDir(null) ?: context.filesDir, "ujibox")

    override val path: String get() = dir.absolutePath

    override fun readIndex(): String? {
        val file = File(dir, "index.json")
        return if (file.isFile) file.readText() else null
    }
}
