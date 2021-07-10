package com.skywire.skycoin.vpn.activities.servers;

import android.app.Dialog;
import android.content.Context;
import android.os.Bundle;
import android.view.KeyEvent;
import android.view.View;
import android.view.Window;
import android.view.inputmethod.EditorInfo;
import android.widget.EditText;

import com.skywire.skycoin.vpn.R;
import com.skywire.skycoin.vpn.controls.ModalWindowButton;
import com.skywire.skycoin.vpn.controls.Select;
import com.skywire.skycoin.vpn.extensible.ClickEvent;
import com.skywire.skycoin.vpn.helpers.CountriesList;
import com.skywire.skycoin.vpn.helpers.HelperFunctions;

import java.util.ArrayList;
import java.util.Collections;
import java.util.Comparator;
import java.util.HashMap;
import java.util.HashSet;

public class FilterModalWindow extends Dialog implements ClickEvent {
    public static class Filters {
        public String countryCode;
        public String name;
        public String location;
        public String pk;
        public String note;
    }

    public interface Confirmed {
        void confirmed(Filters filters);
    }

    private Select selectCountry;
    private EditText editName;
    private EditText editLocation;
    private EditText editPk;
    private EditText editNote;
    private ModalWindowButton buttonCancel;
    private ModalWindowButton buttonConfirm;

    private HashSet<String> countries;
    private Filters currentFilters;
    private Confirmed event;

    public FilterModalWindow(Context ctx, HashSet<String> countries, Filters currentFilters, Confirmed event) {
        super(ctx);

        this.countries = countries;
        this.currentFilters = currentFilters;
        this.event = event;
    }

    @Override
    protected void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        requestWindowFeature(Window.FEATURE_NO_TITLE);
        setContentView(R.layout.view_server_filters_modal);

        selectCountry = findViewById(R.id.selectCountry);
        editName = findViewById(R.id.editName);
        editLocation = findViewById(R.id.editLocation);
        editPk = findViewById(R.id.editPk);
        editNote = findViewById(R.id.editNote);
        buttonCancel = findViewById(R.id.buttonCancel);
        buttonConfirm = findViewById(R.id.buttonConfirm);

        ArrayList<Select.SelectOption> countryOptions = new ArrayList<>();
        Select.SelectOption option = new Select.SelectOption();
        option.text = getContext().getString(R.string.filter_server_any_country_option);
        countryOptions.add(option);

        Comparator<String> comparator = (a, b) -> a.compareTo(b);
        ArrayList<String> countriesList = new ArrayList<>(countries);
        Collections.sort(countriesList, comparator);

        int i = 1;
        HashMap<String, Integer> countryIndexMap = new HashMap<>();
        for (String countryCode : countriesList) {
            countryCode = countryCode.toLowerCase();
            option = new Select.SelectOption();
            option.text = CountriesList.getCountryName(countryCode);
            option.value = countryCode;
            option.iconId = HelperFunctions.getFlagResourceId(countryCode);
            countryOptions.add(option);

            countryIndexMap.put(countryCode, i);
            i++;
        }

        if (currentFilters != null) {
            editName.setText(currentFilters.name);
            editLocation.setText(currentFilters.location);
            editPk.setText(currentFilters.pk);
            editNote.setText(currentFilters.note);
        }

        editName.setSelection(editName.getText().length());

        if (currentFilters != null && currentFilters.countryCode != null) {
            int selectedIndex = countryIndexMap.containsKey(currentFilters.countryCode) ? countryIndexMap.get(currentFilters.countryCode) : 0;
            selectCountry.setValues(countryOptions, selectedIndex);
        } else {
            selectCountry.setValues(countryOptions, 0);
        }

        editName.setImeOptions(EditorInfo.IME_ACTION_NEXT);
        editLocation.setImeOptions(EditorInfo.IME_ACTION_NEXT);
        editPk.setImeOptions(EditorInfo.IME_ACTION_NEXT);
        editNote.setImeOptions(EditorInfo.IME_ACTION_DONE);

        editNote.setOnEditorActionListener((v, actionId, event) -> {
            if (
                actionId == EditorInfo.IME_ACTION_DONE ||
                (event != null && event.getAction() == KeyEvent.ACTION_DOWN && event.getKeyCode() == KeyEvent.KEYCODE_ENTER)
            ) {
                process();
                dismiss();

                return true;
            }

            return false;
        });

        buttonCancel.setClickEventListener(this);
        buttonConfirm.setClickEventListener(this);

        HelperFunctions.configureModalWindow(this);
    }

    @Override
    public void onClick(View view) {
        if (view.getId() == R.id.buttonConfirm) {
            process();
        }

        dismiss();
    }

    private void process() {
        if (event != null) {
            Filters filters = new Filters();

            filters.countryCode = selectCountry.getSelectedValue();

            if (editName.getText() != null && !editName.getText().toString().trim().equals("")) {
                filters.name = editName.getText().toString().trim();
            }
            if (editLocation.getText() != null && !editLocation.getText().toString().trim().equals("")) {
                filters.location = editLocation.getText().toString().trim();
            }
            if (editPk.getText() != null && !editPk.getText().toString().trim().equals("")) {
                filters.pk = editPk.getText().toString().trim();
            }
            if (editNote.getText() != null && !editNote.getText().toString().trim().equals("")) {
                filters.note = editNote.getText().toString().trim();
            }

            event.confirmed(filters);
        }
    }
}
