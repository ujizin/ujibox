package io.ujibox.app.data

import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable

/** The schema of index.json that this app reads. `uji sync` writes it. */
const val INDEX_SCHEMA = 1

/** ujibox/index.json, as `uji sync` writes it (see internal/syncer in the uji repo). */
@Serializable
data class IndexFile(
    val schema: Int = 0,
    @SerialName("updated_at") val updatedAt: String = "",
    val host: String = "",
    val platform: String = "",
    val apps: List<IndexApp> = emptyList(),
)

@Serializable
data class IndexApp(
    val id: String = "",
    val name: String = "",
    val branches: List<IndexBranch> = emptyList(),
)

@Serializable
data class IndexBranch(
    val name: String = "",
    val detached: Boolean = false,
    val builds: List<IndexBuild> = emptyList(),
)

@Serializable
data class IndexBuild(
    val id: String = "",
    @SerialName("package_id") val packageId: String = "",
    val version: IndexVersion = IndexVersion(),
    val sha: String = "",
    val dirty: Boolean = false,
    val pinned: Boolean = false,
    @SerialName("created_at") val createdAt: String = "",
    val size: Long = 0,
    val install: IndexInstall = IndexInstall(),
)

@Serializable
data class IndexVersion(
    val name: String = "",
    val code: String = "",
)

@Serializable
data class IndexInstall(
    val kind: String = "",
    val file: String? = null,
    val command: String? = null,
)
