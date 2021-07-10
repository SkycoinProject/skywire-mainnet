package com.skywire.skycoin.vpn.controls.options;

import android.app.Dialog;
import android.content.Context;
import android.os.Bundle;
import android.view.Window;
import android.widget.LinearLayout;

import com.skywire.skycoin.vpn.R;
import com.skywire.skycoin.vpn.controls.ModalBase;
import com.skywire.skycoin.vpn.extensible.ClickWithIndexEvent;
import com.skywire.skycoin.vpn.helpers.HelperFunctions;

import java.util.ArrayList;

public class OptionsModalWindow extends Dialog implements ClickWithIndexEvent<Void> {
    public interface OptionSelected {
        void optionSelected(int selectedIndex);
    }

    private String title;
    private ModalBase modalBase;
    private LinearLayout container;

    private ArrayList<OptionsItem.SelectableOption> options;
    private OptionSelected event;

    public OptionsModalWindow(Context ctx, String title, ArrayList<OptionsItem.SelectableOption> options, OptionSelected event) {
        super(ctx);

        this.title = title;
        this.options = options;
        this.event = event;
    }

    @Override
    protected void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        requestWindowFeature(Window.FEATURE_NO_TITLE);
        setContentView(R.layout.view_options);

        modalBase = findViewById(R.id.modalBase);
        container = findViewById(R.id.container);

        if (title != null) {
            modalBase.setTitleString(title);
        }

        int i = 0;
        for (OptionsItem.SelectableOption option : options) {
            OptionsItem view = new OptionsItem(getContext());
            view.setParams(option);
            view.setIndex(i++);
            view.setClickWithIndexEventListener(this);
            container.addView(view);
        }

        HelperFunctions.configureModalWindow(this);
    }

    @Override
    public void onClickWithIndex(int index, Void data) {
        if (event != null) {
            event.optionSelected(index);
        }

        dismiss();
    }
}
