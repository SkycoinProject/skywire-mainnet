package com.skywire.skycoin.vpn.activities.apps;

import android.content.Context;
import android.content.pm.ResolveInfo;
import android.view.LayoutInflater;
import android.widget.FrameLayout;
import android.widget.LinearLayout;

import com.skywire.skycoin.vpn.R;
import com.skywire.skycoin.vpn.controls.BoxRowLayout;
import com.skywire.skycoin.vpn.extensible.ClickWithIndexEvent;
import com.skywire.skycoin.vpn.helpers.BoxRowTypes;

public class AppListRow extends FrameLayout implements ClickWithIndexEvent<Void> {
    private BoxRowLayout mainLayout;
    private LinearLayout buttonsContainer;

    private AppListButton[] buttons;
    private ClickWithIndexEvent<Void> clickListener;

    public AppListRow(Context context, int buttonsPerRow) {
        super(context);

        LayoutInflater inflater = (LayoutInflater)context.getSystemService (Context.LAYOUT_INFLATER_SERVICE);
        inflater.inflate(R.layout.view_app_list_row, this, true);

        mainLayout = this.findViewById(R.id.mainLayout);
        buttonsContainer = this.findViewById(R.id.buttonsContainer);

        buttonsContainer.setClipToOutline(true);

        buttons = new AppListButton[buttonsPerRow];
        LinearLayout.LayoutParams layoutParams = new LinearLayout.LayoutParams(LayoutParams.MATCH_PARENT, LayoutParams.WRAP_CONTENT, 1f);
        for (int i = 0; i < buttonsPerRow; i++) {
            AppListButton btn = new AppListButton(context);
            btn.setLayoutParams(layoutParams);
            btn.setClickWithIndexEventListener(this);
            buttons[i] = btn;
            buttonsContainer.addView(btn);
        }
    }

    public void setIndex(int index) {
        for (int i = 0; i < buttons.length; i++) {
            buttons[i].setIndex(index + i);
        }
    }

    public void setClickWithIndexEventListener(ClickWithIndexEvent<Void> listener) {
        clickListener = listener;
    }

    public void changeData(ResolveInfo[] appData) {
        for (int i = 0; i < buttons.length; i++) {
            buttons[i].changeData(appData[i]);
        }
    }

    public void changeData(String[] appPackageName) {
        for (int i = 0; i < buttons.length; i++) {
            buttons[i].changeData(appPackageName[i]);
        }
    }

    public void setBoxRowType(BoxRowTypes type) {
        mainLayout.setType(type);

        boolean showSeparator = true;
        if (type == BoxRowTypes.TOP) {
            buttonsContainer.setBackgroundResource(R.drawable.internal_box_row_rounded_box_1);
        } else if (type == BoxRowTypes.MIDDLE) {
            buttonsContainer.setBackgroundResource(R.drawable.internal_box_row_rounded_box_2);
        } else if (type == BoxRowTypes.BOTTOM) {
            buttonsContainer.setBackgroundResource(R.drawable.internal_box_row_rounded_box_3);
            showSeparator = false;
        } else {
            buttonsContainer.setBackgroundResource(R.drawable.internal_box_row_rounded_box_4);
            showSeparator = false;
        }

        for (int i = 0; i < buttons.length; i++) {
            buttons[i].setSeparatorVisibility(showSeparator);
        }
    }

    public void setChecked(String packageName, boolean checked) {
        for (int i = 0; i < buttons.length; i++) {
            if (buttons[i].getAppPackageName() != null && buttons[i].getAppPackageName().equals(packageName)) {
                buttons[i].setChecked(checked);
            }
        }
    }

    public void setChecked(boolean[] checked) {
        for (int i = 0; i < buttons.length; i++) {
            buttons[i].setChecked(checked[i]);
        }
    }

    @Override
    public void setEnabled(boolean enabled) {
        super.setEnabled(enabled);

        for (int i = 0; i < buttons.length; i++) {
            buttons[i].setEnabled(enabled);
        }
    }

    @Override
    public void onClickWithIndex(int index, Void data) {
        if (clickListener != null) {
            clickListener.onClickWithIndex(index, data);
        }
    }
}
