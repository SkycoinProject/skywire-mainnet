package com.skywire.skycoin.vpn.helpers;

import android.app.Notification;
import android.app.NotificationManager;
import android.app.PendingIntent;
import android.content.Context;

import androidx.core.app.NotificationCompat;

import com.skywire.skycoin.vpn.App;
import com.skywire.skycoin.vpn.R;
import com.skywire.skycoin.vpn.vpn.VPNGeneralPersistentData;
import com.skywire.skycoin.vpn.vpn.VPNStates;

import io.reactivex.rxjava3.disposables.Disposable;
import skywiremob.Skywiremob;

/**
 * Constant values and helper functions for showing notifications.
 */
public class Notifications {
    /**
     * ID of the notification channel for showing the VPN service status.
     */
    public static final String NOTIFICATION_CHANNEL_ID = "SkywireVPN";
    /**
     * ID of the notification channel for showing alerts and errors.
     */
    public static final String ALERT_NOTIFICATION_CHANNEL_ID = "SkywireVPNAlerts";

    /**
     * ID of the VPN service status notification.
     */
    public static final int SERVICE_STATUS_NOTIFICATION_ID = 1;
    /**
     * ID of the notification for informing about errors while trying to automatically start the
     * VPN service during boot.
     */
    public static final int AUTOSTART_ALERT_NOTIFICATION_ID = 10;
    /**
     * ID of the generic error notifications.
     */
    public static final int ERROR_NOTIFICATION_ID = 50;

    /**
     * Units used for showing the data transmission stats.
     */
    private static Globals.DataUnits dataUnits = VPNGeneralPersistentData.getDataUnits();
    /**
     * Subscription for updating the data transmission stats.
     */
    private static Disposable dataUnitsSubscription;

    /**
     * Closes all the alert and error notifications created by the app. Only notifications with
     * the IDs defined in this class will be closed.
     */
    public static void removeAllAlertNotifications() {
        NotificationManager notificationManager = (NotificationManager) App.getContext().getSystemService(Context.NOTIFICATION_SERVICE);

        notificationManager.cancel(AUTOSTART_ALERT_NOTIFICATION_ID);
        notificationManager.cancel(ERROR_NOTIFICATION_ID);
    }

    /**
     * Creates and shows an alert notification.
     * @param ID Notification ID. Please use one of the IDs defined in this class.
     * @param title Notification title.
     * @param content Main notification text.
     * @param contentIntent Intent for when the user presses the notification.
     */
    public static void showAlertNotification(int ID, String title, String content, PendingIntent contentIntent) {
        // Create the style for a multiline notification. It will be ignore if the OS does not
        // support it.
        NotificationCompat.BigTextStyle bigTextStyle = new NotificationCompat.BigTextStyle()
            .setBigContentTitle(title)
            .bigText(content);

        // Create the notification.
        Notification notification = new NotificationCompat.Builder(App.getContext(), ALERT_NOTIFICATION_CHANNEL_ID)
            .setSmallIcon(R.drawable.ic_error)
            .setContentTitle(title)
            .setContentText(content)
            .setStyle(bigTextStyle)
            .setContentIntent(contentIntent)
            .build();

        // Show it.
        NotificationManager notificationManager = (NotificationManager)App.getContext().getSystemService(Context.NOTIFICATION_SERVICE);
        notificationManager.notify(ID, notification);
    }

    /**
     * Creates a notification for displaying the current state of the VPN service. The notification
     * is returned, not displayed.
     * @param currentState Current state of the VPN service.
     * @param protectionEnabled If the network protection has already been activated.
     * @return The created notification.
     */
    public static Notification createStatusNotification(VPNStates currentState, boolean protectionEnabled) {
        // Start updating the data transmission stats, if needed.
        if (dataUnitsSubscription == null) {
            dataUnitsSubscription = VPNGeneralPersistentData.getDataUnitsObservable().subscribe(response -> {
                dataUnits = response;
            });
        }

        // The title is always "preparing", unless the state indicates the service is connected,
        // disconnecting or restoring. For the state numeric values, check the emun documentation.
        int title = R.string.vpn_service_state_preparing;
        if (currentState == VPNStates.CONNECTED) {
            title = VPNStates.getTitleForState(currentState);
        } else {
            if (currentState.val() >= VPNStates.DISCONNECTING.val()) {
                title = R.string.vpn_service_state_finishing;
            } else if (currentState.val() >= VPNStates.RESTORING_VPN.val() && currentState.val() < VPNStates.DISCONNECTING.val()) {
                title = R.string.vpn_service_state_restoring;
            }
        }

        // Main text for the notification.
        String text = App.getContext().getString(VPNStates.getDescriptionForState(currentState));
        // If connected, the connection stats are shown as the main text.
        if (currentState == VPNStates.CONNECTED) {
            text = "\u2191" + HelperFunctions.computeDataAmountString(Skywiremob.vpnBandwidthSent(), true, dataUnits != Globals.DataUnits.OnlyBytes);
            text += "  \u2193" + HelperFunctions.computeDataAmountString(Skywiremob.vpnBandwidthReceived(), true, dataUnits != Globals.DataUnits.OnlyBytes);
            text += "  \u2194" + HelperFunctions.getLatencyValue(Skywiremob.vpnLatency());
        }

        // The lines icon indicates that the service is disconnected and the network protection is
        // not active. The filed icon indicates that the service is connected and working. The
        // alert icon indicates that the network protection is active, but the VPN service is still
        // not working. The error icon is used only if an error stopped the service.
        int icon = R.drawable.ic_lines;
        if (protectionEnabled) {
            if (currentState == VPNStates.CONNECTED) {
                icon = R.drawable.ic_filled;
            } else {
                icon = R.drawable.ic_alert;
            }
        }
        if (currentState == VPNStates.ERROR || currentState == VPNStates.BLOCKING_ERROR) {
            icon = R.drawable.ic_error;
        }

        // Create the style for a multiline notification. It will be ignore if the OS does not
        // support it.
        NotificationCompat.BigTextStyle bigTextStyle = new NotificationCompat.BigTextStyle()
            .bigText(text)
            .setBigContentTitle(App.getContext().getString(title));

        return new NotificationCompat.Builder(App.getContext(), NOTIFICATION_CHANNEL_ID)
            .setSmallIcon(icon)
            .setContentTitle(App.getContext().getString(title))
            .setContentText(text)
            .setStyle(bigTextStyle)
            .setContentIntent(HelperFunctions.getOpenAppPendingIntent())
            .setOnlyAlertOnce(true)
            .setSound(null)
            .build();
    }
}
