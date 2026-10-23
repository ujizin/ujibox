package io.ujibox.app.ui

import androidx.compose.runtime.Composable
import androidx.compose.runtime.remember
import androidx.navigation3.runtime.NavKey
import androidx.navigation3.runtime.entryProvider
import androidx.navigation3.runtime.rememberNavBackStack
import androidx.navigation3.runtime.rememberSaveableStateHolderNavEntryDecorator
import androidx.navigation3.ui.NavDisplay
import androidx.savedstate.serialization.SavedStateConfiguration
import io.ujibox.app.di.AppGraph
import io.ujibox.app.presenter.rememberModel
import io.ujibox.app.ui.apps.AppsScreen
import io.ujibox.app.ui.branches.BranchesScreen
import io.ujibox.app.ui.detail.BuildScreen
import kotlinx.serialization.Serializable
import kotlinx.serialization.modules.SerializersModule
import kotlinx.serialization.modules.polymorphic

@Serializable
data object AppsRoute : NavKey

@Serializable
data class BranchesRoute(val appId: String) : NavKey

@Serializable
data class BuildRoute(val buildId: String) : NavKey

// Outside Android, the back stack needs the serializers of the routes to save its state.
private val navConfig = SavedStateConfiguration {
    serializersModule = SerializersModule {
        polymorphic(NavKey::class) {
            subclass(AppsRoute::class, AppsRoute.serializer())
            subclass(BranchesRoute::class, BranchesRoute.serializer())
            subclass(BuildRoute::class, BuildRoute.serializer())
        }
    }
}

/** The root of the UI on each platform. */
@Composable
fun App(graph: AppGraph) {
    UjiboxTheme {
        val backStack = rememberNavBackStack(navConfig, AppsRoute)
        val back: () -> Unit = { if (backStack.size > 1) backStack.removeLastOrNull() }
        NavDisplay(
            backStack = backStack,
            onBack = back,
            entryDecorators = listOf(rememberSaveableStateHolderNavEntryDecorator()),
            entryProvider = entryProvider {
                entry<AppsRoute> {
                    val presenter = remember { graph.appsPresenter }
                    AppsScreen(rememberModel(presenter), onOpenApp = { backStack.add(BranchesRoute(it)) })
                }
                entry<BranchesRoute> { route ->
                    val presenter = remember(route) { graph.branchesPresenter.create(route.appId) }
                    BranchesScreen(
                        rememberModel(presenter),
                        onBack = back,
                        onOpenBuild = { backStack.add(BuildRoute(it)) },
                    )
                }
                entry<BuildRoute> { route ->
                    val presenter = remember(route) { graph.buildPresenter.create(route.buildId) }
                    BuildScreen(rememberModel(presenter), onBack = back)
                }
            },
        )
    }
}
