package com.skywire.skycoin.vpn.activities.start.connected;

import android.content.Context;
import android.util.AttributeSet;
import android.view.LayoutInflater;
import android.widget.FrameLayout;
import android.widget.TextView;

import com.github.mikephil.charting.charts.LineChart;
import com.github.mikephil.charting.data.Entry;
import com.github.mikephil.charting.data.LineData;
import com.github.mikephil.charting.data.LineDataSet;
import com.github.mikephil.charting.interfaces.datasets.ILineDataSet;
import com.skywire.skycoin.vpn.R;
import com.skywire.skycoin.vpn.helpers.Globals;
import com.skywire.skycoin.vpn.helpers.HelperFunctions;
import com.skywire.skycoin.vpn.vpn.VPNGeneralPersistentData;

import java.io.Closeable;
import java.util.ArrayList;

import io.reactivex.rxjava3.disposables.Disposable;

public class Chart extends FrameLayout implements Closeable {
    public Chart(Context context) {
        super(context);
        Initialize(context, null);
    }
    public Chart(Context context, AttributeSet attrs) {
        super(context, attrs);
        Initialize(context, attrs);
    }
    public Chart(Context context, AttributeSet attrs, int defStyle) {
        super(context, attrs, defStyle);
        Initialize(context, attrs);
    }

    private LineChart chart;
    private FrameLayout chartContainer;
    private TextView textMin;
    private TextView textMid;
    private TextView textMax;

    private Globals.DataUnits dataUnits = VPNGeneralPersistentData.getDataUnits();
    private ArrayList<Long> lastData;
    private boolean showingMs;

    private Disposable dataUnitsSubscription;

    protected void Initialize (Context context, AttributeSet attrs) {
        LayoutInflater inflater = (LayoutInflater) context.getSystemService(Context.LAYOUT_INFLATER_SERVICE);
        inflater.inflate(R.layout.view_start_chart, this, true);

        chart = findViewById(R.id.chart);
        chartContainer = findViewById(R.id.chartContainer);
        textMin = findViewById(R.id.textMin);
        textMid = findViewById(R.id.textMid);
        textMax = findViewById(R.id.textMax);

        chartContainer.setClipToOutline(true);

        chart.getDescription().setEnabled(false);
        chart.getLegend().setEnabled(false);
        chart.setDrawGridBackground(false);
        chart.getXAxis().setEnabled(false);
        chart.getAxisLeft().setEnabled(false);
        chart.getAxisRight().setEnabled(false);

        chart.setViewPortOffsets(0f, 0f, 0f, 0f);
        chart.getAxisLeft().setAxisMinimum(0);
        chart.getAxisLeft().setSpaceTop(0);
        chart.getAxisLeft().setSpaceBottom(0);

        chart.setScaleEnabled(false);
        chart.setTouchEnabled(false);

        dataUnitsSubscription = VPNGeneralPersistentData.getDataUnitsObservable().subscribe(response -> {
            dataUnits = response;

            if (lastData != null) {
                setData(lastData, showingMs);
            }
        });
    }

    public void setData(ArrayList<Long> data, boolean showingMs) {
        this.lastData = data;
        this.showingMs = showingMs;

        ArrayList<Entry> values = new ArrayList<>();

        double max = 0;
        for (int i = 0; i < data.size(); i++) {
            double val = (float)data.get(i);
            values.add(new Entry(i, (float)val));

            if (val > max) {
                max = val;
            }
        }

        if (max == 0) {
            max = 1;
        }

        double mid = max / 2;

        if (chart.getAxisLeft().getAxisMaximum() != max) {
            chart.getAxisLeft().setAxisMaximum((float)max);

            if (showingMs) {
                textMax.setText(HelperFunctions.getLatencyValue(max));
                textMid.setText(HelperFunctions.getLatencyValue(mid));
                textMin.setText(HelperFunctions.getLatencyValue(0));
            } else {
                textMax.setText(HelperFunctions.computeDataAmountString(max, true, dataUnits != Globals.DataUnits.OnlyBytes));
                textMid.setText(HelperFunctions.computeDataAmountString(mid, true, dataUnits != Globals.DataUnits.OnlyBytes));
                textMin.setText(HelperFunctions.computeDataAmountString(0, true, dataUnits != Globals.DataUnits.OnlyBytes));
            }
        }

        LineDataSet dataSet;
        if (chart.getData() != null && chart.getData().getDataSetCount() > 0) {
            dataSet = (LineDataSet) chart.getData().getDataSetByIndex(0);
            dataSet.setValues(values);
            dataSet.notifyDataSetChanged();
            chart.getData().notifyDataChanged();
            chart.notifyDataSetChanged();
            chart.invalidate();
        } else {
            dataSet = new LineDataSet(values, "");
            dataSet.setDrawIcons(false);
            dataSet.setDrawValues(false);
            dataSet.setDrawCircleHole(false);
            dataSet.setDrawCircles(false);

            dataSet.setMode(LineDataSet.Mode.HORIZONTAL_BEZIER);

            dataSet.setColor(0x59000000);
            dataSet.setLineWidth(0f);

            dataSet.setDrawFilled(true);
            dataSet.setFillColor(0x00000000);
            dataSet.setFillAlpha(255);

            ArrayList<ILineDataSet> dataSets = new ArrayList<>();
            dataSets.add(dataSet);
            LineData lineData = new LineData(dataSets);

            chart.setData(lineData);
        }
    }

    @Override
    public void close() {
        dataUnitsSubscription.dispose();
    }
}
