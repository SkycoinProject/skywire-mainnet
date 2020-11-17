package com.skywire.skycoin.vpn;

import android.app.PendingIntent;
import android.content.pm.PackageManager;
import android.net.Network;
import android.net.ProxyInfo;
import android.net.VpnService;
import android.os.ParcelFileDescriptor;
import android.system.OsConstants;
import android.text.TextUtils;

import java.io.FileInputStream;
import java.io.FileOutputStream;
import java.io.IOException;
import java.net.InetSocketAddress;
import java.net.Socket;
import java.net.SocketAddress;
import java.net.SocketException;
import java.nio.ByteBuffer;
import java.nio.channels.DatagramChannel;
import java.util.Set;
import java.util.concurrent.TimeUnit;

import skywiremob.Skywiremob;

public class SkywireVPNConnection implements Runnable {
    /**
     * Callback interface to let the {@link SkywireVPNService} know about new connections
     * and update the foreground notification with connection status.
     */
    public interface OnEstablishListener {
        void onEstablish(ParcelFileDescriptor tunInterface);
    }
    /** Maximum packet size is constrained by the MTU, which is given as a signed short. */
    private static final int MAX_PACKET_SIZE = Short.MAX_VALUE;
    /** Time to wait in between losing the connection and retrying. */
    private static final long RECONNECT_WAIT_MS = TimeUnit.SECONDS.toMillis(3);
    /** Time between keepalives if there is no traffic at the moment.
     *
     * TODO: don't do this; it's much better to let the connection die and then reconnect when
     *       necessary instead of keeping the network hardware up for hours on end in between.
     **/
    private static final long KEEPALIVE_INTERVAL_MS = TimeUnit.SECONDS.toMillis(15);
    /** Time to wait without receiving any response before assuming the server is gone. */
    private static final long RECEIVE_TIMEOUT_MS = TimeUnit.SECONDS.toMillis(20);
    /**
     * Time between polling the VPN interface for new traffic, since it's non-blocking.
     *
     * TODO: really don't do this; a blocking read on another thread is much cleaner.
     */
    private static final long IDLE_INTERVAL_MS = TimeUnit.MILLISECONDS.toMillis(100);
    /**
     * Number of periods of length {@IDLE_INTERVAL_MS} to wait before declaring the handshake a
     * complete and abject failure.
     *
     * TODO: use a higher-level protocol; hand-rolling is a fun but pointless exercise.
     */
    private static final int MAX_HANDSHAKE_ATTEMPTS = 50;
    private final VpnService mService;
    private final int mConnectionId;
    private final String mServerName;
    private final int mServerPort;
    private PendingIntent mConfigureIntent;
    private OnEstablishListener mOnEstablishListener;
    private FromVPNClientRunnable fromVPNClientRunnable;

    private final Object StopMx = new Object();
    private boolean shouldStop = false;

