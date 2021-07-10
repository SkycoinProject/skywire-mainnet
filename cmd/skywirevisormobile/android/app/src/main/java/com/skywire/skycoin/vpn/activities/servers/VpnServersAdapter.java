package com.skywire.skycoin.vpn.activities.servers;

import android.content.Context;
import android.content.res.Resources;
import android.view.View;
import android.view.ViewGroup;

import androidx.annotation.NonNull;
import androidx.recyclerview.widget.RecyclerView;

import com.skywire.skycoin.vpn.R;
import com.skywire.skycoin.vpn.controls.ManualServerModalWindow;
import com.skywire.skycoin.vpn.controls.options.OptionsItem;
import com.skywire.skycoin.vpn.controls.options.OptionsModalWindow;
import com.skywire.skycoin.vpn.extensible.ClickWithIndexEvent;
import com.skywire.skycoin.vpn.extensible.ListViewHolder;
import com.skywire.skycoin.vpn.helpers.BoxRowTypes;
import com.skywire.skycoin.vpn.helpers.HelperFunctions;
import com.skywire.skycoin.vpn.objects.LocalServerData;
import com.skywire.skycoin.vpn.objects.ServerRatings;
import com.skywire.skycoin.vpn.vpn.VPNCoordinator;

import java.util.ArrayList;
import java.util.Collections;
import java.util.Comparator;
import java.util.HashSet;
import java.util.List;

public class VpnServersAdapter extends RecyclerView.Adapter<ListViewHolder<View>> implements ClickWithIndexEvent<Void> {
    public interface VpnServerListEventListener {
        void onVpnServerSelected(VpnServerForList selectedServer);
        void onManualEntered(LocalServerData server);
        void listHasElements(boolean hasElements, boolean emptyBecauseFilters);
        void tabChangeRequested(ServerLists newListType);
    }

    public enum SortableColumns {
        AUTOMATIC,
        DATE,
        COUNTRY,
        NAME,
        LOCATION,
        PK,
        CONGESTION,
        CONGESTION_RATING,
        LATENCY,
        LATENCY_RATING,
        HOPS,
        NOTE;

        public static int getColumnNameId(SortableColumns column) {
            if (column == SortableColumns.DATE) {
                return R.string.tmp_select_server_date_label;
            } else if (column == SortableColumns.COUNTRY) {
                return R.string.tmp_select_server_country_label;
            } else if (column == SortableColumns.LOCATION) {
                return R.string.tmp_select_server_location_label;
            } else if (column == SortableColumns.PK) {
                return R.string.tmp_select_server_public_key_label;
            } else if (column == SortableColumns.CONGESTION) {
                return R.string.tmp_select_server_congestion_label;
            } else if (column == SortableColumns.CONGESTION_RATING) {
                return R.string.tmp_select_server_congestion_rating_label;
            } else if (column == SortableColumns.LATENCY) {
                return R.string.tmp_select_server_latency_label;
            } else if (column == SortableColumns.LATENCY_RATING) {
                return R.string.tmp_select_server_latency_rating_label;
            } else if (column == SortableColumns.HOPS) {
                return R.string.tmp_select_server_hops_label;
            } else {
                return R.string.tmp_select_server_note_label;
            }
        }
    }

    private Context context;
    private List<VpnServerForList> data;
    private List<VpnServerForList> filteredData;
    private ServerLists listType = ServerLists.Public;
    private VpnServerListEventListener listEventListener;
    private boolean showingRows;
    private int initialServerIndex;

    private ArrayList<FilterModalWindow.Filters> filters;
    private ConditionsList conditionsView;

    private ArrayList<SortableColumns> sortBy;
    private ArrayList<Boolean> sortInverse;

    private ArrayList<ServerListButton> premadeButtons = new ArrayList<>();
    private ArrayList<ServerListTableRow> premadeRows = new ArrayList<>();
    private int lastUsedPremadeButtonIdex = 0;

    private ServerListOptions listOptionsView;
    private ServerListTableHeader tableHeader;

