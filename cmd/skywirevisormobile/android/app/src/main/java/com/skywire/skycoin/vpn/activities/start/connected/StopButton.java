package com.skywire.skycoin.vpn.activities.start.connected;

import android.content.Context;
import android.util.AttributeSet;
import android.view.LayoutInflater;
import android.view.MotionEvent;
import android.view.View;
import android.widget.FrameLayout;
import android.widget.ProgressBar;
import android.widget.TextView;

import com.skywire.skycoin.vpn.R;
import com.skywire.skycoin.vpn.extensible.ButtonBase;

public class StopButton extends ButtonBase implements View.OnTouchListener {
    public StopButton(Context context) {
        super(context);
    }
    public StopButton(Context context, AttributeSet attrs) {
        super(context, attrs);
    }
    public StopButton(Context context, AttributeSet attrs, int defStyle) {
        super(context, attrs, defStyle);
    }

    private FrameLayout mainLayout;
    private FrameLayout internalContainer;
    private TextView textIcon;
    private ProgressBar progressAnimation;

    protected void Initialize (Context context, AttributeSet attrs) {
        LayoutInflater inflater = (LayoutInflater)context.getSystemService (Context.LAYOUT_INFLATER_SERVICE);
        inflater.inflate(R.layout.view_stop_button, this, true);

        mainLayout = this.findViewById(R.id.mainLayout);
        internalContainer = this.findViewById(R.id.internalContainer);
        textIcon = this.findViewById(R.id.textIcon);
        progressAnimation = this.findViewById(R.id.progressAnimation);

        progressAnimation.setVisibility(GONE);

        internalContainer.setClipToOutline(true);

        setOnTouchListener(this);
        setViewForCheckingClicks(this);
    }

    @Override
    public boolean onTouch(View v, MotionEvent event) {
        if (event.getAction() == MotionEvent.ACTION_DOWN) {
            mainLayout.setScaleX(0.98f);
            mainLayout.setScaleY(0.98f);
        } else if (event.getAction() == MotionEvent.ACTION_CANCEL || event.getAction() == MotionEvent.ACTION_POINTER_UP || event.getAction() == MotionEvent.ACTION_UP) {
            mainLayout.setScaleX(1.0f);
            mainLayout.setScaleY(1.0f);
        }

        return false;
    }

    @Override
    public void setEnabled(boolean enabled) {
        super.setEnabled(enabled);

        if (enabled) {
            setAlpha(1f);
        } else {
            setAlpha(0.5f);
        }
    }

    public void setBusyState(boolean busy) {
        if (busy) {
            if (!getBusyState()) {
                progressAnimation.setVisibility(VISIBLE);
                textIcon.setVisibility(GONE);
            }
        } else {
            if (getBusyState()) {
                progressAnimation.setVisibility(GONE);
                textIcon.setVisibility(VISIBLE);
            }
        }
    }

    public boolean getBusyState() {
        return progressAnimation.getVisibility() == VISIBLE;
    }
}
