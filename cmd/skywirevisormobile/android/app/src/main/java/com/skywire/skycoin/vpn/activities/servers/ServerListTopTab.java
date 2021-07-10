package com.skywire.skycoin.vpn.activities.servers;

import android.content.Context;
import android.content.res.TypedArray;
import android.graphics.drawable.RippleDrawable;
import android.util.AttributeSet;
import android.view.LayoutInflater;
import android.view.MotionEvent;
import android.view.View;
import android.widget.FrameLayout;
import android.widget.TextView;

import com.skywire.skycoin.vpn.R;
import com.skywire.skycoin.vpn.extensible.ButtonBase;

public class ServerListTopTab extends ButtonBase implements View.OnTouchListener {
    private FrameLayout mainLayout;
    private View clickBackground;
    private TextView text;

    private RippleDrawable rippleDrawable;

    public ServerListTopTab(Context context) {
        super(context);
    }
    public ServerListTopTab(Context context, AttributeSet attrs) {
        super(context, attrs);
    }
    public ServerListTopTab(Context context, AttributeSet attrs, int defStyle) {
        super(context, attrs, defStyle);
    }

    @Override
    protected void Initialize (Context context, AttributeSet attrs) {
        LayoutInflater inflater = (LayoutInflater)context.getSystemService (Context.LAYOUT_INFLATER_SERVICE);
        inflater.inflate(R.layout.view_server_list_top_tab, this, true);

        mainLayout = this.findViewById (R.id.mainLayout);
        clickBackground = this.findViewById (R.id.clickBackground);
        text = this.findViewById (R.id.text);

        rippleDrawable = (RippleDrawable) clickBackground.getBackground();

        if (attrs != null) {
            TypedArray attributes = context.getTheme().obtainStyledAttributes(
                attrs,
                R.styleable.ServerListTopTab,
                0, 0
            );

            int corner = attributes.getInteger(R.styleable.ServerListTopTab_position, 0);
            if (corner != 0) {
                if (corner == 1) {
                    mainLayout.setBackgroundResource(R.drawable.box_clip_area_left);
                } else if (corner == 2) {
                    mainLayout.setBackgroundResource(R.drawable.box_clip_area_right);
                }

                mainLayout.setClipToOutline(true);
            }

            String txt = attributes.getString(R.styleable.ServerListTopTab_text);
            if (txt != null && !txt.trim().equals("")) {
                text.setText(txt);
            }

            attributes.recycle();
        }

        clickBackground.setOnTouchListener(this);
        setViewForCheckingClicks(clickBackground);
    }

    public void changeState(boolean selected) {
        if (selected) {
            clickBackground.setBackgroundResource(R.color.tablet_selected_tab_background);
            rippleDrawable = null;
            this.setClickable(false);
        } else {
            clickBackground.setBackgroundResource(R.drawable.box_ripple);
            rippleDrawable = (RippleDrawable) clickBackground.getBackground();
            this.setClickable(true);
        }
    }

    @Override
    public boolean onTouch(View v, MotionEvent event) {
        if (rippleDrawable != null) {
            rippleDrawable.setHotspot(event.getX(), event.getY());
        }

        return false;
    }
}
