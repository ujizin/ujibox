package io.ujibox.app.ui

import android.content.ClipData
import android.os.Build
import androidx.compose.ui.platform.ClipEntry

actual fun plainText(text: String): ClipEntry = ClipEntry(ClipData.newPlainText(text, text))

actual val systemConfirmsCopy: Boolean = Build.VERSION.SDK_INT >= 33
