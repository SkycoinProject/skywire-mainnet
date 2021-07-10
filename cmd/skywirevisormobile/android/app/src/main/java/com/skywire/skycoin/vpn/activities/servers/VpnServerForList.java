package com.skywire.skycoin.vpn.activities.servers;

import com.skywire.skycoin.vpn.objects.ServerFlags;
import com.skywire.skycoin.vpn.objects.ServerRatings;

import java.util.Date;

public class VpnServerForList {
    public String countryCode;
    public String name;
    public String customName;
    public String location;
    public String pk;
    public double congestion;
    public ServerRatings congestionRating;
    public double latency;
    public ServerRatings latencyRating;
    public int hops;
    public String note;
    public String personalNote;
    public Date lastUsed;
    public boolean inHistory;
    public ServerFlags flag;
    public boolean hasPassword;
    public boolean enteredManually;
}
