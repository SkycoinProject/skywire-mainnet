package com.skywire.skycoin.vpn.extensible;

import android.content.Context;
import android.util.AttributeSet;
import android.view.View;
import android.widget.RelativeLayout;

import com.skywire.skycoin.vpn.controls.BoxRowLayout;
import com.skywire.skycoin.vpn.helpers.ClickTimeManagement;
import com.skywire.skycoin.vpn.helpers.Globals;

import java.util.concurrent.TimeUnit;

import io.reactivex.rxjava3.android.schedulers.AndroidSchedulers;
import io.reactivex.rxjava3.core.Observable;
import io.reactivex.rxjava3.schedulers.Schedulers;

public abstract class ButtonBase extends RelativeLayout implements View.OnClickListener {
    public ButtonBase(Context context) {
        super(context);
        Initialize(context, null);
    }
    public ButtonBase(Context context, AttributeSet attrs) {
        super(context, attrs);
        Initialize(context, attrs);
    }
    public ButtonBase(Context context, AttributeSet attrs, int defStyle) {
        super(context, attrs, defStyle);
        Initialize(context, attrs);
    }

    private ClickEvent clickListener;
    private ClickTimeManagement buttonTimeManager = new ClickTimeManagement();

    abstract protected void Initialize (Context context, AttributeSet attrs);

    protected void setViewForCheckingClicks(View v) {
        v.setOnClickListener(this);
    }

    protected void setClickableBoxView(BoxRowLayout v) {
        v.setClickEventListener(view -> {
            if (clickListener != null) {
                clickListener.onClick(this);
            }
        });
    }

    public void setUseBigFastClickPrevention(boolean useBigFastClickPrevention) {
        if (useBigFastClickPrevention) {
            buttonTimeManager.setDelay(ClickTimeManagement.normalFastClickPreventionDelay);
        } else {
            buttonTimeManager.setDelay(Globals.CLICK_DELAY_MS);
        }
    }

    public void setClickEventListener(ClickEvent listener) {
        clickListener = listener;
    }

    @Override
    public void onClick(View view) {
        if (clickListener != null && buttonTimeManager.canClick()) {
            buttonTimeManager.informClickMade();
            Observable.just(1).delay(Globals.CLICK_DELAY_MS, TimeUnit.MILLISECONDS)
                .subscribeOn(Schedulers.io())
                .observeOn(AndroidSchedulers.mainThread())
                .subscribe(v -> clickListener.onClick(this));
        }
    }
}
