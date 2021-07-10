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

public class Tab extends ButtonBase implements View.OnTouchListener {
    private LinearLayout mainContainer;
    private LinearLayout internalContainer;
    private FrameLayout rightBorder;
    private TextView textIcon;
    private TextView textName;

    private RippleDrawable rippleDrawable;

    public Tab(Context context) {
        super(context);
    }
    public Tab(Context context, AttributeSet attrs) {
        super(context, attrs);
    }
    public Tab(Context context, AttributeSet attrs, int defStyle) {
        super(context, attrs, defStyle);
    }

    @Override
    protected void Initialize (Context context, AttributeSet attrs) {
        LayoutInflater inflater = (LayoutInflater)context.getSystemService (Context.LAYOUT_INFLATER_SERVICE);
        inflater.inflate(R.layout.view_tab, this, true);

        mainContainer = this.findViewById (R.id.mainContainer);
        internalContainer = this.findViewById (R.id.internalContainer);
        rightBorder = this.findViewById (R.id.rightBorder);
        textIcon = this.findViewById (R.id.textIcon);
        textName = this.findViewById (R.id.textName);

        rippleDrawable = (RippleDrawable) internalContainer.getBackground();

        if (attrs != null) {
            TypedArray attributes = context.getTheme().obtainStyledAttributes(
                    attrs,
                    R.styleable.Tab,
                    0, 0
            );

            String iconText = attributes.getString(R.styleable.Tab_icon_text);
            if (iconText != null) {
                textIcon.setText(iconText);
            }

            textName.setText(attributes.getString(R.styleable.Tab_lower_text));

            if (!attributes.getBoolean(R.styleable.Tab_show_right_border, true)) {
                rightBorder.setVisibility(GONE);
            }

            attributes.recycle();
        }

        setOnTouchListener(this);
        setViewForCheckingClicks(this);
    }

    public void changeState(boolean selected) {
        if (selected) {
            mainContainer.setBackgroundResource(R.color.bar_selected);
            internalContainer.setBackground(null);
            rippleDrawable = null;
            this.setClickable(false);
        } else {
            mainContainer.setBackgroundResource(R.color.bar_background);
            internalContainer.setBackgroundResource(R.drawable.box_ripple);
            rippleDrawable = (RippleDrawable) internalContainer.getBackground();
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
