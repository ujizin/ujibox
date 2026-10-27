package io.ujibox.app.data

import kotlin.test.Test
import kotlin.test.assertEquals

class SearchTest {
    private fun build(id: String, branch: String, version: String, sha: String) = BuildItem(
        id = id, appId = "shop", appName = "Shop", branch = branch, packageId = "com.example.shop",
        versionName = version, versionCode = "1", sha = sha, commitUrl = null, dirty = false,
        pinned = false, createdAt = null, size = 0, install = Install.Unsupported, installed = false,
    )

    private val app = AppDetail(
        id = "shop",
        name = "Shop",
        branches = listOf(
            BranchGroup("main", false, listOf(build("a", "main", "1.0.0", "aaaa1111"), build("b", "main", "1.1.0", "bbbb2222"))),
            BranchGroup("feature/login", false, listOf(build("c", "feature/login", "1.2.0", "cccc3333"))),
        ),
    )

    @Test
    fun blankQueryKeepsEverything() {
        assertEquals(app, app.matching("  "))
    }

    @Test
    fun branchNameKeepsAllItsBuilds() {
        val out = app.matching("LOGIN")
        assertEquals(listOf("feature/login"), out.branches.map { it.name })
        assertEquals(listOf("c"), out.branches.single().builds.map { it.id })
    }

    @Test
    fun versionAndCommitKeepOnlyTheMatchingBuilds() {
        assertEquals(listOf("b"), app.matching("1.1").branches.flatMap { b -> b.builds.map { it.id } })
        assertEquals(listOf("c"), app.matching("cccc").branches.flatMap { b -> b.builds.map { it.id } })
    }

    @Test
    fun noMatchGivesNoBranch() {
        assertEquals(emptyList(), app.matching("nothing").branches)
    }

    @Test
    fun appsMatchByName() {
        val apps = listOf(AppSummary("shop", "Shop", 1, 1, null), AppSummary("pay", "Pay", 1, 1, null))
        assertEquals(listOf("pay"), apps.matching("pa").map { it.id })
        assertEquals(apps, apps.matching(""))
    }
}
