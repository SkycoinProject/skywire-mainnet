package com.skywire.skycoin.vpn.activities.apps;

import android.content.Context;
import android.content.pm.ResolveInfo;
import android.content.res.Resources;
import android.view.View;
import android.view.ViewGroup;

import androidx.annotation.NonNull;
import androidx.recyclerview.widget.RecyclerView;

import com.skywire.skycoin.vpn.R;
import com.skywire.skycoin.vpn.helpers.BoxRowTypes;
import com.skywire.skycoin.vpn.extensible.ClickWithIndexEvent;
import com.skywire.skycoin.vpn.helpers.Globals;
import com.skywire.skycoin.vpn.helpers.HelperFunctions;
import com.skywire.skycoin.vpn.extensible.ListViewHolder;
import com.skywire.skycoin.vpn.vpn.VPNGeneralPersistentData;

import java.util.ArrayList;
import java.util.HashSet;
import java.util.List;

public class AppsAdapter extends RecyclerView.Adapter<ListViewHolder<View>> implements ClickWithIndexEvent<Void> {
    public interface AppListChangedListener {
        boolean onAppListChanged();
    }

    private final int installedAppsIndexExtra = 10;
    private final int uninstalledAppsIndexExtra = 1000000;

    private Context context;
    private List<ResolveInfo> appList;
    private List<String> uninstalledApps;
    private AppListChangedListener appListChangedListener;

    private HashSet<String> selectedApps;
    private Globals.AppFilteringModes selectedOption;

    private int[] optionTexts = new int[3];
    private int[] optionDescriptions = new int[3];
    private ArrayList<AppListOptionButton> optionButtons = new ArrayList<>();
    private ArrayList<AppListRow> appRows = new ArrayList<>();

    private ArrayList<AppListRow> premadeRows = new ArrayList<>();
    private int lastUsedPremadeRowIndex = 0;

    private int elementsPerRow = 1;

    private boolean readOnly;

    public AppsAdapter(Context context, boolean readOnly) {
        this.context = context;
        this.readOnly = readOnly;

        selectedApps = VPNGeneralPersistentData.getAppList(new HashSet<>());
        changeSelectedOption(VPNGeneralPersistentData.getAppsSelectionMode());

        appList = HelperFunctions.getDeviceAppsList();

        HashSet<String> filteredApps = HelperFunctions.filterAvailableApps(selectedApps);
        if (filteredApps.size() != selectedApps.size()) {
            uninstalledApps = new ArrayList<>();

            for (String app : selectedApps) {
                if (!filteredApps.contains(app)) {
                    uninstalledApps.add(app);
                }
            }
        }

        optionTexts[0] =  R.string.tmp_select_apps_protect_all_button;
        optionTexts[1] =  R.string.tmp_select_apps_protect_selected_button;
        optionTexts[2] =  R.string.tmp_select_apps_unprotect_selected_button;

        optionDescriptions[0] =  R.string.tmp_select_apps_protect_all_button_desc;
        optionDescriptions[1] =  R.string.tmp_select_apps_protect_selected_button_desc;
        optionDescriptions[2] =  R.string.tmp_select_apps_unprotect_selected_button_desc;

        int screenWidthInDP = (int)(Resources.getSystem().getDisplayMetrics().widthPixels / context.getResources().getDisplayMetrics().density);
        elementsPerRow = Math.max(screenWidthInDP / 360, 1);

        int screenHeightInDP = (int)(Resources.getSystem().getDisplayMetrics().heightPixels / context.getResources().getDisplayMetrics().density);
        int aproxRowsToFillScreen = (int)Math.ceil((screenHeightInDP / AppListButton.APROX_HEIGHT_DP) * 1.3);

        for (int i = 0; i < aproxRowsToFillScreen; i++) {
            premadeRows.add(createNewRow());
        }
    }

    public void setAppListChangedEventListener(AppListChangedListener listener) {
        appListChangedListener = listener;
    }

    private int getInstalledAppsRowsCount() {
        return (int)Math.ceil((double)appList.size() / (double)elementsPerRow);
    }

    private int getUninstalledAppsRowsCount() {
        if (uninstalledApps == null) {
            return 0;
        }

        return (int)Math.ceil((double)uninstalledApps.size() / (double)elementsPerRow);
    }

    @Override
    public int getItemViewType(int position) {
        if (position == 0 || position == 4 || position == 5 + getInstalledAppsRowsCount()) {
            return 2;
        }

        if (position < 4) {
            return 0;
        }

        return 1;
    }

