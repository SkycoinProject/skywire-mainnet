package com.skywire.skycoin.vpn.activities.start.connected;

import android.content.Context;
import android.content.Intent;
import android.util.AttributeSet;
import android.util.TypedValue;
import android.view.Gravity;
import android.view.LayoutInflater;
import android.view.View;
import android.widget.FrameLayout;
import android.widget.ImageView;
import android.widget.LinearLayout;
import android.widget.ProgressBar;
import android.widget.TextView;

import com.skywire.skycoin.vpn.R;
import com.skywire.skycoin.vpn.activities.apps.AppsActivity;
import com.skywire.skycoin.vpn.activities.servers.ServerLists;
import com.skywire.skycoin.vpn.activities.servers.ServersActivity;
import com.skywire.skycoin.vpn.activities.start.StartViewRightPanel;
import com.skywire.skycoin.vpn.controls.ConfirmationModalWindow;
import com.skywire.skycoin.vpn.controls.ServerName;
import com.skywire.skycoin.vpn.extensible.ClickEvent;
import com.skywire.skycoin.vpn.helpers.ClickTimeManagement;
import com.skywire.skycoin.vpn.helpers.Globals;
import com.skywire.skycoin.vpn.helpers.HelperFunctions;
import com.skywire.skycoin.vpn.network.ApiClient;
import com.skywire.skycoin.vpn.vpn.VPNCoordinator;
import com.skywire.skycoin.vpn.vpn.VPNGeneralPersistentData;
import com.skywire.skycoin.vpn.vpn.VPNServersPersistentData;
import com.skywire.skycoin.vpn.vpn.VPNStates;

import java.io.Closeable;
import java.util.ArrayList;
import java.util.Date;
import java.util.HashSet;
import java.util.concurrent.TimeUnit;

import io.reactivex.rxjava3.android.schedulers.AndroidSchedulers;
import io.reactivex.rxjava3.core.Observable;
import io.reactivex.rxjava3.disposables.Disposable;
import io.reactivex.rxjava3.schedulers.Schedulers;

public class StartViewConnected extends FrameLayout implements ClickEvent, Closeable {
    public StartViewConnected(Context context) {
        super(context);
        Initialize(context, null);
    }
    public StartViewConnected(Context context, AttributeSet attrs) {
        super(context, attrs);
        Initialize(context, attrs);
    }
    public StartViewConnected(Context context, AttributeSet attrs, int defStyle) {
        super(context, attrs, defStyle);
        Initialize(context, attrs);
    }

    private final int retryDelay = 20000;

    private TextView textTime;
    private TextView textState;
    private TextView textStateDescription;
    private TextView textLastError;
    private TextView textWaitingIp;
    private TextView textWaitingCountry;
    private TextView textIp;
    private TextView textCountry;
    private TextView textUploadSpeed;
    private TextView textTotalUploaded;
    private TextView textDownloadSpeed;
    private TextView textTotalDownloaded;
    private TextView textLatency;
    private TextView textAppsProtectionMode;
    private TextView textServerNote;
    private TextView textStartedByTheSystem;
    private ServerName serverName;
    private ImageView imageStateLine;
    private Chart downloadChart;
    private Chart uploadChart;
    private Chart latencyChart;
    private LinearLayout leftContainer;
    private LinearLayout ipDataContainer;
    private LinearLayout ipContainer;
    private LinearLayout countryContainer;
    private FrameLayout appsContainer;
    private LinearLayout appsInternalContainer;
    private LinearLayout serverContainer;
    private FrameLayout rightContainer;
    private ProgressBar progressIp;
    private ProgressBar progressCountry;
    private StopButton buttonStop;
    private StartViewRightPanel rightPanel;

    private String previousIp;
    private String currentIp;
    private String previousCountry;
    private VPNCoordinator.ConnectionStats lastStats;
    private boolean updateStats = true;
    private Globals.DataUnits dataUnits = VPNGeneralPersistentData.getDataUnits();

