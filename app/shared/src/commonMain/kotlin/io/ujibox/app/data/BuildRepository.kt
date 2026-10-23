package io.ujibox.app.data

import app.cash.sqldelight.coroutines.asFlow
import app.cash.sqldelight.coroutines.mapToList
import app.cash.sqldelight.coroutines.mapToOneOrNull
import dev.zacsweers.metro.AppScope
import dev.zacsweers.metro.Inject
import dev.zacsweers.metro.SingleIn
import io.ujibox.app.db.Build
import io.ujibox.app.db.UjiboxDatabase
import kotlin.time.Clock
import kotlin.time.Instant
import kotlinx.coroutines.CoroutineDispatcher
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.IO
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.combine
import kotlinx.coroutines.flow.map
import kotlinx.coroutines.sync.Mutex
import kotlinx.coroutines.sync.withLock
import kotlinx.coroutines.withContext
import kotlinx.serialization.json.Json

/**
 * Reads index.json into the database. The screens observe the database, so a
 * refresh updates every screen that is open.
 */
@SingleIn(AppScope::class)
@Inject
class BuildRepository(
    private val db: UjiboxDatabase,
    private val folder: SyncFolder,
) {
    private val io: CoroutineDispatcher = Dispatchers.IO
    private val json = Json { ignoreUnknownKeys = true }
    private val lock = Mutex()
    private val _state = MutableStateFlow<SyncState>(SyncState.Loading)

    /** The modification time of the index that [refresh] read last. */
    private var readStamp: Long? = null

    val state: StateFlow<SyncState> = _state.asStateFlow()

    val folderPath: String get() = folder.path

    fun sync(): Flow<SyncInfo?> =
        db.syncQueries.current().asFlow().mapToOneOrNull(io).map { row ->
            row?.let { SyncInfo(parseInstant(it.updated_at), it.host, it.platform) }
        }

    fun apps(): Flow<List<AppSummary>> =
        db.buildQueries.apps().asFlow().mapToList(io).map { rows ->
            rows.map {
                AppSummary(
                    id = it.app_id,
                    name = it.app_name,
                    builds = it.builds.toInt(),
                    branches = it.branches.toInt(),
                    latest = parseInstant(it.latest),
                )
            }
        }

    /** Emits null if the app is not in the index. */
    fun app(appId: String): Flow<AppDetail?> =
        combine(
            db.buildQueries.forApp(appId).asFlow().mapToList(io),
            installedBuilds(),
        ) { rows, installed ->
            val first = rows.firstOrNull() ?: return@combine null
            AppDetail(
                id = first.app_id,
                name = first.app_name,
                branches = rows
                    .groupBy { it.branch }
                    .map { (name, builds) ->
                        BranchGroup(name, builds.first().detached, builds.map { it.toItem(installed) })
                    },
            )
        }

    /** Emits null if the build is not in the index. */
    fun build(buildId: String): Flow<BuildItem?> =
        combine(
            db.buildQueries.byId(buildId).asFlow().mapToOneOrNull(io),
            installedBuilds(),
        ) { row, installed -> row?.toItem(installed) }

    /** Reads index.json again. The result is in [state]. */
    suspend fun refresh() {
        lock.withLock {
            if (_state.value !is SyncState.Ready) _state.value = SyncState.Loading
            _state.value = withContext(io) { load() }
        }
    }

    /** Reads index.json again only if the file changed after the last read. */
    suspend fun refreshIfChanged() {
        val stamp = withContext(io) { runCatching { folder.indexModified() }.getOrNull() }
        if (stamp != readStamp || _state.value is SyncState.Loading) refresh()
    }

    suspend fun recordInstall(build: BuildItem) {
        withContext(io) {
            db.installQueries.record(build.packageId, build.id, Clock.System.now().toEpochMilliseconds())
        }
    }

    private fun load(): SyncState {
        // Take the time before the read: a write during the read then causes one more read.
        readStamp = runCatching { folder.indexModified() }.getOrNull()
        val text = try {
            folder.readIndex()
        } catch (e: Exception) {
            return SyncState.Failed(e.message ?: e.toString())
        }
        if (text == null) {
            // No index means no builds on this device. Old rows would point to files that are gone.
            db.transaction {
                db.buildQueries.deleteAll()
                db.syncQueries.clear()
            }
            return SyncState.NoIndex
        }
        val index = try {
            json.decodeFromString(IndexFile.serializer(), text)
        } catch (e: Exception) {
            return SyncState.Failed(e.message ?: e.toString())
        }
        if (index.schema > INDEX_SCHEMA) return SyncState.NewerSchema(index.schema)
        db.transaction {
            db.buildQueries.deleteAll()
            index.apps.forEachIndexed { appOrder, app ->
                app.branches.forEachIndexed { branchOrder, branch ->
                    branch.builds.forEachIndexed { buildOrder, b ->
                        db.buildQueries.insert(
                            Build(
                                id = b.id,
                                app_id = app.id,
                                app_name = app.name.ifEmpty { app.id },
                                app_order = appOrder.toLong(),
                                branch = branch.name,
                                detached = branch.detached,
                                branch_order = branchOrder.toLong(),
                                build_order = buildOrder.toLong(),
                                package_id = b.packageId,
                                version_name = b.version.name,
                                version_code = b.version.code,
                                sha = b.sha,
                                dirty = b.dirty,
                                pinned = b.pinned,
                                created_at = epochMillis(b.createdAt),
                                size = b.size,
                                install_kind = b.install.kind,
                                install_file = b.install.file,
                                install_command = b.install.command,
                                commit_url = b.commitUrl?.takeIf { it.startsWith("https://") },
                            ),
                        )
                    }
                }
            }
            db.syncQueries.upsert(epochMillis(index.updatedAt), index.host, index.platform)
        }
        return SyncState.Ready
    }

    private fun installedBuilds(): Flow<Set<String>> =
        db.installQueries.all().asFlow().mapToList(io).map { rows -> rows.mapTo(HashSet()) { it.build_id } }

    private fun Build.toItem(installed: Set<String>) = BuildItem(
        id = id,
        appId = app_id,
        appName = app_name,
        branch = branch,
        packageId = package_id,
        versionName = version_name,
        versionCode = version_code,
        sha = sha,
        commitUrl = commit_url,
        dirty = dirty,
        pinned = pinned,
        createdAt = parseInstant(created_at),
        size = size,
        install = install(install_kind, install_file, install_command),
        installed = id in installed,
    )
}

internal fun install(kind: String, file: String?, command: String?): Install = when {
    kind == "apk" && !file.isNullOrEmpty() && file.split('/').none { it == ".." } -> Install.Apk(file)
    kind == "command" && !command.isNullOrEmpty() -> Install.Command(command)
    else -> Install.Unsupported
}

/** Returns 0 for a time that does not parse; [parseInstant] maps 0 back to null. */
internal fun epochMillis(text: String): Long =
    runCatching { Instant.parse(text).toEpochMilliseconds() }.getOrDefault(0L)

internal fun parseInstant(millis: Long): Instant? =
    if (millis <= 0L) null else Instant.fromEpochMilliseconds(millis)
