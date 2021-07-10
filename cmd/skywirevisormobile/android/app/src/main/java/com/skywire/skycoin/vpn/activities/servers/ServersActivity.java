package com.skywire.skycoin.vpn.activities.servers;

import android.content.SharedPreferences;
import android.os.Bundle;
import android.view.LayoutInflater;
import android.view.View;
import android.view.ViewGroup;
import android.widget.FrameLayout;
import android.widget.ImageView;
import android.widget.LinearLayout;
import android.widget.ProgressBar;
import android.widget.TextView;

import androidx.annotation.NonNull;
import androidx.annotation.Nullable;
import androidx.fragment.app.Fragment;
import androidx.preference.PreferenceManager;
import androidx.recyclerview.widget.LinearLayoutManager;
import androidx.recyclerview.widget.RecyclerView;

import com.google.gson.Gson;
import com.skywire.skycoin.vpn.App;
import com.skywire.skycoin.vpn.R;
import com.skywire.skycoin.vpn.activities.index.IndexPageAdapter;
import com.skywire.skycoin.vpn.controls.Tab;
import com.skywire.skycoin.vpn.extensible.ClickEvent;
import com.skywire.skycoin.vpn.helpers.HelperFunctions;
import com.skywire.skycoin.vpn.objects.LocalServerData;
import com.skywire.skycoin.vpn.objects.ServerFlags;
import com.skywire.skycoin.vpn.objects.ServerRatings;
import com.skywire.skycoin.vpn.vpn.VPNCoordinator;
import com.skywire.skycoin.vpn.vpn.VPNServersPersistentData;

import java.util.ArrayList;
import java.util.Date;
import java.util.concurrent.TimeUnit;

import io.reactivex.rxjava3.android.schedulers.AndroidSchedulers;
import io.reactivex.rxjava3.core.Observable;
import io.reactivex.rxjava3.disposables.Disposable;
import io.reactivex.rxjava3.schedulers.Schedulers;

public class ServersActivity extends Fragment implements VpnServersAdapter.VpnServerListEventListener, ClickEvent {
    public static String ADDRESS_DATA_PARAM = "address";
    private static final String ACTIVE_TAB_KEY = "activeTab";

    private Tab tabPublic;
    private Tab tabHistory;
    private Tab tabFavorites;
    private Tab tabBlocked;
    private RecyclerView recycler;
    private ProgressBar loadingAnimation;
    private TextView textNoResults;
    private LinearLayout noResultsContainer;
    private LinearLayout bottomTabsContainer;
    private FrameLayout internalContainer;
    private ImageView ImageBottomTabsShadow;

    private IndexPageAdapter.RequestTabListener requestTabListener;
    private ServerLists listType = ServerLists.Public;
    private VpnServersAdapter adapter;
    private SharedPreferences settings = PreferenceManager.getDefaultSharedPreferences(App.getContext());

    private Disposable serverSubscription;

    @Nullable
    @Override
    public View onCreateView(@NonNull LayoutInflater inflater, @Nullable ViewGroup container, @Nullable Bundle savedInstanceState) {
        super.onCreateView(inflater, container, savedInstanceState);

        return inflater.inflate(R.layout.activity_server_list, container, true);
    }

    @Override
    public void onViewCreated(View view, Bundle savedInstanceState) {
        super.onViewCreated(view, savedInstanceState);

        tabPublic = view.findViewById(R.id.tabPublic);
        tabHistory = view.findViewById(R.id.tabHistory);
        tabFavorites = view.findViewById(R.id.tabFavorites);
        tabBlocked = view.findViewById(R.id.tabBlocked);
        recycler = view.findViewById(R.id.recycler);
        loadingAnimation = view.findViewById(R.id.loadingAnimation);
        textNoResults = view.findViewById(R.id.textNoResults);
        noResultsContainer = view.findViewById(R.id.noResultsContainer);
        bottomTabsContainer = view.findViewById(R.id.bottomTabsContainer);
        internalContainer = view.findViewById(R.id.internalContainer);
        ImageBottomTabsShadow = view.findViewById(R.id.ImageBottomTabsShadow);

        tabPublic.setClickEventListener(this);
        tabHistory.setClickEventListener(this);
        tabFavorites.setClickEventListener(this);
        tabBlocked.setClickEventListener(this);

        LinearLayoutManager layoutManager = new LinearLayoutManager(getContext());
        recycler.setLayoutManager(layoutManager);

        // This code retrieves the data from the server and populates the list with the recovered
        // data, but is not used right now as the server is returning empty arrays.
        // requestData()

        noResultsContainer.setVisibility(View.GONE);
        loadingAnimation.setVisibility(View.VISIBLE);

        // Initialize the recycler.
        adapter = new VpnServersAdapter(getContext());
        adapter.setVpnServerListEventListener(this);
        adapter.setData(new ArrayList<>(), listType);
        recycler.setAdapter(adapter);

        Gson gson = new Gson();
        String savedlistType = settings.getString(ACTIVE_TAB_KEY, null);
        if (savedlistType != null) {
            listType = gson.fromJson(savedlistType, ServerLists.class);
        }

        showCorrectList();

        if (HelperFunctions.getWidthType(getContext()) != HelperFunctions.WidthTypes.SMALL) {
            bottomTabsContainer.setVisibility(View.GONE);
            ImageBottomTabsShadow.setVisibility(View.GONE);

            FrameLayout.LayoutParams params = (FrameLayout.LayoutParams)internalContainer.getLayoutParams();
            params.bottomMargin = 0;
            internalContainer.setLayoutParams(params);
        }
    }

