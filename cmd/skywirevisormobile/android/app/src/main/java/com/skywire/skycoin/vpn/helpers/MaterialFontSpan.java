package com.skywire.skycoin.vpn.helpers;

import android.content.Context;
import android.graphics.Typeface;
import android.text.TextPaint;
import android.text.style.TypefaceSpan;

import androidx.core.content.res.ResourcesCompat;

import com.skywire.skycoin.vpn.R;

public class MaterialFontSpan extends TypefaceSpan {
    private static Typeface materialFont;

    public MaterialFontSpan(Context context) {
        super("");

        if (materialFont == null) {
            materialFont = ResourcesCompat.getFont(context, R.font.material_font);
        }
    }

    @Override
    public void updateDrawState(TextPaint paint) {
        paint.setTypeface(materialFont);
    }

    @Override
    public void updateMeasureState(TextPaint paint) {
        paint.setTypeface(materialFont);
    }
}
