package com.skywire.skycoin.vpn.vpn;

import com.skywire.skycoin.vpn.R;

import java.util.HashMap;

/**
 * Helper class with the possible states of the VPN service.
 *
 * The states are numeric constants, similar to how http status codes work, to be able to identify
 * state groups just by numeric ranges. The ranges are:
 *
 * State < 10: the service is not running.
 *
 * 10 =< State < 100: The VPN connection is being prepared.
 *
 * 100 =< State < 150: The VPN connection has been made and the internet connectivity should
 * be protected and working.
 *
 * 150 =< State < 200: Temporal errors with the VPN connection.
 *
 * 200 =< State < 300: Closing the VPN connection/service.
 *
 * 300 =< State < 400: VPN connection/service closed.
 *
 * State >= 400 : An error occurred.
 */
public enum VPNStates {
    /**
     * The service is off.
     */
    OFF(1),
    /**
     * Starting the service.
     */
    STARTING(10),
    /**
     * Waiting for the visor to be completely stopped before starting it again.
     */
    WAITING_PREVIOUS_INSTANCE_STOP(12),
    /**
     * Checking for the first time if the device has internet connectivity.
     */
    CHECKING_CONNECTIVITY(15),
    /**
     * No internet connectivity was found and the service is checking again periodically.
     */
    WAITING_FOR_CONNECTIVITY(16),
    /**
     * Starting the Skywire visor.
     */
    PREPARING_VISOR(20),
    /**
     * Starting the VPN client, which is part of Skywiremob and running as part of the visor.
     */
    PREPARING_VPN_CLIENT(30),
    /**
     * Making final preparations for the VPN client, like performing the handshake and start serving.
     */
    FINAL_PREPARATIONS_FOR_VISOR(35),
    /**
     * The visor and VPN client are ready. Preparations may be needed in the app side.
     */
    VISOR_READY(40),
    /**
     * The VPN connection has been fully established and secure internet connectivity should
     * be available.
     */
    CONNECTED(100),
    /**
     * There was an error with the VPN connection and it is being restored automatically.
     */
    RESTORING_VPN(150),
    /**
     * There was an error and the whole VPN service is being restored automatically.
     */
    RESTORING_SERVICE(155),
    /**
     * The VPN service is being stopped.
     */
    DISCONNECTING(200),
    /**
     * The VPN service has been stopped.
     */
    DISCONNECTED(300),
    /**
     * There has been an error, the VPN connection is not available and the service is
     * being stopped.
     */
    ERROR(400),
    /**
     * There has been and error and the VPN connection is not available. The network will remain
     * blocked until the user stops the service manually.
     */
    BLOCKING_ERROR(410);

    /**
     * Allows to easily get the value related to an specific number.
     */
    private static HashMap<Integer, VPNStates> numericValues;

    // Initializes the enum and saves the value.
    private final int val;
    VPNStates(int val) {
        this.val = val;
    }

    /**
     * Gets the associated numeric value.
     */
    public int val() {
        return val;
    }

    /**
     * Class with details about the state of the VPN service.
     */
    public static class StateInfo {
        /**
         * Current state of the service.
         */
        public final VPNStates state;
        /**
         * If the service was started by the OS, which means that the OS is responsible for
         * stopping it.
         */
        public final boolean startedByTheSystem;
        /**
         * If the user already requested the service to be stopped.
         */
        public final boolean stopRequested;

        public StateInfo(VPNStates state, boolean startedByTheSystem, boolean stopRequested) {
            this.state = state;
            this.startedByTheSystem = startedByTheSystem;
            this.stopRequested = stopRequested;
        }
    }

