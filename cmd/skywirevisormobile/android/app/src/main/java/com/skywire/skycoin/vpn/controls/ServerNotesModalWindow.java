package com.skywire.skycoin.vpn.controls;

import android.app.Dialog;
import android.content.Context;
import android.os.Bundle;
import android.view.View;
import android.view.Window;
import android.widget.TextView;

import com.skywire.skycoin.vpn.R;
import com.skywire.skycoin.vpn.activities.servers.VpnServerForList;
import com.skywire.skycoin.vpn.extensible.ClickEvent;
import com.skywire.skycoin.vpn.helpers.HelperFunctions;

public class ServerNotesModalWindow extends Dialog implements ClickEvent {
    private TextView textNoteTitle;
    private TextView textNote;
    private TextView textPersonalNoteTitle;
    private TextView textPersonalNote;

    private ModalWindowButton buttonClose;

    private VpnServerForList server;

    public ServerNotesModalWindow(Context ctx, VpnServerForList server) {
        super(ctx);

        this.server = server;
    }

    @Override
    protected void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        requestWindowFeature(Window.FEATURE_NO_TITLE);
        setContentView(R.layout.view_server_notes_modal);

        textNoteTitle = findViewById(R.id.textNoteTitle);
        textNote = findViewById(R.id.textNote);
        textPersonalNoteTitle = findViewById(R.id.textPersonalNoteTitle);
        textPersonalNote = findViewById(R.id.textPersonalNote);
        buttonClose = findViewById(R.id.buttonClose);

        if ((server.note != null && !server.note.trim().equals("")) && (server.personalNote != null && !server.personalNote.trim().equals(""))) {
            textNote.setText(server.note);
            textPersonalNote.setText(server.personalNote);
        } else {
            textNoteTitle.setVisibility(View.GONE);
            textPersonalNoteTitle.setVisibility(View.GONE);
            textPersonalNote.setVisibility(View.GONE);

            if (server.note != null && !server.note.trim().equals("")) {
                textNote.setText(server.note);
            } else if (server.personalNote != null && !server.personalNote.trim().equals("")) {
                textNote.setText(server.personalNote);
            } else {
                textNote.setVisibility(View.GONE);
            }
        }

        buttonClose.setClickEventListener(this);

        HelperFunctions.configureModalWindow(this);
    }

    @Override
    public void onClick(View view) {
        dismiss();
    }
}
