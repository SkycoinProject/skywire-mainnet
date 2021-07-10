package com.skywire.skycoin.vpn.vpn;

import com.skywire.skycoin.vpn.App;
import com.skywire.skycoin.vpn.helpers.HelperFunctions;
import com.skywire.skycoin.vpn.R;

import java.util.concurrent.TimeUnit;

import io.reactivex.rxjava3.android.schedulers.AndroidSchedulers;
import io.reactivex.rxjava3.core.Observable;
import io.reactivex.rxjava3.core.ObservableOnSubscribe;
import io.reactivex.rxjava3.disposables.Disposable;
import io.reactivex.rxjava3.schedulers.Schedulers;
import io.reactivex.rxjava3.subjects.BehaviorSubject;
import skywiremob.Skywiremob;

/**
 * Class for configuring most of the VPN protection. After creating an instance, the start method
 * can be used to start a series of steps for configuring the local visor and creating the VPN
 * connection. Each instance can be used one time only, so a new instance must be created for
 * starting the VPN protection again.
 */
public class VPNRunnable {
    /**
     * Current VPN work interface.
     */
    private final VPNWorkInterface vpnInterface;
    /**
     * Object for controlling the local visor.
     */
    private VisorRunnable visor;
    /**
     * Object for connecting the visor with the VPN work interface, to make the VPN functional.
     */
    private SkywireVPNConnection vpnConnection;

    /**
     * If the procedure to wait for the visor to be available already finished.
     */
    private boolean waitAvailableFinished = false;
    /**
     * If the procedure to wait for having network connectivity already finished.
     */
    private boolean waitNetworkFinished = false;

    /**
     * If the disconnection procedure already started.
     */
    private boolean disconnectionStarted = false;
    /**
     * Counts how many consecutive times the visor was detected as shut down while disconnecting.
     */
    private int disconnectionVerifications = 0;

    /**
     * Subject for informing about the state of the VPN protection.
     */
    private final BehaviorSubject<VPNStates> eventsSubject = BehaviorSubject.create();
    /**
     * Subject for informing about the state of the VPN protection.
     */
    private Observable<VPNStates> eventsObservable;

    /**
     * Msg string of the last error detected by this instance.
     */
    private String lastErrorMsg;

    private Disposable waitingSubscription;
    private Disposable visorTimeoutSubscription;

    /**
     * Constructor.
     * @param vpnInterface VPN work interface to use. This class will only configure it when
     *                     stabilising the connection, so it will have to be configured before
     *                     using this constructor if the network must be blocked before that.
     *                     Also, this class will not unblock the network after disconnecting, that
     *                     will have to be done by external code.
     */
    public VPNRunnable(VPNWorkInterface vpnInterface) {
        eventsSubject.onNext(VPNStates.OFF);
        this.vpnInterface = vpnInterface;
    }

    /**
     * Starts the initialization procedure for the VPN protection, if it has not already
     * been started.
     * @return Observable for knowing the current state of the VPN protection. The operation is not
     * started by the subscription, it starts just for calling the function, so there is no need
     * for observing in another thread.
     */
    public Observable<VPNStates> start() {
        if (eventsObservable == null) {
            // Prepare for sending events.
            eventsSubject.onNext(VPNStates.STARTING);
            eventsObservable = eventsSubject.hide();
        }

        // Go to the first step.
        waitForVisorToBeAvailableIfNeeded();

        return eventsObservable;
    }

    /**
     * Allows to know if the initialization failed because the server refused the password.
     */
    public boolean getIfPasswordFailed() {
        return visor != null ? visor.getIfPasswordFailed() : false;
    }

