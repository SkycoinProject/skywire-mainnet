package com.skywire.skycoin.vpn.controls;

import android.app.Dialog;
import android.content.Context;
import android.os.Bundle;
import android.text.Editable;
import android.text.TextWatcher;
import android.view.KeyEvent;
import android.view.View;
import android.view.Window;
import android.view.inputmethod.EditorInfo;
import android.widget.EditText;

import com.skywire.skycoin.vpn.R;
import com.skywire.skycoin.vpn.activities.servers.VpnServerForList;
import com.skywire.skycoin.vpn.extensible.ClickEvent;
import com.skywire.skycoin.vpn.helpers.HelperFunctions;
import com.skywire.skycoin.vpn.objects.LocalServerData;
import com.skywire.skycoin.vpn.vpn.VPNServersPersistentData;

public class ServerPasswordModalWindow extends Dialog implements ClickEvent, TextWatcher {
    private EditText editPassword;
    private ModalWindowButton buttonCancel;
    private ModalWindowButton buttonConfirm;

    private VpnServerForList server;

    public ServerPasswordModalWindow(Context ctx, VpnServerForList server) {
        super(ctx);

        this.server = server;
    }

    @Override
    protected void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        requestWindowFeature(Window.FEATURE_NO_TITLE);
        setContentView(R.layout.view_server_password_modal);

        editPassword = findViewById(R.id.editPassword);
        buttonCancel = findViewById(R.id.buttonCancel);
        buttonConfirm = findViewById(R.id.buttonConfirm);

        editPassword.setOnEditorActionListener((v, actionId, event) -> {
            if (
                actionId == EditorInfo.IME_ACTION_DONE ||
                (event != null && event.getAction() == KeyEvent.ACTION_DOWN && event.getKeyCode() == KeyEvent.KEYCODE_ENTER)
            ) {
                if (buttonConfirm.isEnabled()) {
                    makeChange();
                    dismiss();
                }

                return true;
            }

            return false;
        });

        editPassword.addTextChangedListener(this);

        buttonCancel.setClickEventListener(this);
        buttonConfirm.setClickEventListener(this);

        buttonConfirm.setEnabled(false);

        HelperFunctions.configureModalWindow(this);
    }

    @Override
    public void beforeTextChanged(CharSequence s, int start, int count, int after) { }
    @Override
    public void afterTextChanged(Editable s) { }

    @Override
    public void onTextChanged(CharSequence s, int start, int before, int count) {
        if (editPassword.getText() == null || editPassword.getText().toString().equals("")) {
            buttonConfirm.setEnabled(false);
        } else {
            buttonConfirm.setEnabled(true);
        }
    }

    @Override
    public void onClick(View view) {
        if (view.getId() == R.id.buttonConfirm) {
            makeChange();
        }

        dismiss();
    }

    private void makeChange() {
        LocalServerData localServerData = VPNServersPersistentData.getInstance().processFromList(server);

        localServerData.password = editPassword.getText().toString();
        VPNServersPersistentData.getInstance().updateServer(localServerData);

        HelperFunctions.showToast(getContext().getString(R.string.server_password_changes_made_confirmation), true);
    }
}
