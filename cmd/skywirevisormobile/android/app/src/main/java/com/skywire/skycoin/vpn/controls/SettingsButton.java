package com.skywire.skycoin.vpn.controls;

import android.content.Context;
import android.content.res.TypedArray;
import android.util.AttributeSet;
import android.view.LayoutInflater;
import android.view.MotionEvent;
import android.view.View;
import android.widget.TextView;

import com.skywire.skycoin.vpn.R;
import com.skywire.skycoin.vpn.extensible.ButtonBase;

public class SettingsButton extends ButtonBase implements View.OnTouchListener {
    private TextView textIcon;

    public SettingsButton(Context context) {
        super(context);
    }
    public SettingsButton(Context context, AttributeSet attrs) {
        super(context, attrs);
    }
    public SettingsButton(Context context, AttributeSet attrs, int defStyle) {
        super(context, attrs, defStyle);
    }

    @Override
    protected void Initialize (Context context, AttributeSet attrs) {
        LayoutInflater inflater = (LayoutInflater)context.getSystemService (Context.LAYOUT_INFLATER_SERVICE);
        inflater.inflate(R.layout.view_settings_button, this, true);

        textIcon = this.findViewById (R.id.textIcon);

        if (attrs != null) {
            TypedArray attributes = context.getTheme().obtainStyledAttributes(
                attrs,
                R.styleable.SettingsButton,
                0, 0
            );

            boolean useNoteIcon = attributes.getBoolean(R.styleable.SettingsButton_use_note_icon, false);
            if (useNoteIcon) {
                textIcon.setText("\ue88f");
            }

            attributes.recycle();
        }

        setOnTouchListener(this);
        setViewForCheckingClicks(this);
    }

    @Override
    public boolean onTouch(View v, MotionEvent event) {
        if (event.getAction() == MotionEvent.ACTION_DOWN) {
            textIcon.setAlpha(0.5f);
        } else if (event.getAction() == MotionEvent.ACTION_CANCEL || event.getAction() == MotionEvent.ACTION_POINTER_UP || event.getAction() == MotionEvent.ACTION_UP) {
            textIcon.setAlpha(1.0f);
        }

        return false;
    }
}
