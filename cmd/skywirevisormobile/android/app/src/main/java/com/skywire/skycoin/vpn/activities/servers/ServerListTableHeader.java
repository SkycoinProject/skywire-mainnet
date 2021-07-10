package com.skywire.skycoin.vpn.activities.servers;

import android.content.Context;
import android.view.LayoutInflater;
import android.widget.FrameLayout;
import android.widget.LinearLayout;
import android.widget.TextView;

import com.skywire.skycoin.vpn.R;

public class ServerListTableHeader extends FrameLayout {
    private TextView textDate;
    private LinearLayout statsArea;

    public ServerListTableHeader(Context context) {
        super(context);

        LayoutInflater inflater = (LayoutInflater)context.getSystemService (Context.LAYOUT_INFLATER_SERVICE);
        inflater.inflate(R.layout.view_server_list_table_header, this, true);

        textDate = this.findViewById (R.id.textDate);
        statsArea = this.findViewById (R.id.statsArea);
    }

    public void setListType(ServerLists listType) {
        if (listType == ServerLists.Public) {
            statsArea.setVisibility(VISIBLE);
        } else {
            statsArea.setVisibility(GONE);
        }

        if (listType == ServerLists.History) {
            textDate.setVisibility(VISIBLE);
        } else {
            textDate.setVisibility(GONE);
        }
    }
}
