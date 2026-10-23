package io.ujibox.app.presenter

import androidx.compose.runtime.Composable
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import app.cash.molecule.RecompositionMode
import app.cash.molecule.launchMolecule

/**
 * A presenter turns data into the model of one screen. The model carries an
 * event sink for the actions of the user. Molecule runs [present] outside of
 * the UI composition, so a presenter has no UI dependency.
 */
fun interface Presenter<Model> {
    @Composable
    fun present(): Model
}

/** Runs [presenter] while the caller is in the composition. */
@Composable
fun <Model> rememberModel(presenter: Presenter<Model>): Model {
    val scope = rememberCoroutineScope()
    val models = remember(presenter) {
        scope.launchMolecule(RecompositionMode.Immediate) { presenter.present() }
    }
    return models.collectAsState().value
}