    private ClickTimeManagement appsButtonTimeManager = new ClickTimeManagement();
    private ClickTimeManagement serverButtonTimeManager = new ClickTimeManagement();

    private Disposable serviceSubscription;
    private Disposable serverSubscription;
    private Disposable ipSubscription;
    private Disposable statsSubscription;
    private Disposable dataUnitsSubscription;

    protected void Initialize (Context context, AttributeSet attrs) {
        LayoutInflater inflater = (LayoutInflater)context.getSystemService (Context.LAYOUT_INFLATER_SERVICE);
        inflater.inflate(R.layout.view_start_connected, this, true);

        textTime = findViewById(R.id.textTime);
        textState = findViewById(R.id.textState);
        textStateDescription = findViewById(R.id.textStateDescription);
        textLastError = findViewById(R.id.textLastError);
        textWaitingIp = findViewById(R.id.textWaitingIp);
        textWaitingCountry = findViewById(R.id.textWaitingCountry);
        textIp = findViewById(R.id.textIp);
        textCountry = findViewById(R.id.textCountry);
        textUploadSpeed = findViewById(R.id.textUploadSpeed);
        textTotalUploaded = findViewById(R.id.textTotalUploaded);
        textDownloadSpeed = findViewById(R.id.textDownloadSpeed);
        textTotalDownloaded = findViewById(R.id.textTotalDownloaded);
        textLatency = findViewById(R.id.textLatency);
        textAppsProtectionMode = findViewById(R.id.textAppsProtectionMode);
        textServerNote = findViewById(R.id.textServerNote);
        textStartedByTheSystem = findViewById(R.id.textStartedByTheSystem);
        serverName = this.findViewById (R.id.serverName);
        imageStateLine = findViewById(R.id.imageStateLine);
        imageStateLine = findViewById(R.id.imageStateLine);
        downloadChart = findViewById(R.id.downloadChart);
        uploadChart = findViewById(R.id.uploadChart);
        latencyChart = findViewById(R.id.latencyChart);
        leftContainer = findViewById(R.id.leftContainer);
        ipDataContainer = findViewById(R.id.ipDataContainer);
        ipContainer = findViewById(R.id.ipContainer);
        countryContainer = findViewById(R.id.countryContainer);
        appsContainer = findViewById(R.id.appsContainer);
        appsInternalContainer = findViewById(R.id.appsInternalContainer);
        serverContainer = findViewById(R.id.serverContainer);
        rightContainer = findViewById(R.id.rightContainer);
        progressIp = findViewById(R.id.progressIp);
        progressCountry = findViewById(R.id.progressCountry);
        buttonStop = findViewById(R.id.buttonStop);
        rightPanel = findViewById(R.id.rightPanel);

        textLastError.setVisibility(GONE);
        textStartedByTheSystem.setVisibility(GONE);
        ipContainer.setVisibility(GONE);
        countryContainer.setVisibility(GONE);

        if (HelperFunctions.getWidthType(getContext()) != HelperFunctions.WidthTypes.SMALL) {
            float areaWidth = getContext().getResources().getDimension(R.dimen.tablet_status_area_width);
            FrameLayout.LayoutParams params = new FrameLayout.LayoutParams((int)Math.round(areaWidth), LayoutParams.WRAP_CONTENT);
            params.gravity = Gravity.CENTER_HORIZONTAL;
            leftContainer.setLayoutParams(params);

            ipDataContainer.setVisibility(GONE);
            appsContainer.setVisibility(GONE);
            serverContainer.setVisibility(GONE);

            textLastError.setTextSize(TypedValue.COMPLEX_UNIT_PX, getContext().getResources().getDimension(R.dimen.small_text_size));
        } else {
            rightContainer.setVisibility(GONE);
        }

        Globals.AppFilteringModes selectedMode = VPNGeneralPersistentData.getAppsSelectionMode();
        if (selectedMode != Globals.AppFilteringModes.PROTECT_ALL) {
            HashSet<String> selectedApps = HelperFunctions.filterAvailableApps(VPNGeneralPersistentData.getAppList(new HashSet<>()));

            if (HelperFunctions.getWidthType(getContext()) == HelperFunctions.WidthTypes.SMALL) {
                if (selectedApps.size() > 0) {
                    if (selectedMode == Globals.AppFilteringModes.PROTECT_SELECTED) {
                        textAppsProtectionMode.setText(R.string.tmp_status_connected_protecting_selected_apps);
                    } else {
                        textAppsProtectionMode.setText(R.string.tmp_status_connected_ignoring_selected_apps);
                    }

                    appsInternalContainer.setOnClickListener((View v) -> {
                        if (appsButtonTimeManager.canClick()) {
                            appsButtonTimeManager.informClickMade();
                            Intent intent = new Intent(getContext(), AppsActivity.class);
                            intent.putExtra(AppsActivity.READ_ONLY_EXTRA, true);
                            getContext().startActivity(intent);
                        }
                    });
                } else {
                    appsContainer.setVisibility(GONE);
                }
            } else {
                appsContainer.setVisibility(GONE);
            }
        } else {
            appsContainer.setVisibility(GONE);
        }

        if (!VPNGeneralPersistentData.getShowIpActivated()) {
            textWaitingIp.setText(R.string.tmp_status_connected_ip_option_disabled);
            textWaitingCountry.setText(R.string.tmp_status_connected_ip_option_disabled);
        }

        ArrayList<Long> emptyValues = new ArrayList<>();
        emptyValues.add(0L);

        VPNCoordinator.ConnectionStats emptyStats = new VPNCoordinator.ConnectionStats();
        emptyStats.downloadSpeedHistory = emptyValues;
        emptyStats.uploadSpeedHistory = emptyValues;
        emptyStats.latencyHistory = emptyValues;
        emptyStats.currentDownloadSpeed = 0;
        emptyStats.currentUploadSpeed = 0;
        emptyStats.currentLatency = 0;
        emptyStats.totalDownloadedData = 0;
        emptyStats.totalUploadedData = 0;
        updateDisplayedStats(emptyStats);

        downloadChart.setData(emptyValues, false);
        uploadChart.setData(emptyValues, false);
        latencyChart.setData(emptyValues, true);

        serverSubscription = VPNServersPersistentData.getInstance().getCurrentServerObservable().subscribe(server -> {
            serverName.setServer(ServersActivity.convertLocalServerData(server), ServerLists.History, true);

            String note = HelperFunctions.getServerNote(server);
            if (note != null) {
                textServerNote.setText(note);
            } else {
                textServerNote.setText(server.pk);
            }
        });

        if (HelperFunctions.getWidthType(getContext()) == HelperFunctions.WidthTypes.SMALL) {
            serverContainer.setOnClickListener((View v) -> {
                if (serverButtonTimeManager.canClick()) {
                    serverButtonTimeManager.informClickMade();
                    Observable.just(1).delay(Globals.CLICK_DELAY_MS, TimeUnit.MILLISECONDS)
                        .subscribeOn(Schedulers.io())
                        .observeOn(AndroidSchedulers.mainThread())
                        .subscribe(val -> {
                            HelperFunctions.showServerOptions(
                                getContext(),
                                ServersActivity.convertLocalServerData(VPNServersPersistentData.getInstance().getCurrentServer()),
                                ServerLists.History
                            );
                        });
                }
            });
        }

        buttonStop.setClickEventListener(this);

        serviceSubscription = VPNCoordinator.getInstance().getEventsObservable().subscribe(
            state -> {
                int mainText = VPNStates.getTitleForState(state.state);
                if (mainText != -1) {
                    textState.setText(mainText);
                } else {
                    textState.setText("---");
                }

                imageStateLine.setBackgroundResource(VPNStates.getColorForStateTitle(mainText));

                int description = VPNStates.getDescriptionForState(state.state);
                if (description != -1) {
                    textStateDescription.setText(description);
                } else {
                    textStateDescription.setText("---");
                }

                buttonStop.setEnabled(true);

                if (state.startedByTheSystem) {
                    buttonStop.setEnabled(false);
                    textStartedByTheSystem.setVisibility(View.VISIBLE);
                } else {
                    textStartedByTheSystem.setVisibility(View.GONE);
                }

                if (state.stopRequested) {
                    buttonStop.setEnabled(false);
                    buttonStop.setBusyState(true);
                } else {
                    buttonStop.setBusyState(false);
                }

                if (state.state != VPNStates.CONNECTED) {
                    String lastError = VPNGeneralPersistentData.getLastError(null);
                    if (lastError != null) {
                        String start = getContext().getString(R.string.tmp_status_page_last_error);
                        textLastError.setText(start + " " + lastError);
                        textLastError.setVisibility(VISIBLE);
                    } else {
                        textLastError.setVisibility(GONE);
                    }
                } else {
                    textLastError.setVisibility(GONE);
                }

                if (VPNGeneralPersistentData.getShowIpActivated()) {
                    if (HelperFunctions.getWidthType(getContext()) == HelperFunctions.WidthTypes.SMALL) {
                        if (state.state == VPNStates.CONNECTED) {
                            if (ipContainer.getVisibility() == TextView.GONE) {
                                ipContainer.setVisibility(VISIBLE);
                                countryContainer.setVisibility(VISIBLE);
                                textWaitingIp.setVisibility(GONE);
                                textWaitingCountry.setVisibility(GONE);

                                textIp.setText("---");
                                textCountry.setText("---");

                                getIp(0);
                            }
                        } else {
                            if (ipContainer.getVisibility() == TextView.VISIBLE) {
                                ipContainer.setVisibility(GONE);
                                countryContainer.setVisibility(GONE);
                                textWaitingIp.setVisibility(VISIBLE);
                                textWaitingCountry.setVisibility(VISIBLE);

                                cancelIpCheck();
                            }
                        }
                    } else {
                        if (state.state == VPNStates.CONNECTED) {
                            rightPanel.refreshIpData();
                        } else {
                            rightPanel.putInWaitingForVpnState();
                        }
                    }
                }
            }
        );

        statsSubscription = VPNCoordinator.getInstance().getConnectionStats().subscribe(stats -> {
            lastStats = stats;
            if (updateStats) {
                updateDisplayedStats(lastStats);
            }
        });

        dataUnitsSubscription = VPNGeneralPersistentData.getDataUnitsObservable().subscribe(response -> {
            dataUnits = response;

            if (lastStats != null && updateStats) {
                updateDisplayedStats(lastStats);
            }
        });

        updateTime(null);
    }

