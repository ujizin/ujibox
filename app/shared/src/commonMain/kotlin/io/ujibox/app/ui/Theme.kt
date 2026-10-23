package io.ujibox.app.ui

import androidx.compose.foundation.isSystemInDarkTheme
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.darkColorScheme
import androidx.compose.material3.lightColorScheme
import androidx.compose.runtime.Composable
import androidx.compose.ui.graphics.Color

// The Material 3 "fidelity" scheme of the seed #D92F2F: the primary container
// keeps the seed color, so the red stays vivid.
private val LightColors = lightColorScheme(
    primary = Color(0xFFB50F1A),
    onPrimary = Color(0xFFFFFFFF),
    primaryContainer = Color(0xFFD92F2F),
    onPrimaryContainer = Color(0xFFFFF8F7),
    secondary = Color(0xFFA23D37),
    onSecondary = Color(0xFFFFFFFF),
    secondaryContainer = Color(0xFFFC8278),
    onSecondaryContainer = Color(0xFF721A18),
    tertiary = Color(0xFF006280),
    onTertiary = Color(0xFFFFFFFF),
    tertiaryContainer = Color(0xFF007CA2),
    onTertiaryContainer = Color(0xFFF5FAFF),
    error = Color(0xFFBA1A1A),
    onError = Color(0xFFFFFFFF),
    errorContainer = Color(0xFFFFDAD6),
    onErrorContainer = Color(0xFF93000A),
    background = Color(0xFFFFF8F7),
    onBackground = Color(0xFF271716),
    surface = Color(0xFFFFF8F7),
    onSurface = Color(0xFF271716),
    surfaceVariant = Color(0xFFFFDAD6),
    onSurfaceVariant = Color(0xFF5B403D),
    outline = Color(0xFF906F6C),
    outlineVariant = Color(0xFFE4BDB9),
    inverseSurface = Color(0xFF3E2C2A),
    inverseOnSurface = Color(0xFFFFEDEB),
    inversePrimary = Color(0xFFFFB3AC),
    scrim = Color(0xFF000000),
    surfaceTint = Color(0xFFBB161D),
    surfaceBright = Color(0xFFFFF8F7),
    surfaceDim = Color(0xFFF1D3D0),
    surfaceContainer = Color(0xFFFFE9E6),
    surfaceContainerHigh = Color(0xFFFFE2DE),
    surfaceContainerHighest = Color(0xFFFADCD8),
    surfaceContainerLow = Color(0xFFFFF0EF),
    surfaceContainerLowest = Color(0xFFFFFFFF),
)

private val DarkColors = darkColorScheme(
    primary = Color(0xFFFFB3AC),
    onPrimary = Color(0xFF680008),
    primaryContainer = Color(0xFFD92F2F),
    onPrimaryContainer = Color(0xFFFFF8F7),
    secondary = Color(0xFFFFB3AC),
    onSecondary = Color(0xFF630E0F),
    secondaryContainer = Color(0xFF852824),
    onSecondaryContainer = Color(0xFFFF9E96),
    tertiary = Color(0xFF79D1FB),
    onTertiary = Color(0xFF003547),
    tertiaryContainer = Color(0xFF007CA2),
    onTertiaryContainer = Color(0xFFF5FAFF),
    error = Color(0xFFFFB4AB),
    onError = Color(0xFF690005),
    errorContainer = Color(0xFF93000A),
    onErrorContainer = Color(0xFFFFDAD6),
    background = Color(0xFF1E0F0E),
    onBackground = Color(0xFFFADCD8),
    surface = Color(0xFF1E0F0E),
    onSurface = Color(0xFFFADCD8),
    surfaceVariant = Color(0xFF5B403D),
    onSurfaceVariant = Color(0xFFE4BDB9),
    outline = Color(0xFFAB8885),
    outlineVariant = Color(0xFF5B403D),
    inverseSurface = Color(0xFFFADCD8),
    inverseOnSurface = Color(0xFF3E2C2A),
    inversePrimary = Color(0xFFBB161D),
    scrim = Color(0xFF000000),
    surfaceTint = Color(0xFFFFB3AC),
    surfaceBright = Color(0xFF483433),
    surfaceDim = Color(0xFF1E0F0E),
    surfaceContainer = Color(0xFF2C1B1A),
    surfaceContainerHigh = Color(0xFF372624),
    surfaceContainerHighest = Color(0xFF43302E),
    surfaceContainerLow = Color(0xFF271716),
    surfaceContainerLowest = Color(0xFF180A09),
)

@Composable
fun UjiboxTheme(content: @Composable () -> Unit) {
    MaterialTheme(
        colorScheme = if (isSystemInDarkTheme()) DarkColors else LightColors,
        content = content,
    )
}
