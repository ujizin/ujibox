package io.ujibox.app

import android.app.Activity
import android.app.Application
import android.app.PendingIntent
import android.content.BroadcastReceiver
import android.content.Context
import android.content.Intent
import android.content.pm.PackageInstaller
import android.net.Uri
import android.os.Build
import android.os.Bundle
import android.provider.Settings
import io.ujibox.app.data.BuildItem
import io.ujibox.app.data.Install
import io.ujibox.app.data.SyncFolder
import io.ujibox.app.install.InstallResult
import io.ujibox.app.install.Installer
import java.io.File
import java.lang.ref.WeakReference
import java.util.concurrent.ConcurrentHashMap
import kotlinx.coroutines.CompletableDeferred
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext

/** Installs APKs with a PackageInstaller session. The system asks the user to confirm. */
internal class AndroidInstaller(
    private val context: Context,
    private val folder: SyncFolder,
    private val activities: ResumedActivity,
) : Installer {
    override suspend fun install(build: BuildItem): InstallResult {
        val apkPath = (build.install as? Install.Apk)?.file
            ?: return InstallResult.Failed("unsupported install kind")
        val apk = File(folder.resolve(apkPath))
        if (!apk.isFile) return InstallResult.Missing

        val packages = context.packageManager
        if (!packages.canRequestPackageInstalls()) {
            val settings = Intent(Settings.ACTION_MANAGE_UNKNOWN_APP_SOURCES, Uri.parse("package:" + context.packageName))
            // From the activity, Back returns to ujibox. From the app context, Settings opens in its own task.
            val activity = activities.resumed
            if (activity != null) {
                activity.startActivity(settings)
            } else {
                context.startActivity(settings.addFlags(Intent.FLAG_ACTIVITY_NEW_TASK))
            }
            return InstallResult.NeedsPermission
        }

        val installer = packages.packageInstaller
        val params = PackageInstaller.SessionParams(PackageInstaller.SessionParams.MODE_FULL_INSTALL).apply {
            if (build.packageId.isNotEmpty()) setAppPackageName(build.packageId)
            setSize(apk.length())
        }
        val result = CompletableDeferred<InstallResult>()
        val sessionId = withContext(Dispatchers.IO) { installer.createSession(params) }
        InstallReceiver.pending[sessionId] = result
        try {
            withContext(Dispatchers.IO) {
                installer.openSession(sessionId).use { session ->
                    session.openWrite("base.apk", 0, apk.length()).use { out ->
                        apk.inputStream().use { it.copyTo(out) }
                        session.fsync(out)
                    }
                    val intent = Intent(context, InstallReceiver::class.java).setPackage(context.packageName)
                    // The system adds the status extras, so the intent must be mutable.
                    val callback = PendingIntent.getBroadcast(
                        context,
                        sessionId,
                        intent,
                        PendingIntent.FLAG_UPDATE_CURRENT or PendingIntent.FLAG_MUTABLE,
                    )
                    session.commit(callback.intentSender)
                }
            }
            return result.await()
        } catch (e: Exception) {
            runCatching { installer.abandonSession(sessionId) }
            if (e is kotlinx.coroutines.CancellationException) throw e
            return InstallResult.Failed(e.message ?: e.toString())
        } finally {
            InstallReceiver.pending.remove(sessionId)
        }
    }
}

/** Keeps the activity that is on screen, to start other screens from it. */
internal class ResumedActivity(app: Application) : Application.ActivityLifecycleCallbacks {
    private var current: WeakReference<Activity>? = null

    val resumed: Activity? get() = current?.get()

    init {
        app.registerActivityLifecycleCallbacks(this)
    }

    override fun onActivityResumed(activity: Activity) {
        current = WeakReference(activity)
    }

    override fun onActivityPaused(activity: Activity) {
        if (current?.get() === activity) current = null
    }

    override fun onActivityCreated(activity: Activity, savedInstanceState: Bundle?) = Unit

    override fun onActivityStarted(activity: Activity) = Unit

    override fun onActivityStopped(activity: Activity) = Unit

    override fun onActivitySaveInstanceState(activity: Activity, outState: Bundle) = Unit

    override fun onActivityDestroyed(activity: Activity) = Unit
}

/** Receives the status of an install session from the system. */
class InstallReceiver : BroadcastReceiver() {
    override fun onReceive(context: Context, intent: Intent) {
        val sessionId = intent.getIntExtra(PackageInstaller.EXTRA_SESSION_ID, -1)
        val status = intent.getIntExtra(PackageInstaller.EXTRA_STATUS, PackageInstaller.STATUS_FAILURE)
        if (status == PackageInstaller.STATUS_PENDING_USER_ACTION) {
            val confirm = confirmIntent(intent)
            if (confirm == null) {
                pending[sessionId]?.complete(InstallResult.Failed("no confirmation screen"))
                return
            }
            context.startActivity(confirm.addFlags(Intent.FLAG_ACTIVITY_NEW_TASK))
            return
        }
        val result = when (status) {
            PackageInstaller.STATUS_SUCCESS -> InstallResult.Installed
            PackageInstaller.STATUS_FAILURE_ABORTED -> InstallResult.Cancelled
            else -> InstallResult.Failed(
                intent.getStringExtra(PackageInstaller.EXTRA_STATUS_MESSAGE) ?: "status $status",
            )
        }
        pending[sessionId]?.complete(result)
    }

    private fun confirmIntent(intent: Intent): Intent? =
        if (Build.VERSION.SDK_INT >= 33) {
            intent.getParcelableExtra(Intent.EXTRA_INTENT, Intent::class.java)
        } else {
            @Suppress("DEPRECATION")
            intent.getParcelableExtra(Intent.EXTRA_INTENT)
        }

    internal companion object {
        /** The installs that wait for a status, by session id. */
        val pending = ConcurrentHashMap<Int, CompletableDeferred<InstallResult>>()
    }
}
