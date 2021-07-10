package com.skywire.skycoin.vpn.vpn;

import android.app.NotificationManager;
import android.content.Context;
import android.content.Intent;
import android.net.VpnService;
import android.os.Bundle;
import android.os.Message;
import android.os.Messenger;

import com.skywire.skycoin.vpn.R;
import com.skywire.skycoin.vpn.App;
import com.skywire.skycoin.vpn.helpers.HelperFunctions;
import com.skywire.skycoin.vpn.helpers.Notifications;
import com.skywire.skycoin.vpn.objects.ServerFlags;

import java.util.concurrent.TimeUnit;

import io.reactivex.rxjava3.android.schedulers.AndroidSchedulers;
import io.reactivex.rxjava3.core.Observable;
import io.reactivex.rxjava3.disposables.Disposable;
import io.reactivex.rxjava3.schedulers.Schedulers;
import skywiremob.Skywiremob;

/**
 * Service in charge of making the VPN protection work, even if the UI is closed.
 */
public class SkywireVPNService extends VpnService {
    /**
     * Action that must be sent to the service for starting the VPN connection. If
     * the connection has already been started, it continues running normally.
     */
    public static final String ACTION_CONNECT = "com.skywire.android.vpn.START";
    /**
     * Action that must be sent to the service for stopping the VPN connection. The procedure may
     * take some time to complete, so the state events must be monitored.
     */
    public static final String ACTION_DISCONNECT = "com.skywire.android.vpn.STOP";

    /**
     * Param returned by the service as part of the state updates, for including the error
     * message, if the state includes one.
     */
    public static final String ERROR_MSG_PARAM = "ErrorMsg";
    /**
     * Param returned by the service as part of the state updates, for informing if the service is
     * running because the OS requested it (true) or was started by the app itself (false).
     */
    public static final String STARTED_BY_THE_SYSTEM_PARAM = "StartedByTheSystem";
    /**
     * Param returned by the service as part of the state updates, for informing if it has received
     * a request for completely stopping the service. The request may have not been made by
     * the user.
     */
    public static final String STOP_REQUESTED_PARAM = "StopRequested";

    /**
     * ID of the last instance of the service. This is needed because a new instance may be
     * created by the OS while the previous one is still being destroyed and in those cases it is
     * necessary to stop making some operations in the old instance.
     */
    public static int lastInstanceID = 0;
    /**
     * ID of this object instance. If it is not equal to lastInstanceID, this is not the
     * latest instance.
     */
    public int instanceID = 0;

    /**
     * Object for showing notifications.
     */
    private final NotificationManager notificationManager = (NotificationManager) App.getContext().getSystemService(Context.NOTIFICATION_SERVICE);

    /**
     * Instance for communicating with the VPN coordinator class.
     */
    private Messenger messenger;

    /**
     * Object in charge of performing the steps needed for making the VPN protection work.
     */
    private VPNRunnable vpnRunnable;
    /**
     * Current VPN work interface.
     */
    private VPNWorkInterface vpnInterface;

    /**
     * Current state of the VPN protection.
     */
    private VPNStates currentState = VPNStates.STARTING;

    /**
     * If the service is running because the OS requested it (true) or was started by the app
     * itself (false).
     */
    private boolean startedByTheSystem = false;
    /**
     * If true, a condition that makes it not possible to start the service was detected, so
     * the option for retrying the connection must be ignored.
     */
    private boolean impossibleToStart = false;
    /**
     * If there was a request for completely stopping the service.
     */
    private boolean stopRequested = false;
    /**
     * If the service has already been destroyed. The code may still be running cleaning procedures.
     */
    private boolean serviceDestroyed = false;

    /**
     * Msg of the last error detected by this instance.
     */
    private String lastErrorMsg = "";

    private Disposable updateNotificationSubscription;
    private Disposable restartingSubscription;
    private Disposable vpnRunnableSubscription;

