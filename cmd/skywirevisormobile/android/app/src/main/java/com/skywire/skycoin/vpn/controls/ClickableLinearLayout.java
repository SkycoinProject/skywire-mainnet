package com.skywire.skycoin.vpn.controls;

import android.content.Context;
import android.util.AttributeSet;
import android.view.MotionEvent;
import android.view.View;
import android.widget.LinearLayout;

import com.skywire.skycoin.vpn.extensible.ClickEvent;
import com.skywire.skycoin.vpn.helpers.ClickTimeManagement;
import com.skywire.skycoin.vpn.helpers.Globals;

import java.util.concurrent.TimeUnit;

import io.reactivex.rxjava3.android.schedulers.AndroidSchedulers;
import io.reactivex.rxjava3.core.Observable;
import io.reactivex.rxjava3.schedulers.Schedulers;

public class ClickableLinearLayout extends LinearLayout implements View.OnTouchListener, View.OnClickListener {
    private ClickEvent clickListener;
    private ClickTimeManagement buttonTimeManager = new ClickTimeManagement();

    public ClickableLinearLayout(Context context) {
        super(context);
        Initialize(context, null);
    }
    public ClickableLinearLayout(Context context, AttributeSet attrs) {
        super(context, attrs);
        Initialize(context, attrs);
    }
    public ClickableLinearLayout(Context context, AttributeSet attrs, int defStyle) {
        super(context, attrs, defStyle);
        Initialize(context, attrs);
    }

    protected void Initialize (Context context, AttributeSet attrs) {
        setOnTouchListener(this);
        setOnClickListener(this);
    }

    public void setClickEventListener(ClickEvent listener) {
        clickListener = listener;
    }

    @Override
    public boolean onTouch(View v, MotionEvent event) {
        if (event.getAction() == MotionEvent.ACTION_DOWN) {
            setAlpha(0.5f);
        } else if (event.getAction() == MotionEvent.ACTION_CANCEL || event.getAction() == MotionEvent.ACTION_POINTER_UP || event.getAction() == MotionEvent.ACTION_UP) {
            setAlpha(1f);
        }

        return false;
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
