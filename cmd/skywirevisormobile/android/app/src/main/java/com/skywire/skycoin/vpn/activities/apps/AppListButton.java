package com.skywire.skycoin.vpn.activities.apps;

import android.content.Context;
import android.content.pm.ResolveInfo;
import android.graphics.drawable.RippleDrawable;
import android.util.AttributeSet;
import android.view.LayoutInflater;
import android.view.MotionEvent;
import android.view.View;
import android.widget.CheckBox;
import android.widget.FrameLayout;
import android.widget.ImageView;
import android.widget.LinearLayout;
import android.widget.TextView;

import com.skywire.skycoin.vpn.R;
import com.skywire.skycoin.vpn.extensible.ListButtonBase;

public class AppListButton extends ListButtonBase<Void> implements View.OnTouchListener {
    public static final float APROX_HEIGHT_DP = 55;

    private FrameLayout mainLayout;
    private LinearLayout internalLayout;
    private ImageView imageIcon;
    private FrameLayout layoutSeparator;
    private TextView textAppName;
    private CheckBox checkSelected;
    private View separator;

    private RippleDrawable rippleDrawable;

    private String appPackageName;

    public AppListButton(Context context) {
        super(context);
    }
    public AppListButton(Context context, AttributeSet attrs) {
        super(context, attrs);
    }
    public AppListButton(Context context, AttributeSet attrs, int defStyle) {
        super(context, attrs, defStyle);
    }

    @Override
    protected void Initialize (Context context, AttributeSet attrs) {
        LayoutInflater inflater = (LayoutInflater)context.getSystemService (Context.LAYOUT_INFLATER_SERVICE);
        inflater.inflate(R.layout.view_app_list_item, this, true);

        mainLayout = this.findViewById (R.id.mainLayout);
        internalLayout = this.findViewById (R.id.internalLayout);
        imageIcon = this.findViewById (R.id.imageIcon);
        layoutSeparator = this.findViewById (R.id.layoutSeparator);
        textAppName = this.findViewById (R.id.textAppName);
        checkSelected = this.findViewById (R.id.checkSelected);
        separator = this.findViewById (R.id.separator);

        rippleDrawable = (RippleDrawable) mainLayout.getBackground();
        setOnTouchListener(this);
        setViewForCheckingClicks(this);

        setUseBigFastClickPrevention(false);
    }

    public void setSeparatorVisibility(boolean visible) {
        if (visible) {
            separator.setVisibility(VISIBLE);
        } else {
            separator.setVisibility(GONE);
        }
    }

    public void changeData(ResolveInfo appData) {
        if (appData != null) {
            appPackageName = appData.activityInfo.packageName;
            imageIcon.setImageDrawable(appData.activityInfo.loadIcon(this.getContext().getPackageManager()));
            textAppName.setText(appData.activityInfo.loadLabel(this.getContext().getPackageManager()));
            imageIcon.setVisibility(VISIBLE);
            layoutSeparator.setVisibility(VISIBLE);
            setVisibility(VISIBLE);
        } else {
            setVisibility(INVISIBLE);
        }
    }

    public void changeData(String appPackageName) {
        imageIcon.setVisibility(GONE);
        layoutSeparator.setVisibility(GONE);
        if (appPackageName != null) {
            this.appPackageName = appPackageName;
            textAppName.setText(appPackageName);
            setVisibility(VISIBLE);
        } else {
            setVisibility(INVISIBLE);
        }
    }

    public String getAppPackageName() {
        return appPackageName;
    }

    public void setChecked(boolean checked) {
        checkSelected.setChecked(checked);
    }

    @Override
    public void setEnabled(boolean enabled) {
        super.setEnabled(enabled);

        if (enabled) {
            internalLayout.setAlpha(1f);
        } else {
            internalLayout.setAlpha(0.5f);
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
