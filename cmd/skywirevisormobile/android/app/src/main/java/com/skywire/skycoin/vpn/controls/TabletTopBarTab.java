package com.skywire.skycoin.vpn.controls;

import android.content.Context;
import android.content.res.TypedArray;
import android.graphics.drawable.RippleDrawable;
import android.util.AttributeSet;
import android.view.LayoutInflater;
import android.view.MotionEvent;
import android.view.View;
import android.widget.FrameLayout;
import android.widget.LinearLayout;
import android.widget.TextView;

import com.skywire.skycoin.vpn.R;
import com.skywire.skycoin.vpn.extensible.ButtonBase;

public class TabletTopBarTab extends ButtonBase implements View.OnTouchListener {
    private FrameLayout mainContainer;
    private LinearLayout internalContainer;
    private TextView textIcon;
    private TextView textLabel;

    private RippleDrawable rippleDrawable;

    public TabletTopBarTab(Context context) {
        super(context);
    }
    public TabletTopBarTab(Context context, AttributeSet attrs) {
        super(context, attrs);
    }
    public TabletTopBarTab(Context context, AttributeSet attrs, int defStyle) {
        super(context, attrs, defStyle);
    }

    @Override
    protected void Initialize (Context context, AttributeSet attrs) {
        LayoutInflater inflater = (LayoutInflater)context.getSystemService (Context.LAYOUT_INFLATER_SERVICE);
        inflater.inflate(R.layout.view_tablet_top_bar_tab, this, true);

        mainContainer = this.findViewById (R.id.mainContainer);
        internalContainer = this.findViewById (R.id.internalContainer);
        textIcon = this.findViewById (R.id.textIcon);
        textLabel = this.findViewById (R.id.textLabel);

        mainContainer.setClipToOutline(true);

        if (attrs != null) {
            TypedArray attributes = context.getTheme().obtainStyledAttributes(
                attrs,
                R.styleable.TabletTopBarTab,
                0, 0
            );

            String iconText = attributes.getString(R.styleable.TabletTopBarTab_icon_text);
            if (iconText != null) {
                textIcon.setText(iconText);
            }

            textLabel.setText(attributes.getString(R.styleable.TabletTopBarTab_label));

            attributes.recycle();
        }

        setOnTouchListener(this);
        setViewForCheckingClicks(this);

        setSelected(false);
    }

    public void setSelected(boolean selected) {
        if (selected) {
            textIcon.setAlpha(1f);
            textLabel.setAlpha(1f);
            internalContainer.setBackgroundResource(R.drawable.current_server_rounded_box);
            rippleDrawable = null;
            setClickable(false);
        } else {
            textIcon.setAlpha(0.5f);
            textLabel.setAlpha(0.5f);
            internalContainer.setBackgroundResource(R.drawable.current_server_ripple);
            rippleDrawable = (RippleDrawable) internalContainer.getBackground();
            setClickable(true);
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
