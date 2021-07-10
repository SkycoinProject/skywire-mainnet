package com.skywire.skycoin.vpn.extensible;

public interface ClickWithIndexEvent<T> {
    void onClickWithIndex(int index, T data);
}
