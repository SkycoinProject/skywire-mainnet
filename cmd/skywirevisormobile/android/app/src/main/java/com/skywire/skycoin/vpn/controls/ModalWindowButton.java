package com.skywire.skycoin.vpn.controls;

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

public class ModalWindowButton extends ButtonBase implements View.OnTouchListener {
    private FrameLayout mainContainer;
    private FrameLayout effectContainer;
    private TextView text;

    private RippleDrawable rippleDrawable;

    public ModalWindowButton(Context context) {
        super(context);
    }
    public ModalWindowButton(Context context, AttributeSet attrs) {
        super(context, attrs);
    }
    public ModalWindowButton(Context context, AttributeSet attrs, int defStyle) {
        super(context, attrs, defStyle);
    }

    @Override
    protected void Initialize (Context context, AttributeSet attrs) {
        LayoutInflater inflater = (LayoutInflater)context.getSystemService (Context.LAYOUT_INFLATER_SERVICE);
        inflater.inflate(R.layout.view_modal_window_button, this, true);

        mainContainer = this.findViewById (R.id.mainContainer);
        effectContainer = this.findViewById (R.id.effectContainer);
        text = this.findViewById (R.id.text);

        mainContainer.setClipToOutline(true);

        if (attrs != null) {
            TypedArray attributes = context.getTheme().obtainStyledAttributes(
                attrs,
                R.styleable.ModalWindowButton,
                0, 0
            );

            String textForButton = attributes.getString(R.styleable.ModalWindowButton_text);
            if (textForButton != null) {
                text.setText(textForButton);
            }

            if (attributes.getBoolean(R.styleable.ModalWindowButton_use_secondary_color, false)) {
                mainContainer.setBackgroundResource(R.drawable.modal_button_secondary_background);
                effectContainer.setBackgroundResource(R.drawable.modal_button_secondary_ripple);
            }

            attributes.recycle();
        }

        rippleDrawable = (RippleDrawable) effectContainer.getBackground();

        setOnTouchListener(this);
        setViewForCheckingClicks(this);
    }

    public void setText(int resourceId) {
        text.setText(resourceId);
    }

    @Override
    public boolean onTouch(View v, MotionEvent event) {
        if (rippleDrawable != null) {
            rippleDrawable.setHotspot(event.getX(), event.getY());
        }

        return false;
    }

    @Override
    public void setEnabled(boolean enabled) {
        super.setEnabled(enabled);

        if (enabled) {
            this.setAlpha(1);
        } else {
            this.setAlpha(0.35f);
        }
    }
}
