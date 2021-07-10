package com.skywire.skycoin.vpn;

import android.app.Activity;
import android.app.Application;
import android.app.NotificationChannel;
import android.app.NotificationManager;
import android.content.Context;
import android.os.Build;
import android.os.Bundle;

import androidx.annotation.NonNull;
import androidx.annotation.Nullable;

import com.skywire.skycoin.vpn.helpers.HelperFunctions;
import com.skywire.skycoin.vpn.helpers.Notifications;
import com.skywire.skycoin.vpn.vpn.VPNCoordinator;

import io.reactivex.rxjava3.plugins.RxJavaPlugins;

/**
 * Class for the main app instance.
 */
public class App extends Application {
    /**
     * Class used internally to know when there are activities being displayed.
     */
    private static class ActivityLifecycleCallback implements Application.ActivityLifecycleCallbacks {

        // How many activities are being shown.
        private static int foregroundActivities = 0;

        // Functions for knowing when activities start and stop being shown.
        @Override
        public void onActivityResumed(@NonNull final Activity activity) { foregroundActivities++; }
        @Override
        public void onActivityStopped(@NonNull final Activity activity) { foregroundActivities--; }

        /**
         * Returns if there is at least one activity being displayed.
         */
        public static boolean isApplicationInForeground() { return foregroundActivities > 0; }

        // Other functions needed by the interface.
        @Override
        public void onActivityPaused(@NonNull Activity activity) { }
        @Override
        public void onActivitySaveInstanceState(@NonNull Activity activity, @NonNull Bundle outState) { }
        @Override
        public void onActivityDestroyed(@NonNull Activity activity) { }
        @Override
        public void onActivityCreated(@NonNull Activity activity, @Nullable Bundle savedInstanceState) { }
        @Override
        public void onActivityStarted(@NonNull Activity activity) { }
    }

    /**
     * Reference to the current app instance.
     */
    private static Context appContext;

    @Override
    public void onCreate() {
        super.onCreate();
        // Save the current app instance.
        appContext = this;

        // Ensure the singleton is initialized early.
        VPNCoordinator.getInstance();

        // Create the notification channels, but only on API 26+ because
        // the NotificationChannel class is new and not in the support library
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
            // Channel for the VPN service state updates.
            NotificationChannel stateChannel = new NotificationChannel(
                Notifications.NOTIFICATION_CHANNEL_ID,
                getString(R.string.general_app_name),
                NotificationManager.IMPORTANCE_DEFAULT
            );
            stateChannel.setDescription(getString(R.string.general_notification_channel_description));
            stateChannel.setSound(null,null);
            NotificationManager notificationManager = getSystemService(NotificationManager.class);
            notificationManager.createNotificationChannel(stateChannel);

            // Channel for alerts.
            NotificationChannel alertsChannel = new NotificationChannel(
                    Notifications.ALERT_NOTIFICATION_CHANNEL_ID,
                    getString(R.string.general_alert_notification_name),
                    NotificationManager.IMPORTANCE_HIGH
            );
            alertsChannel.setDescription(getString(R.string.general_alert_notification_channel_description));
            notificationManager.createNotificationChannel(alertsChannel);
        }

        // Code for precessing errors which were not caught by the normal error management
        // procedures RxJava has. This prevents the app to be closed by unexpected errors, mainly
        // code trying to report events in closed observables.
        RxJavaPlugins.setErrorHandler(throwable -> {
            HelperFunctions.logError("ERROR INSIDE RX: ", throwable);
        });

        // Detect when activities are started and stopped.
        registerActivityLifecycleCallbacks(new ActivityLifecycleCallback());
    }

    /**
     * Gets the current app context.
     */
    public static Context getContext(){
        return appContext;
    }

    /**
     * Gets if the UI is being displayed.
     */
    public static boolean displayingUI(){
        return ActivityLifecycleCallback.isApplicationInForeground();
    }
}
