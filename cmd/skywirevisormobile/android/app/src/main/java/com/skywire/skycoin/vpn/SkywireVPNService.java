package com.skywire.skycoin.vpn;;

import android.app.Notification;
import android.app.NotificationChannel;
import android.app.NotificationManager;
import android.app.PendingIntent;
import android.content.Intent;
import android.content.SharedPreferences;
import android.net.VpnService;
import android.os.Handler;
import android.os.Message;
import android.os.ParcelFileDescriptor;
import android.util.Pair;
import android.widget.Toast;

import java.io.IOException;
import java.util.Collections;
import java.util.Set;
import java.util.concurrent.atomic.AtomicInteger;
import java.util.concurrent.atomic.AtomicReference;

import skywiremob.Skywiremob;

public class SkywireVPNService extends VpnService implements Handler.Callback {
    public static final String ACTION_CONNECT = "com.skywire.android.vpn.START";
    public static final String ACTION_DISCONNECT = "com.skywire.android.vpn.STOP";
    private SkywireVPNConnection connectionRunnable;

    private static final String TAG = SkywireVPNService.class.getSimpleName();
    private Handler mHandler;
    private static class Connection extends Pair<Thread, ParcelFileDescriptor> {
        public Connection(Thread thread, ParcelFileDescriptor pfd) {
            super(thread, pfd);
        }
    }
    private final AtomicReference<Thread> mConnectingThread = new AtomicReference<>();
    private final AtomicReference<Connection> mConnection = new AtomicReference<>();
    private AtomicInteger mNextConnectionId = new AtomicInteger(1);
    private PendingIntent mConfigureIntent;

    @Override
    public void onCreate() {
        // The handler is only used to show messages.
        if (mHandler == null) {
            mHandler = new Handler(this);
        }
        // Create the intent to "configure" the connection (just start SkywireVPNClient).
        mConfigureIntent = PendingIntent.getActivity(this, 0, new Intent(this, MainActivity.class),
                PendingIntent.FLAG_UPDATE_CURRENT);
    }

    @Override
    public int onStartCommand(Intent intent, int flags, int startId) {
        if (intent != null && ACTION_DISCONNECT.equals(intent.getAction())) {
            Skywiremob.printString("STOPPING ANDROID VPN SERVICE");
            disconnect();
            return START_NOT_STICKY;
        } else {
            Skywiremob.printString("STARTING ANDROID VPN SERVICE");
            connect();
            return START_STICKY;
        }
    }

    @Override
    public void onDestroy() {
        disconnect();
    }

    @Override
    public boolean handleMessage(Message message) {
        Toast.makeText(this, message.what, Toast.LENGTH_SHORT).show();
        if (message.what != R.string.disconnected) {
            updateForegroundNotification(message.what);
        }
        return true;
    }

    private void connect() {
        // Become a foreground service. Background services can be VPN services too, but they can
        // be killed by background check before getting a chance to receive onRevoke().
        updateForegroundNotification(R.string.connecting);
        mHandler.sendEmptyMessage(R.string.connecting);

        try {
            while (!Skywiremob.isVPNReady()) {
                Skywiremob.printString("VPN STILL NOT READY, WAITING...");
                Thread.sleep(1000);
            }
        } catch (Exception e) {
            Skywiremob.printString(e.getMessage());
        }

        Skywiremob.printString("VPN IS READY, LET'S TRY IT OUT");

        startConnection(new SkywireVPNConnection(
                this, mNextConnectionId.getAndIncrement(), "localhost", 7890));
    }

    private void startConnection(final SkywireVPNConnection connection) {
        this.connectionRunnable = connection;
        // Replace any existing connecting thread with the  new one.
        final Thread thread = new Thread(connection, "SkywireVPNThread");
        setConnectingThread(thread);
        // Handler to mark as connected once onEstablish is called.
        connection.setConfigureIntent(mConfigureIntent);
        connection.setOnEstablishListener(tunInterface -> {
            mHandler.sendEmptyMessage(R.string.connected);
            mConnectingThread.compareAndSet(thread, null);
            setConnection(new Connection(thread, tunInterface));
        });
        thread.start();
    }

    private void setConnectingThread(final Thread thread) {
        final Thread oldThread = mConnectingThread.getAndSet(thread);
        if (oldThread != null) {
            oldThread.interrupt();
        }
    }
    private void setConnection(final Connection connection) {
        final Connection oldConnection = mConnection.getAndSet(connection);
        if (oldConnection != null) {
            try {
                oldConnection.first.interrupt();
                oldConnection.second.close();
            } catch (IOException e) {
                Skywiremob.printString(TAG + " Closing VPN interface " + e.getMessage());
            }
        }
    }
    private void disconnect() {
        mHandler.sendEmptyMessage(R.string.disconnected);
        if (connectionRunnable != null) {
            connectionRunnable.Stop();
        }
        setConnectingThread(null);
        setConnection(null);
        stopForeground(true);
    }
    private void updateForegroundNotification(final int message) {
        final String NOTIFICATION_CHANNEL_ID = "SkywireVPN";
        NotificationManager mNotificationManager = (NotificationManager) getSystemService(
                NOTIFICATION_SERVICE);
        mNotificationManager.createNotificationChannel(new NotificationChannel(
                NOTIFICATION_CHANNEL_ID, NOTIFICATION_CHANNEL_ID,
                NotificationManager.IMPORTANCE_DEFAULT));
        startForeground(1, new Notification.Builder(this, NOTIFICATION_CHANNEL_ID)
                .setSmallIcon(R.drawable.ic_vpn)
                .setContentText(getString(message))
                .setContentIntent(mConfigureIntent)
                .build());
    }
}
