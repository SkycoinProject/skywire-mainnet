package com.skywire.skycoin.vpn.activities.settings;

import android.content.Context;
import android.content.res.TypedArray;
import android.util.AttributeSet;
import android.view.LayoutInflater;
import android.widget.CheckBox;
import android.widget.TextView;

import com.skywire.skycoin.vpn.R;
import com.skywire.skycoin.vpn.controls.BoxRowLayout;
import com.skywire.skycoin.vpn.extensible.ButtonBase;
import com.skywire.skycoin.vpn.helpers.BoxRowTypes;

public class SettingsOption extends ButtonBase {
    private BoxRowLayout mainLayout;
    private TextView textAlertIcon;
    private TextView textName;
    private TextView textDescription;
    private CheckBox checkSelected;

    public SettingsOption(Context context) {
        super(context);
    }
    public SettingsOption(Context context, AttributeSet attrs) {
        super(context, attrs);
    }
    public SettingsOption(Context context, AttributeSet attrs, int defStyle) {
        super(context, attrs, defStyle);
    }

    @Override
    protected void Initialize(Context context, AttributeSet attrs) {
        LayoutInflater inflater = (LayoutInflater)context.getSystemService (Context.LAYOUT_INFLATER_SERVICE);
        inflater.inflate(R.layout.view_settings_list_item, this, true);

        mainLayout = this.findViewById (R.id.mainLayout);
        textAlertIcon = this.findViewById (R.id.textAlertIcon);
        textName = this.findViewById (R.id.textName);
        textDescription = this.findViewById (R.id.textDescription);
        checkSelected = this.findViewById (R.id.checkSelected);

        int type = 1;
        String name = "";
        String description = "";

        if (attrs != null) {
            TypedArray attributes = getContext().getTheme().obtainStyledAttributes(
                attrs,
                R.styleable.SettingsOption,
                0, 0
            );

            type = attributes.getInteger(R.styleable.SettingsOption_box_row_type, 1);
            name = attributes.getString(R.styleable.SettingsOption_title);
            description = attributes.getString(R.styleable.SettingsOption_description);

            boolean hideCheckbox = attributes.getBoolean(R.styleable.SettingsOption_hide_checkbox, false);
            if (hideCheckbox) {
                checkSelected.setVisibility(GONE);
            }

            attributes.recycle();
        }

        textName.setText(name);
        textDescription.setText(description);

        if (type == 0) {
            mainLayout.setType(BoxRowTypes.TOP);
        } else if (type == 1) {
            mainLayout.setType(BoxRowTypes.MIDDLE);
        } else if (type == 2) {
            mainLayout.setType(BoxRowTypes.BOTTOM);
        } else if (type == 3) {
            mainLayout.setType(BoxRowTypes.SINGLE);
        }

        textAlertIcon.setVisibility(GONE);

        setClickableBoxView(mainLayout);
    }

    public void setChecked(boolean checked) {
        checkSelected.setChecked(checked);
    }
    public boolean isChecked() {
        return checkSelected.isChecked();
    }

    public void setDescription(int resource, String param) {
        if (param == null) {
            textDescription.setText(resource);
        } else {
            textDescription.setText(String.format(getResources().getString(resource), param));
        }
    }

    public void changeAlertIconVisibility(boolean visible) {
        if (visible) {
            textAlertIcon.setVisibility(VISIBLE);
        } else {
            textAlertIcon.setVisibility(GONE);
        }
    }
}
