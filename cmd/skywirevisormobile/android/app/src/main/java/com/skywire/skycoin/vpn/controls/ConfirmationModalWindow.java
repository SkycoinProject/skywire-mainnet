package com.skywire.skycoin.vpn.controls;

import android.app.Dialog;
import android.content.Context;
import android.os.Bundle;
import android.view.View;
import android.view.Window;
import android.widget.TextView;

import com.skywire.skycoin.vpn.R;
import com.skywire.skycoin.vpn.extensible.ClickEvent;
import com.skywire.skycoin.vpn.helpers.HelperFunctions;

public class ConfirmationModalWindow extends Dialog implements ClickEvent {
    public interface Confirmed {
        void confirmed();
    }

    private TextView text;
    private ModalWindowButton buttonCancel;
    private ModalWindowButton buttonConfirm;

    private int textResource;
    private int confirmBtnResource;
    private int cancelBtnResource;
    private Confirmed event;

    public ConfirmationModalWindow(Context ctx, int textResource, int confirmBtnResource, int cancelBtnResource, Confirmed event) {
        super(ctx);

        this.textResource = textResource;
        this.confirmBtnResource = confirmBtnResource;
        this.cancelBtnResource = cancelBtnResource;
        this.event = event;
    }

    @Override
    protected void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        requestWindowFeature(Window.FEATURE_NO_TITLE);
        setContentView(R.layout.view_confirmation_dialog);

        text = findViewById(R.id.text);
        buttonCancel = findViewById(R.id.buttonCancel);
        buttonConfirm = findViewById(R.id.buttonConfirm);

        text.setText(textResource);
        buttonCancel.setText(cancelBtnResource);
        buttonConfirm.setText(confirmBtnResource);

        buttonCancel.setClickEventListener(this);
        buttonConfirm.setClickEventListener(this);

        HelperFunctions.configureModalWindow(this);
    }

    @Override
    public void onClick(View view) {
        if (view.getId() == R.id.buttonConfirm && event != null) {
            event.confirmed();
        }

        dismiss();
    }
}
