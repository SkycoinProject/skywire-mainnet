package com.skywire.skycoin.vpn.activities.servers;

import android.content.Context;
import android.util.AttributeSet;
import android.view.LayoutInflater;
import android.view.View;
import android.view.ViewGroup;
import android.widget.FrameLayout;

import androidx.recyclerview.widget.RecyclerView;

import com.skywire.skycoin.vpn.R;
import com.skywire.skycoin.vpn.controls.BoxRowLayout;
import com.skywire.skycoin.vpn.extensible.ClickEvent;
import com.skywire.skycoin.vpn.extensible.ClickWithIndexEvent;
import com.skywire.skycoin.vpn.helpers.HelperFunctions;

public class ServerListOptions extends FrameLayout implements ClickEvent {
    public static final int filterIndex = -1;
    public static final int addIndex = -2;
    public static final int sortIndex = -3;
    public static final int showPublicIndex = -10;
    public static final int showHistoryIndex = -11;
    public static final int showFavoritesIndex = -12;
    public static final int showBlockedIndex = -13;

    public ServerListOptions(Context context) {
        super(context);
        Initialize(context, null);
    }
    public ServerListOptions(Context context, AttributeSet attrs) {
        super(context, attrs);
        Initialize(context, attrs);
    }
    public ServerListOptions(Context context, AttributeSet attrs, int defStyle) {
        super(context, attrs, defStyle);
        Initialize(context, attrs);
    }

    private BoxRowLayout tabsContainer;
    private ServerListTopTab tabPublic;
    private ServerListTopTab tabHistory;
    private ServerListTopTab tabFavorites;
    private ServerListTopTab tabBlocked;
    private ServerListOptionButton buttonSort;
    private ServerListOptionButton buttonFilter;
    private ServerListOptionButton buttonAdd;

    private ClickWithIndexEvent<Void> clickListener;

    protected void Initialize (Context context, AttributeSet attrs) {
        LayoutInflater inflater = (LayoutInflater)context.getSystemService (Context.LAYOUT_INFLATER_SERVICE);
        View rootView = inflater.inflate(R.layout.view_server_list_options, this, true);

        tabsContainer = this.findViewById (R.id.tabsContainer);
        tabPublic = this.findViewById (R.id.tabPublic);
        tabHistory = this.findViewById (R.id.tabHistory);
        tabFavorites = this.findViewById (R.id.tabFavorites);
        tabBlocked = this.findViewById (R.id.tabBlocked);
        buttonSort = this.findViewById (R.id.buttonSort);
        buttonFilter = this.findViewById (R.id.buttonFilter);
        buttonAdd = this.findViewById (R.id.buttonAdd);

        tabPublic.setClickEventListener(this);
        tabHistory.setClickEventListener(this);
        tabFavorites.setClickEventListener(this);
        tabBlocked.setClickEventListener(this);
        buttonSort.setClickEventListener(this);
        buttonFilter.setClickEventListener(this);
        buttonAdd.setClickEventListener(this);

        RecyclerView.LayoutParams params = new RecyclerView.LayoutParams(ViewGroup.LayoutParams.MATCH_PARENT, ViewGroup.LayoutParams.WRAP_CONTENT);
        rootView.setLayoutParams(params);

        if (HelperFunctions.getWidthType(getContext()) == HelperFunctions.WidthTypes.SMALL) {
            tabsContainer.setVisibility(GONE);
        }
    }

    public void setClickWithIndexEventListener(ClickWithIndexEvent<Void> listener) {
        clickListener = listener;
    }

    public void selectCorrectTab(ServerLists currentListType) {
        tabPublic.changeState(false);
        tabHistory.changeState(false);
        tabFavorites.changeState(false);
        tabBlocked.changeState(false);

        if (currentListType == ServerLists.Public) {
            tabPublic.changeState(true);
        } else if (currentListType == ServerLists.History) {
            tabHistory.changeState(true);
        } else if (currentListType == ServerLists.Favorites) {
            tabFavorites.changeState(true);
        } else if (currentListType == ServerLists.Blocked) {
            tabBlocked.changeState(true);
        }
    }

    @Override
    public void onClick(View view) {
        if (clickListener != null) {
            if (view.getId() == R.id.tabPublic) {
                clickListener.onClickWithIndex(showPublicIndex, null);
            } else if (view.getId() == R.id.tabHistory) {
                clickListener.onClickWithIndex(showHistoryIndex, null);
            } else if (view.getId() == R.id.tabFavorites) {
                clickListener.onClickWithIndex(showFavoritesIndex, null);
            } else if (view.getId() == R.id.tabBlocked) {
                clickListener.onClickWithIndex(showBlockedIndex, null);
            } else if (view.getId() == R.id.buttonSort) {
                clickListener.onClickWithIndex(sortIndex, null);
            } else if (view.getId() == R.id.buttonAdd) {
                clickListener.onClickWithIndex(addIndex, null);
            } else if (view.getId() == R.id.buttonFilter) {
                clickListener.onClickWithIndex(filterIndex, null);
            }
        }
    }
}
