package com.skywire.skycoin.vpn.activities.servers;

import android.content.Context;
import android.util.AttributeSet;
import android.view.LayoutInflater;
import android.widget.ImageView;
import android.widget.LinearLayout;
import android.widget.TextView;

import androidx.annotation.NonNull;
import androidx.core.content.ContextCompat;

import com.skywire.skycoin.vpn.R;
import com.skywire.skycoin.vpn.controls.BoxRowLayout;
import com.skywire.skycoin.vpn.controls.ServerName;
import com.skywire.skycoin.vpn.controls.SettingsButton;
import com.skywire.skycoin.vpn.extensible.ListButtonBase;
import com.skywire.skycoin.vpn.helpers.BoxRowTypes;
import com.skywire.skycoin.vpn.helpers.HelperFunctions;
import com.skywire.skycoin.vpn.objects.ServerRatings;

import java.text.DateFormat;
import java.text.SimpleDateFormat;

public class ServerListButton extends ListButtonBase<Void> {
    public static final float APROX_HEIGHT_DP = 55;

    private static DateFormat dateFormat = new SimpleDateFormat("yyyy/MM/dd hh:mm a");

    private BoxRowLayout mainLayout;
    private ImageView imageFlag;
    private ServerName serverName;
    private TextView textDate;
    private TextView textLocation;
    private TextView textLatency;
    private TextView textCongestion;
    private TextView textHops;
    private TextView textLatencyRating;
    private TextView textCongestionRating;
    private TextView textNote;
    private TextView textPersonalNote;
    private LinearLayout statsArea1;
    private LinearLayout statsArea2;
    private LinearLayout noteArea;
    private LinearLayout personalNoteArea;
    private SettingsButton buttonSettings;

    private VpnServerForList server;
    private ServerLists listType;

    public ServerListButton (Context context) {
        super(context);
    }

    @Override
    protected void Initialize (Context context, AttributeSet attrs) {
        LayoutInflater inflater = (LayoutInflater)context.getSystemService (Context.LAYOUT_INFLATER_SERVICE);
        inflater.inflate(R.layout.view_server_list_item, this, true);

        mainLayout = this.findViewById (R.id.mainLayout);
        imageFlag = this.findViewById (R.id.imageFlag);
        serverName = this.findViewById (R.id.serverName);
        textDate = this.findViewById (R.id.textDate);
        textLocation = this.findViewById (R.id.textLocation);
        textLatency = this.findViewById (R.id.textLatency);
        textCongestion = this.findViewById (R.id.textCongestion);
        textHops = this.findViewById (R.id.textHops);
        textLatencyRating = this.findViewById (R.id.textLatencyRating);
        textCongestionRating = this.findViewById (R.id.textCongestionRating);
        textNote = this.findViewById (R.id.textNote);
        textPersonalNote = this.findViewById (R.id.textPersonalNote);
        statsArea1 = this.findViewById (R.id.statsArea1);
        statsArea2 = this.findViewById (R.id.statsArea2);
        noteArea = this.findViewById (R.id.noteArea);
        personalNoteArea = this.findViewById (R.id.personalNoteArea);
        buttonSettings = this.findViewById (R.id.buttonSettings);

        imageFlag.setClipToOutline(true);

        buttonSettings.setClickEventListener(view -> showOptions());

        setClickableBoxView(mainLayout);
    }

    public void changeData(@NonNull VpnServerForList serverData, ServerLists listType) {
        server = serverData;
        this.listType = listType;

        imageFlag.setImageResource(HelperFunctions.getFlagResourceId(serverData.countryCode));
        serverName.setServer(serverData, listType, false);

        if (serverData.location != null && !serverData.location.trim().equals("")) {
            String pk = serverData.pk;
            if (pk.length() > 5) {
                pk = pk.substring(0, 5);
            }
            textLocation.setText("(" + pk + ") " + serverData.location);
        } else {
            textLocation.setText(serverData.pk);
        }

        if (serverData.note != null && serverData.note.trim() != "") {
            noteArea.setVisibility(VISIBLE);
            textNote.setText(serverData.note);
        } else {
            noteArea.setVisibility(GONE);
        }
        if (serverData.personalNote != null && serverData.personalNote.trim() != "") {
            personalNoteArea.setVisibility(VISIBLE);
            textPersonalNote.setText(serverData.personalNote);
        } else {
            personalNoteArea.setVisibility(GONE);
        }

        if (listType == ServerLists.Public) {
            statsArea1.setVisibility(VISIBLE);
            statsArea2.setVisibility(VISIBLE);

            textLatency.setText(HelperFunctions.getLatencyValue(serverData.latency));
            textCongestion.setText(HelperFunctions.zeroDecimalsFormatter.format(serverData.congestion) + "%");
            textHops.setText(serverData.hops + "");

            textLatencyRating.setText(ServerRatings.getTextForRating(serverData.latencyRating));
            textLatencyRating.setTextColor(getRatingColor(serverData.latencyRating));
            textCongestionRating.setText(ServerRatings.getTextForRating(serverData.congestionRating));
            textCongestionRating.setTextColor(getRatingColor(serverData.congestionRating));

            textCongestion.setTextColor(HelperFunctions.getCongestionNumberColor((int)serverData.congestion));
            textLatency.setTextColor(HelperFunctions.getLatencyNumberColor((int)serverData.latency));
            textHops.setTextColor(HelperFunctions.getHopsNumberColor((int)serverData.hops));
        } else {
            statsArea1.setVisibility(GONE);
            statsArea2.setVisibility(GONE);
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

    private int getRatingColor(ServerRatings rating) {
        int colorId = R.color.bronze;

        if (rating == ServerRatings.Gold) {
            colorId = R.color.gold;
        } else if (rating == ServerRatings.Silver) {
            colorId = R.color.silver;
        }

        return ContextCompat.getColor(getContext(), colorId);
    }

    private void showOptions() {
        HelperFunctions.showServerOptions(getContext(), server, listType);
    }
}
