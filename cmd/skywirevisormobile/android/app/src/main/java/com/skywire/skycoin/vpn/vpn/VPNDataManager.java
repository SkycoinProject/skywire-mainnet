package com.skywire.skycoin.vpn.vpn;

import java.io.FileInputStream;
import java.io.FileOutputStream;
import java.io.InterruptedIOException;
import java.nio.ByteBuffer;
import java.nio.channels.DatagramChannel;

import io.reactivex.rxjava3.core.Observable;
import io.reactivex.rxjava3.core.ObservableOnSubscribe;

/**
 * Helper class for creating an observable for sending or getting data to or from the visor.
 */
public class VPNDataManager {
    /**
     * Creates an observable for sending or getting data to or from the visor.
     * @param vpnInterface Interface currently used for the VPN connection.
     * @param tunnel Socket for communicating with the visor.
     * @param forSending True if the observable will be used for sending the data from the OS to the
     *                   visor, false if it is for sending the data from the visor to the OS.
     */
    static public Observable<Integer> createObservable(VPNWorkInterface vpnInterface, DatagramChannel tunnel, boolean forSending) {
        return Observable.create((ObservableOnSubscribe<Integer>) emitter -> {
            // Streams for receiving and sending packages.
            final FileInputStream in;
            final FileOutputStream out;
            // Only the stream needed is initialized.
            if (forSending) {
                in = vpnInterface.getInputStream();
                out = null;
            } else {
                in = null;
                out = vpnInterface.getOutputStream();
            }

            ByteBuffer packet = ByteBuffer.allocate(Short.MAX_VALUE);

            // Get or send data while the emitter is still valid.
            while(!emitter.isDisposed()) {
                try {
                    if (forSending) {
                        // Read the outgoing packet from the input stream. The operation must block
                        // blocks the thread.
                        int length = in.read(packet.array());
                        if (length > 0) {
                            // Write the outgoing packet to the tunnel.
                            packet.limit(length);
                            tunnel.write(packet);
                            packet.clear();
                        }
                    }

                    if (!forSending) {
                        // Read the incoming packet from the visor socket. The operation must block
                        // blocks the thread.
                        int length = tunnel.read(packet);
                        if (length > 0) {
                            // Ignore control messages, which start with zero.
                            if (packet.get(0) != 0) {
                                // Write the incoming packet to the output stream.
                                out.write(packet.array(), 0, length);
                            }
                            packet.clear();
                        }
                    }
                } catch (InterruptedIOException e) {
                    // This error is thrown if there is a timeout while waiting data from the socket.
                    // It is ignored so that the loop repeats itself to wait for data again.
                } catch (Exception e) {
                    // Emit the error only if the emitter is still valid.
                    if (!emitter.isDisposed()) {
                        emitter.onError(e);
                        return;
                    }

                    break;
                }
            }

            // Indicate the observable finished.
            emitter.onComplete();
        });
    }
}
