package com.skywire.skycoin.vpn.activities.main;

import android.content.Intent;
import android.os.Bundle;
import android.view.View;
import android.widget.Button;
import android.widget.EditText;
import android.widget.TextView;

import androidx.annotation.NonNull;
import androidx.appcompat.app.AppCompatActivity;

import com.skywire.skycoin.vpn.R;
import com.skywire.skycoin.vpn.activities.settings.SettingsActivity;
import com.skywire.skycoin.vpn.activities.start.StartActivity;
import com.skywire.skycoin.vpn.helpers.Notifications;
import com.skywire.skycoin.vpn.objects.LocalServerData;
import com.skywire.skycoin.vpn.objects.ManualVpnServerData;
import com.skywire.skycoin.vpn.vpn.VPNCoordinator;
import com.skywire.skycoin.vpn.vpn.VPNServersPersistentData;
import com.skywire.skycoin.vpn.vpn.VPNGeneralPersistentData;
import com.skywire.skycoin.vpn.vpn.VPNStates;
import com.skywire.skycoin.vpn.activities.apps.AppsActivity;
import com.skywire.skycoin.vpn.activities.servers.ServersActivity;
import com.skywire.skycoin.vpn.helpers.Globals;
import com.skywire.skycoin.vpn.helpers.HelperFunctions;

import java.util.HashSet;

import io.reactivex.rxjava3.disposables.Disposable;
import skywiremob.Skywiremob;

public class MainActivity extends AppCompatActivity implements View.OnClickListener {

    private EditText editTextRemotePK;
    private EditText editTextPasscode;
    private Button buttonStart;
    private Button buttonStop;
    private Button buttonSelect;
    private Button buttonApps;
    private Button buttonSettings;
    private Button buttonStartPage;
    private TextView textLastError1;
    private TextView textLastError2;
    private TextView textStatus;
    private TextView textFinishAlert;
    private TextView textStopAlert;

    private Disposable serviceSubscription;

    @Override
    protected void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        setContentView(R.layout.activity_main);

        editTextRemotePK = findViewById(R.id.editTextRemotePK);
        editTextPasscode = findViewById(R.id.editTextPasscode);
        buttonStart = findViewById(R.id.buttonStart);
        buttonStop = findViewById(R.id.buttonStop);
        buttonSelect = findViewById(R.id.buttonSelect);
        buttonApps = findViewById(R.id.buttonApps);
        buttonSettings = findViewById(R.id.buttonSettings);
        buttonStartPage = findViewById(R.id.buttonStartPage);
        textStatus = findViewById(R.id.textStatus);
        textFinishAlert = findViewById(R.id.textFinishAlert);
        textLastError1 = findViewById(R.id.textLastError1);
        textLastError2 = findViewById(R.id.textLastError2);
        textStopAlert = findViewById(R.id.textStopAlert);

        buttonStart.setOnClickListener(this);
        buttonStop.setOnClickListener(this);
        buttonSelect.setOnClickListener(this);
        buttonApps.setOnClickListener(this);
        buttonSettings.setOnClickListener(this);
        buttonStartPage.setOnClickListener(this);

        LocalServerData currentServer = VPNServersPersistentData.getInstance().getCurrentServer();
        String savedPk = currentServer != null ? currentServer.pk : null;
        String savedPassword = currentServer != null && currentServer.password != null ? currentServer.password : "";

