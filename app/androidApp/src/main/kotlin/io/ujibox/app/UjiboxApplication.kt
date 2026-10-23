package io.ujibox.app

import android.app.Application
import io.ujibox.app.di.AppGraph

class UjiboxApplication : Application() {
    val graph: AppGraph by lazy { createAppGraph(this) }
}
