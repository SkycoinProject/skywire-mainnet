package com.skywire.skycoin.vpn.activities.apps;

import android.content.Context;
import android.util.AttributeSet;
import android.view.LayoutInflater;
import android.widget.RadioButton;
import android.widget.TextView;

import com.skywire.skycoin.vpn.R;
import com.skywire.skycoin.vpn.controls.BoxRowLayout;
import com.skywire.skycoin.vpn.extensible.ListButtonBase;
import com.skywire.skycoin.vpn.helpers.BoxRowTypes;

public class AppListOptionButton extends ListButtonBase<Void> {
    private BoxRowLayout mainLayout;
    private TextView textOption;
    private TextView textDescription;
    private RadioButton radioSelected;

    public AppListOptionButton(Context context) {
        super(context);
    }

    @Override
    protected void Initialize (Context context, AttributeSet attrs) {
        LayoutInflater inflater = (LayoutInflater)context.getSystemService (Context.LAYOUT_INFLATER_SERVICE);
        inflater.inflate(R.layout.view_app_list_selection_option, this, true);

        mainLayout = this.findViewById (R.id.mainLayout);
        textOption = this.findViewById (R.id.textOption);
        textDescription = this.findViewById (R.id.textDescription);
        radioSelected = this.findViewById (R.id.radioSelected);

        radioSelected.setChecked(false);

        setClickableBoxView(mainLayout);
    }

    public void setBoxRowType(BoxRowTypes type) {
        mainLayout.setType(type);
    }

    public void changeData(int textResource, int descriptionResource) {
        textOption.setText(textResource);
        textDescription.setText(descriptionResource);
    }

    public void setChecked(boolean checked) {
        radioSelected.setChecked(checked);
    }

    @Override
    public void setEnabled(boolean enabled) {
        super.setEnabled(enabled);

        if (enabled) {
            setAlpha(1f);
        } else {
            setAlpha(0.5f);
        }
    }
}