        if (savedPk != null && savedPassword != null) {
            editTextRemotePK.setText(savedPk);
            editTextPasscode.setText(savedPassword);
        }
    }

    @Override
    public void onRestoreInstanceState(Bundle savedInstanceState) {
        editTextRemotePK.setText(savedInstanceState.getString("pk"));
        editTextPasscode.setText(savedInstanceState.getString("password"));
    }

    @Override
    public void onSaveInstanceState(@NonNull Bundle savedInstanceState) {
        super.onSaveInstanceState(savedInstanceState);
        savedInstanceState.putString("pk", editTextRemotePK.getText().toString());
        savedInstanceState.putString("password", editTextPasscode.getText().toString());
    }

    @Override
    protected void onStart() {
        super.onStart();

        Notifications.removeAllAlertNotifications();

        displayInitialState();

        serviceSubscription = VPNCoordinator.getInstance().getEventsObservable().subscribe(
            state -> {
                if (state.state.val() < 10) {
                    displayInitialState();
                } else if (state.state != VPNStates.ERROR && state.state != VPNStates.BLOCKING_ERROR && state.state != VPNStates.DISCONNECTED) {
                    int stateText = VPNStates.getDescriptionForState(state.state);

                    displayWorkingState();

                    if (state.startedByTheSystem) {
                        this.buttonStop.setEnabled(false);
                        textStopAlert.setVisibility(View.VISIBLE);
                    }

                    if (state.stopRequested) {
                        this.buttonStop.setEnabled(false);
                    }

                    if (stateText != -1) {
                        textStatus.setText(stateText);
                    }
                } else if (state.state == VPNStates.DISCONNECTED) {
                    textStatus.setText(R.string.vpn_state_disconnected);
                    displayInitialState();
                } else {
                    textStatus.setText(VPNStates.getDescriptionForState(state.state));
                    displayErrorState(state.stopRequested);
                }
            }
        );
    }

    @Override
    protected void onStop() {
        super.onStop();

        serviceSubscription.dispose();
    }

    @Override
    public void onClick(View view) {
        switch (view.getId()) {
            case R.id.buttonStart:
                start();
                break;
            case R.id.buttonStop:
                stop();
                break;
            case R.id.buttonSelect:
                selectServer();
                break;
            case R.id.buttonApps:
                selectApps();
                break;
            case R.id.buttonSettings:
                openSettings();
                break;
            case R.id.buttonStartPage:
                openStarPage();
                break;
        }
    }

    @Override
    protected void onActivityResult(int request, int result, Intent data) {
        super.onActivityResult(request, result, data);

        if (request == VPNCoordinator.VPN_PREPARATION_REQUEST_CODE) {
            VPNCoordinator.getInstance().onActivityResult(request, result, data);
        } else if (request == 1 && data != null) {
            String address = data.getStringExtra(ServersActivity.ADDRESS_DATA_PARAM);
            if (address != null) {
                editTextRemotePK.setText(address);
                editTextPasscode.setText("");
            }

            start();
        }
    }

    private void start() {
        // Check if the pk is valid.
        String remotePK = editTextRemotePK.getText().toString().trim();
        long err = Skywiremob.isPKValid(remotePK).getCode();
        if (err != Skywiremob.ErrCodeNoError) {
            HelperFunctions.showToast(getString(R.string.vpn_coordinator_invalid_credentials_error) + remotePK, false);
            return;
        } else {
            Skywiremob.printString("PK is correct");
        }

        Globals.AppFilteringModes selectedMode = VPNGeneralPersistentData.getAppsSelectionMode();
        if (selectedMode != Globals.AppFilteringModes.PROTECT_ALL) {
            HashSet<String> selectedApps = HelperFunctions.filterAvailableApps(VPNGeneralPersistentData.getAppList(new HashSet<>()));

            if (selectedApps.size() == 0) {
                if (selectedMode == Globals.AppFilteringModes.PROTECT_SELECTED) {
                    HelperFunctions.showToast(getString(R.string.vpn_no_apps_to_protect_warning), false);
                } else {
                    HelperFunctions.showToast(getString(R.string.vpn_no_apps_to_ignore_warning), false);
                }
            }
        }

        ManualVpnServerData intermediaryServerData = new ManualVpnServerData();
        intermediaryServerData.pk = remotePK;
        intermediaryServerData.password = editTextPasscode.getText().toString();
        LocalServerData server = VPNServersPersistentData.getInstance().processFromManual(intermediaryServerData);

        VPNCoordinator.getInstance().startVPN(
            this,
            server
        );
    }

    private void stop() {
        VPNCoordinator.getInstance().stopVPN();
    }

    private void selectServer() {
        Intent intent = new Intent(this, ServersActivity.class);
        startActivityForResult(intent, 1);
    }

    private void selectApps() {
        Intent intent = new Intent(this, AppsActivity.class);
        startActivity(intent);
    }

    private void openSettings() {
        Intent intent = new Intent(this, SettingsActivity.class);
        startActivity(intent);
    }

    private void openStarPage() {
        Intent intent = new Intent(this, StartActivity.class);
        startActivity(intent);
    }

    private void displayInitialState() {
        textStatus.setText(R.string.vpn_state_details_off);

        editTextRemotePK.setEnabled(true);
        editTextPasscode.setEnabled(true);
        buttonStart.setEnabled(true);
        buttonStop.setEnabled(false);
        buttonSelect.setEnabled(true);
        buttonApps.setEnabled(true);
        buttonSettings.setEnabled(true);
        textFinishAlert.setVisibility(View.GONE);
        textStopAlert.setVisibility(View.GONE);

        String lastError = VPNGeneralPersistentData.getLastError(null);
        if (lastError != null) {
            textLastError1.setVisibility(View.VISIBLE);
            textLastError2.setVisibility(View.VISIBLE);
            textLastError2.setText(lastError);
        } else {
            textLastError1.setVisibility(View.GONE);
            textLastError2.setVisibility(View.GONE);
        }
    }

    private void displayWorkingState() {
        editTextRemotePK.setEnabled(false);
        editTextPasscode.setEnabled(false);
        buttonStart.setEnabled(false);
        buttonStop.setEnabled(true);
        buttonSelect.setEnabled(false);
        buttonApps.setEnabled(false);
        buttonSettings.setEnabled(false);
        textFinishAlert.setVisibility(View.GONE);
        textStopAlert.setVisibility(View.GONE);

        textLastError1.setVisibility(View.GONE);
        textLastError2.setVisibility(View.GONE);
    }

    private void displayErrorState(boolean stopRequested) {
        editTextRemotePK.setEnabled(false);
        editTextPasscode.setEnabled(false);
        buttonStart.setEnabled(false);
        buttonStop.setEnabled(!stopRequested);
        buttonSelect.setEnabled(false);
        buttonApps.setEnabled(false);
        buttonSettings.setEnabled(false);
        textFinishAlert.setVisibility(stopRequested ? View.VISIBLE : View.GONE);
        textStopAlert.setVisibility(View.GONE);

        textLastError1.setVisibility(View.VISIBLE);
        textLastError2.setVisibility(View.VISIBLE);

        String lastError = VPNGeneralPersistentData.getLastError(null);
        textLastError2.setText(lastError);
    }
}
