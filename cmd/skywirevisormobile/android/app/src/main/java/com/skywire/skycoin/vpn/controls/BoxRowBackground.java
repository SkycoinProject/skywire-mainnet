package com.skywire.skycoin.vpn.controls;

import android.content.Context;
import android.graphics.Bitmap;
import android.graphics.BitmapFactory;
import android.graphics.Canvas;
import android.graphics.Rect;
import android.graphics.Shader;
import android.graphics.drawable.BitmapDrawable;
import android.util.AttributeSet;
import android.view.View;
import android.view.ViewOutlineProvider;

import com.skywire.skycoin.vpn.R;
import com.skywire.skycoin.vpn.helpers.BoxRowTypes;

public class BoxRowBackground extends View {
    public BoxRowBackground(Context context) {
        super(context);
        Initialize(context, null);
    }
    public BoxRowBackground(Context context, AttributeSet attrs) {
        super(context, attrs);
        Initialize(context, attrs);
    }
    public BoxRowBackground(Context context, AttributeSet attrs, int defStyle) {
        super(context, attrs, defStyle);
        Initialize(context, attrs);
    }

    BitmapDrawable bitmapDrawable;

    private void Initialize (Context context, AttributeSet attrs) {
        setOutlineProvider(ViewOutlineProvider.BACKGROUND);
        setClipToOutline(true);

        Bitmap bitmap = BitmapFactory.decodeResource(getResources(), R.drawable.box_pattern);

        bitmapDrawable = new BitmapDrawable(context.getResources(), bitmap);
        bitmapDrawable.setTileModeXY(Shader.TileMode.REPEAT, Shader.TileMode.REPEAT);

        setType(BoxRowTypes.TOP);
    }

    @Override
    protected void onDraw(Canvas canvas) {
        bitmapDrawable.setBounds(new Rect(0, 0, canvas.getWidth(), canvas.getHeight()));
        bitmapDrawable.draw(canvas);

        super.onDraw(canvas);
    }

    public void setType(BoxRowTypes type) {
        if (type == BoxRowTypes.TOP) {
            setBackgroundResource(R.drawable.box_row_rounded_box_1);
        } else if (type == BoxRowTypes.MIDDLE) {
            setBackgroundResource(R.drawable.box_row_rounded_box_2);
        } else if (type == BoxRowTypes.BOTTOM) {
            setBackgroundResource(R.drawable.box_row_rounded_box_3);
        } else {
            setBackgroundResource(R.drawable.box_row_rounded_box_4);
        }

        this.invalidate();
    }
}
