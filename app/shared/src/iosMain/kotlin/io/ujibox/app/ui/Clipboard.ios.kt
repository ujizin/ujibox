package io.ujibox.app.ui

import androidx.compose.ui.ExperimentalComposeUiApi
import androidx.compose.ui.platform.ClipEntry

@OptIn(ExperimentalComposeUiApi::class)
actual fun plainText(text: String): ClipEntry = ClipEntry.withPlainText(text)

actual val systemConfirmsCopy: Boolean = false
