package io.ujibox.app.data

/** The ujibox folder in the storage of the app. `uji sync` writes into it. */
interface SyncFolder {
    /** The absolute path of the folder. */
    val path: String

    /** Returns the text of index.json, or null if the file does not exist. */
    fun readIndex(): String?

    /** Returns the absolute path of [relative], a path from index.json. */
    fun resolve(relative: String): String = path.trimEnd('/') + "/" + relative
}
