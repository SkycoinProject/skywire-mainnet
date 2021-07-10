package com.skywire.skycoin.vpn.controls;

import android.app.Dialog;
import android.content.Context;
import android.os.Bundle;
import android.text.SpannableStringBuilder;
import android.text.Spanned;
import android.text.style.ForegroundColorSpan;
import android.text.style.RelativeSizeSpan;
import android.view.View;
import android.view.Window;
import android.widget.LinearLayout;
import android.widget.TextView;

import androidx.core.content.res.ResourcesCompat;

import com.skywire.skycoin.vpn.R;
import com.skywire.skycoin.vpn.activities.servers.ServerLists;
import com.skywire.skycoin.vpn.activities.servers.VpnServerForList;
import com.skywire.skycoin.vpn.extensible.ClickEvent;
import com.skywire.skycoin.vpn.helpers.CountriesList;
import com.skywire.skycoin.vpn.helpers.HelperFunctions;
import com.skywire.skycoin.vpn.helpers.MaterialFontSpan;
import com.skywire.skycoin.vpn.objects.ServerFlags;
import com.skywire.skycoin.vpn.objects.ServerRatings;
import com.skywire.skycoin.vpn.vpn.VPNServersPersistentData;

import java.text.DateFormat;
import java.text.SimpleDateFormat;

public class ServerInfoModalWindow extends Dialog implements ClickEvent {
    private ForegroundColorSpan lightColorSpan =
        new ForegroundColorSpan(ResourcesCompat.getColor(getContext().getResources(), R.color.modal_window_light_text, null));
    private ForegroundColorSpan superLightColorSpan =
        new ForegroundColorSpan(ResourcesCompat.getColor(getContext().getResources(), R.color.modal_window_super_light_text, null));
    private DateFormat dateFormat = new SimpleDateFormat("yyyy/MM/dd hh:mm a");

    private TextView textName;
    private TextView textCustomName;
    private TextView textPk;
    private TextView textNote;
    private TextView textPersonalNote;
    private TextView textLastTimeUsed;

    private TextView textCountry;
    private TextView textCountryCode;
    private TextView textLocation;

    private LinearLayout connectivityContainer;
    private TextView textCongestion;
    private TextView textCongestionRating;
    private TextView textLatency;
    private TextView textLatencyRating;
    private TextView textHops;

    private LinearLayout specialContainer;
    private TextView textIsCurrent;
    private TextView textIsFavorite;
    private TextView textBlocked;
    private TextView textInHistory;
    private TextView textEnteredManually;
    private TextView textHasPassword;

    private ModalWindowButton buttonClose;

    private VpnServerForList server;
    private ServerLists listType;

    public ServerInfoModalWindow(Context ctx, VpnServerForList server, ServerLists listType) {
        super(ctx);

        this.server = server;
        this.listType = listType;
    }

    @Override
    protected void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        requestWindowFeature(Window.FEATURE_NO_TITLE);
        setContentView(R.layout.view_server_info_modal);

        textName = findViewById(R.id.textName);
        textCustomName = findViewById(R.id.textCustomName);
        textPk = findViewById(R.id.textPk);
        textNote = findViewById(R.id.textNote);
        textPersonalNote = findViewById(R.id.textPersonalNote);
        textLastTimeUsed = findViewById(R.id.textLastTimeUsed);

        textCountry = findViewById(R.id.textCountry);
        textCountryCode = findViewById(R.id.textCountryCode);
        textLocation = findViewById(R.id.textLocation);

        connectivityContainer = findViewById(R.id.connectivityContainer);
        textCongestion = findViewById(R.id.textCongestion);
        textCongestionRating = findViewById(R.id.textCongestionRating);
        textLatency = findViewById(R.id.textLatency);
        textLatencyRating = findViewById(R.id.textLatencyRating);
        textHops = findViewById(R.id.textHops);

