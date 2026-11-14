package com.deranjer.cookira

import android.app.Application
import android.graphics.Bitmap
import android.graphics.BitmapFactory
import android.net.Uri
import com.deranjer.cookira.data.ApiException
import com.deranjer.cookira.data.ApiProvider
import com.deranjer.cookira.data.CookiraRepository
import com.deranjer.cookira.data.RecipeCache
import com.deranjer.cookira.data.TokenHolder
import okhttp3.OkHttpClient
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNotNull
import org.junit.Assert.assertThrows
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.RuntimeEnvironment
import org.robolectric.annotation.Config
import org.robolectric.annotation.GraphicsMode
import java.io.File

@RunWith(RobolectricTestRunner::class)
@Config(sdk = [35], application = Application::class)
@GraphicsMode(GraphicsMode.Mode.NATIVE)
class RecipePhotoTest {
    private val context = RuntimeEnvironment.getApplication()
    private val repository = CookiraRepository(
        ApiProvider(OkHttpClient()), TokenHolder(), context, RecipeCache(context),
    )

    @Test
    fun `valid gallery and camera photos are encoded for upload`() {
        for (format in listOf(Bitmap.CompressFormat.PNG, Bitmap.CompressFormat.JPEG)) {
            val file = File.createTempFile("recipe-photo", ".image", context.cacheDir)
            try {
                val original = Bitmap.createBitmap(64, 32, Bitmap.Config.ARGB_8888)
                try {
                    file.outputStream().use { original.compress(format, 90, it) }
                } finally {
                    original.recycle()
                }

                val bytes = repository.readImageBytes(Uri.fromFile(file))
                val decoded = BitmapFactory.decodeByteArray(bytes, 0, bytes.size)
                assertNotNull(decoded)
                try {
                    assertEquals(64, decoded.width)
                    assertEquals(32, decoded.height)
                    assertEquals(0xff, bytes[0].toInt() and 0xff)
                    assertEquals(0xd8, bytes[1].toInt() and 0xff)
                } finally {
                    decoded.recycle()
                }
            } finally {
                file.delete()
            }
        }
    }

    @Test
    fun `unreadable image data still reports a photo error`() {
        val file = File.createTempFile("invalid-photo", ".image", context.cacheDir)
        try {
            file.writeText("not an image")
            val error = assertThrows(ApiException::class.java) {
                repository.readImageBytes(Uri.fromFile(file))
            }
            assertEquals("Couldn't read that photo.", error.message)
        } finally {
            file.delete()
        }
    }
}
