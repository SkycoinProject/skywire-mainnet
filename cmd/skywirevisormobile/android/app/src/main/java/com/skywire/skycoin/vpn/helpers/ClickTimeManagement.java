package com.skywire.skycoin.vpn.helpers;

import java.util.concurrent.TimeUnit;

import io.reactivex.rxjava3.android.schedulers.AndroidSchedulers;
import io.reactivex.rxjava3.core.Observable;
import io.reactivex.rxjava3.disposables.Disposable;
import io.reactivex.rxjava3.schedulers.Schedulers;

public class ClickTimeManagement {
    public static final int normalFastClickPreventionDelay = 700;

    private Disposable timeSubscription;
    private int delay = normalFastClickPreventionDelay;

    public void setDelay(int delay) {
        this.delay = delay;
    }

    public void informClickMade() {
        removeDelay();

        timeSubscription = Observable.just(1).delay(delay, TimeUnit.MILLISECONDS)
            .subscribeOn(Schedulers.io())
            .observeOn(AndroidSchedulers.mainThread())
            .subscribe(v -> timeSubscription = null);
    }

    public boolean canClick() {
        return timeSubscription == null;
    }

    public void removeDelay() {
        if (timeSubscription != null) {
            timeSubscription.dispose();
        }

        timeSubscription = null;
    }
}
