package com.deranjer.cookira.timers

import android.Manifest
import android.app.AlarmManager
import android.app.NotificationChannel
import android.app.NotificationManager
import android.app.PendingIntent
import android.content.BroadcastReceiver
import android.content.Context
import android.content.Intent
import android.content.pm.PackageManager
import android.os.Build
import androidx.core.app.NotificationCompat
import androidx.core.app.NotificationManagerCompat
import androidx.core.content.ContextCompat
import com.deranjer.cookira.MainActivity
import com.deranjer.cookira.R
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.update

data class CookTimer(val id: Int, val label: String, val recipe: String, val endAt: Long)

/**
 * Kitchen timers. The countdown shown in the app is just [CookTimer.endAt] minus the clock; the
 * ring comes from an alarm-clock alarm, which Android delivers on time even in Doze and with the
 * screen off and needs no special permission. The list lives in memory (a timer outlives the
 * screen, not the process), but the alarm and its notification do not depend on the app being alive.
 */
object CookTimers {
    private const val CHANNEL = "timers"
    private val _timers = MutableStateFlow<List<CookTimer>>(emptyList())
    val timers: StateFlow<List<CookTimer>> = _timers.asStateFlow()
    private var nextId = 1

    fun start(context: Context, label: String, recipe: String, seconds: Int) {
        val app = context.applicationContext
        val timer = synchronized(this) {
            CookTimer(nextId++, label, recipe, System.currentTimeMillis() + seconds * 1000L)
        }
        _timers.update { it + timer }
        val alarms = app.getSystemService(AlarmManager::class.java)
        val open = PendingIntent.getActivity(app, 0, Intent(app, MainActivity::class.java), PendingIntent.FLAG_IMMUTABLE)
        alarms.setAlarmClock(AlarmManager.AlarmClockInfo(timer.endAt, open), ringIntent(app, timer))
    }

    fun cancel(context: Context, id: Int) {
        val app = context.applicationContext
        _timers.value.firstOrNull { it.id == id }?.let { app.getSystemService(AlarmManager::class.java).cancel(ringIntent(app, it)) }
        _timers.update { list -> list.filterNot { it.id == id } }
        NotificationManagerCompat.from(app).cancel(id)
    }

    private fun ringIntent(context: Context, t: CookTimer) = PendingIntent.getBroadcast(
        context,
        t.id,
        Intent(context, TimerReceiver::class.java)
            .putExtra("id", t.id).putExtra("label", t.label).putExtra("recipe", t.recipe),
        PendingIntent.FLAG_IMMUTABLE or PendingIntent.FLAG_UPDATE_CURRENT,
    )

    internal fun ring(context: Context, id: Int, label: String, recipe: String) {
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.TIRAMISU &&
            ContextCompat.checkSelfPermission(context, Manifest.permission.POST_NOTIFICATIONS) != PackageManager.PERMISSION_GRANTED
        ) return
        context.getSystemService(NotificationManager::class.java).createNotificationChannel(
            NotificationChannel(CHANNEL, "Cooking timers", NotificationManager.IMPORTANCE_HIGH).apply {
                description = "Rings when a cooking timer finishes"
                enableVibration(true)
            },
        )
        val open = PendingIntent.getActivity(context, 0, Intent(context, MainActivity::class.java), PendingIntent.FLAG_IMMUTABLE)
        val n = NotificationCompat.Builder(context, CHANNEL)
            .setSmallIcon(R.drawable.ic_launcher_monochrome)
            .setContentTitle("Timer done: $label")
            .setContentText(recipe)
            .setCategory(NotificationCompat.CATEGORY_ALARM)
            .setPriority(NotificationCompat.PRIORITY_HIGH)
            .setDefaults(NotificationCompat.DEFAULT_ALL)
            .setContentIntent(open)
            .setAutoCancel(true)
            .build()
        NotificationManagerCompat.from(context).notify(id, n)
    }
}

class TimerReceiver : BroadcastReceiver() {
    override fun onReceive(context: Context, intent: Intent) {
        CookTimers.ring(
            context,
            intent.getIntExtra("id", 0),
            intent.getStringExtra("label").orEmpty(),
            intent.getStringExtra("recipe").orEmpty(),
        )
    }
}
