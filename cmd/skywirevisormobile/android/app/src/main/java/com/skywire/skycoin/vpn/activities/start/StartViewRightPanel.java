package com.skywire.skycoin.vpn.activities.start;

import android.content.ClipData;
import android.content.ClipboardManager;
import android.content.Context;
import android.content.Intent;
import android.content.res.TypedArray;
import android.text.SpannableStringBuilder;
import android.text.Spanned;
import android.text.style.RelativeSizeSpan;
import android.util.AttributeSet;
import android.view.LayoutInflater;
import android.view.View;
import android.widget.FrameLayout;
import android.widget.LinearLayout;
import android.widget.ProgressBar;
import android.widget.TextView;

import com.skywire.skycoin.vpn.R;
import com.skywire.skycoin.vpn.activities.apps.AppsActivity;
import com.skywire.skycoin.vpn.activities.servers.ServerLists;
import com.skywire.skycoin.vpn.activities.servers.ServersActivity;
import com.skywire.skycoin.vpn.controls.ClickableLinearLayout;
import com.skywire.skycoin.vpn.controls.ServerName;
import com.skywire.skycoin.vpn.extensible.ClickEvent;
import com.skywire.skycoin.vpn.helpers.AlphaSpan;
import com.skywire.skycoin.vpn.helpers.Globals;
import com.skywire.skycoin.vpn.helpers.HelperFunctions;
import com.skywire.skycoin.vpn.helpers.MaterialFontSpan;
import com.skywire.skycoin.vpn.network.ApiClient;
import com.skywire.skycoin.vpn.objects.LocalServerData;
import com.skywire.skycoin.vpn.vpn.VPNGeneralPersistentData;
import com.skywire.skycoin.vpn.vpn.VPNServersPersistentData;

import java.io.Closeable;
import java.util.Date;
import java.util.HashSet;
import java.util.concurrent.TimeUnit;

import io.reactivex.rxjava3.android.schedulers.AndroidSchedulers;
import io.reactivex.rxjava3.core.Observable;
import io.reactivex.rxjava3.disposables.Disposable;
import io.reactivex.rxjava3.schedulers.Schedulers;

public class StartViewRightPanel extends FrameLayout implements ClickEvent, Closeable {
    public StartViewRightPanel(Context context) {
        super(context);
        Initialize(context, null);
    }
    public StartViewRightPanel(Context context, AttributeSet attrs) {
        super(context, attrs);
        Initialize(context, attrs);
    }
    public StartViewRightPanel(Context context, AttributeSet attrs, int defStyle) {
        super(context, attrs, defStyle);
        Initialize(context, attrs);
    }

    private final int retryDelay = 20000;

    private TextView textWaitingIp;
    private TextView textIp;
    private TextView textWaitingCountry;
    private TextView textCountry;
    private TextView textRemotePk;
    private TextView textLocalPk;
    private TextView textAppProtection;
    private ServerName serverName;
    private ClickableLinearLayout ipClickableLayout;
    private ClickableLinearLayout serverClickableLayout;
    private ClickableLinearLayout remotePkClickableLayout;
    private ClickableLinearLayout localPkClickableLayout;
    private ClickableLinearLayout appProtectionClickableLayout;
    private LinearLayout loadingIpContainer;
    private LinearLayout ipContainer;
    private LinearLayout countryContainer;
    private LinearLayout bottomPartContainer;
    private ProgressBar progressCountry;

    private LocalServerData currentServer;

    private String previousIp;
    private String currentIp;
    private String previousCountry;
    private Date lastIpRefresDate;

    private Disposable serverSubscription;
    private Disposable ipSubscription;

