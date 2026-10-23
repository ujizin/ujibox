package io.ujibox.app.data

import app.cash.sqldelight.driver.jdbc.sqlite.JdbcSqliteDriver
import io.ujibox.app.db.UjiboxDatabase
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertIs
import kotlin.test.assertNull
import kotlinx.coroutines.flow.first
import kotlinx.coroutines.test.runTest

class BuildRepositoryTest {
    private class FakeFolder(var index: String?) : SyncFolder {
        override val path = "/sync/ujibox"

        override fun readIndex() = index
    }

    private val folder = FakeFolder(null)
    private val repository = BuildRepository(
        UjiboxDatabase(JdbcSqliteDriver(JdbcSqliteDriver.IN_MEMORY).also { UjiboxDatabase.Schema.create(it) }),
        folder,
    )

    // The shape that `uji sync` writes; see internal/syncer in the uji repo.
    private val index = """
        {
          "schema": 1,
          "updated_at": "2026-10-22T17:18:25.123456Z",
          "host": "my-mac",
          "platform": "android",
          "some_future_field": true,
          "apps": [
            {
              "id": "shop",
              "name": "Shop",
              "branches": [
                {
                  "name": "feature/login",
                  "detached": false,
                  "builds": [
                    {
                      "id": "B2", "package_id": "com.example.shop.dev",
                      "version": { "name": "1.4.0", "code": "140" },
                      "sha": "3f9a01c2aa", "dirty": true, "pinned": false,
                      "created_at": "2026-10-22T14:30:12Z", "size": 84123456,
                      "manifest": "builds/B2/manifest.json",
                      "install": { "kind": "apk", "file": "builds/B2/app.apk" }
                    },
                    {
                      "id": "B1", "package_id": "com.example.shop.dev",
                      "version": { "name": "1.4.0", "code": "139" },
                      "sha": "77d0e4b1", "created_at": "2026-10-21T14:30:12Z", "size": 1,
                      "install": { "kind": "apk", "file": "../outside.apk" }
                    }
                  ]
                },
                {
                  "name": "@wt", "detached": true,
                  "builds": [
                    {
                      "id": "B0", "package_id": "com.example.shop",
                      "version": { "name": "1.3.2", "code": "132" },
                      "created_at": "not a time",
                      "install": { "kind": "command", "command": "xcrun simctl install X '/a b.app'" }
                    }
                  ]
                }
              ]
            },
            { "id": "notes", "name": "", "branches": [ { "name": "main", "builds": [ { "id": "N1", "install": { "kind": "zip" } } ] } ] }
          ]
        }
    """.trimIndent()

    @Test
    fun importsTheIndexInOrder() = runTest {
        folder.index = index
        repository.refresh()

        assertEquals(SyncState.Ready, repository.state.value)
        val apps = repository.apps().first()
        assertEquals(listOf("shop", "notes"), apps.map { it.id })
        assertEquals("notes", apps[1].name, "an empty name falls back to the id")
        assertEquals(2, apps[0].branches)
        assertEquals(3, apps[0].builds)

        val shop = repository.app("shop").first()!!
        assertEquals(listOf("feature/login", "@wt"), shop.branches.map { it.name })
        assertEquals(listOf("B2", "B1"), shop.branches[0].builds.map { it.id })
        assertEquals(true, shop.branches[1].detached)

        val b2 = shop.branches[0].builds[0]
        assertEquals(Install.Apk("builds/B2/app.apk"), b2.install)
        assertEquals(true, b2.dirty)
        assertEquals("3f9a01c2", b2.shortSha)
        assertEquals(Install.Unsupported, shop.branches[0].builds[1].install, "a path out of the folder is refused")
        assertIs<Install.Command>(shop.branches[1].builds[0].install)
        assertNull(shop.branches[1].builds[0].createdAt, "a bad time has no value")
        assertEquals(Install.Unsupported, repository.build("N1").first()!!.install)

        val sync = repository.sync().first()!!
        assertEquals("my-mac", sync.host)
    }

    @Test
    fun recordsTheInstalledBuild() = runTest {
        folder.index = index
        repository.refresh()
        val b2 = repository.build("B2").first()!!
        repository.recordInstall(b2)
        assertEquals(true, repository.build("B2").first()!!.installed)
        assertEquals(false, repository.build("B1").first()!!.installed)
    }

    @Test
    fun noIndexClearsTheBuilds() = runTest {
        folder.index = index
        repository.refresh()
        folder.index = null
        repository.refresh()
        assertEquals(SyncState.NoIndex, repository.state.value)
        assertEquals(emptyList(), repository.apps().first())
        assertNull(repository.sync().first())
    }

    @Test
    fun aBadIndexKeepsTheLastBuilds() = runTest {
        folder.index = index
        repository.refresh()
        folder.index = "{ not json"
        repository.refresh()
        assertIs<SyncState.Failed>(repository.state.value)
        assertEquals(2, repository.apps().first().size)

        folder.index = """{ "schema": 2, "apps": [] }"""
        repository.refresh()
        assertEquals(SyncState.NewerSchema(2), repository.state.value)
        assertEquals(2, repository.apps().first().size)
    }
}