        specialContainer = findViewById(R.id.specialContainer);
        textIsCurrent = findViewById(R.id.textIsCurrent);
        textIsFavorite = findViewById(R.id.textIsFavorite);
        textBlocked = findViewById(R.id.textBlocked);
        textInHistory = findViewById(R.id.textInHistory);
        textEnteredManually = findViewById(R.id.textEnteredManually);
        textHasPassword = findViewById(R.id.textHasPassword);

        buttonClose = findViewById(R.id.buttonClose);

        putValue(textName, R.string.server_info_name, server.name, null, null);
        putValue(textCustomName, R.string.server_info_custom_name, server.customName, null, null);
        putValue(textPk, R.string.server_info_pk, server.pk, null, null);
        if ((server.note != null && !server.note.trim().equals("")) && (server.personalNote != null && !server.personalNote.trim().equals(""))) {
            putValue(textNote, R.string.server_info_original_note, server.note, null, null);
            putValue(textPersonalNote, R.string.server_info_personal_note, server.personalNote, null, null);
        } else if (server.note != null && !server.note.trim().equals("")) {
            putValue(textNote, R.string.server_info_note, server.note, null, null);
            textPersonalNote.setVisibility(View.GONE);
        } else if (server.personalNote != null && !server.personalNote.trim().equals("")) {
            putValue(textPersonalNote, R.string.server_info_note, server.personalNote, null, null);
            textNote.setVisibility(View.GONE);
        } else {
            putValue(textNote, R.string.server_info_note, null, null, null);
            textPersonalNote.setVisibility(View.GONE);
        }
        if (server.inHistory) {
            putValue(textLastTimeUsed, R.string.server_info_last_time_used, dateFormat.format(server.lastUsed), null, null);
        } else {
            textLastTimeUsed.setVisibility(View.GONE);
        }

        putValue(textCountry, R.string.server_info_country, CountriesList.getCountryName(server.countryCode), null, null);
        if (!server.countryCode.toUpperCase().equals("ZZ")) {
            putValue(textCountryCode, R.string.server_info_country_code, server.countryCode.toUpperCase(), null, null);
        } else {
            textCountryCode.setVisibility(View.GONE);
        }
        putValue(textLocation, R.string.server_info_location, server.location, null, null);

        if (listType == ServerLists.Public) {
            putValue(textCongestion, R.string.server_info_congestion,
                HelperFunctions.zeroDecimalsFormatter.format(server.congestion) + "%", null, null
            );
            putValue(textCongestionRating, R.string.server_info_congestion_rating,
                getContext().getText(ServerRatings.getTextForRating(server.congestionRating)).toString(), getRatingColor(server.congestionRating), null
            );
            putValue(textLatency, R.string.server_info_latency,
                HelperFunctions.getLatencyValue(server.latency), null, null
            );
            putValue(textLatencyRating, R.string.server_info_latency_rating,
                getContext().getText(ServerRatings.getTextForRating(server.latencyRating)).toString(), getRatingColor(server.latencyRating), null
            );
            putValue(textHops, R.string.server_info_hops,
                server.hops + "", null, null
            );
        } else {
            connectivityContainer.setVisibility(View.GONE);
        }

        boolean hasSpecialCondition = false;
        boolean isTheCurrentServer = VPNServersPersistentData.getInstance().getCurrentServer() != null &&
            VPNServersPersistentData.getInstance().getCurrentServer().pk.toLowerCase().equals(server.pk.toLowerCase());

