package com.skywire.skycoin.vpn;

import android.content.BroadcastReceiver;
import android.content.Context;
import android.content.Intent;

import com.skywire.skycoin.vpn.vpn.VPNCoordinator;
import com.skywire.skycoin.vpn.vpn.VPNGeneralPersistentData;

/**
 * Class for receiving the system boot event broadcast.
 */
public class Receiver extends BroadcastReceiver {
    public void onReceive(Context context, Intent intent) {
        if (intent.getAction().equals(Intent.ACTION_BOOT_COMPLETED)) {
            // If the option for starting the service automatically after booting the OS is active
            // and the service is not currently running, start the service.
            if (VPNGeneralPersistentData.getStartOnBoot() && !VPNCoordinator.getInstance().isServiceRunning()) {
                VPNCoordinator.getInstance().activateAutostart();
            }
        }
    }
}
