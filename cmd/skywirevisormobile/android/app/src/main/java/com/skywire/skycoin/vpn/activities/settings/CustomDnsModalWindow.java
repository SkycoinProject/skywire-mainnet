package com.skywire.skycoin.vpn.activities.settings;

import android.app.Dialog;
import android.content.Context;
import android.os.Bundle;
import android.view.KeyEvent;
import android.view.View;
import android.view.Window;
import android.view.inputmethod.EditorInfo;
import android.widget.EditText;

import com.skywire.skycoin.vpn.R;
import com.skywire.skycoin.vpn.controls.ModalWindowButton;
import com.skywire.skycoin.vpn.extensible.ClickEvent;
import com.skywire.skycoin.vpn.helpers.HelperFunctions;
import com.skywire.skycoin.vpn.vpn.VPNGeneralPersistentData;

import java.util.regex.Matcher;

import static androidx.core.util.PatternsCompat.IP_ADDRESS;

public class CustomDnsModalWindow extends Dialog implements ClickEvent {
    public interface Confirmed {
        void confirmed(String newIp);
    }

    private EditText editValue;
    private ModalWindowButton buttonCancel;
    private ModalWindowButton buttonConfirm;

    private Confirmed event;

    public CustomDnsModalWindow(Context ctx, Confirmed event) {
        super(ctx);

        this.event = event;
    }

    @Override
    protected void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        requestWindowFeature(Window.FEATURE_NO_TITLE);
        setContentView(R.layout.view_settings_dns_modal);

        editValue = findViewById(R.id.editValue);
        buttonCancel = findViewById(R.id.buttonCancel);
        buttonConfirm = findViewById(R.id.buttonConfirm);

        String currentServer = VPNGeneralPersistentData.getCustomDns();
        if (currentServer != null) {
            editValue.setText(currentServer);
        }

        editValue.setOnEditorActionListener((v, actionId, event) -> {
            if (
                actionId == EditorInfo.IME_ACTION_DONE ||
                (event != null && event.getAction() == KeyEvent.ACTION_DOWN && event.getKeyCode() == KeyEvent.KEYCODE_ENTER)
            ) {
                makeChange();

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
        } else {
            dismiss();
        }
    }

    private void makeChange() {
        boolean valid = false;
        String ip = null;

        if (editValue.getText() == null || editValue.getText().toString().trim().length() == 0) {
            valid = true;
        } else {
            ip = editValue.getText().toString().trim();
            Matcher matcher = IP_ADDRESS.matcher(ip);
            if (matcher.matches()) {
                valid = true;
            }
        }

        if (valid) {
            if (event != null) {
                event.confirmed(ip);
            }

            dismiss();
        } else {
            HelperFunctions.showToast(getContext().getString(R.string.tmp_dns_validation_error), true);
        }
    }
}
