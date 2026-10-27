package io.ujibox.app.data

/** The apps whose name contains [query]. A blank query keeps every app. */
fun List<AppSummary>.matching(query: String): List<AppSummary> {
    val q = query.trim()
    if (q.isEmpty()) return this
    return filter { it.name.contains(q, ignoreCase = true) }
}

/**
 * The branches whose name contains [query], with all their builds, and the
 * builds of other branches whose version, package or commit matches.
 * A blank query keeps every build.
 */
fun AppDetail.matching(query: String): AppDetail {
    val q = query.trim()
    if (q.isEmpty()) return this
    val groups = branches.mapNotNull { branch ->
        if (branch.name.contains(q, ignoreCase = true)) return@mapNotNull branch
        val builds = branch.builds.filter { it.matches(q) }
        if (builds.isEmpty()) null else branch.copy(builds = builds)
    }
    return copy(branches = groups)
}

private fun BuildItem.matches(q: String): Boolean =
    versionName.contains(q, ignoreCase = true) ||
        versionCode.contains(q, ignoreCase = true) ||
        packageId.contains(q, ignoreCase = true) ||
        sha.startsWith(q, ignoreCase = true)
