package com.skywire.skycoin.vpn.activities.index;

import android.content.Intent;
import android.os.Bundle;
import android.view.View;
import android.widget.FrameLayout;
import android.widget.ImageView;

import androidx.appcompat.app.AppCompatActivity;
import androidx.viewpager2.widget.ViewPager2;

import com.google.android.material.tabs.TabLayout;
import com.google.android.material.tabs.TabLayoutMediator;
import com.skywire.skycoin.vpn.R;
import com.skywire.skycoin.vpn.controls.TabletTopBar;
import com.skywire.skycoin.vpn.controls.TopBar;
import com.skywire.skycoin.vpn.controls.TopTab;
import com.skywire.skycoin.vpn.extensible.ClickWithIndexEvent;
import com.skywire.skycoin.vpn.helpers.HelperFunctions;
import com.skywire.skycoin.vpn.vpn.VPNCoordinator;

public class IndexActivity extends AppCompatActivity implements IndexPageAdapter.RequestTabListener, ClickWithIndexEvent<Void> {
    private ImageView imageBackground;
    private ImageView imageTopBarShadow;
    private ViewPager2 pager;
    private TopBar topBar;
    private TabletTopBar tabletTopBar;
    private TabLayout tabs;

    private TabLayoutMediator tabLayoutMediator;

    @Override
    protected void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        setContentView(R.layout.activity_index);

        imageBackground = findViewById(R.id.imageBackground);
        imageTopBarShadow = findViewById(R.id.imageTopBarShadow);
        pager = findViewById(R.id.pager);
        topBar = findViewById(R.id.topBar);
        tabletTopBar = findViewById(R.id.tabletTopBar);
        tabs = findViewById(R.id.tabs);

        if (HelperFunctions.showBackgroundForVerticalScreen()) {
            imageBackground.setVisibility(View.GONE);
        }

        IndexPageAdapter adapter = new IndexPageAdapter(this);
        adapter.setRequestTabListener(this);
        pager.setAdapter(adapter);

        tabLayoutMediator = new TabLayoutMediator(tabs, pager, (tab, position) -> {
            if (position == 0) {
                tab.setCustomView(new TopTab(this, R.string.tmp_status_page_title));
            } else if (position == 1) {
                tab.setCustomView(new TopTab(this, R.string.tmp_select_server_title));
            } else {
                tab.setCustomView(new TopTab(this, R.string.tmp_options_title));
            }

            if (position != 0) {
                tab.getCustomView().setAlpha(0.4f);
            }
        });
        tabLayoutMediator.attach();

        pager.setOffscreenPageLimit(3);

        if (HelperFunctions.getWidthType(this) == HelperFunctions.WidthTypes.SMALL) {
            tabletTopBar.setVisibility(View.GONE);
            tabletTopBar.close();

            tabs.addOnTabSelectedListener(new TabLayout.OnTabSelectedListener() {
                @Override
                public void onTabSelected(TabLayout.Tab tab) {
                    tab.getCustomView().setAlpha(1f);
                }
                @Override
                public void onTabUnselected(TabLayout.Tab tab) {
                    tab.getCustomView().setAlpha(0.4f);
                }
                @Override
                public void onTabReselected(TabLayout.Tab tab) { }
            });
        } else {
            topBar.setVisibility(View.GONE);
            tabs.setVisibility(View.GONE);
            imageTopBarShadow.setVisibility(View.GONE);

            FrameLayout.LayoutParams params = (FrameLayout.LayoutParams)imageBackground.getLayoutParams();
            params.topMargin = 0;
            imageBackground.setLayoutParams(params);

            params = (FrameLayout.LayoutParams)pager.getLayoutParams();
            params.topMargin = (int)Math.round(getResources().getDimension(R.dimen.tablet_top_bar_height));
            pager.setLayoutParams(params);

            tabletTopBar.setSelectedTab(TabletTopBar.statusTabIndex);

            pager.registerOnPageChangeCallback(new ViewPager2.OnPageChangeCallback() {
                @Override
                public void onPageScrolled(int position, float positionOffset, int positionOffsetPixels) {
                    super.onPageScrolled(position, positionOffset, positionOffsetPixels);
                }

                @Override
                public void onPageSelected(int position) {
                    super.onPageSelected(position);

                    tabletTopBar.setSelectedTab(position);
                }

                @Override
                public void onPageScrollStateChanged(int state) {
                    super.onPageScrollStateChanged(state);
                }
            });

            tabletTopBar.setClickWithIndexEventListener(this);
        }
    }

    @Override
    public void onResume() {
        super.onResume();

        if (tabletTopBar.getVisibility() != View.GONE) {
            tabletTopBar.onResume();
        }
    }

    @Override
    public void onPause() {
        super.onPause();

        if (tabletTopBar.getVisibility() != View.GONE) {
            tabletTopBar.onPause();
        }
    }

    @Override
    protected void onDestroy() {
        super.onDestroy();

        tabLayoutMediator.detach();
        tabletTopBar.close();
    }

    @Override
    public void onBackPressed() {
        if (pager.getCurrentItem() != 0) {
            pager.setCurrentItem(0);
        } else {
            super.onBackPressed();

            if (VPNCoordinator.getInstance().isServiceRunning()) {
                HelperFunctions.showToast(getString(R.string.general_service_running_notification), false);
            }
        }
    }

    @Override
    public void onOpenStatusRequested() {
        pager.setCurrentItem(0);
    }

    @Override
    public void onOpenServerListRequested() {
        pager.setCurrentItem(1);
    }

    @Override
    protected void onActivityResult(int request, int result, Intent data) {
        super.onActivityResult(request, result, data);

        if (request == VPNCoordinator.VPN_PREPARATION_REQUEST_CODE) {
            VPNCoordinator.getInstance().onActivityResult(request, result, data);
        }
    }

    @Override
    public void onClickWithIndex(int index, Void data) {
        pager.setCurrentItem(index);
    }
}
