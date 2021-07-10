package com.skywire.skycoin.vpn.vpn;

import android.net.VpnService;
import android.os.ParcelFileDescriptor;

import com.skywire.skycoin.vpn.App;
import com.skywire.skycoin.vpn.helpers.Globals;
import com.skywire.skycoin.vpn.helpers.HelperFunctions;
import com.skywire.skycoin.vpn.R;

import java.io.Closeable;
import java.io.FileInputStream;
import java.io.FileOutputStream;
import java.io.IOException;
import java.util.HashSet;

import skywiremob.Skywiremob;

/**
 * Object used for starting the VPN protection and sending/receiving data. After created, to start
 * the VPN protection the object must be configured.
 */
public class VPNWorkInterface implements Closeable {
    /**
     * Modes in which the VPN interface can be configured.
     */
    public enum Modes {
        /**
         * Used just for blocking the network connectivity before configuring the visor, to avoid
         * data leaks.
         */
        BLOCKING,
        /**
         * Normal mode for sending and receiving data using the VPN protection.
         */
        WORKING,
        /**
         * Mode used just for configuring a VPN interface and closing it immediately after that, to
         * force the OS to disable the VPN protection, due to a bug in old Android versions.
         */
        DELETING,
    }

    /**
     * Current VPN service instance.
     */
    private final VpnService service;
    /**
     * Current VPN communication object, created by the system.
     */
    private ParcelFileDescriptor vpnInterface = null;
    /**
     * Input stream to be used with the current communication object created by the system.
     */
    private FileInputStream inStream = null;
    /**
     * Output stream to be used with the current communication object created by the system.
     */
    private FileOutputStream outStream = null;

    public VPNWorkInterface(VpnService service) {
        this.service = service;
    }

    /**
     * Terminates the VPN protections and cleans the used resources.
     */
    @Override
    public void close() {
        if (vpnInterface != null) {
            try {
                vpnInterface.close();
                vpnInterface = null;
            } catch (IOException e) {
                HelperFunctions.logError("Unable to close interface", e);
            }

            cleanInputStream();
            cleanOutputStream();
        }
    }

    /**
     * Checks if the interface has already been configured for the first time.
     */
    public boolean alreadyConfigured() {
        return vpnInterface != null;
    }

    /**
     * Configures and activates the VPN interface. After calling this function the OS starts
     * routing the data using the interface, so all network connections will be blocked if the VPN
     * is not working properly. This method can be called several times, which allows to restore
     * the connection in case of errors or change the mode.
     * @param mode Mode in which the VPN interface will be configured.
     */
    public void configure(Modes mode) throws Exception {
        // Save a reference to the current interface, if any, to close it after creating the
        // new one, to avoid leaking data while the new interface is created.
        ParcelFileDescriptor oldVpnInterface = null;
        if (vpnInterface != null) {
            oldVpnInterface = vpnInterface;
        }

        // Create and configure a builder.
        VpnService.Builder builder = service.new Builder();
        builder.setMtu((short)Skywiremob.getMTU());
        if (mode == Modes.WORKING) {
            Skywiremob.printString("TUN IP: " + Skywiremob.tunip());
            // Get the address from the visor.
            builder.addAddress(Skywiremob.tunip(), (int) Skywiremob.getTUNIPPrefix());
        } else {
            // Use an address for blocking all connections.
            builder.addAddress("8.8.8.8", 32);
        }

        // Use the custom DNS server, if any.
        String dnsServer = VPNGeneralPersistentData.getCustomDns();
        if (dnsServer != null && dnsServer.trim().length() > 0) {
            builder.addDnsServer(dnsServer.trim());
        }

        builder.addRoute("0.0.0.0", 0);
        // This makes the streams created with the interface synchronous, so that the data can be
        // read blocking an independent thread in an efficient way.
        builder.setBlocking(true);

        // Allows to know if there was an error allowing or disallowing apps.
        boolean errorIgnoringApps = false;

        if (mode == Modes.WORKING || mode == Modes.BLOCKING) {
            String upperCaseAppPackage = App.getContext().getPackageName().toUpperCase();
            Globals.AppFilteringModes appsSelectionMode = VPNGeneralPersistentData.getAppsSelectionMode();

            if (appsSelectionMode != Globals.AppFilteringModes.PROTECT_ALL) {
                // Get the package name of all the apps selected by the user which are
                // currently installed.
                for (String packageName : HelperFunctions.filterAvailableApps(VPNGeneralPersistentData.getAppList(new HashSet<>()))) {
                    try {
                        if (appsSelectionMode == Globals.AppFilteringModes.PROTECT_SELECTED) {
                            // Protect all selected apps, but ignore this app.
                            if (!upperCaseAppPackage.equals(packageName.toUpperCase())) {
                                builder.addAllowedApplication(packageName);
                            }
                        } else {
                            // Avoid protecting the selected apps, but ignore this app.
                            if (!upperCaseAppPackage.equals(packageName.toUpperCase())) {
                                builder.addDisallowedApplication(packageName);
                            }
                        }
                    } catch (Exception e) {
                        errorIgnoringApps = true;
                        HelperFunctions.logError("Unable to add " + packageName + " to the VPN service", e);
                        break;
                    }
                }
            }

            // Make the VPN protection ignore this app, as free access is needed for configuring
            // the visor, specially in case of errors, when it is needed to restart components.
            if (!errorIgnoringApps) {
                try {
                    if (appsSelectionMode != Globals.AppFilteringModes.PROTECT_SELECTED) {
                        builder.addDisallowedApplication(App.getContext().getPackageName());
                    }
                } catch (Exception e) {
                    errorIgnoringApps = true;
                    HelperFunctions.logError("Unable to add VPN app rule to the VPN service", e);
                }
            }
        } else {
            // Block this app only, to be able to avoid a bug in old Android versions.
            builder.addAllowedApplication(App.getContext().getPackageName());
        }

        if (errorIgnoringApps) {
            throw new Exception(App.getContext().getString(R.string.vpn_service_configuring_app_rules_error));
        }

        // Create the new interface using the builder.
        builder.setConfigureIntent(HelperFunctions.getOpenAppPendingIntent());
        synchronized (service) {
            vpnInterface = builder.establish();
        }
        Skywiremob.printString("New interface: " + vpnInterface);

        // Close the previous interface and streams, if any.
        if (oldVpnInterface != null) {
            oldVpnInterface.close();
        }
        cleanInputStream();
        cleanOutputStream();
    }

    /**
     * Gets the input stream for reading the packages from the system that must be sent using the
     * VPN. NOTE: if the interface is closed or configured again, the stream is closed.
     */
    public FileInputStream getInputStream() {
        if (inStream == null) {
            inStream = new FileInputStream(vpnInterface.getFileDescriptor());
        }
        return inStream;
    }

    /**
     * Gets the output stream that must be used for sending to the system the packages received via
     * the VPN. NOTE: if the interface is closed or configured again, the stream is closed.
     */
    public FileOutputStream getOutputStream() {
        if (outStream == null) {
            outStream = new FileOutputStream(vpnInterface.getFileDescriptor());
        }
        return outStream;
    }

    /**
     * Cleans and removes the current input stream, if any.
     */
    private void cleanInputStream() {
        if (inStream != null) {
            try {
                inStream.close();
            } catch (Exception e) { }

            inStream = null;
        }
    }

    /**
     * Cleans and removes the current output stream, if any.
     */
    private void cleanOutputStream() {
        if (outStream != null) {
            try {
                outStream.close();
            } catch (Exception e) { }

            outStream = null;
        }
    }
}