    protected void Initialize (Context context, AttributeSet attrs) {
        LayoutInflater inflater = (LayoutInflater)context.getSystemService (Context.LAYOUT_INFLATER_SERVICE);
        inflater.inflate(R.layout.view_start_right_panel, this, true);

        textWaitingIp = findViewById(R.id.textWaitingIp);
        textIp = findViewById(R.id.textIp);
        textWaitingCountry = findViewById(R.id.textWaitingCountry);
        textCountry = findViewById(R.id.textCountry);
        textRemotePk = findViewById(R.id.textRemotePk);
        textLocalPk = findViewById(R.id.textLocalPk);
        textAppProtection = findViewById(R.id.textAppProtection);
        serverName = findViewById(R.id.serverName);
        ipClickableLayout = findViewById(R.id.ipClickableLayout);
        serverClickableLayout = findViewById(R.id.serverClickableLayout);
        remotePkClickableLayout = findViewById(R.id.remotePkClickableLayout);
        localPkClickableLayout = findViewById(R.id.localPkClickableLayout);
        appProtectionClickableLayout = findViewById(R.id.appProtectionClickableLayout);
        loadingIpContainer = findViewById(R.id.loadingIpContainer);
        ipContainer = findViewById(R.id.ipContainer);
        countryContainer = findViewById(R.id.countryContainer);
        bottomPartContainer = findViewById(R.id.bottomPartContainer);
        progressCountry = findViewById(R.id.progressCountry);

        ipClickableLayout.setClickEventListener(this);
        serverClickableLayout.setClickEventListener(this);
        remotePkClickableLayout.setClickEventListener(this);
        localPkClickableLayout.setClickEventListener(this);
        appProtectionClickableLayout.setClickEventListener(this);

        localPkClickableLayout.setVisibility(View.GONE);
        ipClickableLayout.setVisibility(View.GONE);
        ipContainer.setVisibility(View.GONE);
        countryContainer.setVisibility(View.GONE);

        if (attrs != null) {
            TypedArray attributes = context.getTheme().obtainStyledAttributes(
                attrs,
                R.styleable.StartViewRightPanel,
                0, 0
            );

            if (attributes.getBoolean(R.styleable.StartViewRightPanel_hide_bottom_part, false)) {
                bottomPartContainer.setVisibility(GONE);
            }

            attributes.recycle();
        }

        if (!isInEditMode()) {
            updateData();

            if (!VPNGeneralPersistentData.getShowIpActivated()) {
                textWaitingIp.setText(R.string.tmp_status_connected_ip_option_disabled);
                textWaitingCountry.setText(R.string.tmp_status_connected_ip_option_disabled);
            }
        }
    }

    public void updateData() {
        if (serverSubscription == null) {
            serverSubscription = VPNServersPersistentData.getInstance().getCurrentServerObservable().subscribe(server -> {
                currentServer = server;
                serverName.setServer(ServersActivity.convertLocalServerData(currentServer), ServerLists.History, true);
                putTextWithIcon(textRemotePk, currentServer.pk, "  \ue14d");
            });
        }

        Globals.AppFilteringModes selectedMode = VPNGeneralPersistentData.getAppsSelectionMode();
        if (selectedMode != Globals.AppFilteringModes.PROTECT_ALL) {
            HashSet<String> selectedApps = HelperFunctions.filterAvailableApps(VPNGeneralPersistentData.getAppList(new HashSet<>()));

            if (selectedApps.size() > 0) {
                appProtectionClickableLayout.setVisibility(VISIBLE);

                String text;
                if (selectedMode == Globals.AppFilteringModes.PROTECT_SELECTED) {
                    text = getContext().getString(R.string.tmp_status_connected_protecting_selected_apps);
                } else {
                    text = getContext().getString(R.string.tmp_status_connected_ignoring_selected_apps);
                }

                putTextWithIcon(textAppProtection, text, "  \ue8f4");
            } else {
                appProtectionClickableLayout.setVisibility(GONE);
            }
        } else {
            appProtectionClickableLayout.setVisibility(GONE);
        }
    }

    public void putInWaitingForVpnState() {
        cancelIpCheck();

        ipClickableLayout.setVisibility(GONE);
        loadingIpContainer.setVisibility(VISIBLE);

        textWaitingIp.setVisibility(VISIBLE);
        textWaitingCountry.setVisibility(VISIBLE);
        ipContainer.setVisibility(View.GONE);
        countryContainer.setVisibility(View.GONE);
    }

