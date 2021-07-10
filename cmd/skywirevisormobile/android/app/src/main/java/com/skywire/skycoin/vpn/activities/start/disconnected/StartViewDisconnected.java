package com.skywire.skycoin.vpn.activities.start.disconnected;

import android.app.Activity;
import android.content.Context;
import android.util.AttributeSet;
import android.util.TypedValue;
import android.view.LayoutInflater;
import android.view.View;
import android.widget.FrameLayout;
import android.widget.TextView;

import com.skywire.skycoin.vpn.R;
import com.skywire.skycoin.vpn.activities.index.IndexPageAdapter;
import com.skywire.skycoin.vpn.activities.servers.ServerLists;
import com.skywire.skycoin.vpn.activities.servers.ServersActivity;
import com.skywire.skycoin.vpn.activities.start.StartViewRightPanel;
import com.skywire.skycoin.vpn.extensible.ClickEvent;
import com.skywire.skycoin.vpn.helpers.HelperFunctions;
import com.skywire.skycoin.vpn.objects.LocalServerData;
import com.skywire.skycoin.vpn.vpn.VPNGeneralPersistentData;
import com.skywire.skycoin.vpn.vpn.VPNServersPersistentData;

import java.io.Closeable;

import io.reactivex.rxjava3.disposables.Disposable;

public class StartViewDisconnected extends FrameLayout implements ClickEvent, Closeable {
    public StartViewDisconnected(Context context) {
        super(context);
        Initialize(context, null);
    }
    public StartViewDisconnected(Context context, AttributeSet attrs) {
        super(context, attrs);
        Initialize(context, attrs);
    }
    public StartViewDisconnected(Context context, AttributeSet attrs, int defStyle) {
        super(context, attrs, defStyle);
        Initialize(context, attrs);
    }

    private CurrentServerButton viewCurrentServerButton;
    private StartButton startButton;
    private TextView textServerNote;
    private TextView textLastError;
    private FrameLayout rightContainer;
    private StartViewRightPanel rightPanel;

    private Activity parentActivity;
    private IndexPageAdapter.RequestTabListener requestTabListener;
    private Disposable currentServerSubscription;

    protected void Initialize (Context context, AttributeSet attrs) {
        LayoutInflater inflater = (LayoutInflater)context.getSystemService (Context.LAYOUT_INFLATER_SERVICE);
        inflater.inflate(R.layout.view_start_disconnected, this, true);

        viewCurrentServerButton = findViewById(R.id.viewCurrentServerButton);
        startButton = findViewById(R.id.startButton);
        textServerNote = findViewById(R.id.textServerNote);
        textLastError = findViewById(R.id.textLastError);
        rightContainer = findViewById(R.id.rightContainer);
        rightPanel = findViewById(R.id.rightPanel);

        viewCurrentServerButton.setClickEventListener(this);
        startButton.setClickEventListener(this);

        currentServerSubscription = VPNServersPersistentData.getInstance().getCurrentServerObservable().subscribe(currentServer -> {
            viewCurrentServerButton.setData(currentServer);
            updateNote(currentServer);
        });

        setErrorMsg(VPNGeneralPersistentData.getLastError(null));

        if (HelperFunctions.getWidthType(getContext()) == HelperFunctions.WidthTypes.SMALL) {
            rightContainer.setVisibility(GONE);
        } else {
            textServerNote.setTextSize(TypedValue.COMPLEX_UNIT_PX, getContext().getResources().getDimension(R.dimen.small_text_size));
            textLastError.setTextSize(TypedValue.COMPLEX_UNIT_PX, getContext().getResources().getDimension(R.dimen.small_text_size));
            rightPanel.refreshIpData();
        }
    }

    public void setRequestTabListener(IndexPageAdapter.RequestTabListener listener) {
        requestTabListener = listener;
    }

    public void setParentActivity(Activity activity) {
        parentActivity = activity;
    }

    public void startAnimation() {
        startButton.startAnimation();
    }

    public void stopAnimation() {
        startButton.stopAnimation();
    }

    public void updateRightBar() {
        rightPanel.updateData();
    }

    public void setErrorMsg(String errorMsg) {
        if (errorMsg != null) {
            String start = getContext().getString(R.string.tmp_status_page_last_error);
            textLastError.setText(start + " " + errorMsg);
            textLastError.setVisibility(VISIBLE);
        } else {
            textLastError.setVisibility(GONE);
        }
    }

    private void updateNote(LocalServerData currentServer) {
        if (currentServer == null) {
            textServerNote.setVisibility(GONE);

            return;
        }

        String note = HelperFunctions.getServerNote(currentServer);

        if (note != null) {
            textServerNote.setText(note);
            textServerNote.setVisibility(VISIBLE);
        } else {
            textServerNote.setVisibility(GONE);
        }
    }

    @Override
    public void close() {
        currentServerSubscription.dispose();
        rightPanel.close();
        stopAnimation();
    }

    @Override
    public void onClick(View view) {
        LocalServerData currentServer = VPNServersPersistentData.getInstance().getCurrentServer();
        if (currentServer != null) {
            if (view.getId() == R.id.viewCurrentServerButton) {
                HelperFunctions.showServerOptions(getContext(), ServersActivity.convertLocalServerData(currentServer), ServerLists.History);
            } else {
                if (parentActivity != null) {
                    boolean starting = HelperFunctions.prepareAndStartVpn(parentActivity, currentServer);
                    if (starting) {
                        startButton.setEnabled(false);
                    }
                }
            }
        } else {
            if (requestTabListener != null) {
                requestTabListener.onOpenServerListRequested();
            }
        }
    }
}