    private void updateDisplayedStats(VPNCoordinator.ConnectionStats stats) {
        if (stats != null) {
            updateTime(stats.lastConnectionDate);

            downloadChart.setData(stats.downloadSpeedHistory, false);
            uploadChart.setData(stats.uploadSpeedHistory, false);
            latencyChart.setData(stats.latencyHistory, true);

            textDownloadSpeed.setText(HelperFunctions.computeDataAmountString(stats.currentDownloadSpeed, true, dataUnits != Globals.DataUnits.OnlyBytes));
            textUploadSpeed.setText(HelperFunctions.computeDataAmountString(stats.currentUploadSpeed, true, dataUnits != Globals.DataUnits.OnlyBytes));
            textLatency.setText(HelperFunctions.getLatencyValue(stats.currentLatency));

            textTotalDownloaded.setText(String.format(
                getContext().getText(R.string.tmp_status_connected_total_data).toString(),
                HelperFunctions.computeDataAmountString(stats.totalDownloadedData, false, dataUnits == Globals.DataUnits.OnlyBits)
            ));

            textTotalUploaded.setText(String.format(
                getContext().getText(R.string.tmp_status_connected_total_data).toString(),
                HelperFunctions.computeDataAmountString(stats.totalUploadedData, false, dataUnits == Globals.DataUnits.OnlyBits)
            ));
        }
    }