    @NonNull
    @Override
    public ListViewHolder<View> onCreateViewHolder(@NonNull ViewGroup parent, int viewType) {
        if (viewType == 0) {
            AppListOptionButton view = new AppListOptionButton(context);
            view.setClickWithIndexEventListener(this);
            optionButtons.add(view);

            if (readOnly) {
                view.setEnabled(false);
            }

            return new ListViewHolder<>(view);
        } else if (viewType == 1) {
            AppListRow view;

            if (lastUsedPremadeRowIndex < premadeRows.size()) {
                view = premadeRows.get(lastUsedPremadeRowIndex);
                lastUsedPremadeRowIndex += 1;
            } else {
                view = createNewRow();
            }

            return new ListViewHolder<>(view);
        }

        AppListSeparator view = new AppListSeparator(context);

        return new ListViewHolder<>(view);
    }

    private AppListRow createNewRow() {
        AppListRow view = new AppListRow(context, elementsPerRow);
        view.setClickWithIndexEventListener(this);
        view.setEnabled(selectedOption != Globals.AppFilteringModes.PROTECT_ALL);
        appRows.add(view);

        if (readOnly) {
            view.setEnabled(false);
        }

        return view;
    }

    @Override
    public void onBindViewHolder(@NonNull ListViewHolder<View> holder, int position) {
        if (holder.getItemViewType() == 0) {
            boolean showChecked = false;
            if (position == 1 && selectedOption == Globals.AppFilteringModes.PROTECT_ALL) { showChecked = true; }
            if (position == 2 && selectedOption == Globals.AppFilteringModes.PROTECT_SELECTED) { showChecked = true; }
            if (position == 3 && selectedOption == Globals.AppFilteringModes.IGNORE_SELECTED) { showChecked = true; }

            ((AppListOptionButton)(holder.itemView)).setIndex(position);
            ((AppListOptionButton)(holder.itemView)).changeData(optionTexts[position - 1], optionDescriptions[position - 1]);
            ((AppListOptionButton)(holder.itemView)).setChecked(showChecked);

            if (position == 1) {
                ((AppListOptionButton)holder.itemView).setBoxRowType(BoxRowTypes.TOP);
            } else if (position == 2) {
                ((AppListOptionButton)holder.itemView).setBoxRowType(BoxRowTypes.MIDDLE);
            } else {
                ((AppListOptionButton)holder.itemView).setBoxRowType(BoxRowTypes.BOTTOM);
            }

            return;
        } else if (holder.getItemViewType() == 2) {
            if (position == 0) {
                ((AppListSeparator)holder.itemView).changeTitle(R.string.tmp_select_apps_mode_title);
            } else if (position == 4) {
                if (this.uninstalledApps != null) {
                    ((AppListSeparator) holder.itemView).changeTitle(R.string.tmp_select_apps_installed_apps_title);
                } else {
                    ((AppListSeparator) holder.itemView).changeTitle(R.string.tmp_select_apps_apps_title);
                }
            } else {
                ((AppListSeparator)holder.itemView).changeTitle(R.string.tmp_select_apps_uninstalled_apps_title);
            }

            return;
        }

        int initialInstalledAppsRowIndex = 5;
        if (position < initialInstalledAppsRowIndex + getInstalledAppsRowsCount()) {
            int rowIndex = (position - initialInstalledAppsRowIndex);

            ResolveInfo[] dataForRow = new ResolveInfo[elementsPerRow];
            boolean[] checkedListForRow = new boolean[elementsPerRow];
            for (int i = 0; i < elementsPerRow; i++){
                int appIndex = (rowIndex * elementsPerRow) + i;
                if (appIndex < appList.size()) {
                    dataForRow[i] = appList.get(appIndex);
                    checkedListForRow[i] = selectedApps.contains(appList.get(appIndex).activityInfo.packageName);
                }
            }

            ((AppListRow) (holder.itemView)).setIndex(installedAppsIndexExtra + (rowIndex * elementsPerRow));
            ((AppListRow) (holder.itemView)).changeData(dataForRow);
            ((AppListRow) (holder.itemView)).setChecked(checkedListForRow);

            if (getInstalledAppsRowsCount() == 1) {
                ((AppListRow)holder.itemView).setBoxRowType(BoxRowTypes.SINGLE);
            } else if (rowIndex == 0) {
                ((AppListRow)holder.itemView).setBoxRowType(BoxRowTypes.TOP);
            } else if (rowIndex == getInstalledAppsRowsCount() - 1) {
                ((AppListRow)holder.itemView).setBoxRowType(BoxRowTypes.BOTTOM);
            } else {
                ((AppListRow)holder.itemView).setBoxRowType(BoxRowTypes.MIDDLE);
            }
        } else {
            int initialUninstalledAppsRowIndex = initialInstalledAppsRowIndex + getInstalledAppsRowsCount() + 1;
            int rowIndex = (position - initialUninstalledAppsRowIndex);

            String[] dataForRow = new String[elementsPerRow];
            boolean[] checkedListForRow = new boolean[elementsPerRow];
            for (int i = 0; i < elementsPerRow; i++){
                int appIndex = (rowIndex * elementsPerRow) + i;
                if (appIndex < uninstalledApps.size()) {
                    dataForRow[i] = uninstalledApps.get(appIndex);
                    checkedListForRow[i] = selectedApps.contains(uninstalledApps.get(appIndex));
                }
            }

            ((AppListRow) (holder.itemView)).setIndex(uninstalledAppsIndexExtra + (rowIndex * elementsPerRow));
            ((AppListRow) (holder.itemView)).changeData(dataForRow);
            ((AppListRow) (holder.itemView)).setChecked(checkedListForRow);

            if (getUninstalledAppsRowsCount() == 1) {
                ((AppListRow)holder.itemView).setBoxRowType(BoxRowTypes.SINGLE);
            } else if (rowIndex == 0) {
                ((AppListRow)holder.itemView).setBoxRowType(BoxRowTypes.TOP);
            } else if (rowIndex == getUninstalledAppsRowsCount() - 1) {
                ((AppListRow)holder.itemView).setBoxRowType(BoxRowTypes.BOTTOM);
            } else {
                ((AppListRow)holder.itemView).setBoxRowType(BoxRowTypes.MIDDLE);
            }
        }
    }