    public void setRequestTabListener(IndexPageAdapter.RequestTabListener listener) {
        requestTabListener = listener;
    }

    @Override
    public void tabChangeRequested(ServerLists newListType) {
        if (newListType != listType) {
            listType = newListType;

            finishChangingTab();
        }
    }

    @Override
    public void onClick(View view) {
        if (view.getId() == R.id.tabPublic) {
            listType = ServerLists.Public;
        } else if (view.getId() == R.id.tabHistory) {
            listType = ServerLists.History;
        } else if (view.getId() == R.id.tabFavorites) {
            listType = ServerLists.Favorites;
        } else if (view.getId() == R.id.tabBlocked) {
            listType = ServerLists.Blocked;
        }

        finishChangingTab();
    }

    private void finishChangingTab() {
        Gson gson = new Gson();
        String listTypeString = gson.toJson(listType);
        settings.edit()
            .putString(ACTIVE_TAB_KEY, listTypeString)
            .apply();

        showCorrectList();
    }

    private void showCorrectList() {
        tabPublic.changeState(false);
        tabHistory.changeState(false);
        tabFavorites.changeState(false);
        tabBlocked.changeState(false);

        if (listType == ServerLists.Public) {
            tabPublic.changeState(true);
            // Use test data, for now.
            showTestServers();
        } else {
            if (listType == ServerLists.History) {
                tabHistory.changeState(true);
            } else if (listType == ServerLists.Favorites) {
                tabFavorites.changeState(true);
            } else if (listType == ServerLists.Blocked) {
                tabBlocked.changeState(true);
            }

            requestLocalData();
        }
    }

    private void requestData() {
        if (serverSubscription != null) {
            serverSubscription.dispose();
        }

        /*
        serverSubscription = ApiClient.getVpnServers()
            .subscribeOn(Schedulers.io())
            .observeOn(AndroidSchedulers.mainThread())
            .subscribe(response -> {
                VpnServersAdapter adapter = new VpnServersAdapter(this, response.body());
                adapter.setVpnSelectedEventListener(this);
                recycler.setAdapter(adapter);

                // TODO: addSavedData will remove all blocked servers, so it will have to be called
                // every time the blocked servers list changes.
            }, err -> {
                this.requestData();
            });
        */
    }

    private void requestLocalData() {
        if (serverSubscription != null) {
            serverSubscription.dispose();
        }

        adapter.setData(new ArrayList<>(), listType);
        noResultsContainer.setVisibility(View.GONE);
        loadingAnimation.setVisibility(View.VISIBLE);

        Observable<ArrayList<LocalServerData>> request;
        if (listType == ServerLists.History) {
            request = VPNServersPersistentData.getInstance().history();
        } else if (listType == ServerLists.Favorites) {
            request = VPNServersPersistentData.getInstance().favorites();
        } else {
            request = VPNServersPersistentData.getInstance().blocked();
        }

        serverSubscription = request.subscribe(response -> {
            ArrayList<VpnServerForList> list = new ArrayList<>();

            for (LocalServerData server : response) {
                list.add(convertLocalServerData(server));
            }

            loadingAnimation.setVisibility(View.GONE);

            adapter.setData(list, listType);
        });
    }

    public static VpnServerForList convertLocalServerData(LocalServerData server) {
        if (server == null) {
            return null;
        }

        VpnServerForList converted = new VpnServerForList();

        converted.countryCode = server.countryCode;
        converted.name = server.name;
        converted.customName = server.customName;
        converted.location = server.location;
        converted.pk = server.pk;
        converted.note = server.note;
        converted.personalNote = server.personalNote;
        converted.lastUsed = server.lastUsed;
        converted.inHistory = server.inHistory;
        converted.flag = server.flag;
        converted.enteredManually = server.enteredManually;
        converted.hasPassword = server.password != null && !server.password.equals("");

        return converted;
    }

    @Override
    public void onResume() {
        super.onResume();
    }

    @Override
    public void onDestroyView() {
        super.onDestroyView();

        if (serverSubscription != null) {
            serverSubscription.dispose();
        }
    }

    @Override
    public void onVpnServerSelected(VpnServerForList selectedServer) {
        start(VPNServersPersistentData.getInstance().processFromList(selectedServer));
    }

    @Override
    public void onManualEntered(LocalServerData server) {
        start(server);
    }

