package com.skywire.skycoin.vpn.activities.servers;

import android.content.Context;
import android.util.AttributeSet;
import android.view.LayoutInflater;
import android.widget.ImageView;
import android.widget.LinearLayout;
import android.widget.TextView;

import androidx.annotation.NonNull;

import com.skywire.skycoin.vpn.R;
import com.skywire.skycoin.vpn.controls.BoxRowLayout;
import com.skywire.skycoin.vpn.controls.ServerName;
import com.skywire.skycoin.vpn.controls.ServerNotesModalWindow;
import com.skywire.skycoin.vpn.controls.SettingsButton;
import com.skywire.skycoin.vpn.extensible.ListButtonBase;
import com.skywire.skycoin.vpn.helpers.BoxRowTypes;
import com.skywire.skycoin.vpn.helpers.HelperFunctions;
import com.skywire.skycoin.vpn.objects.ServerRatings;

import java.text.DateFormat;
import java.text.SimpleDateFormat;

public class ServerListTableRow extends ListButtonBase<Void> {
    public static final float APROX_HEIGHT_DP = 50;

    private static DateFormat dateFormat = new SimpleDateFormat("yyyy/MM/dd hh:mm a");

    private BoxRowLayout mainLayout;
    private ImageView imageFlag;
    private ImageView imageCongestionRating;
    private ImageView imageLatencyRating;
    private ServerName serverName;
    private TextView textDate;
    private TextView textLocation;
    private TextView textPk;
    private TextView textCongestion;
    private TextView textLatency;
    private TextView textHops;
    private LinearLayout statsArea;
    private SettingsButton buttonNote;
    private SettingsButton buttonSettings;

    private VpnServerForList server;
    private ServerLists listType;

    public ServerListTableRow(Context context) {
        super(context);
    }

    @Override
    protected void Initialize (Context context, AttributeSet attrs) {
        LayoutInflater inflater = (LayoutInflater)context.getSystemService (Context.LAYOUT_INFLATER_SERVICE);
        inflater.inflate(R.layout.view_server_list_table_row, this, true);

        mainLayout = this.findViewById (R.id.mainLayout);
        imageFlag = this.findViewById (R.id.imageFlag);
        imageCongestionRating = this.findViewById (R.id.imageCongestionRating);
        imageLatencyRating = this.findViewById (R.id.imageLatencyRating);
        serverName = this.findViewById (R.id.serverName);
        textDate = this.findViewById (R.id.textDate);
        textLocation = this.findViewById (R.id.textLocation);
        textPk = this.findViewById (R.id.textPk);
        textCongestion = this.findViewById (R.id.textCongestion);
        textLatency = this.findViewById (R.id.textLatency);
        textHops = this.findViewById (R.id.textHops);
        statsArea = this.findViewById (R.id.statsArea);
        buttonNote = this.findViewById (R.id.buttonNote);
        buttonSettings = this.findViewById (R.id.buttonSettings);

        imageFlag.setClipToOutline(true);

        buttonNote.setClickEventListener(view -> showNotes());
        buttonSettings.setClickEventListener(view -> showOptions());

        setClickableBoxView(mainLayout);
    }

    public void changeData(@NonNull VpnServerForList serverData, ServerLists listType) {
        server = serverData;
        this.listType = listType;

        imageFlag.setImageResource(HelperFunctions.getFlagResourceId(serverData.countryCode));
        serverName.setServer(serverData, listType, false);

        if (serverData.location != null && serverData.location.trim().length() > 0) {
            textLocation.setText(serverData.location);
        } else {
            textLocation.setText(R.string.tmp_select_server_unknown_location);
        }

        textPk.setText(serverData.pk);

        if ((serverData.note == null || serverData.note.equals("")) && (serverData.personalNote == null || serverData.personalNote.equals(""))) {
            buttonNote.setVisibility(GONE);
        } else {
            buttonNote.setVisibility(VISIBLE);
        }

        if (listType == ServerLists.Public) {
            statsArea.setVisibility(VISIBLE);

            textCongestion.setText(HelperFunctions.zeroDecimalsFormatter.format(serverData.congestion) + "%");
            textLatency.setText(HelperFunctions.getLatencyValue(serverData.latency));
            textHops.setText(serverData.hops + "");

            textCongestion.setTextColor(HelperFunctions.getCongestionNumberColor((int)serverData.congestion));
            textLatency.setTextColor(HelperFunctions.getLatencyNumberColor((int)serverData.latency));
            textHops.setTextColor(HelperFunctions.getHopsNumberColor((int)serverData.hops));

            imageCongestionRating.setImageResource(getRatingResource(serverData.congestionRating));
            imageLatencyRating.setImageResource(getRatingResource(serverData.latencyRating));
        } else {
            statsArea.setVisibility(GONE);
        }

        if (listType == ServerLists.History) {
            textDate.setVisibility(VISIBLE);
            textDate.setText(dateFormat.format(serverData.lastUsed));
        } else {
            textDate.setVisibility(GONE);
        }
    }

    public void setBoxRowType(BoxRowTypes type) {
        mainLayout.setType(type);
    }

    private int getRatingResource(ServerRatings rating) {
        if (rating == ServerRatings.Gold) {
            return R.drawable.gold_rating;
        } else if (rating == ServerRatings.Silver) {
            return R.drawable.silver_rating;
        }

        return R.drawable.bronze_rating;
    }

    private void showNotes() {
        ServerNotesModalWindow modal = new ServerNotesModalWindow(getContext(), server);
        modal.show();
    }

    private void showOptions() {
        HelperFunctions.showServerOptions(getContext(), server, listType);
    }
}
