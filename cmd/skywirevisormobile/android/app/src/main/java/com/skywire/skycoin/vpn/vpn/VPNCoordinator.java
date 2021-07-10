package com.skywire.skycoin.vpn.vpn;

import android.app.Activity;
import android.app.ActivityManager;
import android.content.Context;
import android.content.Intent;
import android.net.VpnService;
import android.os.Build;
import android.os.Handler;
import android.os.Message;
import android.os.Messenger;

import com.skywire.skycoin.vpn.R;
import com.skywire.skycoin.vpn.App;
import com.skywire.skycoin.vpn.helpers.HelperFunctions;
import com.skywire.skycoin.vpn.helpers.Notifications;
import com.skywire.skycoin.vpn.objects.LocalServerData;

import java.util.ArrayList;
import java.util.Date;
import java.util.concurrent.TimeUnit;

import io.reactivex.rxjava3.android.schedulers.AndroidSchedulers;
import io.reactivex.rxjava3.core.Observable;
import io.reactivex.rxjava3.disposables.Disposable;
import io.reactivex.rxjava3.schedulers.Schedulers;
import io.reactivex.rxjava3.subjects.BehaviorSubject;
import skywiremob.Skywiremob;

import static android.app.Activity.RESULT_OK;

/**
 * Class for communication between the app UI and the VPN service. It is accessed via a singleton.
 */
public class VPNCoordinator implements Handler.Callback {
    public static class ConnectionStats {
        public Date lastConnectionDate = null;
        public long currentDownloadSpeed = 0;
        public long currentUploadSpeed = 0;
        public long currentLatency = 0;
        public long totalDownloadedData = 0;
        public long totalUploadedData = 0;
        public ArrayList<Long> downloadSpeedHistory = new ArrayList<>();
        public ArrayList<Long> uploadSpeedHistory  = new ArrayList<>();
        public ArrayList<Long> latencyHistory = new ArrayList<>();

        public ConnectionStats() {
            for (int i = 0; i < 10; i++) {
                downloadSpeedHistory.add(0L);
                uploadSpeedHistory.add(0L);
                latencyHistory.add(0L);
            }
        }
    }

    /**
     * Value the onActivityResult function will get after asking the user for permission.
     */
    public static final int VPN_PREPARATION_REQUEST_CODE = 10100;

    /**
     * Singleton instance.
     */
    private static final VPNCoordinator instance = new VPNCoordinator();
    /**
     * Gets the singleton for using the class.
     */
    public static VPNCoordinator getInstance() { return instance; }

    private Disposable updateStatsSubscription;

    private ConnectionStats connectionStats = new ConnectionStats();

    /**
     * App context.
     */
    private final Context ctx = App.getContext();

    /**
     * Handler used for receiving messages from the VPN service.
     */
    private final Handler serviceCommunicationHandler;
    /**
     * Subject for sending events via RxJava, indicating the current state of the VPN service.
     */
    private final BehaviorSubject<VPNStates.StateInfo> eventsSubject = BehaviorSubject.create();

    private final BehaviorSubject<ConnectionStats> connectionStatsSubject = BehaviorSubject.create();

    private VPNCoordinator() {
        serviceCommunicationHandler = new Handler(this);

        // Add a default current state.
        eventsSubject.onNext(new VPNStates.StateInfo(VPNStates.OFF, false, false));
    }

    public Observable<ConnectionStats> getConnectionStats() {
        return connectionStatsSubject.hide();
    }

    /**
     * Handles the messages received from the VPN service.
     */
    @Override
    public boolean handleMessage(Message msg) {
        // Save the error as the one which made the last execution of the VPN service fail.
        // Must be done before sending the event.
        String errorMsg = msg.getData().getString(SkywireVPNService.ERROR_MSG_PARAM);
        if (errorMsg != null && !errorMsg.equals("") && !errorMsg.equals(VPNGeneralPersistentData.getLastError(null))) {
            VPNGeneralPersistentData.setLastError(errorMsg);
        }

        if (updateStatsSubscription == null) {
            continuallyUpdateStats();
        }

        if (VPNStates.valueOf(msg.what) == VPNStates.CONNECTED) {
            // Erase the error which made not possible to connect the last time.
            VPNGeneralPersistentData.removeLastError();

            if (connectionStats.lastConnectionDate == null) {
                connectionStats.lastConnectionDate = new Date();
            }
        } else {
            if (VPNStates.valueOf(msg.what) == VPNStates.DISCONNECTED || VPNStates.valueOf(msg.what) == VPNStates.OFF) {
                if (updateStatsSubscription != null) {
                    updateStatsSubscription.dispose();
                    updateStatsSubscription = null;
                }

                connectionStats = new ConnectionStats();
                connectionStatsSubject.onNext(connectionStats);
            } else {
                connectionStats.lastConnectionDate = null;
            }
        }

        // Create the state object with the params returned by the VPN service.
        VPNStates.StateInfo state = new VPNStates.StateInfo(
            VPNStates.valueOf(msg.what),
            msg.getData().getBoolean(SkywireVPNService.STARTED_BY_THE_SYSTEM_PARAM),
            msg.getData().getBoolean(SkywireVPNService.STOP_REQUESTED_PARAM)
        );

        // Inform the new state.
        eventsSubject.onNext(state);

        return true;
    }

    private void continuallyUpdateStats() {
        if (updateStatsSubscription != null) {
            updateStatsSubscription.dispose();
        }

        sendStats();

        updateStatsSubscription = Observable.interval(1000L, TimeUnit.MILLISECONDS)
            .subscribeOn(Schedulers.newThread())
            .observeOn(AndroidSchedulers.mainThread())
            .subscribe(val -> {
                sendStats();
            });
    }