    @Override
    public int getItemCount() {
        int result = 3 + 2 + getInstalledAppsRowsCount();

        if (getUninstalledAppsRowsCount() > 0) {
            result += 1 + getUninstalledAppsRowsCount();
        }

        return result;
    }

    @Override
    public void onClickWithIndex(int index, Void data) {
        if (appListChangedListener != null) {
            if (!appListChangedListener.onAppListChanged()) {
                return;
            }
        }

        if (index < installedAppsIndexExtra) {
            if (index == 1) {
                changeSelectedOption(Globals.AppFilteringModes.PROTECT_ALL);
            } else if (index == 2) {
                changeSelectedOption(Globals.AppFilteringModes.PROTECT_SELECTED);
            } else if (index == 3) {
                changeSelectedOption(Globals.AppFilteringModes.IGNORE_SELECTED);
            }
        } else {
            processAppClicked(index);
        }
    }

    private void changeSelectedOption(Globals.AppFilteringModes option) {
        if (option != selectedOption) {
            if (option == Globals.AppFilteringModes.PROTECT_ALL) {
                for (AppListRow row : appRows) {
                    row.setEnabled(false);
                }
            } else if (selectedOption == Globals.AppFilteringModes.PROTECT_ALL) {
                for (AppListRow row : appRows) {
                    row.setEnabled(true);
                }
            }

            selectedOption = option;
            VPNGeneralPersistentData.setAppsSelectionMode(selectedOption);

            for (AppListOptionButton optionButton : optionButtons) {
                optionButton.setChecked(
                    (optionButton.getIndex() == 1 && selectedOption == Globals.AppFilteringModes.PROTECT_ALL) ||
                    (optionButton.getIndex() == 2 && selectedOption == Globals.AppFilteringModes.PROTECT_SELECTED) ||
                    (optionButton.getIndex() == 3 && selectedOption == Globals.AppFilteringModes.IGNORE_SELECTED)
                );
            }
        }
    }

    private void processAppClicked(int index) {
        String app;

        if (index < uninstalledAppsIndexExtra) {
            app = appList.get(index - installedAppsIndexExtra).activityInfo.packageName;
        } else {
            app = uninstalledApps.get(index - uninstalledAppsIndexExtra);
        }

        boolean showAppChecked;
        if (selectedApps.contains(app)) {
            selectedApps.remove(app);
            showAppChecked = false;
        } else {
            selectedApps.add(app);
            showAppChecked = true;
        }

        for (AppListRow row : appRows) {
            row.setChecked(app, showAppChecked);
        }

        VPNGeneralPersistentData.setAppList(selectedApps);
    }
}
