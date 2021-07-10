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
import com.skywire.skycoin.vpn.extensible.ClickEvent;
import com.skywire.skycoin.vpn.helpers.HelperFunctions;
import com.skywire.skycoin.vpn.objects.LocalServerData;
import com.skywire.skycoin.vpn.objects.ManualVpnServerData;
import com.skywire.skycoin.vpn.vpn.VPNServersPersistentData;

import skywiremob.Skywiremob;

public class ManualServerModalWindow extends Dialog implements ClickEvent, TextWatcher {
    public interface Confirmed {
        void confirmed(LocalServerData server);
    }

    private EditText editPk;
    private EditText editPassword;
    private EditText editName;
    private EditText editNote;
    private ModalWindowButton buttonCancel;
    private ModalWindowButton buttonConfirm;

    private Confirmed event;
    private boolean hasError;

    public ManualServerModalWindow(Context ctx, Confirmed event) {
        super(ctx);

        this.event = event;
    }

    @Override
    protected void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        requestWindowFeature(Window.FEATURE_NO_TITLE);
        setContentView(R.layout.view_manual_server_modal);

        editPk = findViewById(R.id.editPk);
        editPassword = findViewById(R.id.editPassword);
        editName = findViewById(R.id.editName);
        editNote = findViewById(R.id.editNote);
        buttonCancel = findViewById(R.id.buttonCancel);
        buttonConfirm = findViewById(R.id.buttonConfirm);

        editPk.addTextChangedListener(this);

        editPk.setImeOptions(EditorInfo.IME_ACTION_NEXT);
        editName.setImeOptions(EditorInfo.IME_ACTION_NEXT);
        editNote.setImeOptions(EditorInfo.IME_ACTION_DONE);

        editPk.setSelection(editName.getText().length());

        editNote.setOnEditorActionListener((v, actionId, event) -> {
            if (
                actionId == EditorInfo.IME_ACTION_DONE ||
                (event != null && event.getAction() == KeyEvent.ACTION_DOWN && event.getKeyCode() == KeyEvent.KEYCODE_ENTER)
            ) {
                if (!hasError) {
                    process();
                    dismiss();
                }

                return true;
            }

            return false;
        });

        buttonCancel.setClickEventListener(this);
        buttonConfirm.setClickEventListener(this);

        buttonConfirm.setEnabled(false);
        hasError = true;

        HelperFunctions.configureModalWindow(this);
    }

    @Override
    public void beforeTextChanged(CharSequence s, int start, int count, int after) { }
    @Override
    public void afterTextChanged(Editable s) { }

    @Override
    public void onTextChanged(CharSequence s, int start, int before, int count) {
        hasError = false;
        if (editPk.getText().length() < 66) {
            editPk.setError(getContext().getText(R.string.add_server_pk_length_error));
            hasError = true;
        } else if (Skywiremob.isPKValid(editPk.getText().toString()).getCode() != Skywiremob.ErrCodeNoError) {
            editPk.setError(getContext().getText(R.string.add_server_pk_invalid_error));
            hasError = true;
        }

        if (hasError) {
            buttonConfirm.setEnabled(false);
        } else {
            buttonConfirm.setEnabled(true);
        }
    }

    @Override
    public void onClick(View view) {
        if (view.getId() == R.id.buttonConfirm) {
            process();
        }

        dismiss();
    }

    private void process() {
        if (hasError) {
            return;
        }

        LocalServerData savedVersion = VPNServersPersistentData.getInstance().getSavedVersion(editPk.getText().toString().trim());

        ManualVpnServerData serverData = new ManualVpnServerData();
        serverData.pk = editPk.getText().toString().trim();

        String password = editPassword.getText().toString();
        if (password != null && !password.equals("")) {
            serverData.password = password;
        }

        if (editName.getText() != null && !editName.getText().toString().trim().equals("")) {
            serverData.name = editName.getText().toString().trim();
        } else if (savedVersion != null && savedVersion.customName != null && !savedVersion.customName.equals("")) {
            serverData.name = savedVersion.customName;
        }

        if (editNote.getText() != null && !editNote.getText().toString().trim().equals("")) {
            serverData.note = editNote.getText().toString().trim();
        } else if (savedVersion != null && savedVersion.personalNote != null && !savedVersion.personalNote.equals("")) {
            serverData.note = savedVersion.personalNote;
        }

        LocalServerData localServerData = VPNServersPersistentData.getInstance().processFromManual(serverData);
        if (event != null) {
            event.confirmed(localServerData);
        }
    }
}