    private void sendStats() {
        connectionStats.currentDownloadSpeed = Skywiremob.vpnBandwidthReceived();
        connectionStats.downloadSpeedHistory.remove(0);
        connectionStats.downloadSpeedHistory.add(connectionStats.currentDownloadSpeed);

        connectionStats.currentUploadSpeed = Skywiremob.vpnBandwidthSent();
        connectionStats.uploadSpeedHistory.remove(0);
        connectionStats.uploadSpeedHistory.add(connectionStats.currentUploadSpeed);

        connectionStats.currentLatency = Skywiremob.vpnLatency();
        connectionStats.latencyHistory.remove(0);
        connectionStats.latencyHistory.add(connectionStats.currentLatency);

        connectionStatsSubject.onNext(connectionStats);
    }

    /**
     * Allows to know if the VPN service is currently running.
     */
    public boolean isServiceRunning() {
        ActivityManager manager = (ActivityManager) App.getContext().getSystemService(Context.ACTIVITY_SERVICE);
        for (ActivityManager.RunningServiceInfo service : manager.getRunningServices(Integer.MAX_VALUE)) {
            // Check if any of the running services is the VPN service.
            if (SkywireVPNService.class.getName().equals(service.service.getClassName())) {
                return true;
            }
        }
        return false;
    }

    /**
     * Returns an observable that emits every time the state of the VPN service changes. The
     * observable does not emit errors and never completes.
     */
    public Observable<VPNStates.StateInfo> getEventsObservable() {
        return eventsSubject.hide();
    }

    /**
     * Makes the preparations and starts the VPN service. If it is already running, nothing happens.
     * @param requestingActivity Activity requesting the service to be started. Please note
     * that the onActivityResult function of that activity may be called with the value of
     * VPN_PREPARATION_REQUEST_CODE as the first param. In that case the activity must call the
     * onActivityResult function of this instance with all the params, to be able to process
     * permission requests
     * @param server Data about the remote visor.
     */
    public void startVPN(Activity requestingActivity, LocalServerData server) {
        if (!isServiceRunning()) {
            // Save the remote visor and password.
            VPNServersPersistentData.getInstance().modifyCurrentServer(server);
            VPNServersPersistentData.getInstance().updateHistory();

            // As the service will be started again, erase the error which made it fail the last
            // time it ran, to indicate that no error has stopped the current instance.
            VPNGeneralPersistentData.removeLastError();

            eventsSubject.onNext(new VPNStates.StateInfo(VPNStates.STARTING, false, false));

            // Get the permission request intent from the OS.
            Intent intent = VpnService.prepare(requestingActivity);
            if (intent != null) {
                // Ask for permission before continuing.
                requestingActivity.startActivityForResult(intent, VPN_PREPARATION_REQUEST_CODE);
            } else {
                starVpnServiceIfNeeded();
            }
        }
    }

    /**
     * Function for starting the VPN service after boot. If the service is already running,
     * nothing happens.
     */
    public void activateAutostart() {
        if (!isServiceRunning()) {
            // Check if permission is needed. If it is, fail.
            Intent intent = VpnService.prepare(ctx);
            if (intent != null) {
                HelperFunctions.showToast(ctx.getString(R.string.general_autostart_failed_error), false);

                String errorMsg = ctx.getString(R.string.general_no_permissions_error);
                VPNGeneralPersistentData.setLastError(errorMsg);

                Notifications.showAlertNotification(
                        Notifications.AUTOSTART_ALERT_NOTIFICATION_ID,
                        ctx.getString(R.string.general_app_name),
                        errorMsg,
                        HelperFunctions.getOpenAppPendingIntent()
                );

                return;
            }

            // As the service will be started again, erase the error which made it fail the last
            // time it ran, to indicate that no error has stopped the current instance.
            VPNGeneralPersistentData.removeLastError();

            starVpnServiceIfNeeded();
        }
    }

    /**
     * Asks the VPN service to stop. It will not be stopped immediately, the state change events
     * must be checked for knowing when it is really stopped.
     */
    public void stopVPN() {
        ctx.startService(getServiceIntent().setAction(SkywireVPNService.ACTION_DISCONNECT));
    }

    /**
     * Must be called by the activity used for calling startVPN, if the same function is called
     * in the activity and the value of VPN_PREPARATION_REQUEST_CODE was received as request.
     * The same params received in the activity must be provided.
     */
    public void onActivityResult(int request, int result, Intent data) {
        if (request == VPN_PREPARATION_REQUEST_CODE) {
            if (result == RESULT_OK) {
                starVpnServiceIfNeeded();
            } else {
                eventsSubject.onNext(new VPNStates.StateInfo(VPNStates.OFF, false, true));
            }
        }
    }

    /**
     * Starts the VPN service if it is not already running.
     */
    private void starVpnServiceIfNeeded() {
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
            ctx.startForegroundService(getServiceIntent().setAction(SkywireVPNService.ACTION_CONNECT));
        } else {
            ctx.startService(getServiceIntent().setAction(SkywireVPNService.ACTION_CONNECT));
        }
    }

    /**
     * Gets the VPN service intent, without action.
     */
    private Intent getServiceIntent() {
        return new Intent(ctx, SkywireVPNService.class);
    }

    /**
     * Gets a Messenger object for communicating with this instance.
     */
    public Messenger getCommunicationMessenger() {
        return new Messenger(serviceCommunicationHandler);
    }
}
