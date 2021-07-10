package com.skywire.skycoin.vpn.controls;

import android.app.Activity;
import android.content.Context;
import android.content.res.TypedArray;
import android.util.AttributeSet;
import android.view.LayoutInflater;
import android.view.View;
import android.widget.ImageView;
import android.widget.LinearLayout;
import android.widget.TextView;

import com.skywire.skycoin.vpn.R;
import com.skywire.skycoin.vpn.extensible.ClickEvent;
import com.skywire.skycoin.vpn.extensible.ClickWithIndexEvent;
import com.skywire.skycoin.vpn.helpers.UiMaterialIcons;

public class TopBar extends LinearLayout implements ClickEvent {
    public TopBar(Context context) {
        super(context);
        Initialize(context, null);
    }
    public TopBar(Context context, AttributeSet attrs) {
        super(context, attrs);
        Initialize(context, attrs);
    }
    public TopBar(Context context, AttributeSet attrs, int defStyle) {
        super(context, attrs, defStyle);
        Initialize(context, attrs);
    }

    private TopBarButton buttonLeft;
    private ImageView imageIcon;
    private TextView textTitle;

    private ClickWithIndexEvent<Void> clickListener;
    private boolean goBack = false;

    private void Initialize (Context context, AttributeSet attrs) {
        LayoutInflater inflater = (LayoutInflater)context.getSystemService (Context.LAYOUT_INFLATER_SERVICE);
        inflater.inflate(R.layout.view_top_bar, this, true);

        buttonLeft = this.findViewById (R.id.buttonLeft);
        imageIcon = this.findViewById (R.id.imageIcon);
        textTitle = this.findViewById (R.id.textTitle);

        buttonLeft.setClickEventListener(this);

        if (attrs != null) {
            TypedArray attributes = context.getTheme().obtainStyledAttributes(
                attrs,
                R.styleable.TopBar,
                0, 0);

            String title = attributes.getString(R.styleable.TopBar_title);
            if (title == null || title.trim() == "") {
                textTitle.setVisibility(GONE);
            } else {
                imageIcon.setVisibility(GONE);
                textTitle.setText(title);
            }

            int leftButtonIcon = attributes.getInteger(R.styleable.TopBar_left_button_icon, -1);
            if (leftButtonIcon == 0) {
                buttonLeft.setIcon(UiMaterialIcons.MENU);
            } else if (leftButtonIcon == 1) {
                buttonLeft.setIcon(UiMaterialIcons.BACK);
                goBack = true;
            } else {
                buttonLeft.setVisibility(GONE);
            }

            attributes.recycle();
        } else {
            textTitle.setVisibility(GONE);
            buttonLeft.setVisibility(GONE);
        }
    }

    public void setClickWithIndexEventListener(ClickWithIndexEvent<Void> listener) {
        clickListener = listener;
    }

    @Override
    public void onClick(View view) {
        if (clickListener != null) {
            clickListener.onClickWithIndex(0, null);
        }

        if (goBack) {
            ((Activity)getContext()).finish();
        }
    }
}
