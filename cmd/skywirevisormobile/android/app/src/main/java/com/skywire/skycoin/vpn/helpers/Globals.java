package com.skywire.skycoin.vpn.helpers;

import androidx.annotation.NonNull;

/**
 * Constant values used in various parts of the app.
 */
public class Globals {
    /**
     * Time to wait before sending a click event after the user clicks a button. This is for
     * allowing the UI to show the click effect.
     */
    public static final int CLICK_DELAY_MS = 150;
    /**
     * Address of the local Skywire node.
     */
    public static final String LOCAL_VISOR_ADDRESS = "localhost";
    /**
     * Port of the local Skywire node.
     */
    public static final int LOCAL_VISOR_PORT = 7890;

    /**
     * Addresses used for checking if the device has internet connectivity. Any number of
     * addresses, but at least 1, can be used. Addresses will be checked sequentially and only
     * until being able to connect with one.
     */
    public static final String[] INTERNET_CHECKING_ADDRESSES = new String[]{"https://dmsg.discovery.skywire.skycoin.com", "https://www.skycoin.com"};

    /**
     * Options for how to show the VPN data transmission stats.
     */
    public enum DataUnits {
        BitsSpeedAndBytesVolume,
        OnlyBytes,
        OnlyBits,
    }

    /**
     * List with all the possible app selection modes. Each option has an associated string value.
     */
    public enum AppFilteringModes {
        /**
         * All apps must be protected by the VPN service, no matter which apps have been selected
         * by the user.
         */
        PROTECT_ALL("PROTECT_ALL"),
        /**
         * Only the apps selected by the user must be protected by the VPN service.
         */
        PROTECT_SELECTED("PROTECT_SELECTED"),
        /**
         * Apps selected by the user must NOT be protected by the VPN service. All other apps
         * must be protected.
         */
        IGNORE_SELECTED("IGNORE_SELECTED");

        private final String val;

        AppFilteringModes(final String val) {
            this.val = val;
        }

        @NonNull
        @Override
        public String toString() {
            return val;
        }
    }
}
