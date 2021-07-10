package com.skywire.skycoin.vpn.activities.servers;

import android.content.Context;
import android.content.res.TypedArray;
import android.util.AttributeSet;
import android.view.LayoutInflater;
import android.widget.TextView;

import com.skywire.skycoin.vpn.R;
import com.skywire.skycoin.vpn.controls.BoxRowLayout;
import com.skywire.skycoin.vpn.extensible.ButtonBase;

public class ServerListOptionButton extends ButtonBase {

    private BoxRowLayout mainLayout;
    private TextView textIcon;

    public ServerListOptionButton(Context context) {
        super(context);
    }
    public ServerListOptionButton(Context context, AttributeSet attrs) {
        super(context, attrs);
    }
    public ServerListOptionButton(Context context, AttributeSet attrs, int defStyle) {
        super(context, attrs, defStyle);
    }

    @Override
    protected void Initialize (Context context, AttributeSet attrs) {
        LayoutInflater inflater = (LayoutInflater)context.getSystemService (Context.LAYOUT_INFLATER_SERVICE);
        inflater.inflate(R.layout.view_server_list_option_button, this, true);

        mainLayout = this.findViewById (R.id.mainLayout);
        textIcon = this.findViewById (R.id.textIcon);

        if (attrs != null) {
            TypedArray attributes = context.getTheme().obtainStyledAttributes(
                    attrs,
                    R.styleable.ServerListOptionButton,
                    0, 0
            );

            String content = attributes.getString(R.styleable.ServerListOptionButton_content);
            if (content != null && content.trim() != "") {
                textIcon.setText(content);
            }

            attributes.recycle();
        }

        setClickableBoxView(mainLayout);
    }
}
