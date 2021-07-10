package com.skywire.skycoin.vpn.activities.index;

import androidx.appcompat.app.AppCompatActivity;
import androidx.fragment.app.Fragment;
import androidx.viewpager2.adapter.FragmentStateAdapter;

import com.skywire.skycoin.vpn.activities.servers.ServersActivity;
import com.skywire.skycoin.vpn.activities.settings.SettingsActivity;
import com.skywire.skycoin.vpn.activities.start.StartActivity;

public class IndexPageAdapter extends FragmentStateAdapter {
    public interface RequestTabListener {
        void onOpenStatusRequested();
        void onOpenServerListRequested();
    }

    private StartActivity tab1 = new StartActivity();
    private ServersActivity tab2 = new ServersActivity();
    private SettingsActivity tab3 = new SettingsActivity();

    public IndexPageAdapter(AppCompatActivity activity) {
        super(activity);
    }

    public void setRequestTabListener(RequestTabListener listener) {
        tab1.setRequestTabListener(listener);
        tab2.setRequestTabListener(listener);
    }

    @Override
    public Fragment createFragment(int position) {
        Fragment response;

        if (position == 0) {
            response = tab1;
        } else if (position == 1) {
            response = tab2;
        } else {
            response = tab3;
        }

        return response;
    }

    @Override
    public int getItemCount() {
        return 3;
    }
}
