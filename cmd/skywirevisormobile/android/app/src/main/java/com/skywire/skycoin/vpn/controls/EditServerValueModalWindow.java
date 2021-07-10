package com.skywire.skycoin.vpn.controls;

import android.app.Dialog;
import android.content.Context;
import android.os.Bundle;
import android.view.KeyEvent;
import android.view.View;
import android.view.Window;
import android.view.inputmethod.EditorInfo;
import android.widget.EditText;

import com.google.android.material.textfield.TextInputLayout;
import com.skywire.skycoin.vpn.R;
import com.skywire.skycoin.vpn.activities.servers.VpnServerForList;
import com.skywire.skycoin.vpn.extensible.ClickEvent;
import com.skywire.skycoin.vpn.helpers.HelperFunctions;
import com.skywire.skycoin.vpn.objects.LocalServerData;
import com.skywire.skycoin.vpn.vpn.VPNServersPersistentData;

public class EditServerValueModalWindow extends Dialog implements ClickEvent {
    private ModalBase modalBase;
    private TextInputLayout editContainer;
    private EditText editValue;
    private ModalWindowButton buttonCancel;
    private ModalWindowButton buttonConfirm;

    private boolean editingName;
    private VpnServerForList server;

    public EditServerValueModalWindow(Context ctx, boolean editingName, VpnServerForList server) {
        super(ctx);

        this.editingName = editingName;
        this.server = server;
    }

    @Override
    protected void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        requestWindowFeature(Window.FEATURE_NO_TITLE);
        setContentView(R.layout.view_edit_server_value_modal);

        modalBase = findViewById(R.id.modalBase);
        editContainer = findViewById(R.id.editContainer);
        editValue = findViewById(R.id.editValue);
        buttonCancel = findViewById(R.id.buttonCancel);
        buttonConfirm = findViewById(R.id.buttonConfirm);

        LocalServerData localServerData = VPNServersPersistentData.getInstance().processFromList(server);
        if (editingName) {
            modalBase.setTitle(R.string.tmp_edit_value_name_title);
            editContainer.setHint(getContext().getText(R.string.tmp_edit_value_name_label));

            if (localServerData.customName != null) {
                editValue.setText(localServerData.customName);
            } else {
                editValue.setText("");
            }
        } else {
            modalBase.setTitle(R.string.tmp_edit_value_note_title);
            editContainer.setHint(getContext().getText(R.string.tmp_edit_value_note_label));

            if (localServerData.personalNote != null) {
                editValue.setText(localServerData.personalNote);
            } else {
                editValue.setText("");
            }
        }

        editValue.setOnEditorActionListener((v, actionId, event) -> {
            if (
                actionId == EditorInfo.IME_ACTION_DONE ||
                (event != null && event.getAction() == KeyEvent.ACTION_DOWN && event.getKeyCode() == KeyEvent.KEYCODE_ENTER)
            ) {
                makeChange();
                dismiss();

                return true;
            }

            return false;
        });

        editValue.setSelection(editValue.getText().length());

        buttonCancel.setClickEventListener(this);
        buttonConfirm.setClickEventListener(this);

        HelperFunctions.configureModalWindow(this);
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

        String newValue = editValue.getText().toString().trim();
        String currentValue = editingName ? localServerData.customName : localServerData.personalNote;
        if (currentValue == null) {
            currentValue = "";
        }
        if (newValue.equals(currentValue)) {
            return;
        }

        if (editingName) {
            localServerData.customName = newValue;
        } else {
            localServerData.personalNote = newValue;
        }
        VPNServersPersistentData.getInstance().updateServer(localServerData);

        HelperFunctions.showToast(getContext().getString(R.string.tmp_edit_value_changes_made_confirmation), true);
    }
}