    /**
     * Informs the current state to the VPN coordinator, updates the state notification and shows
     * toast notifications, if needed. It also updates the current state var.
     */
    private void informNewState(VPNStates newState) {
        // Cancel the operation if there is a newer instance of the service.
        if (lastInstanceID != instanceID) {
            return;
        }

        // Create a new message for informing the VPN coordinator about the new state.
        Message msg = Message.obtain();
        msg.what = newState.val();

        // Add the additional data to the message.
        Bundle dataBundle = new Bundle();
        dataBundle.putBoolean(STARTED_BY_THE_SYSTEM_PARAM, startedByTheSystem);
        dataBundle.putBoolean(STOP_REQUESTED_PARAM, stopRequested);

        // Get the last error from vpnRunnable.getLastErrorMsg(). The lastErrorMsg must be used
        // to avoid errors because vpnRunnable may be null.
        lastErrorMsg = vpnRunnable != null ? vpnRunnable.getLastErrorMsg() : lastErrorMsg;
        dataBundle.putString(ERROR_MSG_PARAM, lastErrorMsg);

        msg.setData(dataBundle);

        // Show toast notifications for certain states if the UI is not being shown.
        if (!App.displayingUI() && currentState != newState) {
            // Only if the service has not been destroyed.
            if (!serviceDestroyed && (newState == VPNStates.CONNECTED ||
                newState == VPNStates.RESTORING_VPN ||
                newState == VPNStates.RESTORING_SERVICE ||
                newState == VPNStates.ERROR ||
                newState == VPNStates.BLOCKING_ERROR))
            {
                HelperFunctions.showToast(getString(VPNStates.getDescriptionForState(newState)), false);
            }

            // Even if the service has been destroyed.
            if (newState == VPNStates.DISCONNECTED || newState == VPNStates.DISCONNECTING || newState == VPNStates.OFF) {
                HelperFunctions.showToast(getString(VPNStates.getDescriptionForState(newState)), false);
            }
        }

        currentState = newState;

        // Send the message to the VPN coordinator.
        try {
            messenger.send(msg);
        } catch (Exception e) { }

        // Update the notification.
        updateForegroundNotification();

        // Procedure for periodically updating the notification with the connection stats, if the
        // VPN protection is active.
        if (updateNotificationSubscription != null) {
            updateNotificationSubscription.dispose();
        }
        if (newState == VPNStates.CONNECTED) {
            updateNotificationSubscription = Observable.interval(2000, TimeUnit.MILLISECONDS)
                .subscribeOn(Schedulers.newThread())
                .observeOn(AndroidSchedulers.mainThread())
                .subscribe(val -> updateForegroundNotification());
        }
    }

    /**
     * Function that must be called when there are changes in the state of the VPN protection. It
     * processes the new state, makes some preparations and informs it.
     */
    private void updateState(VPNStates newState) {
        // State that will be reported at the end of the function. It may be modified.
        VPNStates processedState = newState;

        // If the current state is for indicating an error and the new state is for indicating
        // that the VPN protection is being disconnected, the current state is maintained, to
        // avoid replacing the error indications, which is more useful than a generic indication
        // about the service being stopped. This also prevents the code from "forgetting" that
        // there was an error, which may be important later.
        if (processedState.val() >= 200 && processedState.val() < 300 && currentState.val() >= 400 && currentState.val() <= 500) {
            processedState = currentState;
        }

        boolean failedBecausePassword = false;
        // If the state indicates that vpnRunnable finished, remove the instance.
        if (processedState.val() >= 300 && processedState.val() < 400) {
            // Check if the process finished due to an error cause by a wrong password. This data is
            // used if the protection has to be restarted.
            if (vpnRunnable != null && vpnRunnable.getIfPasswordFailed()) {
                failedBecausePassword = true;
            }
            vpnRunnable = null;
            if (vpnRunnableSubscription != null) {
                vpnRunnableSubscription.dispose();
            }
        }

        // Only needed if the service is not forced to terminate.
        if (!stopRequested && !serviceDestroyed) {
            // If the new state is for informing about an error.
            if (processedState.val() >= 400 && processedState.val() < 500) {
                if (VPNGeneralPersistentData.getMustRestartVpn() && !impossibleToStart) {
                    // If the option for restarting the protection automatically is active, update
                    // the state.
                    processedState = VPNStates.RESTORING_SERVICE;
                } else if (processedState == VPNStates.ERROR) {
                    // If the error was not a blocking one, which would mean that the network must
                    // remain blocked, indicate that the service must be closed after closing
                    // the VPN.
                    stopRequested = true;
                }
            }

            // If the service is being restored, hide the states about the connection being
            // closed and restored.
            if (currentState == VPNStates.RESTORING_SERVICE) {
                // Restart the whole VPN connection after a small delay when receiving the state
                // indicating that vpnRunnable finished. If the error was because the password was
                // wrong, the delay is much longer.
                if (processedState.val() >= 300 && processedState.val() < 400) {
                    int delay = failedBecausePassword ? 60000 : 1;
                    restartingSubscription = Observable.just(0).delay(delay, TimeUnit.MILLISECONDS)
                        .subscribeOn(Schedulers.newThread())
                        .observeOn(AndroidSchedulers.mainThread())
                        .subscribe(val -> runVpn());
                }

                if (processedState.val() >= 150 && processedState.val() < 400) {
                    processedState = VPNStates.RESTORING_SERVICE;
                }
            } else {
                // If the service is not being restored, close the whole service when receiving
                // the state indicating that vpnRunnable finished.
                if (processedState.val() >= 300 && processedState.val() < 400) {
                    processedState = currentState;
                    finishIfAppropriate();
                }
            }
        } else {
            // Close the whole service when receiving the state indicating that
            // vpnRunnable finished.
            if (processedState.val() >= 300 && processedState.val() < 400) {
                processedState = currentState;
                finishIfAppropriate();
            }
        }

        // Inform the new state to the VPN coordinator and update the notifications.
        informNewState(processedState);
    }

