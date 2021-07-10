package com.skywire.skycoin.vpn.activities.start.disconnected;

import android.content.Context;
import android.graphics.drawable.RippleDrawable;
import android.util.AttributeSet;
import android.view.LayoutInflater;
import android.view.MotionEvent;
import android.view.View;
import android.widget.FrameLayout;
import android.widget.ImageView;
import android.widget.LinearLayout;
import android.widget.TextView;

import com.skywire.skycoin.vpn.R;
import com.skywire.skycoin.vpn.activities.servers.ServerLists;
import com.skywire.skycoin.vpn.activities.servers.ServersActivity;
import com.skywire.skycoin.vpn.controls.ServerName;
import com.skywire.skycoin.vpn.extensible.ButtonBase;
import com.skywire.skycoin.vpn.helpers.HelperFunctions;
import com.skywire.skycoin.vpn.objects.LocalServerData;

public class CurrentServerButton extends ButtonBase implements View.OnTouchListener {
    public CurrentServerButton(Context context) {
        super(context);
    }
    public CurrentServerButton(Context context, AttributeSet attrs) {
        super(context, attrs);
    }
    public CurrentServerButton(Context context, AttributeSet attrs, int defStyle) {
        super(context, attrs, defStyle);
    }

    private FrameLayout mainContainer;
    private FrameLayout internalContainer;
    private LinearLayout serverContainer;
    private ImageView imageFlag;
    private ServerName serverName;
    private TextView textBottom;
    private TextView textNoServer;

    private RippleDrawable rippleDrawable;

    @Override
    protected void Initialize (Context context, AttributeSet attrs) {
        LayoutInflater inflater = (LayoutInflater)context.getSystemService (Context.LAYOUT_INFLATER_SERVICE);
        inflater.inflate(R.layout.view_current_server_button, this, true);

        mainContainer = this.findViewById (R.id.mainContainer);
        internalContainer = this.findViewById (R.id.internalContainer);
        serverContainer = this.findViewById (R.id.serverContainer);
        imageFlag = this.findViewById (R.id.imageFlag);
        serverName = this.findViewById (R.id.serverName);
        textBottom = this.findViewById (R.id.textBottom);
        textNoServer = this.findViewById (R.id.textNoServer);

        rippleDrawable = (RippleDrawable) internalContainer.getBackground();

        mainContainer.setClipToOutline(true);
        imageFlag.setClipToOutline(true);

        setOnTouchListener(this);
        setViewForCheckingClicks(this);
    }

    public void setData (LocalServerData currentServer) {
        if (currentServer == null || currentServer.pk == null) {
            textNoServer.setVisibility(VISIBLE);
            serverContainer.setVisibility(GONE);

            return;
        }

        serverContainer.setVisibility(VISIBLE);
        textNoServer.setVisibility(GONE);

        serverName.setServer(ServersActivity.convertLocalServerData(currentServer), ServerLists.History, true);
        textBottom.setText(currentServer.pk);
        imageFlag.setImageResource(HelperFunctions.getFlagResourceId(currentServer.countryCode));
    }

    @Override
    public boolean onTouch(View v, MotionEvent event) {
        if (rippleDrawable != null) {
            rippleDrawable.setHotspot(event.getX(), event.getY());
        }

        return false;
    }
}
