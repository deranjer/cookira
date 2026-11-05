package com.deranjer.cookira.ui.theme

import androidx.compose.foundation.isSystemInDarkTheme
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Typography
import androidx.compose.material3.darkColorScheme
import androidx.compose.material3.lightColorScheme
import androidx.compose.runtime.Composable
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.sp
import com.deranjer.cookira.data.ThemeMode

// Mantine's orange ramp, as in the web UI.
private val Orange8 = Color(0xFFD9480F)
private val Orange6 = Color(0xFFF76707)
private val Orange1 = Color(0xFFFFE8CC)
private val Orange9 = Color(0xFF7C2D12)

private val LightColors = lightColorScheme(
    primary = Orange8,
    onPrimary = Color.White,
    primaryContainer = Orange1,
    onPrimaryContainer = Orange9,
    secondary = Color(0xFF8A5A3C),
    secondaryContainer = Color(0xFFFFEDD5),
    onSecondaryContainer = Color(0xFF4A2A14),
    background = Color(0xFFFFFBF7),
    surface = Color(0xFFFFFBF7),
    surfaceContainer = Color(0xFFFBF1E8),
    surfaceContainerHigh = Color(0xFFF6EBE0),
)

private val DarkColors = darkColorScheme(
    primary = Color(0xFFFFA94D),
    onPrimary = Color(0xFF3B1A00),
    primaryContainer = Color(0xFF7C2D12),
    onPrimaryContainer = Orange1,
    secondary = Color(0xFFE0B99B),
    secondaryContainer = Color(0xFF4A2A14),
    onSecondaryContainer = Color(0xFFFFEDD5),
    background = Color(0xFF1A1512),
    surface = Color(0xFF1A1512),
    surfaceContainer = Color(0xFF241D18),
    surfaceContainerHigh = Color(0xFF2E2620),
)

// Serif headings mirror the web UI (Georgia); everything else is the system sans.
private val Heading = FontFamily.Serif
private val CookiraTypography = Typography().let { t ->
    t.copy(
        headlineLarge = t.headlineLarge.copy(fontFamily = Heading, fontWeight = FontWeight.SemiBold),
        headlineMedium = t.headlineMedium.copy(fontFamily = Heading, fontWeight = FontWeight.SemiBold),
        headlineSmall = t.headlineSmall.copy(fontFamily = Heading, fontWeight = FontWeight.SemiBold),
        titleLarge = t.titleLarge.copy(fontFamily = Heading, fontWeight = FontWeight.SemiBold),
        titleMedium = t.titleMedium.copy(fontFamily = Heading, fontWeight = FontWeight.SemiBold),
        bodyLarge = TextStyle(fontSize = 16.sp, lineHeight = 24.sp),
    )
}

@Composable
fun CookiraTheme(mode: ThemeMode = ThemeMode.System, content: @Composable () -> Unit) {
    val dark = when (mode) {
        ThemeMode.System -> isSystemInDarkTheme()
        ThemeMode.Light -> false
        ThemeMode.Dark -> true
    }
    MaterialTheme(
        colorScheme = if (dark) DarkColors else LightColors,
        typography = CookiraTypography,
        content = content,
    )
}

val BrandOrange = Orange6