    public VpnServersAdapter(Context context) {
        this.context = context;

        int screenHeightInDP = (int)(Resources.getSystem().getDisplayMetrics().heightPixels / context.getResources().getDisplayMetrics().density);
        showingRows = HelperFunctions.getWidthType(context) != HelperFunctions.WidthTypes.SMALL;

        if (!showingRows) {
            int aproxButtonsToFillScreen = (int)Math.ceil((screenHeightInDP / ServerListButton.APROX_HEIGHT_DP) * 1.3);
            for (int i = 0; i < aproxButtonsToFillScreen; i++) {
                premadeButtons.add(createNewServerButton());
            }
            initialServerIndex = 2;
        } else {
            int aproxButtonsToFillScreen = (int)Math.ceil((screenHeightInDP / ServerListTableRow.APROX_HEIGHT_DP) * 1.3);
            for (int i = 0; i < aproxButtonsToFillScreen; i++) {
                premadeRows.add(createNewServerRow());
            }
            initialServerIndex = 3;
        }
    }

    public void setData(List<VpnServerForList> data, ServerLists listType) {
        this.data = data;
        this.listType = listType;

        if (listOptionsView != null) {
            listOptionsView.selectCorrectTab(listType);
        }

        if (tableHeader != null) {
            tableHeader.setListType(listType);
        }

        processData();
    }

    private void processData() {
        if (filters == null) {
            filters = new ArrayList<>();
            sortBy = new ArrayList<>();
            sortInverse = new ArrayList<>();

            for (int i = 0; i < 4; i++) {
                filters.add(null);
                sortBy.add(SortableColumns.AUTOMATIC);
                sortInverse.add(false);
            }
        }

        FilterModalWindow.Filters currentFilters = filters.get(getCurrentListTypeIntVal());

        if (currentFilters == null) {
            filteredData = data;
        } else {
            filteredData = new ArrayList<>();

            for (VpnServerForList element : data) {
                boolean valid = true;

                if (valid && currentFilters.countryCode != null && !currentFilters.countryCode.equals("")) {
                    String elementVal = element.countryCode != null ? element.countryCode.toUpperCase() : "";
                    if (!elementVal.equals(currentFilters.countryCode.toUpperCase())) {
                        valid = false;
                    }
                }

                if (valid && currentFilters.name != null && !currentFilters.name.equals("")) {
                    if (!HelperFunctions.getServerName(element, "").toUpperCase().contains(currentFilters.name.toUpperCase())) {
                        valid = false;
                    }
                }

                if (valid && currentFilters.location != null && !currentFilters.location.equals("")) {
                    String elementVal = element.location != null ? element.location.toUpperCase() : "";
                    if (!elementVal.contains(currentFilters.location.toUpperCase())) {
                        valid = false;
                    }
                }

                if (valid && currentFilters.pk != null && !currentFilters.pk.equals("")) {
                    if (!element.pk.toUpperCase().contains(currentFilters.pk.toUpperCase())) {
                        valid = false;
                    }
                }

                if (valid && currentFilters.note != null && !currentFilters.note.equals("")) {
                    String elementVal1 = element.note != null ? element.note.toUpperCase() : "";
                    String elementVal2 = element.personalNote != null ? element.personalNote.toUpperCase() : "";
                    String filterVal = currentFilters.note.toUpperCase();
                    if (!elementVal1.contains(filterVal) && !elementVal2.contains(filterVal)) {
                        valid = false;
                    }
                }

                if (valid) {
                    filteredData.add(element);
                }
            }
        }

        if (listEventListener != null) {
            if (data.size() == 0) {
                listEventListener.listHasElements(false, false);
            } else {
                if (filteredData.size() == 0) {
                    listEventListener.listHasElements(false, true);
                } else {
                    listEventListener.listHasElements(true, false);
                }
            }
        }

        sortList();
    }

