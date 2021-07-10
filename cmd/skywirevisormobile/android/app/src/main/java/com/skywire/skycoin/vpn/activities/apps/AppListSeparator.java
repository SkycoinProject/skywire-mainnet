package com.skywire.skycoin.vpn.activities.apps;

import android.content.Context;
import android.view.LayoutInflater;
import android.widget.LinearLayout;
import android.widget.TextView;

import com.skywire.skycoin.vpn.R;
import com.skywire.skycoin.vpn.helpers.HelperFunctions;

public class AppListSeparator extends LinearLayout {
    private TextView textTitle;

    public AppListSeparator(Context context) {
        super(context);

        LayoutInflater inflater = (LayoutInflater)context.getSystemService (Context.LAYOUT_INFLATER_SERVICE);
        inflater.inflate(R.layout.view_app_list_separator, this, true);

        textTitle = this.findViewById (R.id.textTitle);

        int tabletExtraHorizontalPadding = HelperFunctions.getTabletExtraHorizontalPadding(getContext());
        setPadding(tabletExtraHorizontalPadding, 0, tabletExtraHorizontalPadding, 0);
    }

    public void changeTitle(int title) {
        textTitle.setText(title);
    }
}