    @Override
    public void listHasElements(boolean hasElements, boolean emptyBecauseFilters) {
        if (hasElements || loadingAnimation.getVisibility() != View.GONE) {
            noResultsContainer.setVisibility(View.GONE);
        } else {
            noResultsContainer.setVisibility(View.VISIBLE);

            if (emptyBecauseFilters) {
                textNoResults.setText(R.string.tmp_select_server_empty_with_filter);
            } else {
                if (listType == ServerLists.History) {
                    textNoResults.setText(R.string.tmp_select_server_empty_history);
                } else if (listType == ServerLists.Favorites) {
                    textNoResults.setText(R.string.tmp_select_server_empty_favorites);
                } else if (listType == ServerLists.Blocked) {
                    textNoResults.setText(R.string.tmp_select_server_empty_blocked);
                } else {
                    textNoResults.setText(R.string.tmp_select_server_empty_discovery);
                }
            }
        }
    }

    private void start(LocalServerData server) {
        if (VPNCoordinator.getInstance().isServiceRunning()) {
            HelperFunctions.showToast(getContext().getText(R.string.tmp_select_server_running_error).toString(), true);
            return;
        }

        boolean starting = HelperFunctions.prepareAndStartVpn(getActivity(), server);

        if (starting) {
            if (requestTabListener != null) {
                requestTabListener.onOpenStatusRequested();
            }
        }
    }

    private void showTestServers() {
        ArrayList<VpnServerForList> servers = new ArrayList<>();

        VpnServerForList testServer = new VpnServerForList();
        testServer.lastUsed = new Date();
        testServer.countryCode = "au";
        testServer.name = "Server name";
        testServer.location = "Melbourne";
        testServer.pk = "024ec47420176680816e0406250e7156465e4531f5b26057c9f6297bb0303558c7";
        testServer.congestion = 20;
        testServer.congestionRating = ServerRatings.Gold;
        testServer.latency = 123;
        testServer.latencyRating = ServerRatings.Gold;
        testServer.hops = 3;
        testServer.note = "Note";
        servers.add(testServer);

        testServer = new VpnServerForList();
        testServer.lastUsed = new Date();
        testServer.countryCode = "br";
        testServer.name = "Test server 14";
        testServer.location = "Rio de Janeiro";
        testServer.pk = "034ec47420176680816e0406250e7156465e4531f5b26057c9f6297bb0303558c7";
        testServer.congestion = 20;
        testServer.congestionRating = ServerRatings.Silver;
        testServer.latency = 12345;
        testServer.latencyRating = ServerRatings.Gold;
        testServer.hops = 3;
        testServer.note = "Note";
        servers.add(testServer);

        testServer = new VpnServerForList();
        testServer.lastUsed = new Date();
        testServer.countryCode = "de";
        testServer.name = "Test server 20";
        testServer.location = "Berlin";
        testServer.pk = "044ec47420176680816e0406250e7156465e4531f5b26057c9f6297bb0303558c7";
        testServer.congestion = 20;
        testServer.congestionRating = ServerRatings.Gold;
        testServer.latency = 123;
        testServer.latencyRating = ServerRatings.Bronze;
        testServer.hops = 7;
        servers.add(testServer);

        VPNServersPersistentData.getInstance().updateFromDiscovery(servers);

        if (serverSubscription != null) {
            serverSubscription.dispose();
        }

        adapter.setData(new ArrayList<>(), listType);
        noResultsContainer.setVisibility(View.GONE);
        loadingAnimation.setVisibility(View.VISIBLE);

        serverSubscription = Observable.just(servers).delay(50, TimeUnit.MILLISECONDS).flatMap(serversList ->
            VPNServersPersistentData.getInstance().history()
        ).subscribeOn(Schedulers.io()).observeOn(AndroidSchedulers.mainThread()).subscribe(r -> {
            loadingAnimation.setVisibility(View.GONE);

            ArrayList<VpnServerForList> serversCopy = new ArrayList<>(servers);

            removeSavedData(serversCopy);
            addSavedData(serversCopy);
            adapter.setData(serversCopy, ServerLists.Public);
        });

    }

    private void addSavedData(ArrayList<VpnServerForList> servers) {
        ArrayList<VpnServerForList> remove = new ArrayList();
        for (VpnServerForList server : servers) {
            LocalServerData savedVersion = VPNServersPersistentData.getInstance().getSavedVersion(server.pk);

            if (savedVersion != null) {
                server.customName = savedVersion.customName;
                server.personalNote = savedVersion.personalNote;
                server.inHistory = savedVersion.inHistory;
                server.flag = savedVersion.flag;
                server.enteredManually = savedVersion.enteredManually;
                server.hasPassword = savedVersion.password != null && !savedVersion.password.equals("");
            }

            if (server.flag == ServerFlags.Blocked) {
                remove.add(server);
            }
        }

        servers.removeAll(remove);
    }

    private void removeSavedData(ArrayList<VpnServerForList> servers) {
        for (VpnServerForList server : servers) {
            server.customName = null;
            server.personalNote = null;
            server.inHistory = false;
            server.flag = ServerFlags.None;
            server.enteredManually = false;
            server.hasPassword = false;
        }
    }
}
