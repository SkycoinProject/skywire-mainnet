package com.skywire.skycoin.vpn.vpn;

import android.content.SharedPreferences;

import androidx.preference.PreferenceManager;

import com.google.gson.Gson;
import com.google.gson.reflect.TypeToken;
import com.skywire.skycoin.vpn.App;
import com.skywire.skycoin.vpn.activities.servers.VpnServerForList;
import com.skywire.skycoin.vpn.objects.LocalServerData;
import com.skywire.skycoin.vpn.objects.ManualVpnServerData;
import com.skywire.skycoin.vpn.objects.ServerFlags;

import java.lang.reflect.Type;
import java.util.ArrayList;
import java.util.Collections;
import java.util.Comparator;
import java.util.Date;
import java.util.HashMap;

import io.reactivex.rxjava3.core.Observable;
import io.reactivex.rxjava3.subjects.ReplaySubject;

/**
 * Helper class for saving and getting data related to the VPN servers to and from the
 * persistent storage.
 */
public class VPNServersPersistentData {
    /**
     * Singleton instance.
     */
    private static final VPNServersPersistentData instance = new VPNServersPersistentData();
    /**
     * Gets the singleton for using the class.
     */
    public static VPNServersPersistentData getInstance() { return instance; }

    private final int maxHistoryElements = 30;

    // Keys for persistent storage.
    private final String CURRENT_SERVER_PK = "serverPK";
    private final String SERVER_LIST = "serverList";

    private SharedPreferences settings = PreferenceManager.getDefaultSharedPreferences(App.getContext());

    private String currentServerPk;
    private HashMap<String, LocalServerData> serversMap;

    private ReplaySubject<LocalServerData> currentServerSubject = ReplaySubject.createWithSize(1);
    private ReplaySubject<ArrayList<LocalServerData>> historySubject = ReplaySubject.createWithSize(1);
    private ReplaySubject<ArrayList<LocalServerData>> favoritesSubject = ReplaySubject.createWithSize(1);
    private ReplaySubject<ArrayList<LocalServerData>> blockedSubject = ReplaySubject.createWithSize(1);

    private VPNServersPersistentData() {
        currentServerPk = settings.getString(CURRENT_SERVER_PK, "");

        String serversList = settings.getString(SERVER_LIST, null);
        if (serversList != null) {
            Gson gson = new Gson();
            Type mapType = new TypeToken<HashMap<String, LocalServerData>>() {}.getType();
            serversMap = gson.fromJson(serversList, mapType);

            LocalServerData currentServer = this.serversMap.get(currentServerPk);
            this.currentServerSubject.onNext(currentServer != null ? currentServer : new LocalServerData());
        } else {
            serversMap = new HashMap<>();
            this.currentServerSubject.onNext(new LocalServerData());
        }

        this.launchListEvents();
    }

    public LocalServerData getCurrentServer() {
        return serversMap.get(this.currentServerPk);
    }

    public Observable<LocalServerData> getCurrentServerObservable() {
        return currentServerSubject.hide();
    }

    public Observable<ArrayList<LocalServerData>> history() {
        return this.historySubject.hide();
    }

    public Observable<ArrayList<LocalServerData>> favorites() {
        return this.favoritesSubject.hide();
    }

    public Observable<ArrayList<LocalServerData>> blocked() {
        return this.blockedSubject.hide();
    }

    public LocalServerData getSavedVersion(String pk) {
        return this.serversMap.get(pk);
    }

    public void updateFromDiscovery(ArrayList<VpnServerForList> serverList) {
        for (VpnServerForList server : serverList) {
            if (this.serversMap.containsKey(server.pk)) {
                LocalServerData savedServer = this.serversMap.get(server.pk);

                savedServer.countryCode = server.countryCode;
                savedServer.name = server.name;
                savedServer.location = server.location;
                savedServer.note = server.note;
            }
        }

        this.saveData();
    }

    public void updateServer(LocalServerData server) {
        this.serversMap.put(server.pk, server);
        this.cleanServers();
        this.saveData();
    }

    public LocalServerData processFromList(VpnServerForList newServer) {
        LocalServerData retrievedServer = this.serversMap.get(newServer.pk);
        if (retrievedServer != null) {
            retrievedServer.countryCode = newServer.countryCode;
            retrievedServer.name = newServer.name;
            retrievedServer.location = newServer.location;
            retrievedServer.note = newServer.note;

            this.saveData();

            return retrievedServer;
        }

        LocalServerData response = new LocalServerData();
        response.countryCode = newServer.countryCode;
        response.name = newServer.name;
        response.customName = null;
        response.pk = newServer.pk;
        response.lastUsed = new Date(0);
        response.inHistory = false;
        response.flag = ServerFlags.None;
        response.location = newServer.location;
        response.personalNote = null;
        response.note = newServer.note;
        response.enteredManually = false;
        response.password = null;

        return response;
    }