    public void refreshIpData() {
        getIp(0);
    }

    private void getIp(int delayMs) {
        if (!VPNGeneralPersistentData.getShowIpActivated()) {
            return;
        }

        cancelIpCheck();

        ipClickableLayout.setVisibility(GONE);
        loadingIpContainer.setVisibility(VISIBLE);

        textWaitingIp.setVisibility(GONE);
        textWaitingCountry.setVisibility(GONE);
        progressCountry.setVisibility(VISIBLE);
        ipContainer.setVisibility(View.VISIBLE);
        countryContainer.setVisibility(View.VISIBLE);
        textIp.setText("---");
        textCountry.setText("---");

        ipSubscription = Observable.just(0).delay(delayMs, TimeUnit.MILLISECONDS).flatMap(v -> ApiClient.getCurrentIp())
            .subscribeOn(Schedulers.io())
            .observeOn(AndroidSchedulers.mainThread())
            .subscribe(response -> {
                if (response.body() != null) {
                    lastIpRefresDate = new Date();

                    ipClickableLayout.setVisibility(VISIBLE);
                    loadingIpContainer.setVisibility(GONE);

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

        ipSubscription = Observable.just(0).delay(delayMs, TimeUnit.MILLISECONDS).flatMap(v -> ApiClient.getIpCountry(currentIp))
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

    private void cancelIpCheck() {
        if (ipSubscription != null) {
            ipSubscription.dispose();
        }
    }

    private void putTextWithIcon(TextView textView, String text, String iconText) {
        MaterialFontSpan materialFontSpan = new MaterialFontSpan(getContext());
        RelativeSizeSpan relativeSizeSpan = new RelativeSizeSpan(0.75f);
        AlphaSpan alphaSpan = new AlphaSpan(128);

        SpannableStringBuilder finalText = new SpannableStringBuilder(text.toString() + iconText);
        finalText.setSpan(materialFontSpan, finalText.length() - iconText.length(), finalText.length(), Spanned.SPAN_EXCLUSIVE_EXCLUSIVE);
        finalText.setSpan(relativeSizeSpan, finalText.length() - iconText.length(), finalText.length(), Spanned.SPAN_EXCLUSIVE_EXCLUSIVE);
        finalText.setSpan(alphaSpan, finalText.length() - iconText.length(), finalText.length(), Spanned.SPAN_EXCLUSIVE_EXCLUSIVE);

        textView.setText(finalText);
    }

    @Override
    public void onClick(View view) {
        if (view.getId() == R.id.ipClickableLayout) {
            long msToWait = 10000;
            long elapsedTime = (new Date()).getTime() - lastIpRefresDate.getTime();

            if (elapsedTime < msToWait) {
                HelperFunctions.showToast(String.format(
                    getContext().getText(R.string.tmp_status_connected_ip_refresh_time_warning).toString(),
                    HelperFunctions.zeroDecimalsFormatter.format(Math.ceil((msToWait - elapsedTime)) / 1000d)
                ), true);
            } else {
                this.refreshIpData();
            }
        } else if (view.getId() == R.id.serverClickableLayout) {
            HelperFunctions.showServerOptions(getContext(), ServersActivity.convertLocalServerData(currentServer), ServerLists.History);
        } else if (view.getId() == R.id.appProtectionClickableLayout) {
            Intent intent = new Intent(getContext(), AppsActivity.class);
            intent.putExtra(AppsActivity.READ_ONLY_EXTRA, true);
            getContext().startActivity(intent);
        } else {
            String textToCopy = currentServer.pk;

            ClipboardManager clipboard = (ClipboardManager)getContext().getSystemService(Context.CLIPBOARD_SERVICE);
            ClipData clipData = ClipData.newPlainText("", textToCopy);
            clipboard.setPrimaryClip(clipData);
            HelperFunctions.showToast(getContext().getString(R.string.general_copied), true);
        }
    }

    @Override
    public void close() {
        if (serverSubscription != null) {
            serverSubscription.dispose();
        }
        cancelIpCheck();
    }
}
