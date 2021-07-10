package com.skywire.skycoin.vpn.vpn;

import com.skywire.skycoin.vpn.App;
import com.skywire.skycoin.vpn.R;
import com.skywire.skycoin.vpn.helpers.HelperFunctions;
import com.skywire.skycoin.vpn.objects.LocalServerData;

import io.reactivex.rxjava3.core.Observable;
import io.reactivex.rxjava3.core.ObservableEmitter;
import io.reactivex.rxjava3.core.ObservableOnSubscribe;
import skywiremob.Skywiremob;

/**
 * Allows to easily control the starting and stopping procedures of the the visor and VPN client
 * included in Skywiremob.
 */
public class VisorRunnable {
    /**
     * If Skywiremob.prepareVPNClient has already been called without errors.
     */
    private boolean vpnClientStarted = false;
    /**
     * If Skywiremob.startListeningUDP() has already been called without errors.
     */
    private boolean listeningUdp = false;
    /**
     * If true, the initialization failed because the server refused the password.
     */
    private boolean passwordFailed = false;

    /**
     * Allows to know if the initialization failed because the server refused the password.
     */
    public boolean getIfPasswordFailed() {
        return passwordFailed;
    }

    /**
     * Starts stopping the visor. It returns before the visor has been completely stopped.
     */
    public void startStoppingVisor() {
        skywiremob.Error err = Skywiremob.stopVisor();
        if (err.getCode() != Skywiremob.ErrCodeNoError) {
            Skywiremob.printString(gerErrorMsg(err));
            HelperFunctions.showToast(gerErrorMsg(err), false);
        }
        Skywiremob.printString("Visor stopped");
    }

    /**
     * Stops the VPN client without stopping the visor.
     */
    public void stopVpnConnection() {
        if (vpnClientStarted) {
            Skywiremob.stopVPNClient();
            vpnClientStarted = false;
        }
        if (listeningUdp) {
            Skywiremob.stopListeningUDP();
            listeningUdp = false;
        }
        Skywiremob.printString("VPN connection stopped");
    }

    /**
     * Starts the Skywire visor.
     * @return Observable that will emit the current state of the process, as variables defined in
     * VPNStates, and will complete after starting the visor.
     */
    public Observable<VPNStates> runVisor() {
        return Observable.create((ObservableOnSubscribe<VPNStates>) emitter -> {
            if (emitter.isDisposed()) { return; }
            emitter.onNext(VPNStates.PREPARING_VISOR);

            // Start the visor if the emitter is still valid.
            if (emitter.isDisposed()) { return; }
            skywiremob.Error err = Skywiremob.prepareVisor();
            if (err.getCode() != Skywiremob.ErrCodeNoError) {
                HelperFunctions.logError("Visor startup procedure, code " + err.getCode(), gerErrorMsg(err));
                if (emitter.isDisposed()) { return; }
                emitter.onError(new Exception(gerErrorMsg(err)));
                return;
            }

            // Block the thread while the visor is starting.
            err = Skywiremob.waitVisorReady();
            if (err.getCode() != Skywiremob.ErrCodeNoError) {
                HelperFunctions.logError("Visor startup procedure, code " + err.getCode(), gerErrorMsg(err));
                if (emitter.isDisposed()) { return; }
                emitter.onError(new Exception(gerErrorMsg(err)));
                return;
            }

            // Finish.
            Skywiremob.printString("Prepared visor");
            if (emitter.isDisposed()) { return; }
            emitter.onNext(VPNStates.VISOR_READY);
            emitter.onComplete();
        });
    }

    /**
     * Starts the VPN client. This function was made to be used inside an observable which emits
     * the state of the VPN service.
     * @param parentEmitter Emitter of the observable from which this function was called, to be
     *                      able to emit the state changes.
     */
    public void runVpnClient(ObservableEmitter<VPNStates> parentEmitter) throws Exception {
        passwordFailed = false;

        // Update the state.
        if (parentEmitter.isDisposed()) { return; }
        parentEmitter.onNext(VPNStates.PREPARING_VPN_CLIENT);

        // Prepare the VPN client with the last saved public key and password.
        if (parentEmitter.isDisposed()) { return; }
        LocalServerData currentServer = VPNServersPersistentData.getInstance().getCurrentServer();
        String savedPk = currentServer != null ? currentServer.pk : "";
        String savedPassword = currentServer != null && currentServer.password != null ? currentServer.password : "";
        skywiremob.Error err = Skywiremob.prepareVPNClient(savedPk, savedPassword);
        if (err.getCode() != Skywiremob.ErrCodeNoError) {
            throw new Exception(gerErrorMsg(err));
        }
        vpnClientStarted = true;
        Skywiremob.printString("Prepared VPN client");
        if (parentEmitter.isDisposed()) { return; }
        parentEmitter.onNext(VPNStates.FINAL_PREPARATIONS_FOR_VISOR);

        // Perform the handshake.
        if (parentEmitter.isDisposed()) { return; }
        err = Skywiremob.shakeHands();
        if (err.getCode() != Skywiremob.ErrCodeNoError) {
            // Check if the server refused the password.
            if (err.getCode() == Skywiremob.ErrCodeHandshakeFailed && err.getError().toUpperCase().contains("4 (Forbidden)".toUpperCase())) {
                passwordFailed = true;
            }
            throw new Exception(gerErrorMsg(err));
        }

        // Start listening.
        if (parentEmitter.isDisposed()) { return; }
        err = Skywiremob.startListeningUDP();
        listeningUdp = true;
        if (err.getCode() != Skywiremob.ErrCodeNoError) {
            throw new Exception(gerErrorMsg(err));
        }

        // Start serving.
        if (parentEmitter.isDisposed()) { return; }
        err = Skywiremob.serveVPN();
        if (err.getCode() != Skywiremob.ErrCodeNoError) {
            throw new Exception(gerErrorMsg(err));
        }
    }