    public LocalServerData processFromManual(ManualVpnServerData newServer) {
        LocalServerData retrievedServer = this.serversMap.get(newServer.pk);
        if (retrievedServer != null) {
            retrievedServer.password = newServer.password;
            retrievedServer.customName = newServer.name;
            retrievedServer.personalNote = newServer.note;
            retrievedServer.enteredManually = true;

            this.saveData();

            return retrievedServer;
        }

        LocalServerData response = new LocalServerData();
        response.countryCode = "zz";
        response.name = null;
        response.customName = newServer.name;
        response.pk = newServer.pk;
        response.lastUsed = new Date(0);
        response.inHistory = false;
        response.flag = ServerFlags.None;
        response.location = null;
        response.personalNote = newServer.note;
        response.note = null;
        response.enteredManually = true;
        response.password = newServer.password;

        return response;
    }

    public void changeFlag(LocalServerData server, ServerFlags flag) {
        LocalServerData retrievedServer = this.serversMap.get(server.pk);
        if (retrievedServer != null) {
            server = retrievedServer;
        }

        if (server.flag == flag) {
            return;
        }
        server.flag = flag;

        if (!this.serversMap.containsKey(server.pk)) {
            this.serversMap.put(server.pk, server);
        }

        this.cleanServers();
        this.saveData();
    }

    public void removePassword(String pk) {
        LocalServerData retrievedServer = this.serversMap.get(pk);
        if (retrievedServer == null || retrievedServer.password == null || retrievedServer.password.equals("")) {
            return;
        }

        retrievedServer.password = null;
        this.cleanServers();
        this.saveData();
    }

    public void removeFromHistory(String pk) {
        LocalServerData retrievedServer = this.serversMap.get(pk);
        if (retrievedServer == null || !retrievedServer.inHistory) {
            return;
        }

        retrievedServer.inHistory = false;
        this.cleanServers();
        this.saveData();
    }

    public void modifyCurrentServer(LocalServerData newServer) {
        if (!this.serversMap.containsKey(newServer.pk)) {
            this.serversMap.put(newServer.pk, newServer);
        }

        this.currentServerPk = newServer.pk;

        LocalServerData currentServer = this.serversMap.get(currentServerPk);
        this.currentServerSubject.onNext(currentServer);

        this.cleanServers();
        this.saveData();
    }

    public void updateHistory() {
        LocalServerData currentServer = this.serversMap.get(currentServerPk);
        // This should not happen.
        if (currentServer == null) {
            return;
        }

        currentServer.lastUsed = new Date();
        currentServer.inHistory = true;

        // Make a list with the servers in the history and sort it by usage date.
        ArrayList<LocalServerData> historyList = new ArrayList();
        for (LocalServerData server : serversMap.values()) {
            if (server.inHistory) {
                historyList.add(server);
            }
        }
        Comparator<LocalServerData> comparator = (a, b) -> (int)((b.lastUsed.getTime() - a.lastUsed.getTime()) / 1000);
        Collections.sort(historyList, comparator);

        // Remove from the history the old servers.
        int historyElementsFound = 0;
        for (LocalServerData server : historyList) {
            if (historyElementsFound < this.maxHistoryElements) {
                historyElementsFound += 1;
            } else {
                server.inHistory = false;
            }
        }

        this.cleanServers();
        this.saveData();
    }

    private void cleanServers() {
        ArrayList<String> unneeded = new ArrayList();
        for (LocalServerData server : serversMap.values()) {
            if (
                !server.inHistory &&
                server.flag == ServerFlags.None &&
                !server.pk.equals(this.currentServerPk) &&
                (server.customName == null || server.customName.equals("")) &&
                (server.personalNote == null || server.personalNote.equals(""))
            ) {
                unneeded.add(server.pk);
            }
        }

        for (String pk : unneeded) {
            this.serversMap.remove(pk);
        }
    }

    private void saveData() {
        Gson gson = new Gson();
        String servers = gson.toJson(serversMap);

        settings
            .edit()
            .putString(SERVER_LIST, servers)
            .putString(CURRENT_SERVER_PK, currentServerPk)
            .apply();

        this.launchListEvents();
    }

    private void launchListEvents() {
        ArrayList<LocalServerData> history = new ArrayList();
        ArrayList<LocalServerData> favorites = new ArrayList();
        ArrayList<LocalServerData> blocked = new ArrayList();

        for (LocalServerData server : serversMap.values()) {
            if (server.inHistory) {
                history.add(server);
            }
            if (server.flag == ServerFlags.Favorite) {
                favorites.add(server);
            }
            if (server.flag == ServerFlags.Blocked) {
                blocked.add(server);
            }
        }

        this.historySubject.onNext(history);
        this.favoritesSubject.onNext(favorites);
        this.blockedSubject.onNext(blocked);
        this.currentServerSubject.onNext(currentServerSubject.getValue());
    }
}
