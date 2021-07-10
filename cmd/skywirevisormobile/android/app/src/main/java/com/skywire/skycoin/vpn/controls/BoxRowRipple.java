package com.skywire.skycoin.vpn.controls;

import android.content.Context;
import android.graphics.drawable.RippleDrawable;
import android.util.AttributeSet;
import android.view.MotionEvent;
import android.view.View;
import android.view.ViewOutlineProvider;
import android.widget.FrameLayout;

import com.skywire.skycoin.vpn.R;
import com.skywire.skycoin.vpn.extensible.ButtonBase;
import com.skywire.skycoin.vpn.helpers.BoxRowTypes;

public class BoxRowRipple extends ButtonBase implements View.OnTouchListener {
    public BoxRowRipple(Context context) {
        super(context);
    }
    public BoxRowRipple(Context context, AttributeSet attrs) {
        super(context, attrs);
    }
    public BoxRowRipple(Context context, AttributeSet attrs, int defStyle) {
        super(context, attrs, defStyle);
    }

    RippleDrawable rippleDrawable;

    @Override
    protected void Initialize (Context context, AttributeSet attrs) {
        setOutlineProvider(ViewOutlineProvider.BACKGROUND);
        setClipToOutline(true);
        setClickable(true);

        View ripple = new View(context);
        FrameLayout.LayoutParams rippleLayoutParams = new FrameLayout.LayoutParams(LayoutParams.MATCH_PARENT, LayoutParams.MATCH_PARENT);
        ripple.setLayoutParams(rippleLayoutParams);
        ripple.setBackgroundResource(R.drawable.box_ripple);
        this.addView(ripple);

        rippleDrawable = (RippleDrawable) ripple.getBackground();

        ripple.setOnTouchListener(this);
        setViewForCheckingClicks(ripple);

        setType(BoxRowTypes.TOP);
    }

    public void setType(BoxRowTypes type) {
        if (type == BoxRowTypes.TOP) {
            setBackgroundResource(R.drawable.box_row_rounded_box_1);
        } else if (type == BoxRowTypes.MIDDLE) {
            setBackgroundResource(R.drawable.box_row_rounded_box_2);
        } else if (type == BoxRowTypes.BOTTOM) {
            setBackgroundResource(R.drawable.box_row_rounded_box_3);
        } else {
            setBackgroundResource(R.drawable.box_row_rounded_box_4);
        }

        this.invalidate();
    }

    @Override
    public boolean onTouch(View v, MotionEvent event) {
        rippleDrawable.setHotspot(event.getX(), event.getY());

        return false;
    }
}
