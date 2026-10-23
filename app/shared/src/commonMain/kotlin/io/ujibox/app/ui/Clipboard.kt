package io.ujibox.app.ui

import androidx.compose.ui.platform.ClipEntry

/** A clipboard entry with plain text. Each platform builds it its own way. */
expect fun plainText(text: String): ClipEntry

/** True when the system confirms a copy itself (Android 13 and later), so the app shows no message. */
expect val systemConfirmsCopy: Boolean