    public void pauseUpdatingStats() {
        updateStats = false;
    }

    public void continueUpdatingStats() {
        updateStats = true;
        updateDisplayedStats(lastStats);
    }

    public void updateRightBar() {
        rightPanel.updateData();
    }

    private void updateTime(Date lastConnectionDate) {
        if (lastConnectionDate == null) {
            textTime.setText(R.string.tmp_status_connected_waiting);
        } else {
            long connectionMs = (new Date()).getTime() - lastConnectionDate.getTime();

            String time = String.format("%02d", connectionMs / 3600000) + ":";
            time += String.format("%02d", (connectionMs / 60000) % 60) + ":";
            time += String.format("%02d", (connectionMs / 1000) % 60);

            textTime.setText(time);
        }
    }

    private void getIp(int delayMs) {
        if (!VPNGeneralPersistentData.getShowIpActivated()) {
            return;
        }

        if (ipSubscription != null) {
            ipSubscription.dispose();
        }

        progressIp.setVisibility(VISIBLE);
        progressCountry.setVisibility(VISIBLE);

        this.ipSubscription = Observable.just(0).delay(delayMs, TimeUnit.MILLISECONDS).flatMap(v -> ApiClient.getCurrentIp())
            .subscribeOn(Schedulers.io())
            .observeOn(AndroidSchedulers.mainThread())
            .subscribe(response -> {
                if (response.body() != null) {
                    progressIp.setVisibility(GONE);

                    currentIp = response.body().ip;
                    textIp.setText(currentIp);

                    if (currentIp.equals(previousIp) && previousCountry != null) {
                        textCountry.setText(previousCountry);
                        progressCountry.setVisibility(GONE);
                    } else {
                        getIpCountry(0);
                    }

                    previousIp = currentIp;
                } else {
                    getIp(retryDelay);
                }
            }, err -> {
                getIp(retryDelay);
            });
    }

