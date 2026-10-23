package io.ujibox.app.ui

import androidx.compose.runtime.Composable
import io.ujibox.app.data.BuildItem
import io.ujibox.app.resources.Res
import io.ujibox.app.resources.time_days
import io.ujibox.app.resources.time_hours
import io.ujibox.app.resources.time_minutes
import io.ujibox.app.resources.time_now
import io.ujibox.app.resources.version
import kotlin.time.Clock
import kotlin.time.Instant
import org.jetbrains.compose.resources.stringResource

/** "2h ago", like `uji builds`. Returns an empty string for an unknown time. */
@Composable
fun ago(instant: Instant?, now: Instant = Clock.System.now()): String {
    if (instant == null) return ""
    val minutes = (now - instant).inWholeMinutes.coerceAtLeast(0)
    return when {
        minutes < 1 -> stringResource(Res.string.time_now)
        minutes < 60 -> stringResource(Res.string.time_minutes, minutes.toInt())
        minutes < 60 * 24 -> stringResource(Res.string.time_hours, (minutes / 60).toInt())
        else -> stringResource(Res.string.time_days, (minutes / (60 * 24)).toInt())
    }
}

@Composable
fun versionLabel(build: BuildItem): String = stringResource(Res.string.version, build.versionName, build.versionCode)

/** "84.1 MB", in the format of `uji builds`. */
fun formatSize(bytes: Long): String = when {
    bytes >= 1L shl 30 -> "${tenths(bytes, 1L shl 30)} GB"
    bytes >= 1L shl 20 -> "${tenths(bytes, 1L shl 20)} MB"
    bytes >= 1L shl 10 -> "${(bytes + 512) / 1024} KB"
    else -> "$bytes B"
}

private fun tenths(bytes: Long, unit: Long): String {
    val t = (bytes * 10 + unit / 2) / unit
    return "${t / 10}.${t % 10}"
}
