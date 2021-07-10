package com.skywire.skycoin.vpn.controls;

import android.animation.Animator;
import android.animation.AnimatorInflater;
import android.animation.AnimatorSet;
import android.content.Context;
import android.util.AttributeSet;
import android.view.LayoutInflater;
import android.widget.FrameLayout;
import android.widget.TextView;

import androidx.core.content.ContextCompat;

import com.skywire.skycoin.vpn.R;
import com.skywire.skycoin.vpn.helpers.Globals;
import com.skywire.skycoin.vpn.helpers.HelperFunctions;
import com.skywire.skycoin.vpn.vpn.VPNCoordinator;
import com.skywire.skycoin.vpn.vpn.VPNGeneralPersistentData;
import com.skywire.skycoin.vpn.vpn.VPNStates;

import java.io.Closeable;

import io.reactivex.rxjava3.disposables.Disposable;

public class TabletTopBarStats extends FrameLayout implements Animator.AnimatorListener, Closeable {
    private TextView textConnectionIconAnim;
    private TextView textConnectionIcon;
    private TextView textConnection;
    private TextView textLatency;
    private TextView textUploadSpeed;
    private TextView textDownloadSpeed;

    private VPNStates currentState = VPNStates.OFF;
    private VPNCoordinator.ConnectionStats currentStats = new VPNCoordinator.ConnectionStats();
    private Globals.DataUnits dataUnits = VPNGeneralPersistentData.getDataUnits();

    private AnimatorSet animSet;

    private boolean animPaused = false;
    private boolean closed = false;
    private Disposable eventsSubscription;
    private Disposable statsSubscription;
    private Disposable dataUnitsSubscription;

    public TabletTopBarStats(Context context) {
        super(context);
        Initialize(context, null);
    }
    public TabletTopBarStats(Context context, AttributeSet attrs) {
        super(context, attrs);
        Initialize(context, attrs);
    }
    public TabletTopBarStats(Context context, AttributeSet attrs, int defStyle) {
        super(context, attrs, defStyle);
        Initialize(context, attrs);
    }

    protected void Initialize (Context context, AttributeSet attrs) {
        LayoutInflater inflater = (LayoutInflater)context.getSystemService (Context.LAYOUT_INFLATER_SERVICE);
        inflater.inflate(R.layout.view_tablet_top_bar_stats, this, true);

        textConnectionIconAnim = this.findViewById (R.id.textConnectionIconAnim);
        textConnectionIcon = this.findViewById (R.id.textConnectionIcon);
        textConnection = this.findViewById (R.id.textConnection);
        textLatency = this.findViewById (R.id.textLatency);
        textUploadSpeed = this.findViewById (R.id.textUploadSpeed);
        textDownloadSpeed = this.findViewById (R.id.textDownloadSpeed);

        animSet = (AnimatorSet) AnimatorInflater.loadAnimator(getContext(), R.animator.anim_state);
        animSet.setTarget(textConnectionIconAnim);
    }

    public void onResume() {
        if (!closed) {
            animPaused = false;
            animSet.addListener(this);
            animSet.start();

            updateData();

            eventsSubscription = VPNCoordinator.getInstance().getEventsObservable().subscribe(response -> {
                currentState = response.state;
                updateData();
            });

            statsSubscription = VPNCoordinator.getInstance().getConnectionStats().subscribe(stats -> {
                currentStats = stats;
                updateData();
            });

            dataUnitsSubscription = VPNGeneralPersistentData.getDataUnitsObservable().subscribe(response -> {
                dataUnits = response;
                updateData();
            });
        }
    }

    public void onPause() {
        animPaused = true;
        animSet.removeAllListeners();
        animSet.cancel();

        eventsSubscription.dispose();
        statsSubscription.dispose();
        dataUnitsSubscription.dispose();
    }

    @Override
    public void onAnimationStart(Animator animation) { }
    @Override
    public void onAnimationCancel(Animator animation) { }
    @Override
    public void onAnimationRepeat(Animator animation) { }
    @Override
    public void onAnimationEnd(Animator animation) {
        if (!closed && !animPaused) {
            animSet.start();
        }
    }

    private void updateData() {
        int stateText = VPNStates.getTitleForState(currentState);
        if (stateText != -1) {
            textConnection.setText(stateText);
        } else {
            textConnection.setText("---");
        }

        int stateColor = ContextCompat.getColor(getContext(), VPNStates.getColorForStateTitle(stateText));
        textConnectionIconAnim.setTextColor(stateColor);
        textConnection.setTextColor(stateColor);
        textConnectionIcon.setTextColor(stateColor);

        textLatency.setText(HelperFunctions.getLatencyValue(currentStats.currentLatency));
        textDownloadSpeed.setText(HelperFunctions.computeDataAmountString(currentStats.currentDownloadSpeed, true, dataUnits != Globals.DataUnits.OnlyBytes));
        textUploadSpeed.setText(HelperFunctions.computeDataAmountString(currentStats.currentUploadSpeed, true, dataUnits != Globals.DataUnits.OnlyBytes));
    }

    @Override
    public void close() {
        closed = true;

        if (eventsSubscription != null) {
            eventsSubscription.dispose();
            statsSubscription.dispose();
        }
    }
}
