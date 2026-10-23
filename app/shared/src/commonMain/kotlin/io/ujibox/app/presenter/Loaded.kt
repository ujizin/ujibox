package io.ujibox.app.presenter

import androidx.compose.runtime.Composable
import androidx.compose.runtime.State
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.remember
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.map

/** A value that arrived. It tells "not read yet" apart from a null value. */
data class Loaded<T>(val value: T)

/** Collects the flow that [create] returns. The state is null until the first value. */
@Composable
fun <T> collectLoaded(vararg keys: Any?, create: () -> Flow<T>): State<Loaded<T>?> =
    remember(*keys) { create().map { Loaded(it) } }.collectAsState(null)
