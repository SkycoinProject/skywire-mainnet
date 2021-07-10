package com.skywire.skycoin.vpn.activities.apps;

import android.os.Bundle;

import androidx.appcompat.app.AppCompatActivity;
import androidx.recyclerview.widget.LinearLayoutManager;
import androidx.recyclerview.widget.RecyclerView;

import com.skywire.skycoin.vpn.R;
import com.skywire.skycoin.vpn.helpers.HelperFunctions;

public class AppsActivity extends AppCompatActivity implements AppsAdapter.AppListChangedListener {
    public static final String READ_ONLY_EXTRA = "ReadOnly";

    private RecyclerView recycler;

    private boolean readOnly;

    @Override
    protected void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        setContentView(R.layout.activity_app_list);

        recycler = findViewById(R.id.recycler);

        readOnly = getIntent().getBooleanExtra(READ_ONLY_EXTRA, false);

        LinearLayoutManager layoutManager = new LinearLayoutManager(this);
        recycler.setLayoutManager(layoutManager);
        // This could be useful in the future.
        // recycler.setHasFixedSize(true);

        AppsAdapter adapter = new AppsAdapter(this, readOnly);
        adapter.setAppListChangedEventListener(this);
        recycler.setAdapter(adapter);
    }

    @Override
    protected void onResume() {
        super.onResume();
        if (!readOnly) {
            HelperFunctions.closeActivityIfServiceRunning(this);
        }
    }

    @Override
    public boolean onAppListChanged() {
        return !HelperFunctions.closeActivityIfServiceRunning(this);
    }
}
