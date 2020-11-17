# Skywire VPN draft (Android)

## Prerequisites
This project uses Skywire mobile library (`skywiremob`) to use all the needed Skywire infrastructure. In order to build
and use this library, one needs to install `gomobile` (https://github.com/golang/mobile).

## Building and Running
To build the library you need to use `gomobile`. Main `Makefile` already contains target `build-android` which 
can be used. 
IMPORTANT: regardless of go modules and other great stuff done by the Go team, in order to 
use `gomobile` you need to put Skywire code according to `GOPATH`. Otherwise you'll get all kinds of errors.
The output file (`.aar` for Android) may be used straight from the mobile app code.

## Skywire Mobile API
- `PrintString(string)`: Logs string argument using info log level. May be useful to use it instead of standard logging features of mobile apps for debugging. 
All the output strings are prefixed with `GoLog`, so printing logs with this func one may grep all the logs both from `skywiremob` 
internal and mobile application;
- `IsPKValid(string) string`: Checks if passed pub key is valid. Returns non-empty string with error in case of failure;
- `GetMTU() int`: Gets VPN connection MTU;
- `GetTUNIPPrefix() int`: Gets netmask prefix of TUN IP address;
- `IsVPNReady() bool`: Checks whether VPN client is ready on the Go side. Once it is, the mobile application is free to start 
forwarding packets. Starts returning `true` after the `ServerVPN` call;
- `PrepareVisor() string`: Creates and runs visor instance. Returns non-empty string with error in case of failure;
- `NextDmsgSocket() int`: Returns file descriptor of the Dmsg socket. There may be more than one socket in use by the dmsg client, so 
this function should be called repeatedly until next call returns 0.
- `PrepareVPNClient(string, string) string`: Creates VPN client instance. First string argument is remote VPN server pub key, second one is passcode to 
authenticate within the server. Returns non-empty string with error in case of failure;
- `ShakeHands() string`: Requires `PrepareVPNClient` to be called first. Performs handshake between the client and the server. 
Returns non-empty string with error in case of failure;
- `TUNIP() string`: Requires `ShakeHands` to be called first. Returns the assigned TUN IP;
- `TUNGateway() string`: Requires `ShakeHands` to be called first. Returns the assigned TUN gateway;
- `StopVisor() string`: Stops currently running visor. Returns non-empty string with error in case of failure;
- `SetMobileAppAddr(string)`: Passes address of the UDP connection opened on the mobile application side;
- `ServeVPN()`: Starts off the goroutine serving VPN connection. After this call `IsVPNReady` starts returning `true`;
- `StartListeningUDP() string`: Opens UDP listener on the Go side. Returns non-empty string with error in case of failure;
- `IsVisorStarting() bool`: Checks if visor is starting. Will get `false` when it's fully functional;
- `IsVisorRunning() bool`: Checks if visor is running. Will get `true` whn visor is fully functional;
- `WaitVisorReady() string`: Blocks until visor gets fully initialized. Returns non-empty error string in case of failure;
- `StopVPNClient`: Stops VPN client without stopping visor itself;
- `StopListeningUDP`: Closes UDP socket;
- `VPNBandwidthSent`: Returns amount of bandwidth sent over VPN (bytes);
- `VPNBandwidthReceived`: Returns amount of bandwidth received over VPN (bytes);
- `VPNLatency`: Returns latency (ms);
- `VPNThroughput`: Returns throughput (bytes/s).


## Mobile/Go Communication
API may seem a bit complicated at first. Currently tested for Android devices, should be used with caution on iOS. 
Mobile app communicates with the Go part via UDP. All the packets are sent to the Go part via UDP and then get resent 
to the Skywire network. 

To setup the Go side properly you need to call at least:
- `PrepareVisor` to run the visor;
- `PrepareVPNClient` to run the VPN client;
- `ShakeHands` to perform handshake with the server;
- `StartListeningUDP` to open the UDP listener on the Go side;
- `ServeVPN` to start forwarding traffic.

All other calls should be done as needed.

### Android
Consult this page: https://developer.android.com/guide/topics/connectivity/vpn

In the example mobile app communicates with the remote server via `DatagramChannel`. Socket opened to the server gets protected 
with the `protect` method. We do the same here. But instead of a remote server we open the `DatagramChannel` to the Go part of the app.
We protect not only the tunnel socket, but also we need to protect all the sockets used for `Dmsg` communication to let traffic go back and forth freely. 