    public void Stop() {
        synchronized (StopMx) {
            shouldStop = true;
        }

        fromVPNClientRunnable.Stop();
    }
    // Allowed/Disallowed packages for VPN usage
    //private final boolean mAllow;
    //private final Set<String> mPackages;
    public SkywireVPNConnection(final VpnService service, final int connectionId,
                            final String serverName, final int serverPort/*, final byte[] sharedSecret,
                            boolean allow,
                            final Set<String> packages*/) {
        mService = service;
        mConnectionId = connectionId;
        mServerName = serverName;
        mServerPort= serverPort;
        //mAllow = allow;
        //mPackages = packages;
    }
    /**
     * Optionally, set an intent to configure the VPN. This is {@code null} by default.
     */
    public void setConfigureIntent(PendingIntent intent) {
        mConfigureIntent = intent;
    }
    public void setOnEstablishListener(OnEstablishListener listener) {
        mOnEstablishListener = listener;
    }
    @Override
    public void run() {
        try {
            Skywiremob.printString(getTag() + " Starting");
            // If anything needs to be obtained using the network, get it now.
            // This greatly reduces the complexity of seamless handover, which
            // tries to recreate the tunnel without shutting down everything.
            // In this demo, all we need to know is the server address.
            final SocketAddress serverAddress = new InetSocketAddress(mServerName, mServerPort);
            // We try to create the tunnel several times.
            // TODO: The better way is to work with ConnectivityManager, trying only when the
            // network is available.
            // Here we just use a counter to keep things simple.
            for (int attempt = 0; attempt < 10; ++attempt) {
                // Reset the counter if we were connected.
                if (run(serverAddress)) {
                    attempt = 0;
                }
                // Sleep for a while. This also checks if we got interrupted.
                Thread.sleep(3000);
            }
            Skywiremob.printString(getTag() + " Giving");
        } catch (IOException | InterruptedException | IllegalArgumentException e) {
            Skywiremob.printString(getTag() + " Connection failed, exiting " + e.getMessage());
        }
    }
    private boolean run(SocketAddress server)
            throws IOException, InterruptedException, IllegalArgumentException {
        ParcelFileDescriptor iface = null;
        boolean connected = false;

        // Create a DatagramChannel as the VPN tunnel.
        try (DatagramChannel tunnel = DatagramChannel.open()) {
            // Protect the tunnel before connecting to avoid loopback.
            if (!mService.protect(tunnel.socket())) {
                throw new IllegalStateException("Cannot protect the tunnel");
            }
            for (int fd = (int)Skywiremob.nextDmsgSocket(); fd != 0; fd = (int)Skywiremob.nextDmsgSocket()) {
                Skywiremob.printString("PRINTING FD " + fd);
                if (!mService.protect(fd)) {
                    throw new IllegalStateException("Cannot protect the tunnel");
                }
            }
            // Connect to the server.
            tunnel.connect(server);

            Skywiremob.setMobileAppAddr(tunnel.getLocalAddress().toString());

            // For simplicity, we use the same thread for both reading and
            // writing. Here we put the tunnel into non-blocking mode.
            tunnel.configureBlocking(false);
            // Configure the virtual network interface.
            iface = configure();
            // Now we are connected. Set the flag.
            connected = true;
            // Packets to be sent are queued in this input stream.
            FileInputStream in = new FileInputStream(iface.getFileDescriptor());
            // Packets received need to be written to this output stream.
            FileOutputStream out = new FileOutputStream(iface.getFileDescriptor());

            this.fromVPNClientRunnable = new FromVPNClientRunnable(out, tunnel);
            new Thread(this.fromVPNClientRunnable).start();
            // Allocate the buffer for a single packet.
            ByteBuffer packet = ByteBuffer.allocate(MAX_PACKET_SIZE);
            // We keep forwarding packets till something goes wrong.
            Skywiremob.printString("Start forwarding packets on Android");
            while (true) {
                synchronized (StopMx) {
                    if (shouldStop) {
                        break;
                    }
                }

                // Assume that we did not make any progress in this iteration.
                // Read the outgoing packet from the input stream.
                int length = in.read(packet.array());
                if (length > 0) {
                    // Write the outgoing packet to the tunnel.
                    packet.limit(length);
                    tunnel.write(packet);
                    packet.clear();
                }
            }
        } catch (SocketException e) {
            Skywiremob.printString(getTag() + " Cannot use socket " + e.getMessage());
        } finally {
            if (iface != null) {
                try {
                    iface.close();
                } catch (IOException e) {
                    Skywiremob.printString(getTag() + " Unable to close interface " + e.getMessage());
                }
            }
        }
        return connected;
    }

    private ParcelFileDescriptor configure() throws IllegalArgumentException {
        // Configure a builder while parsing the parameters.
        VpnService.Builder builder = mService.new Builder();

        builder.setMtu((short)Skywiremob.getMTU());
        Skywiremob.printString("TUN IP: " + Skywiremob.tunip());
        builder.addAddress(Skywiremob.tunip(), (int)Skywiremob.getTUNIPPrefix());
        builder.allowFamily(OsConstants.AF_INET);
        builder.addDnsServer("8.8.8.8");
        //builder.addDnsServer("192.168.1.1");
        builder.addRoute("0.0.0.0", 1);
        builder.addRoute("128.0.0.0", 1);

        // Create a new interface using the builder and save the parameters.
        final ParcelFileDescriptor vpnInterface;

        builder.setSession(mServerName).setConfigureIntent(mConfigureIntent);
        synchronized (mService) {
            vpnInterface = builder.establish();
            if (mOnEstablishListener != null) {
                mOnEstablishListener.onEstablish(vpnInterface);
            }
        }
        Skywiremob.printString(getTag() + " New interface: " + vpnInterface);
        return vpnInterface;
    }
    private final String getTag() {
        return SkywireVPNConnection.class.getSimpleName() + "[" + mConnectionId + "]";
    }
}