    /**
     * Allows to get the resource ID of the string with the title for a state of the
     * VPN service. If no resource is found for the state, -1 is returned.
     */
    public static int getTitleForState(VPNStates state) {
        if (state == OFF) {
            return R.string.vpn_state_disconnected;
        } else if (state == STARTING) {
            return R.string.vpn_state_connecting;
        } else if (state == WAITING_PREVIOUS_INSTANCE_STOP) {
            return R.string.vpn_state_connecting;
        } else if (state == CHECKING_CONNECTIVITY) {
            return R.string.vpn_state_connecting;
        } else if (state == WAITING_FOR_CONNECTIVITY) {
            return R.string.vpn_state_connecting;
        } else if (state == PREPARING_VISOR) {
            return R.string.vpn_state_connecting;
        } else if (state == PREPARING_VPN_CLIENT) {
            return R.string.vpn_state_connecting;
        } else if (state == FINAL_PREPARATIONS_FOR_VISOR) {
            return R.string.vpn_state_connecting;
        } else if (state == VISOR_READY) {
            return R.string.vpn_state_connecting;
        } else if (state == CONNECTED) {
            return R.string.vpn_state_connected;
        } else if (state == RESTORING_VPN) {
            return R.string.vpn_state_restarting;
        } else if (state == RESTORING_SERVICE) {
            return R.string.vpn_state_restarting;
        } else if (state == DISCONNECTING) {
            return R.string.vpn_state_disconnecting;
        } else if (state == DISCONNECTED) {
            return R.string.vpn_state_disconnected;
        } else if (state == ERROR) {
            return R.string.vpn_state_error;
        } else if (state == BLOCKING_ERROR) {
            return R.string.vpn_state_error;
        }

        return -1;
    }

    /**
     * Allows to get the resource ID of the color for the title of a state of the
     * VPN service. If no resource is found for the title, red is returned.
     */
    public static int getColorForStateTitle(int titleResource) {
        if (titleResource == R.string.vpn_state_disconnected) {
            return R.color.red;
        } else if (titleResource == R.string.vpn_state_connecting) {
            return R.color.yellow;
        } else if (titleResource == R.string.vpn_state_connected) {
            return R.color.green;
        } else if (titleResource == R.string.vpn_state_restarting) {
            return R.color.yellow;
        } else if (titleResource == R.string.vpn_state_disconnecting) {
            return R.color.yellow;
        } else if (titleResource == R.string.vpn_state_error) {
            return R.color.red;
        }

        return R.color.red;
    }

    /**
     * Allows to get the resource ID of the string with the description of a state of the
     * VPN service. If no resource is found for the state, -1 is returned.
     */
    public static int getDescriptionForState(VPNStates state) {
        if (state == OFF) {
            return R.string.vpn_state_details_off;
        } else if (state == STARTING) {
            return R.string.vpn_state_details_initializing;
        } else if (state == WAITING_PREVIOUS_INSTANCE_STOP) {
            return R.string.vpn_state_details_waiting_previous_instance_stop;
        } else if (state == CHECKING_CONNECTIVITY) {
            return R.string.vpn_state_details_checking_connectivity;
        } else if (state == WAITING_FOR_CONNECTIVITY) {
            return R.string.vpn_state_details_waiting_connectivity;
        } else if (state == PREPARING_VISOR) {
            return R.string.vpn_state_details_starting_visor;
        } else if (state == PREPARING_VPN_CLIENT) {
            return R.string.vpn_state_details_starting_vpn_app;
        } else if (state == FINAL_PREPARATIONS_FOR_VISOR) {
            return R.string.vpn_state_details_additional_visor_initializations;
        } else if (state == VISOR_READY) {
            return R.string.vpn_state_details_connecting;
        } else if (state == CONNECTED) {
            return R.string.vpn_state_details_connected;
        } else if (state == RESTORING_VPN) {
            return R.string.vpn_state_details_restoring;
        } else if (state == RESTORING_SERVICE) {
            return R.string.vpn_state_details_restoring_service;
        } else if (state == DISCONNECTING) {
            return R.string.vpn_state_details_disconnecting;
        } else if (state == DISCONNECTED) {
            return R.string.vpn_state_details_disconnected;
        } else if (state == ERROR) {
            return R.string.vpn_state_details_error;
        } else if (state == BLOCKING_ERROR) {
            return R.string.vpn_state_details_blocking_error;
        }

        return -1;
    }

    /**
     * Allows to get the value associated with a numeric value. If there is no value for the
     * provided number, the OFF state is returned.
     * @param value Value to check.
     */
    public static VPNStates valueOf(int value) {
        // Initialize the map for getting the values, if needed.
        if (numericValues == null) {
            numericValues = new HashMap<>();

            for (VPNStates v : VPNStates.values()) {
                numericValues.put(v.val(), v);
            }
        }

        if (!numericValues.containsKey(value)) {
            return OFF;
        }

        return numericValues.get(value);
    }
}