        if (isTheCurrentServer) {
            putValue(textIsCurrent, R.string.server_info_is_current, getBooleanString(true), null, "\ue876");
            hasSpecialCondition = true;
        } else {
            textIsCurrent.setVisibility(View.GONE);
        }
        if (server.flag == ServerFlags.Favorite) {
            ForegroundColorSpan iconColor = new ForegroundColorSpan(ResourcesCompat.getColor(getContext().getResources(),R.color.yellow, null));
            putValue(textIsFavorite, R.string.server_info_is_favorite, getBooleanString(true), iconColor, "\ue838");
            hasSpecialCondition = true;
        } else {
            textIsFavorite.setVisibility(View.GONE);
        }
        if (server.flag == ServerFlags.Blocked) {
            ForegroundColorSpan iconColor = new ForegroundColorSpan(ResourcesCompat.getColor(getContext().getResources(),R.color.red, null));
            putValue(textBlocked, R.string.server_info_is_blocked, getBooleanString(true), iconColor, "\ue14c");
            hasSpecialCondition = true;
        } else {
            textBlocked.setVisibility(View.GONE);
        }
        if (server.inHistory && !isTheCurrentServer) {
            putValue(textInHistory, R.string.server_info_is_in_history, getBooleanString(true), null, "\ue889");
            hasSpecialCondition = true;
        } else {
            textInHistory.setVisibility(View.GONE);
        }
        if (server.enteredManually) {
            putValue(textEnteredManually, R.string.server_info_entered_manually, getBooleanString(true), null, null);
            hasSpecialCondition = true;
        } else {
            textEnteredManually.setVisibility(View.GONE);
        }
        if (server.enteredManually && server.hasPassword) {
            putValue(textHasPassword, R.string.server_info_has_password, getBooleanString(true), null, "\ue899");
            hasSpecialCondition = true;
        } else {
            textHasPassword.setVisibility(View.GONE);
        }
        if (!hasSpecialCondition) {
            specialContainer.setVisibility(View.GONE);
        }

        buttonClose.setClickEventListener(this);

        HelperFunctions.configureModalWindow(this);
    }

    @Override
    public void onClick(View view) {
        dismiss();
    }

    private void putValue(TextView textView, int titleResurce, String value, ForegroundColorSpan valueColor, String icon) {
        SpannableStringBuilder finalText = new SpannableStringBuilder(getContext().getString(titleResurce));
        finalText.setSpan(lightColorSpan, 0, finalText.length(), Spanned.SPAN_EXCLUSIVE_EXCLUSIVE);

        finalText.append("\n");
        int initialValuePos = finalText.length();

        if (value != null && !value.trim().equals("")) {
            if (icon == null) {
                finalText.append(value);

                if (valueColor != null) {
                    finalText.setSpan(valueColor, initialValuePos, finalText.length(), Spanned.SPAN_EXCLUSIVE_EXCLUSIVE);
                }
            } else {
                finalText.append(icon + " ");
                finalText.setSpan(new MaterialFontSpan(getContext()), initialValuePos, finalText.length(), Spanned.SPAN_EXCLUSIVE_EXCLUSIVE);
                finalText.setSpan(new RelativeSizeSpan(0.75f), initialValuePos, finalText.length(), Spanned.SPAN_EXCLUSIVE_EXCLUSIVE);
                if (valueColor != null) {
                    finalText.setSpan(valueColor, initialValuePos, finalText.length(), Spanned.SPAN_EXCLUSIVE_EXCLUSIVE);
                }

                finalText.append(value);
            }
        } else {
            finalText.append(getContext().getString(R.string.server_info_without_value));
            finalText.setSpan(superLightColorSpan, initialValuePos, finalText.length(), Spanned.SPAN_EXCLUSIVE_EXCLUSIVE);
        }

        textView.setText(finalText);
    }

    private String getBooleanString(boolean value) {
        if (value) {
            return getContext().getText(R.string.general_yes).toString();
        }

        return getContext().getText(R.string.general_no).toString();
    }

    private ForegroundColorSpan getRatingColor(ServerRatings rating) {
        if (rating == ServerRatings.Gold) {
            return new ForegroundColorSpan(ResourcesCompat.getColor(getContext().getResources(), R.color.gold, null));
        } else if (rating == ServerRatings.Silver) {
            return new ForegroundColorSpan(ResourcesCompat.getColor(getContext().getResources(), R.color.silver, null));
        }

        return new ForegroundColorSpan(ResourcesCompat.getColor(getContext().getResources(), R.color.bronze, null));
    }
}
