package com.skywire.skycoin.vpn.controls;

import android.content.Context;
import android.view.LayoutInflater;
import android.widget.FrameLayout;
import android.widget.TextView;

import com.skywire.skycoin.vpn.R;

public class TopTab extends FrameLayout {
    private TextView text;

    public TopTab(Context context, int textResource) {
        super(context);

        LayoutInflater inflater = (LayoutInflater)context.getSystemService (Context.LAYOUT_INFLATER_SERVICE);
        inflater.inflate(R.layout.view_top_tab, this, true);

        text = this.findViewById (R.id.text);
        text.setText(textResource);
    }
}
