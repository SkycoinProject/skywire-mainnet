package com.skywire.skycoin.vpn.vpn;

import android.content.SharedPreferences;

import androidx.preference.PreferenceManager;

import com.google.gson.Gson;
import com.skywire.skycoin.vpn.App;
import com.skywire.skycoin.vpn.helpers.Globals;

import java.util.HashSet;

import io.reactivex.rxjava3.core.Observable;
import io.reactivex.rxjava3.subjects.BehaviorSubject;

/**
 * Helper class for saving and getting general data related to the VPN to and from the
 * persistent storage.
 */
public class VPNGeneralPersistentData {
    // Keys for persistent storage.
    private static final String LAST_ERROR = "lastError";
    private static final String DATA_UNITS = "dataUnits";
    private static final String CUSTOM_DNS = "customDns";
    private static final String APPS_SELECTION_MODE = "appsMode";
    private static final String APPS_LIST = "appsList";
    private static final String SHOW_IP = "showIp";
    private static final String KILL_SWITCH = "killSwitch";
    private static final String RESTART_VPN = "restartVpn";
    private static final String START_ON_BOOT = "startOnBoot";
    private static final String PROTECT_BEFORE_CONNECTED = "protectBeforeConnected";

    private static final SharedPreferences settings = PreferenceManager.getDefaultSharedPreferences(App.getContext());

    private static BehaviorSubject<Globals.DataUnits> dataUnitsSubject;

    /////////////////////////////////////////////////////////////
    // Setters.
    /////////////////////////////////////////////////////////////

    /**
     * Saves the message of the error which caused the VPN service to fail the last time it
     * ran, if any.
     */
    public static void setLastError(String val) {
        settings.edit().putString(LAST_ERROR, val).apply();
    }

    /**
     * Saves the data units that must be shown in the UI.
     */
    public static void setDataUnits(Globals.DataUnits val) {
        Gson gson = new Gson();
        String valString = gson.toJson(val);
        settings.edit().putString(DATA_UNITS, valString).apply();

        // Inform the change.
        if (dataUnitsSubject != null) {
            dataUnitsSubject.onNext(val);
        }
    }

    /**
     * Saves the IP of the custom DNS server.
     */
    public static void setCustomDns(String val) {
        settings.edit().putString(CUSTOM_DNS, val).apply();
    }

    /**
     * Saves the mode the VPN service must use to protect or ignore the apps selected by the user.
     */
    public static void setAppsSelectionMode(Globals.AppFilteringModes val) {
        settings.edit().putString(APPS_SELECTION_MODE, val.toString()).apply();
    }

    /**
     * Saves the list with the package names of all apps selected by the user in the app list.
     */
    public static void setAppList(HashSet<String> val) {
        settings.edit().putStringSet(APPS_LIST, val).apply();
    }

    /**
     * Sets if the functionality for showing the IP must be active.
     */
    public static void setShowIpActivated(boolean val) {
        settings.edit().putBoolean(SHOW_IP, val).apply();
    }

    /**
     * Sets if the kill switch functionality must be active.
     */
    public static void setKillSwitchActivated(boolean val) {
        settings.edit().putBoolean(KILL_SWITCH, val).apply();
    }

    /**
     * Sets if the VPN connection must be automatically restarted if there is an error.
     */
    public static void setMustRestartVpn(boolean val) {
        settings.edit().putBoolean(RESTART_VPN, val).apply();
    }

    /**
     * Sets if the VPN protection must be activated as soon as possible after booting the OS.
     */
    public static void setStartOnBoot(boolean val) {
        settings.edit().putBoolean(START_ON_BOOT, val).apply();
    }

    /**
     * Sets if the network protection must be activated just after starting the VPN service, which
     * would disable the internet connectivity for the rest of the apps while configuring the visor.
     */
    public static void setProtectBeforeConnected(boolean val) {
        settings.edit().putBoolean(PROTECT_BEFORE_CONNECTED, val).apply();
    }