    /**
     * Waits for the visor to be totally stopped. After that, goes to the next step for
     * starting the VPN protection. If this step was already finished, the function does nothing.
     */
    private void waitForVisorToBeAvailableIfNeeded() {
        if (!waitAvailableFinished) {
            // Avoid having multiple simultaneous procedures.
            if (waitingSubscription != null) {
                waitingSubscription.dispose();
            }

            // Check if the local visor is not running. If true, continue to the next step.
            if (!Skywiremob.isVisorStarting() && !Skywiremob.isVisorRunning()) {
                waitAvailableFinished = true;
                checkInternetConnectionIfNeeded(true);
            } else {
                // Update the state.
                if (eventsSubject.getValue() != VPNStates.WAITING_PREVIOUS_INSTANCE_STOP) {
                    Skywiremob.printString("WAITING FOR THE PREVIOUS INSTANCE TO BE FULLY STOPPED");
                    eventsSubject.onNext(VPNStates.WAITING_PREVIOUS_INSTANCE_STOP);
                }

                // Retry after a delay.
                waitingSubscription = Observable.just(0).delay(1000, TimeUnit.MILLISECONDS)
                    .subscribeOn(Schedulers.newThread())
                    .observeOn(AndroidSchedulers.mainThread())
                    .subscribe(val -> waitForVisorToBeAvailableIfNeeded());
            }
        }
    }

    /**
     * Waits until there is connection via internet to at least one of the testing URLs set in the
     * globals class. After that, goes to the next step for starting the VPN protection. If this
     * step was already finished, the function does nothing.
     * @param firstTry True if the function is not being called automatically by the function
     *                 itself, to retry the operation.
     */
    private void checkInternetConnectionIfNeeded(boolean firstTry) {
        if (!waitNetworkFinished) {
            Skywiremob.printString("CHECKING CONNECTION");

            // Update the state.
            if (firstTry) {
                eventsSubject.onNext(VPNStates.CHECKING_CONNECTIVITY);
            }

            // Avoid having multiple simultaneous procedures.
            if (waitingSubscription != null) {
                waitingSubscription.dispose();
            }

            // Check if there is connection.
            waitingSubscription = HelperFunctions.checkInternetConnectivity(firstTry)
                .subscribeOn(Schedulers.newThread())
                .observeOn(AndroidSchedulers.mainThread())
                .subscribe(hasInternetConnection -> {
                    if (hasInternetConnection) {
                        // Go to the next step.
                        waitNetworkFinished = true;
                        startVisorIfNeeded();
                    } else {
                        eventsSubject.onNext(VPNStates.WAITING_FOR_CONNECTIVITY);
                        waitingSubscription.dispose();

                        // Retry after a delay.
                        waitingSubscription = Observable.just(0).delay(1000, TimeUnit.MILLISECONDS)
                            .subscribeOn(Schedulers.newThread())
                            .observeOn(AndroidSchedulers.mainThread())
                            .subscribe(val -> checkInternetConnectionIfNeeded(false));
                    }
                });
        }
    }

    /**
     * Starts the local visor. After that, goes to the next step for starting the VPN protection.
     * If this step was already started, the function does nothing.
     */
    private void startVisorIfNeeded() {
        if (visor == null) {
            Skywiremob.printString("STARTING VISOR");

            // Create the instance for managing the local visor.
            visor = new VisorRunnable();

            if (waitingSubscription != null) {
                waitingSubscription.dispose();
            }

            // Start the local visor and listen to the state changes.
            waitingSubscription = visor.runVisor()
                .subscribeOn(Schedulers.newThread())
                .observeOn(AndroidSchedulers.mainThread())
                .subscribe(state -> {
                    eventsSubject.onNext(state);

                    // Create an observable which stops the operation if there is no progress after
                    // some time. The observable is reset after each state change.
                    if (visorTimeoutSubscription != null) {
                        visorTimeoutSubscription.dispose();
                    }
                    visorTimeoutSubscription = Observable.just(0).delay(45000, TimeUnit.MILLISECONDS)
                        .subscribeOn(Schedulers.newThread())
                        .observeOn(AndroidSchedulers.mainThread())
                        .subscribe(val -> {
                            // Cancel the operation.
                            HelperFunctions.logError("VPN service", "Timeout preparing the visor.");
                            putInErrorState(App.getContext().getString(R.string.vpn_timeout_error));
                        });
                }, err -> {
                    // Report the error.
                    if (visorTimeoutSubscription != null) {
                        visorTimeoutSubscription.dispose();
                    }
                    putInErrorState(err.getLocalizedMessage());
                }, () -> {
                    // Go to the next step.
                    visorTimeoutSubscription.dispose();
                    startConnection();
                });
        }
    }