    private void sortList() {
        if (conditionsView != null) {
            conditionsView.setConditions(sortBy.get(getCurrentListTypeIntVal()), sortInverse.get(getCurrentListTypeIntVal()), filters.get(getCurrentListTypeIntVal()));
        }

        Comparator<VpnServerForList> comparator = (a, b) -> {
            SortableColumns sortColumn = sortBy.get(getCurrentListTypeIntVal());

            if (sortColumn == SortableColumns.AUTOMATIC) {
                if (listType == ServerLists.History) {
                    sortColumn = SortableColumns.DATE;
                } else {
                    sortColumn = SortableColumns.COUNTRY;
                }
            }

            int result = 0;
            if (sortColumn == SortableColumns.DATE) {
                result = (int)((b.lastUsed.getTime() - a.lastUsed.getTime()) / 1000);
            } else if (sortColumn == SortableColumns.COUNTRY) {
                result = a.countryCode.compareTo(b.countryCode);
            } else if (sortColumn == SortableColumns.NAME) {
                result = HelperFunctions.getServerName(a, "").compareTo(HelperFunctions.getServerName(b, ""));
            } else if (sortColumn == SortableColumns.LOCATION) {
                result = (a.location != null ? a.location : "").compareTo((b.location != null ? b.location : ""));
            } else if (sortColumn == SortableColumns.PK) {
                result = (a.pk != null ? a.pk : "").compareTo((b.pk != null ? b.pk : ""));
            } else if (sortColumn == SortableColumns.CONGESTION) {
                result = (int)(a.congestion - b.congestion);
            } else if (sortColumn == SortableColumns.CONGESTION_RATING) {
                result = ServerRatings.getNumberForRating(b.congestionRating) - ServerRatings.getNumberForRating(a.congestionRating);
            } else if (sortColumn == SortableColumns.LATENCY) {
                result = (int)(a.latency - b.latency);
            } else if (sortColumn == SortableColumns.LATENCY_RATING) {
                result = ServerRatings.getNumberForRating(b.latencyRating) - ServerRatings.getNumberForRating(a.latencyRating);
            } else if (sortColumn == SortableColumns.HOPS) {
                result = (int)(a.hops - b.hops);
            } else if (sortColumn == SortableColumns.NOTE) {
                String noteA = ((a.note != null ? a.note : "") + " " + (a.personalNote != null ? a.personalNote : "")).trim();
                String noteB = ((b.note != null ? b.note : "") + " " + (b.personalNote != null ? b.personalNote : "")).trim();
                if (noteA.equals("") && !noteB.equals("")) {
                    result = 1;
                } else if (noteB.equals("") && !noteA.equals("")) {
                    result = -1;
                } else {
                    result = noteA.compareTo(noteB);
                }
            }

            if (result == 0 && sortColumn != SortableColumns.NAME) {
                result = HelperFunctions.getServerName(a, "").compareTo(HelperFunctions.getServerName(b, ""));
            }

            if (result == 0 && sortColumn != SortableColumns.PK) {
                result = (a.pk != null ? a.pk : "").compareTo((b.pk != null ? b.pk : ""));
            }

            boolean mustSortInverse = sortInverse.get(getCurrentListTypeIntVal());

            if (mustSortInverse) {
                result *= -1;
            }

            return result;
        };

        Collections.sort(filteredData, comparator);

        this.notifyDataSetChanged();
    }

    private int getCurrentListTypeIntVal() {
        if (listType == ServerLists.Public) {
            return 0;
        } else if (listType == ServerLists.History) {
            return 1;
        } else if (listType == ServerLists.Favorites) {
            return 2;
        }

        return 3;
    }

    public void setVpnServerListEventListener(VpnServerListEventListener listener) {
        listEventListener = listener;
    }

    @Override
    public int getItemViewType(int position) {
        if (position == 0) {
            return 0;
        } else if (position == 1) {
            return 1;
        } else if (position == 2 && showingRows) {
            return 3;
        }

        return 2;
    }