    /////////////////////////////////////////////////////////////
    // Getters.
    /////////////////////////////////////////////////////////////

    /**
     * Gets the message of the error which caused the VPN service to fail the last time it
     * ran, if any.
     * @param defaultValue Value to return if no saved data is found.
     */
    public static String getLastError(String defaultValue) {
        return settings.getString(LAST_ERROR, defaultValue);
    }

    /**
     * Returns the data units that must be shown in the UI. If the user has not changed
     * the setting, it returns DataUnits.BitsSpeedAndBytesVolume by default.
     */
    public static Globals.DataUnits getDataUnits() {
        Gson gson = new Gson();
        String savedVal = settings.getString(DATA_UNITS, null);
        if (savedVal != null) {
            return gson.fromJson(savedVal, Globals.DataUnits.class);
        }

        return Globals.DataUnits.BitsSpeedAndBytesVolume;
    }

    /**
     * Emits every time the data units that must be shown in the UI are changed. It emits the most
     * recent value immediately after subscription.
     */
    public static Observable<Globals.DataUnits> getDataUnitsObservable() {
        if (dataUnitsSubject == null) {
            dataUnitsSubject = BehaviorSubject.create();
            dataUnitsSubject.onNext(getDataUnits());
        }

        return dataUnitsSubject.hide();
    }

    /**
     * Gets the IP of the custom DNS server.
     */
    public static String getCustomDns() {
        return settings.getString(CUSTOM_DNS, null);
    }

    /**
     * Gets the mode the VPN service must use to protect or ignore the apps selected by the user.
     */
    public static Globals.AppFilteringModes getAppsSelectionMode() {
        String savedValue = settings.getString(APPS_SELECTION_MODE, null);

        if (savedValue == null || savedValue.equals(Globals.AppFilteringModes.PROTECT_ALL.toString())) {
            return Globals.AppFilteringModes.PROTECT_ALL;
        } else if (savedValue.equals(Globals.AppFilteringModes.PROTECT_SELECTED.toString())) {
            return Globals.AppFilteringModes.PROTECT_SELECTED;
        } else if (savedValue.equals(Globals.AppFilteringModes.IGNORE_SELECTED.toString())) {
            return Globals.AppFilteringModes.IGNORE_SELECTED;
        }

        return Globals.AppFilteringModes.PROTECT_ALL;
    }

    /**
     * Gets the list with the package names of all apps selected by the user in the app list.
     * @param defaultValue Value to return if no saved data is found.
     */
    public static HashSet<String> getAppList(HashSet<String> defaultValue) {
        return new HashSet<>(settings.getStringSet(APPS_LIST, defaultValue));
    }

    /**
     * Gets if the functionality for showing the IP must be active.
     */
    public static boolean getShowIpActivated() {
        return settings.getBoolean(SHOW_IP, true);
    }

    /**
     * Gets if the kill switch functionality must be active.
     */
    public static boolean getKillSwitchActivated() {
        return settings.getBoolean(KILL_SWITCH, true);
    }

    /**
     * Gets if the VPN connection must be automatically restarted if there is an error.
     */
    public static boolean getMustRestartVpn() {
        return settings.getBoolean(RESTART_VPN, true);
    }

    /**
     * Gets if the VPN protection must be activated as soon as possible after booting the OS.
     */
    public static boolean getStartOnBoot() {
        return settings.getBoolean(START_ON_BOOT, false);
    }

    /**
     * Gets if the network protection must be activated just after starting the VPN service, which
     * would disable the internet connectivity for the rest of the apps while configuring the visor.
     */
    public static boolean getProtectBeforeConnected() {
        return settings.getBoolean(PROTECT_BEFORE_CONNECTED, true);
    }

    /////////////////////////////////////////////////////////////
    // Other operations.
    /////////////////////////////////////////////////////////////

    /**
     * Removes the message of the error which caused the VPN service to fail the last time it ran.
     */
    public static void removeLastError() {
        settings.edit().remove(LAST_ERROR).apply();
    }
}
