/*
 * Copyright 2015 The Go Authors. All rights reserved.
 * Use of this source code is governed by a BSD-style
 * license that can be found in the LICENSE file.
 */

package com.skywire.skycoin.vpn;

import android.app.Activity;
import android.content.Intent;
import android.net.VpnService;
import android.os.Bundle;
import android.os.Handler;
import android.os.Looper;
import android.os.Message;
import android.util.Log;
import android.view.View;
import android.widget.Button;
import android.widget.EditText;
import android.widget.TextView;
import android.widget.Toast;

import skywiremob.Skywiremob;

public class MainActivity extends Activity implements Handler.Callback {

    private EditText mRemotePK;
    private EditText mPasscode;
    private Button mStart;
    private Button mStop;

    private final Object visorMx = new Object();
    private VisorRunnable visor = null;

    @Override
    public boolean handleMessage(Message msg) {
        String err = msg.getData().getString("text");
        showToast(err);
        return false;
    }

    public void showToast(String text) {
        Toast toast = Toast.makeText(getApplicationContext(), text, Toast.LENGTH_SHORT);
        toast.show();
    }

    public void startVPNService() {
        Intent intent = VpnService.prepare(MainActivity.this);
        if (intent != null) {
            startActivityForResult(intent, 0);
        } else {
            onActivityResult(0, RESULT_OK, null);
        }
    }

    @Override
    protected void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        setContentView(R.layout.activity_main);
        mRemotePK = (EditText) findViewById(R.id.editTextRemotePK);
        mPasscode = (EditText) findViewById(R.id.editTextPasscode);
        mStart = (Button) findViewById(R.id.buttonStart);
        mStop = (Button)findViewById(R.id.buttonStop);

        mStart.setOnClickListener(new View.OnClickListener() {
            @Override
            public void onClick(View v) {
                String remotePK = mRemotePK.getText().toString();
                String passcode = mPasscode.getText().toString();

                skywiremob.Error err = Skywiremob.isPKValid(remotePK);
                if (err.getCode() != Skywiremob.ErrCodeNoError) {
                    Toast toast = Toast.makeText(getApplicationContext(),
                            "Invalid credentials" , Toast.LENGTH_SHORT);
                    toast.show();
                    return;
                } else {
                    Skywiremob.printString("PK is correct");
                }

                synchronized (visorMx) {
                    if (visor != null) {
                        visor.stopVisor();
                        visor = null;
                        stopService(getServiceIntent().setAction(SkywireVPNService.ACTION_DISCONNECT));
                    }

                    visor = new VisorRunnable(getApplicationContext(), MainActivity.this,
                            remotePK, passcode);

                    new Thread(visor).start();
                }
            }
        });

        mStop.setOnClickListener(new View.OnClickListener() {
            @Override
            public void onClick(View v) {
                startService(getServiceIntent().setAction(SkywireVPNService.ACTION_DISCONNECT));

                synchronized (visorMx) {
                    if (visor != null) {
                        visor.stopVisor();
                    }
                }
            }
        });
    }

    @Override
    protected void onActivityResult(int request, int result, Intent data) {
        if (result == RESULT_OK) {
            startService(getServiceIntent().setAction(SkywireVPNService.ACTION_CONNECT));
        }
    }

    private Intent getServiceIntent() {
        return new Intent(this, SkywireVPNService.class);
    }
}