    @NonNull
    @Override
    public ListViewHolder<View> onCreateViewHolder(@NonNull ViewGroup parent, int viewType) {
        if (viewType == 0) {
            listOptionsView = new ServerListOptions(context);
            listOptionsView.setClickWithIndexEventListener(this);
            listOptionsView.selectCorrectTab(listType);
            return new ListViewHolder<>(listOptionsView);
        } else if (viewType == 1) {
            conditionsView = new ConditionsList(context);
            conditionsView.setConditions(sortBy.get(getCurrentListTypeIntVal()), sortInverse.get(getCurrentListTypeIntVal()), filters.get(getCurrentListTypeIntVal()));

            conditionsView.setClickEventListener(v -> {
                if (conditionsView.showingFilters() && conditionsView.showingOrder()) {
                    ArrayList<OptionsItem.SelectableOption> options = new ArrayList();

                    OptionsItem.SelectableOption option = new OptionsItem.SelectableOption();
                    option.translatableLabelId = R.string.tmp_select_server_remove_filters_button;
                    options.add(option);

                    option = new OptionsItem.SelectableOption();
                    option.translatableLabelId = R.string.tmp_select_server_remove_custom_sorting_button;
                    options.add(option);

                    option = new OptionsItem.SelectableOption();
                    option.translatableLabelId = R.string.tmp_select_server_remove_both_button;
                    options.add(option);

                    OptionsModalWindow modal = new OptionsModalWindow(context, null, options, (int selectedOption) -> {
                        if (selectedOption == 0 || selectedOption == 2) {
                            filters.set(getCurrentListTypeIntVal(), null);
                        }
                        if (selectedOption == 1 || selectedOption == 2) {
                            sortBy.set(getCurrentListTypeIntVal(), SortableColumns.AUTOMATIC);
                            sortInverse.set(getCurrentListTypeIntVal(), false);
                        }

                        processData();
                    });

                    modal.show();
                } else if (conditionsView.showingFilters()) {
                    filters.set(getCurrentListTypeIntVal(), null);
                    processData();
                } else if (conditionsView.showingOrder()) {
                    sortBy.set(getCurrentListTypeIntVal(), SortableColumns.AUTOMATIC);
                    sortInverse.set(getCurrentListTypeIntVal(), false);
                    processData();
                }
            });

            return new ListViewHolder<>(conditionsView);
        } else if (viewType == 3) {
            tableHeader = new ServerListTableHeader(context);
            tableHeader.setListType(listType);
            return new ListViewHolder<>(tableHeader);
        }

        if (!showingRows) {
            ServerListButton view;
            if (lastUsedPremadeButtonIdex < premadeButtons.size()) {
                view = premadeButtons.get(lastUsedPremadeButtonIdex);
                lastUsedPremadeButtonIdex += 1;
            } else {
                view = createNewServerButton();
            }

            return new ListViewHolder<>(view);
        } else {
            ServerListTableRow view;
            if (lastUsedPremadeButtonIdex < premadeRows.size()) {
                view = premadeRows.get(lastUsedPremadeButtonIdex);
                lastUsedPremadeButtonIdex += 1;
            } else {
                view = createNewServerRow();
            }

            return new ListViewHolder<>(view);
        }
    }

    private ServerListButton createNewServerButton() {
        ServerListButton view = new ServerListButton(context);
        view.setClickWithIndexEventListener(this);
        return view;
    }

    private ServerListTableRow createNewServerRow() {
        ServerListTableRow view = new ServerListTableRow(context);
        view.setClickWithIndexEventListener(this);
        return view;
    }

    @Override
    public void onBindViewHolder(@NonNull ListViewHolder<View> holder, int position) {
        if (position >= initialServerIndex) {
            position -= initialServerIndex;

            if (!showingRows) {
                ((ServerListButton) holder.itemView).setIndex(position);
                ((ServerListButton) holder.itemView).changeData(filteredData.get(position), listType);

                if (filteredData.size() == 1) {
                    ((ServerListButton) holder.itemView).setBoxRowType(BoxRowTypes.SINGLE);
                } else if (position == 0) {
                    ((ServerListButton) holder.itemView).setBoxRowType(BoxRowTypes.TOP);
                } else if (position == filteredData.size() - 1) {
                    ((ServerListButton) holder.itemView).setBoxRowType(BoxRowTypes.BOTTOM);
                } else {
                    ((ServerListButton) holder.itemView).setBoxRowType(BoxRowTypes.MIDDLE);
                }
            } else {
                ((ServerListTableRow) holder.itemView).setIndex(position);
                ((ServerListTableRow) holder.itemView).changeData(filteredData.get(position), listType);

                if (position == filteredData.size() - 1) {
                    ((ServerListTableRow) holder.itemView).setBoxRowType(BoxRowTypes.BOTTOM);
                } else {
                    ((ServerListTableRow) holder.itemView).setBoxRowType(BoxRowTypes.MIDDLE);
                }
            }
        }
    }

    @Override
    public int getItemCount() {
        if (!showingRows) {
            return filteredData != null ? (filteredData.size() + 2) : 2;
        }

        if (filteredData == null || filteredData.size() == 0) {
            return 2;
        }
        return filteredData.size() + 3;
    }