    /**
     * Function called by the OS just after receiving an instruction for starting the service.
     */
    @Override
    public int onStartCommand(Intent intent, int flags, int startId) {
        // Update the ID of this instance, to make sure no old instance is considered newer than
        // this one.
        lastInstanceID += 1;
        instanceID = lastInstanceID;

        if (intent != null && ACTION_DISCONNECT.equals(intent.getAction())) {
            // If this function was called to stop the VPN protection.

            stopRequested = true;

            // Stop the connection. If it was already stopped, finish the service directly.
            if (vpnRunnable != null) {
                vpnRunnable.disconnect();
            } else {
                finishIfAppropriate();
            }

            // Needed for informing the new value of the stopRequested var.
            updateState(currentState);
        } else {
            // If the function was not called for stopping the VPN protection, it is considered
            // that it was called for starting it. In this case, the instruction for starting the
            // service may have been made by the OS or the app itself. if the ACTION_CONNECT action
            // is not detected, it is considered that the request was made by the OS.

            // Get the object for communicating with the VPN coordinator.
            if (messenger == null) {
                messenger = VPNCoordinator.getInstance().getCommunicationMessenger();
            }

            if (vpnInterface == null) {
                // Become a foreground service. Background services can be VPN services too, but
                // they can be killed by background check before getting a chance to
                // receive onRevoke().
                makeForeground();

                vpnInterface = new VPNWorkInterface(this);
            }

            // If the option for blocking the network while configuring the service is active or
            // the request was made by the OS, the VPN work interface is configured, to block all
            // network connections. The action is always made when the service is started by the OS
            // because the OS will only stop the service after the user request it if the interface
            // is configured (appears like a bug in the OS).
            if (!vpnInterface.alreadyConfigured() && (VPNGeneralPersistentData.getProtectBeforeConnected() || intent == null || !ACTION_CONNECT.equals(intent.getAction()))) {
                try {
                    vpnInterface.configure(VPNWorkInterface.Modes.BLOCKING);
                } catch (Exception e) {
                    // Report the error and finish the service.
                    HelperFunctions.logError("Configuring VPN work interface before connecting", e);
                    lastErrorMsg = getString(R.string.vpn_service_network_protection_error);
                    updateState(VPNStates.ERROR);
                    finishIfAppropriate();

                    return START_NOT_STICKY;
                }

                if (intent == null || !ACTION_CONNECT.equals(intent.getAction())) {
                    HelperFunctions.showToast(getString(R.string.vpn_service_network_unavailable_warning), false);
                }
            }

            // Update if the service was started by the OS and notify it in a state event. Note
            // that this code updates the previous value if the service was originally started by
            // the app, this is intended.
            if (intent == null || !ACTION_CONNECT.equals(intent.getAction())) {
                startedByTheSystem = true;
            }
            updateState(currentState);

            // Check if no server has been selected and if the selected server has been blocked.
            String errorMsg = null;
            if (
                VPNServersPersistentData.getInstance().getCurrentServer() == null ||
                VPNServersPersistentData.getInstance().getCurrentServer().pk == null ||
                VPNServersPersistentData.getInstance().getCurrentServer().pk.trim().equals("")
            ) {
                errorMsg = App.getContext().getText(R.string.skywiremob_error_no_server).toString();
            } else if (VPNServersPersistentData.getInstance().getCurrentServer().flag == ServerFlags.Blocked) {
                errorMsg = App.getContext().getText(R.string.skywiremob_error_server_blocked).toString();
            }

            // If any of the previous conditions was found, put the service in error state.
            if (errorMsg != null) {
                HelperFunctions.logError("Starting VPN service", errorMsg);
                lastErrorMsg = errorMsg;
                impossibleToStart = true;
                updateState(VPNStates.ERROR);
            } else {
                // Start the VPN protection.
                runVpn();
            }
        }

        return START_NOT_STICKY;
    }

