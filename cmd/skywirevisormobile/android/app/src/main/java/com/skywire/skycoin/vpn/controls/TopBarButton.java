package com.skywire.skycoin.vpn.controls;

import android.content.Context;
import android.content.res.TypedArray;
import android.util.AttributeSet;
import android.view.LayoutInflater;
import android.widget.TextView;

import com.skywire.skycoin.vpn.R;
import com.skywire.skycoin.vpn.extensible.ButtonBase;
import com.skywire.skycoin.vpn.helpers.UiMaterialIcons;

public class TopBarButton extends ButtonBase {
    public TopBarButton(Context context) {
        super(context);
    }
    public TopBarButton(Context context, AttributeSet attrs) {
        super(context, attrs);
    }
    public TopBarButton(Context context, AttributeSet attrs, int defStyle) {
        super(context, attrs, defStyle);
    }

    private TextView textIcon;

    @Override
    protected void Initialize (Context context, AttributeSet attrs) {
        LayoutInflater inflater = (LayoutInflater)context.getSystemService (Context.LAYOUT_INFLATER_SERVICE);
        inflater.inflate(R.layout.view_top_bar_button, this, true);

        textIcon = this.findViewById (R.id.textIcon);

        if (attrs != null) {
            TypedArray attributes = context.getTheme().obtainStyledAttributes(
                attrs,
                R.styleable.TopBarButton,
                0, 0);

            if (attributes.getInteger(R.styleable.TopBarButton_material_icon, 0) == 0) {
                textIcon.setText("\ue5d2");
            } else {
                textIcon.setText("\ue5c4");
            }

            attributes.recycle();
        } else {
            textIcon.setText("\ue5d2");
        }

        setViewForCheckingClicks(this);
    }

    public void setIcon(UiMaterialIcons icon) {
        if (icon == UiMaterialIcons.MENU) {
            textIcon.setText("\ue5d2");
        } else {
            textIcon.setText("\ue5c4");
        }
    }
}