    /**
     * Gets the error string for an specific error returned by Skywiremob.
     */
    private static String gerErrorMsg(skywiremob.Error error) {
        int resource = -1;

        if (error.getCode() == Skywiremob.ErrCodeInvalidPK) {
            resource = R.string.skywiremob_error_invalid_pk;
        } else if (error.getCode() == Skywiremob.ErrCodeInvalidVisorConfig) {
            resource = R.string.skywiremob_error_invalid_visor_config;
        } else if (error.getCode() == Skywiremob.ErrCodeInvalidAddrResolverURL) {
            resource = R.string.skywiremob_error_invalid_addr_resolver_url;
        } else if (error.getCode() == Skywiremob.ErrCodeSTCPInitFailed) {
            resource = R.string.skywiremob_error_stcp_init_failed;
        } else if (error.getCode() == Skywiremob.ErrCodeSTCPRInitFailed) {
            resource = R.string.skywiremob_error_stcpr_init_failed;
        } else if (error.getCode() == Skywiremob.ErrCodeSUDPHInitFailed) {
            resource = R.string.skywiremob_error_sudph_init_failed;
        } else if (error.getCode() == Skywiremob.ErrCodeDmsgListenFailed) {
            resource = R.string.skywiremob_error_dmsg_listen_failed;
        } else if (error.getCode() == Skywiremob.ErrCodeTpDiscUnavailable) {
            resource = R.string.skywiremob_error_tp_disc_unavailable;
        } else if (error.getCode() == Skywiremob.ErrCodeFailedToStartRouter) {
            resource = R.string.skywiremob_error_failed_to_start_router;
        } else if (error.getCode() == Skywiremob.ErrCodeFailedToSetupHVGateway) {
            resource = R.string.skywiremob_error_failed_to_setup_hv_gateway;
        } else if (error.getCode() == Skywiremob.ErrCodeVisorNotRunning) {
            resource = R.string.skywiremob_error_visor_not_running;
        } else if (error.getCode() == Skywiremob.ErrCodeInvalidRemotePK) {
            resource = R.string.skywiremob_error_invalid_remote_pk;
        } else if (error.getCode() == Skywiremob.ErrCodeFailedToSaveTransport) {
            resource = R.string.skywiremob_error_failed_to_save_transport;
        } else if (error.getCode() == Skywiremob.ErrCodeVPNServerUnavailable) {
            resource = R.string.skywiremob_error_vpn_server_unavailable;
        } else if (error.getCode() == Skywiremob.ErrCodeVPNClientNotRunning) {
            resource = R.string.skywiremob_error_vpn_client_not_running;
        } else if (error.getCode() == Skywiremob.ErrCodeHandshakeFailed) {
            if (error.getError().toUpperCase().contains("4 (Forbidden)".toUpperCase())) {
                resource = R.string.skywiremob_error_wrong_password;
            } else {
                resource = R.string.skywiremob_error_handshake_failed;
            }
        } else if (error.getCode() == Skywiremob.ErrCodeInvalidAddr) {
            resource = R.string.skywiremob_error_invalid_addr;
        } else if (error.getCode() == Skywiremob.ErrCodeAlreadyListeningUDP) {
            resource = R.string.skywiremob_error_already_listening_udp;
        } else if (error.getCode() == Skywiremob.ErrCodeUDPListenFailed) {
            resource = R.string.skywiremob_error_udp_listen_failed;
        }

        String response;
        if (resource != -1) {
            response = App.getContext().getString(resource);
        } else {
            response = error.getError();
            if (response == null || response.trim().equals("")) {
                response = App.getContext().getString(R.string.skywiremob_error_unknown);
            }
        }

        return response;
    }
}
