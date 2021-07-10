package com.skywire.skycoin.vpn.controls.options;

import android.content.Context;
import android.content.res.TypedArray;
import android.graphics.drawable.RippleDrawable;
import android.util.AttributeSet;
import android.view.LayoutInflater;
import android.view.MotionEvent;
import android.view.View;
import android.widget.ImageView;
import android.widget.LinearLayout;
import android.widget.TextView;

import com.skywire.skycoin.vpn.R;
import com.skywire.skycoin.vpn.extensible.ListButtonBase;

public class OptionsItem extends ListButtonBase<Void> implements View.OnTouchListener {
    public static class SelectableOption {
        public String icon;
        public Integer drawableId;
        public String label;
        public int translatableLabelId = -1;
        public boolean disabled = false;
    }

    private LinearLayout mainContainer;
    private ImageView imageBitmap;
    private TextView textIcon;
    private TextView text;

    private RippleDrawable rippleDrawable;

    public OptionsItem(Context context) {
        super(context);
    }
    public OptionsItem(Context context, AttributeSet attrs) {
        super(context, attrs);
    }
    public OptionsItem(Context context, AttributeSet attrs, int defStyle) {
        super(context, attrs, defStyle);
    }

    @Override
    protected void Initialize (Context context, AttributeSet attrs) {
        LayoutInflater inflater = (LayoutInflater)context.getSystemService (Context.LAYOUT_INFLATER_SERVICE);
        inflater.inflate(R.layout.view_options_item, this, true);

        mainContainer = this.findViewById (R.id.mainContainer);
        imageBitmap = this.findViewById (R.id.imageBitmap);
        textIcon = this.findViewById (R.id.textIcon);
        text = this.findViewById (R.id.text);

        rippleDrawable = (RippleDrawable) mainContainer.getBackground();

        setOnTouchListener(this);

        if (attrs != null) {
            TypedArray attributes = context.getTheme().obtainStyledAttributes(
                attrs,
                R.styleable.OptionsItem,
                0, 0
            );

            String iconText = attributes.getString(R.styleable.OptionsItem_icon_text);
            if (iconText != null) {
                textIcon.setText(iconText);
            }

            text.setText(attributes.getString(R.styleable.OptionsItem_text));

            attributes.recycle();
        }

        setViewForCheckingClicks(this);
    }

    public void setParams(SelectableOption params) {
        if (params.icon != null) {
            textIcon.setText(params.icon);
            textIcon.setVisibility(VISIBLE);
            imageBitmap.setVisibility(GONE);
        } else {
            textIcon.setVisibility(GONE);

            if (params.drawableId != null) {
                imageBitmap.setImageResource(params.drawableId);
                imageBitmap.setVisibility(VISIBLE);
            } else {
                imageBitmap.setVisibility(GONE);
            }
        }

        if (params.translatableLabelId != -1) {
            text.setText(params.translatableLabelId);
        } else if (params.label != null) {
            text.setText(params.label);
        }

        if (params.disabled) {
            this.setAlpha(0.5f);
            this.setClickable(false);
        } else {
            this.setAlpha(1f);
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