    /**
     * Starts the VPN connection, which finishes making the VPN protection functional.
     */
    private void startConnection() {
        if (vpnConnection == null) {
            // Create the instance for managing the connection.
            vpnConnection = new SkywireVPNConnection(visor, vpnInterface);

            waitingSubscription.dispose();

            // Make the connection work. Also, check the state changes.
            waitingSubscription = vpnConnection.getObservable()
                .subscribeOn(Schedulers.newThread())
                .observeOn(AndroidSchedulers.mainThread())
                .subscribe(
                    val -> {
                        // Inform the state changes.
                        eventsSubject.onNext(val);
                    }, err -> {
                        // Close the connection (this does not means that the network
                        // will be unblocked) and inform about the error.
                        putInErrorState(err.getLocalizedMessage());
                    }, () -> {
                        // This event is not expected, but it would mean that the vpn connection
                        // is not longer active.
                        HelperFunctions.logError("VPN connection ended unexpectedly", "VPN connection ended unexpectedly");
                        disconnect();
                    }
                );
        }
    }

    /**
     * Reverts all the steps made by this class, which means closing the connection and stopping
     * the visor. If the network connections were blocked, that does not change, as this function
     * does not make changes to the VPN work interface. Calling this function again after the
     * first call does nothing.
     */
    public void disconnect() {
        if (!disconnectionStarted) {
            disconnectionStarted = true;

            Skywiremob.printString("DISCONNECTING VPN RUNNABLE");

            // Inform the new state.
            eventsSubject.onNext(VPNStates.DISCONNECTING);

            // Remove the subscriptions and close the vpn connection.
            if (waitingSubscription != null) {
                waitingSubscription.dispose();
            }
            if (visorTimeoutSubscription != null) {
                visorTimeoutSubscription.dispose();
            }
            if (this.vpnConnection != null) {
                this.vpnConnection.close();
            }

            // Stop the visor in another thread.
            Observable.create((ObservableOnSubscribe<Integer>) emitter -> {
                if (visor != null) {
                    visor.startStoppingVisor();
                }
                emitter.onComplete();
            }).subscribeOn(Schedulers.newThread()).subscribe(val -> {});

            // Wait until the visor is completely stopped. 2 consecutive checks must be passed,
            // to avoid a very unlikely but possible race condition.
            Observable.timer(100, TimeUnit.MILLISECONDS).repeatUntil(() -> {
                if (!Skywiremob.isVisorStarting() && !Skywiremob.isVisorRunning()) {
                    if (disconnectionVerifications == 2) {
                        return true;
                    } else {
                        disconnectionVerifications += 1;
                    }
                } else {
                    if (disconnectionVerifications != 0) {
                        if (visor != null) {
                            visor.startStoppingVisor();
                        }
                    }

                    disconnectionVerifications = 0;
                }

                return false;
            })
            .subscribeOn(Schedulers.newThread())
            .observeOn(AndroidSchedulers.mainThread())
            .subscribe(val -> {}, err -> {}, () -> eventsSubject.onNext(VPNStates.DISCONNECTED));
        }
    }

    /**
     * Informs about an error and closes the VPN connection.
     * @param errorMsg Msg string of the error.
     */
    private void putInErrorState(String errorMsg) {
        lastErrorMsg = errorMsg;

        // If the network is already blocked and the kill switch is active, inform that the
        // current error will close the VPN connection but the network will still be blocked until
        // the user stops the service manually. That behavior is not managed by this class.
        if (!vpnInterface.alreadyConfigured() || !VPNGeneralPersistentData.getKillSwitchActivated()) {
            eventsSubject.onNext(VPNStates.ERROR);
        } else {
            eventsSubject.onNext(VPNStates.BLOCKING_ERROR);
        }

        disconnect();
    }

    /**
     * Returns the msg of the last error detected by the current instance.
     */
    public String getLastErrorMsg() {
        return lastErrorMsg;
    }
}
