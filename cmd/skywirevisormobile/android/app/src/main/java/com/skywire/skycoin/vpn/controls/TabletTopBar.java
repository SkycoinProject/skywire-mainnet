package com.skywire.skycoin.vpn.controls;

import android.content.Context;
import android.util.AttributeSet;
import android.view.LayoutInflater;
import android.view.View;
import android.widget.FrameLayout;
import android.widget.LinearLayout;

import com.skywire.skycoin.vpn.R;
import com.skywire.skycoin.vpn.extensible.ClickEvent;
import com.skywire.skycoin.vpn.extensible.ClickWithIndexEvent;

import java.io.Closeable;

public class TabletTopBar extends FrameLayout implements ClickEvent, Closeable {
    public TabletTopBar(Context context) {
        super(context);
        Initialize(context, null);
    }
    public TabletTopBar(Context context, AttributeSet attrs) {
        super(context, attrs);
        Initialize(context, attrs);
    }
    public TabletTopBar(Context context, AttributeSet attrs, int defStyle) {
        super(context, attrs, defStyle);
        Initialize(context, attrs);
    }

    public static int statusTabIndex = 0;
    public static int serversTabIndex = 1;
    public static int settingsTabIndex = 2;

    private TabletTopBarTab tabStatus;
    private TabletTopBarTab tabServers;
    private TabletTopBarTab tabSettings;
    private TabletTopBarStats stats;

    private ClickWithIndexEvent<Void> clickListener;

    private void Initialize (Context context, AttributeSet attrs) {
        LayoutInflater inflater = (LayoutInflater)context.getSystemService (Context.LAYOUT_INFLATER_SERVICE);
        inflater.inflate(R.layout.view_tablet_top_bar, this, true);

        tabStatus = this.findViewById (R.id.tabStatus);
        tabServers = this.findViewById (R.id.tabServers);
        tabSettings = this.findViewById (R.id.tabSettings);
        stats = this.findViewById (R.id.stats);

        stats.setVisibility(INVISIBLE);

        tabStatus.setClickEventListener(this);
        tabServers.setClickEventListener(this);
        tabSettings.setClickEventListener(this);
    }

    public void onResume() {
        if (stats.getVisibility() == VISIBLE) {
            stats.onResume();
        }
    }

    public void onPause() {
        if (stats.getVisibility() == VISIBLE) {
            stats.onPause();
        }
    }

    public void setSelectedTab(int tabIndex) {
        tabStatus.setSelected(false);
        tabServers.setSelected(false);
        tabSettings.setSelected(false);

        if (tabIndex == statusTabIndex) {
            tabStatus.setSelected(true);

            if (stats.getVisibility() == VISIBLE) {
                stats.setVisibility(INVISIBLE);
                stats.onPause();
            }
        } else if (tabIndex == serversTabIndex) {
            tabServers.setSelected(true);

            if (stats.getVisibility() != VISIBLE) {
                stats.setVisibility(VISIBLE);
                stats.onResume();
            }
        } else if (tabIndex == settingsTabIndex) {
            tabSettings.setSelected(true);

            if (stats.getVisibility() != VISIBLE) {
                stats.setVisibility(VISIBLE);
                stats.onResume();
            }
        }
    }

    public void setClickWithIndexEventListener(ClickWithIndexEvent<Void> listener) {
        clickListener = listener;
    }

    @Override
    public void onClick(View view) {
        if (clickListener != null) {
            if (view.getId() == R.id.tabStatus) {
                clickListener.onClickWithIndex(statusTabIndex, null);
            } else if (view.getId() == R.id.tabServers) {
                clickListener.onClickWithIndex(serversTabIndex, null);
            } else if (view.getId() == R.id.tabSettings) {
                clickListener.onClickWithIndex(settingsTabIndex, null);
            }
        }
    }

    @Override
    public void close() {
        stats.close();
    }
}