    private void getIpCountry(int delayMs) {
        if (!VPNGeneralPersistentData.getShowIpActivated()) {
            return;
        }

        ipSubscription.dispose();

        this.ipSubscription = Observable.just(0).delay(delayMs, TimeUnit.MILLISECONDS).flatMap(v -> ApiClient.getIpCountry(currentIp))
            .subscribeOn(Schedulers.io())
            .observeOn(AndroidSchedulers.mainThread())
            .subscribe(response -> {
                if (response.body() != null) {
                    progressCountry.setVisibility(GONE);

                    String[] dataParts = response.body().split(";");
                    if (dataParts.length == 4) {
                        textCountry.setText(dataParts[3]);
                    } else {
                        textCountry.setText(getContext().getText(R.string.general_unknown));
                    }

                    previousCountry = textCountry.getText().toString();
                } else {
                    getIpCountry(retryDelay);
                }
            }, err -> {
                getIpCountry(retryDelay);
            });
    }

    @Override
    public void close() {
        serverSubscription.dispose();
        serviceSubscription.dispose();
        statsSubscription.dispose();
        dataUnitsSubscription.dispose();
        rightPanel.close();
        downloadChart.close();
        uploadChart.close();
        latencyChart.close();
        cancelIpCheck();
    }

    private void cancelIpCheck() {
        if (ipSubscription != null) {
            ipSubscription.dispose();
        }
    }

    @Override
    public void onClick(View view) {
        if (!VPNGeneralPersistentData.getKillSwitchActivated()) {
            VPNCoordinator.getInstance().stopVPN();
        } else {
            ConfirmationModalWindow confirmationModal = new ConfirmationModalWindow(
                getContext(),
                R.string.tmp_status_connected_disconnect_confirmation,
                R.string.tmp_confirmation_yes,
                R.string.tmp_confirmation_no,
                () -> {
                    VPNCoordinator.getInstance().stopVPN();
                    buttonStop.setEnabled(false);
                }
            );
            confirmationModal.show();
        }
    }
}