    /**
     * Function called by the OS when the service is destroyed.
     */
    @Override
    public void onDestroy() {
        Skywiremob.printString("VPN service destroyed.");
        serviceDestroyed = true;

        // Stop the connection. If it was already stopped, finish the service directly.
        if (vpnRunnable != null) {
            vpnRunnable.disconnect();
        } else {
            finishIfAppropriate();
        }
    }

    /**
     * Function called by the OS when the user revokes the permission for the VPN.
     */
    @Override
    public void onRevoke() {
        super.onRevoke();
        Skywiremob.printString("onRevoke called");
        // Destroy the service.
        this.stopSelf();
    }

    /**
     * Starts the VPN protection, if it is not already active or starting.
     */
    private void runVpn() {
        if (vpnRunnable == null) {
            vpnRunnable = new VPNRunnable(vpnInterface);
        }

        if (vpnRunnableSubscription != null) {
            vpnRunnableSubscription.dispose();
        }

        // Initialize the VPN. Also, get and process the state updates.
        vpnRunnableSubscription = vpnRunnable.start().subscribe(state -> updateState(state));
    }

    /**
     * Cleans the resources used by the service and stops it, but only if vpnRunnable
     * already finished.
     */
    private void finishIfAppropriate() {
        if (vpnRunnable == null) {
            if (vpnInterface == null ||
                !vpnInterface.alreadyConfigured() ||
                stopRequested ||
                serviceDestroyed ||
                currentState.val() < 400 ||
                currentState.val() >= 500 ||
                !VPNGeneralPersistentData.getKillSwitchActivated()
            ) {
                // Steps that must be performed only if there is no a newer instance of the service.
                if (lastInstanceID == instanceID) {
                    // Clean the VPN interface (which stops blocking the network connections).
                    if (vpnInterface != null) {
                        vpnInterface.close();

                        // Create another interface and close it immediately to avoid a bug in
                        // older Android versions when the app is added to the ignore list.
                        vpnInterface = new VPNWorkInterface(this);
                        try {
                            vpnInterface.configure(VPNWorkInterface.Modes.DELETING);
                        } catch (Exception e) { }
                        vpnInterface.close();
                    }

                    // Remove the state notification.
                    notificationManager.cancel(Notifications.SERVICE_STATUS_NOTIFICATION_ID);

                    // Report the new state after a delay, to avoid interferences with any new
                    // state reported by the code which called this function.
                    Observable.just(0).delay(100, TimeUnit.MILLISECONDS)
                        .subscribeOn(Schedulers.newThread())
                        .observeOn(AndroidSchedulers.mainThread())
                        .subscribe(val -> updateState(VPNStates.OFF));

                    // If there was an error in the last execution, the UI is not being displayed
                    // and the kill switch is not active, show a notification informing that
                    // the VPN protection was terminated due to an error.
                    if (!App.displayingUI() && !VPNGeneralPersistentData.getKillSwitchActivated() && VPNGeneralPersistentData.getLastError(null) != null) {
                        Notifications.showAlertNotification(
                            Notifications.ERROR_NOTIFICATION_ID,
                            getString(R.string.general_app_name),
                            getString(R.string.general_connection_error),
                            HelperFunctions.getOpenAppPendingIntent()
                        );
                    }
                }

                // Remove the objects and close the subscriptions.
                vpnInterface = null;
                vpnRunnable = null;
                if (vpnRunnableSubscription != null) {
                    vpnRunnableSubscription.dispose();
                }
                if (restartingSubscription != null) {
                    restartingSubscription.dispose();
                }

                // Terminate the service.
                stopForeground(true);
                stopSelf();
            }
        }
    }

    /**
     * Updates the state notification shown while the service is running in the foreground.
     */
    private void updateForegroundNotification() {
        if (!serviceDestroyed) {
            notificationManager.notify(
                Notifications.SERVICE_STATUS_NOTIFICATION_ID,
                Notifications.createStatusNotification(currentState, vpnInterface != null && vpnInterface.alreadyConfigured())
            );
        }
    }

    /**
     * Converts the service into a foreground service, to prevent it to be destroyed by the OS.
     */
    private void makeForeground() {
        startForeground(
            Notifications.SERVICE_STATUS_NOTIFICATION_ID,
            Notifications.createStatusNotification(currentState, vpnInterface != null && vpnInterface.alreadyConfigured())
        );
    }
}
