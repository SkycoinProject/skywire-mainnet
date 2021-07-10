package com.skywire.skycoin.vpn.helpers;

import android.text.TextPaint;
import android.text.style.TypefaceSpan;

public class AlphaSpan extends TypefaceSpan {
    private int alpha;

    public AlphaSpan(int alpha) {
        super("");

        this.alpha = alpha;
    }

    @Override
    public void updateDrawState(TextPaint paint) {
        paint.setAlpha(alpha);
    }

    @Override
    public void updateMeasureState(TextPaint paint) {
        paint.setAlpha(alpha);
    }
}