    @Override
    public void onClickWithIndex(int index, Void data) {
        if (listEventListener != null) {
            if (index >= 0) {
                listEventListener.onVpnServerSelected(this.filteredData.get(index));
            } else {
                if (index <= ServerListOptions.showPublicIndex) {
                    if (index == ServerListOptions.showPublicIndex) {
                        listEventListener.tabChangeRequested(ServerLists.Public);
                    } else if (index == ServerListOptions.showHistoryIndex) {
                        listEventListener.tabChangeRequested(ServerLists.History);
                    } else if (index == ServerListOptions.showFavoritesIndex) {
                        listEventListener.tabChangeRequested(ServerLists.Favorites);
                    } else if (index == ServerListOptions.showBlockedIndex) {
                        listEventListener.tabChangeRequested(ServerLists.Blocked);
                    }
                } else if (index == ServerListOptions.sortIndex) {
                    SortableColumns currentSortBy = sortBy.get(getCurrentListTypeIntVal());
                    boolean currentSortInverse = sortInverse.get(getCurrentListTypeIntVal());

                    ArrayList<SortableColumns> optionValues = new ArrayList();
                    if (listType == ServerLists.History) {
                        optionValues.add(SortableColumns.DATE);
                    }
                    optionValues.add(SortableColumns.COUNTRY);
                    optionValues.add(SortableColumns.LOCATION);
                    optionValues.add(SortableColumns.PK);
                    if (listType == ServerLists.Public) {
                        optionValues.add(SortableColumns.CONGESTION);
                        optionValues.add(SortableColumns.CONGESTION_RATING);
                        optionValues.add(SortableColumns.LATENCY);
                        optionValues.add(SortableColumns.LATENCY_RATING);
                        optionValues.add(SortableColumns.HOPS);
                    }
                    optionValues.add(SortableColumns.NOTE);

                    ArrayList<OptionsItem.SelectableOption> options = new ArrayList();
                    OptionsItem.SelectableOption option = new OptionsItem.SelectableOption();
                    option.translatableLabelId = R.string.tmp_select_server_automatic_label;
                    if (currentSortBy == SortableColumns.AUTOMATIC) {
                        option.icon = "\ue876";
                    }
                    options.add(option);

                    for(int i = 0; i < optionValues.size(); i++) {
                        option = new OptionsItem.SelectableOption();
                        option.translatableLabelId = SortableColumns.getColumnNameId(optionValues.get(i));
                        if (optionValues.get(i) == currentSortBy && !currentSortInverse) {
                            option.icon = "\ue876";
                        }
                        options.add(option);

                        option = new OptionsItem.SelectableOption();
                        option.label = context.getText(SortableColumns.getColumnNameId(optionValues.get(i))) + " " + context.getText(R.string.tmp_select_server_reversed_suffix);
                        if (optionValues.get(i) == currentSortBy && currentSortInverse) {
                            option.icon = "\ue876";
                        }
                        options.add(option);
                    }

                    OptionsModalWindow modal = new OptionsModalWindow(context, context.getString(R.string.tmp_select_server_sort_title), options, (int selectedOption) -> {
                        if (selectedOption == 0) {
                            sortBy.set(getCurrentListTypeIntVal(), SortableColumns.AUTOMATIC);
                            sortInverse.set(getCurrentListTypeIntVal(), false);
                        } else {
                            selectedOption -= 1;
                            sortBy.set(getCurrentListTypeIntVal(), optionValues.get((int)(selectedOption / 2)));
                            sortInverse.set(getCurrentListTypeIntVal(), selectedOption % 2 != 0);
                        }

                        sortList();
                    });

                    modal.show();
                } else if (index == ServerListOptions.addIndex) {
                    if (VPNCoordinator.getInstance().isServiceRunning()) {
                        HelperFunctions.showToast(context.getText(R.string.tmp_select_server_running_error).toString(), true);
                        return;
                    }

                    ManualServerModalWindow modal = new ManualServerModalWindow(context, server -> listEventListener.onManualEntered(server));
                    modal.show();
                } else if (index == ServerListOptions.filterIndex) {
                    HashSet<String> countries = new HashSet<>();
                    for (VpnServerForList element : this.data) {
                        countries.add(element.countryCode);
                    }

                    FilterModalWindow modal = new FilterModalWindow(context, countries, filters.get(getCurrentListTypeIntVal()), newFilters -> {
                        filters.set(getCurrentListTypeIntVal(), newFilters);
                        processData();
                    });
                    modal.show();
                }
            }
        }
    }
}
