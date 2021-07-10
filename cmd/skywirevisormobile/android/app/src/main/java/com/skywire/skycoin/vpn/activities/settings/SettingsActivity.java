package com.skywire.skycoin.vpn.activities.settings;

import android.content.Intent;
import android.os.Bundle;
import android.view.LayoutInflater;
import android.view.View;
import android.view.ViewGroup;

import androidx.annotation.NonNull;
import androidx.annotation.Nullable;
import androidx.fragment.app.Fragment;

import com.skywire.skycoin.vpn.activities.apps.AppsActivity;
import com.skywire.skycoin.vpn.controls.options.OptionsItem;
import com.skywire.skycoin.vpn.controls.options.OptionsModalWindow;
import com.skywire.skycoin.vpn.extensible.ClickEvent;
import com.skywire.skycoin.vpn.R;
import com.skywire.skycoin.vpn.helpers.Globals;
import com.skywire.skycoin.vpn.helpers.HelperFunctions;
import com.skywire.skycoin.vpn.vpn.VPNCoordinator;
import com.skywire.skycoin.vpn.vpn.VPNGeneralPersistentData;
import com.skywire.skycoin.vpn.vpn.VPNServersPersistentData;

import java.util.ArrayList;
import java.util.HashSet;

public class SettingsActivity extends Fragment implements ClickEvent {
    private SettingsOption optionApps;
    private SettingsOption optionShowIp;
    private SettingsOption optionKillSwitch;
    private SettingsOption optionResetAfterErrors;
    private SettingsOption optionProtectBeforeConnecting;
    private SettingsOption optionStartOnBoot;
    private SettingsOption optionDataUnits;
    private SettingsOption optionDns;

    // Units that must be used for displaying the data stats.
    private Globals.DataUnits dataUnitsOption = VPNGeneralPersistentData.getDataUnits();

    @Nullable
    @Override
    public View onCreateView(@NonNull LayoutInflater inflater, @Nullable ViewGroup container, @Nullable Bundle savedInstanceState) {
        super.onCreateView(inflater, container, savedInstanceState);

        return inflater.inflate(R.layout.activity_settings, container, true);
    }

    @Override
    public void onViewCreated(View view, Bundle savedInstanceState) {
        super.onViewCreated(view, savedInstanceState);

        optionApps = view.findViewById(R.id.optionApps);
        optionShowIp = view.findViewById(R.id.optionShowIp);
        optionKillSwitch = view.findViewById(R.id.optionKillSwitch);
        optionResetAfterErrors = view.findViewById(R.id.optionResetAfterErrors);
        optionProtectBeforeConnecting = view.findViewById(R.id.optionProtectBeforeConnecting);
        optionStartOnBoot = view.findViewById(R.id.optionStartOnBoot);
        optionDataUnits = view.findViewById(R.id.optionDataUnits);
        optionDns = view.findViewById(R.id.optionDns);

        optionShowIp.setChecked(VPNGeneralPersistentData.getShowIpActivated());
        optionKillSwitch.setChecked(VPNGeneralPersistentData.getKillSwitchActivated());
        optionResetAfterErrors.setChecked(VPNGeneralPersistentData.getMustRestartVpn());
        optionProtectBeforeConnecting.setChecked(VPNGeneralPersistentData.getProtectBeforeConnected());
        optionStartOnBoot.setChecked(VPNGeneralPersistentData.getStartOnBoot());

        optionApps.setClickEventListener(this);
        optionShowIp.setClickEventListener(this);
        optionKillSwitch.setClickEventListener(this);
        optionResetAfterErrors.setClickEventListener(this);
        optionProtectBeforeConnecting.setClickEventListener(this);
        optionStartOnBoot.setClickEventListener(this);
        optionDataUnits.setClickEventListener(this);
        optionDns.setClickEventListener(this);

        optionDataUnits.setDescription(getUnitsOptionText(dataUnitsOption), null);

        setDnsOptionText(VPNGeneralPersistentData.getCustomDns());
    }

