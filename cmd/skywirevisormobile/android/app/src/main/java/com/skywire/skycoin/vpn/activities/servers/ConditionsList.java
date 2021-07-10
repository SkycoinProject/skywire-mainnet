package com.skywire.skycoin.vpn.activities.servers;

import android.content.Context;
import android.util.AttributeSet;
import android.view.LayoutInflater;
import android.view.MotionEvent;
import android.view.View;
import android.widget.FrameLayout;
import android.widget.LinearLayout;
import android.widget.TextView;

import com.skywire.skycoin.vpn.R;
import com.skywire.skycoin.vpn.extensible.ButtonBase;
import com.skywire.skycoin.vpn.helpers.CountriesList;

public class ConditionsList extends ButtonBase implements View.OnTouchListener {
    private FrameLayout mainContainer;
    private LinearLayout filtersContainer;
    private LinearLayout orderContainer;
    private TextView textFilters;
    private TextView textOrder;

    public ConditionsList(Context context) {
        super(context);
    }
    public ConditionsList(Context context, AttributeSet attrs) {
        super(context, attrs);
    }
    public ConditionsList(Context context, AttributeSet attrs, int defStyle) {
        super(context, attrs, defStyle);
    }

    @Override
    protected void Initialize (Context context, AttributeSet attrs) {
        LayoutInflater inflater = (LayoutInflater)context.getSystemService (Context.LAYOUT_INFLATER_SERVICE);
        inflater.inflate(R.layout.view_server_list_condition_list, this, true);

        mainContainer = this.findViewById (R.id.mainContainer);
        filtersContainer = this.findViewById (R.id.filtersContainer);
        orderContainer = this.findViewById (R.id.orderContainer);
        textFilters = this.findViewById (R.id.textFilters);
        textOrder = this.findViewById (R.id.textOrder);

        mainContainer.setVisibility(GONE);

        setOnTouchListener(this);
        setViewForCheckingClicks(this);
    }

    public void setConditions(VpnServersAdapter.SortableColumns column, boolean sortingReversed, FilterModalWindow.Filters filters) {
        if (filters == null && column == VpnServersAdapter.SortableColumns.AUTOMATIC) {
            mainContainer.setVisibility(GONE);
        } else {
            boolean showingValues = false;

            if (filters != null) {
                String filterList = "";
                if (filters.countryCode != null && !filters.countryCode.equals("")) {
                    filterList += getContext().getText(R.string.filter_server_country_label) + " \"" + CountriesList.getCountryName(filters.countryCode) + "\"";
                }

                if (filters.name != null && !filters.name.equals("")) {
                    if (filterList.length() > 0) {
                        filterList += " / ";
                    }

                    filterList += getContext().getText(R.string.filter_server_name_label) + " \"" + filters.name + "\"";
                }

                if (filters.location != null && !filters.location.equals("")) {
                    if (filterList.length() > 0) {
                        filterList += " / ";
                    }

                    filterList += getContext().getText(R.string.filter_server_location_label) + " \"" + filters.location + "\"";
                }

                if (filters.pk != null && !filters.pk.equals("")) {
                    if (filterList.length() > 0) {
                        filterList += " / ";
                    }

                    filterList += getContext().getText(R.string.filter_server_public_key_label) + " \"" + filters.pk + "\"";
                }

                if (filters.note != null && !filters.note.equals("")) {
                    if (filterList.length() > 0) {
                        filterList += " / ";
                    }

                    filterList += getContext().getText(R.string.filter_server_note_label) + " \"" + filters.note + "\"";
                }

                if (filterList.length() > 0) {
                    filtersContainer.setVisibility(VISIBLE);
                    textFilters.setText(filterList);

                    showingValues = true;
                } else {
                    filtersContainer.setVisibility(GONE);
                }
            } else {
                filtersContainer.setVisibility(GONE);
            }

            if (column != VpnServersAdapter.SortableColumns.AUTOMATIC) {
                String columnName = getContext().getText(VpnServersAdapter.SortableColumns.getColumnNameId(column)).toString();

                if (sortingReversed) {
                    columnName += " " + getContext().getText(R.string.tmp_select_server_reversed_suffix);
                }

                orderContainer.setVisibility(VISIBLE);
                textOrder.setText(getContext().getText(R.string.tmp_select_server_sorted_by_prefix) + " \"" + columnName + "\"");

                showingValues = true;
            } else {
                orderContainer.setVisibility(GONE);
            }

            if (showingValues) {
                mainContainer.setVisibility(VISIBLE);
            } else {
                mainContainer.setVisibility(GONE);
            }
        }
    }

    public boolean showingFilters() {
        return mainContainer.getVisibility() != GONE && filtersContainer.getVisibility() != GONE;
    }

    public boolean showingOrder() {
        return mainContainer.getVisibility() != GONE && orderContainer.getVisibility() != GONE;
    }

    @Override
    public boolean onTouch(View v, MotionEvent event) {
        if (event.getAction() == MotionEvent.ACTION_DOWN) {
            setAlpha(0.5f);
        } else if (event.getAction() == MotionEvent.ACTION_CANCEL || event.getAction() == MotionEvent.ACTION_POINTER_UP || event.getAction() == MotionEvent.ACTION_UP) {
            setAlpha(1f);
        }

        return false;
    }
}