    @Override
    public void onResume() {
        super.onResume();

        Globals.AppFilteringModes appsMode = VPNGeneralPersistentData.getAppsSelectionMode();
        if (appsMode == Globals.AppFilteringModes.PROTECT_ALL) {
            optionApps.setDescription(R.string.tmp_options_apps_description, null);
            optionApps.setChecked(false);
            optionApps.changeAlertIconVisibility(false);
        } else {
            HashSet<String> selectedApps = HelperFunctions.filterAvailableApps(VPNGeneralPersistentData.getAppList(new HashSet<>()));

            if (appsMode == Globals.AppFilteringModes.PROTECT_SELECTED) {
                optionApps.setDescription(R.string.tmp_options_apps_include_description, selectedApps.size() + "");
            } else if (appsMode == Globals.AppFilteringModes.IGNORE_SELECTED) {
                optionApps.setDescription(R.string.tmp_options_apps_exclude_description, selectedApps.size() + "");
            }

            optionApps.setChecked(true);
            optionApps.changeAlertIconVisibility(true);
        }
    }

    /**
     * Gets the ID of the string for a data units selection.
     */
    private int getUnitsOptionText(Globals.DataUnits units) {
        if (units == Globals.DataUnits.OnlyBits) {
            return R.string.tmp_options_data_units_only_bits;
        } else if (units == Globals.DataUnits.OnlyBytes) {
            return R.string.tmp_options_data_units_only_bytes;
        }

        return R.string.tmp_options_data_units_bits_speed_and_bytes_volume;
    }

    private void setDnsOptionText(String customIp) {
        if (customIp == null || customIp.trim().length() == 0) {
            optionDns.setDescription(R.string.tmp_options_dns_default, null);
            optionDns.changeAlertIconVisibility(false);
        } else {
            optionDns.setDescription(R.string.tmp_options_dns_description, customIp);
            optionDns.changeAlertIconVisibility(true);
        }
    }

    @Override
    public void onClick(View view) {
        if (view.getId() == R.id.optionDataUnits) {
            ArrayList<OptionsItem.SelectableOption> options = new ArrayList();
            Globals.DataUnits[] unitOptions = new Globals.DataUnits[3];
            unitOptions[0] = Globals.DataUnits.BitsSpeedAndBytesVolume;
            unitOptions[1] = Globals.DataUnits.OnlyBytes;
            unitOptions[2] = Globals.DataUnits.OnlyBits;

            for (Globals.DataUnits unitOption : unitOptions) {
                OptionsItem.SelectableOption option = new OptionsItem.SelectableOption();
                option.icon = dataUnitsOption == unitOption ? "\ue876" : null;
                option.translatableLabelId = getUnitsOptionText(unitOption);
                options.add(option);
            }

            OptionsModalWindow modal = new OptionsModalWindow(getContext(), null, options, (int selectedOption) -> {
                dataUnitsOption = unitOptions[selectedOption];
                optionDataUnits.setDescription(getUnitsOptionText(dataUnitsOption), null);
                VPNGeneralPersistentData.setDataUnits(dataUnitsOption);
            });
            modal.show();

            return;
        }

        if (VPNCoordinator.getInstance().isServiceRunning()) {
            HelperFunctions.showToast(getContext().getText(R.string.general_server_running_error).toString(), true);

            return;
        }

        if (view.getId() == R.id.optionApps) {
            Intent intent = new Intent(getContext(), AppsActivity.class);
            startActivity(intent);

            return;
        }

        if (view.getId() == R.id.optionDns) {
            CustomDnsModalWindow modal = new CustomDnsModalWindow(getContext(), (String newIp) -> {
                VPNGeneralPersistentData.setCustomDns(newIp);
                setDnsOptionText(newIp);

                HelperFunctions.showToast(getContext().getString(R.string.tmp_dns_changes_made_confirmation), true);
            });
            modal.show();
        }

        if (view.getId() == R.id.optionStartOnBoot && VPNServersPersistentData.getInstance().getCurrentServer() == null) {
            HelperFunctions.showToast(getContext().getText(R.string.tmp_options_start_on_boot_without_server_error).toString(), true);

            return;
        }

        ((SettingsOption)view).setChecked(!((SettingsOption)view).isChecked());

        if (view.getId() == R.id.optionShowIp) {
            VPNGeneralPersistentData.setShowIpActivated(((SettingsOption)view).isChecked());
        } else if (view.getId() == R.id.optionKillSwitch) {
            VPNGeneralPersistentData.setKillSwitchActivated(((SettingsOption)view).isChecked());
        } else if (view.getId() == R.id.optionResetAfterErrors) {
            VPNGeneralPersistentData.setMustRestartVpn(((SettingsOption)view).isChecked());
        } else if (view.getId() == R.id.optionProtectBeforeConnecting) {
            VPNGeneralPersistentData.setProtectBeforeConnected(((SettingsOption)view).isChecked());
        } else {
            VPNGeneralPersistentData.setStartOnBoot(((SettingsOption)view).isChecked());
        }
    }
}
