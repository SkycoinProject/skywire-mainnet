package com.skywire.skycoin.vpn.helpers;

import android.app.Activity;
import android.app.Dialog;
import android.app.PendingIntent;
import android.content.ClipData;
import android.content.ClipboardManager;
import android.content.Context;
import android.content.Intent;
import android.content.pm.ResolveInfo;
import android.content.res.Resources;
import android.graphics.Color;
import android.graphics.drawable.ColorDrawable;
import android.os.Handler;
import android.os.Looper;
import android.util.TypedValue;
import android.view.Window;
import android.view.WindowManager;
import android.widget.Toast;

import androidx.core.content.ContextCompat;

import com.skywire.skycoin.vpn.App;
import com.skywire.skycoin.vpn.R;
import com.skywire.skycoin.vpn.activities.main.MainActivity;
import com.skywire.skycoin.vpn.activities.servers.ServerLists;
import com.skywire.skycoin.vpn.activities.servers.VpnServerForList;
import com.skywire.skycoin.vpn.controls.ConfirmationModalWindow;
import com.skywire.skycoin.vpn.controls.EditServerValueModalWindow;
import com.skywire.skycoin.vpn.controls.ServerInfoModalWindow;
import com.skywire.skycoin.vpn.controls.ServerPasswordModalWindow;
import com.skywire.skycoin.vpn.controls.options.OptionsItem;
import com.skywire.skycoin.vpn.controls.options.OptionsModalWindow;
import com.skywire.skycoin.vpn.network.ApiClient;
import com.skywire.skycoin.vpn.objects.LocalServerData;
import com.skywire.skycoin.vpn.objects.ServerFlags;
import com.skywire.skycoin.vpn.vpn.VPNCoordinator;
import com.skywire.skycoin.vpn.vpn.VPNGeneralPersistentData;
import com.skywire.skycoin.vpn.vpn.VPNServersPersistentData;

import java.text.DecimalFormat;
import java.util.ArrayList;
import java.util.HashSet;
import java.util.List;

import io.reactivex.rxjava3.core.Observable;
import skywiremob.Skywiremob;

/**
 * General helper functions for different parts of the app.
 */
public class HelperFunctions {
    public enum WidthTypes {
        SMALL,
        BIG,
        BIGGER,
    }

    // Helpers for showing only a max number of decimals.
    public static final DecimalFormat twoDecimalsFormatter = new DecimalFormat("#.##");
    public static final DecimalFormat oneDecimalsFormatter = new DecimalFormat("#.#");
    public static final DecimalFormat zeroDecimalsFormatter = new DecimalFormat("#");

    // Last toast notification shown.
    private static Toast lastToast;

    /**
     * Displays debug information about an error in the console. It includes the several details.
     * @param prefix Text to show before the error details.
     * @param e Error.
     */
    public static void logError(String prefix, Throwable e) {
        // Print the basic error msgs.
        StringBuilder errorMsg = new StringBuilder(prefix + ": " + e.getMessage() + "\n");
        errorMsg.append(e.toString()).append("\n");

        // Print the stack.
        StackTraceElement[] stackTrace = e.getStackTrace();
        for (StackTraceElement stackTraceElement : stackTrace) {
            errorMsg.append(stackTraceElement.toString()).append("\n");
    }

        // Display in the console.
        Skywiremob.printString(errorMsg.toString());
    }

    /**
     * Displays an error msg in the console.
     * @param prefix Text to show before the error msg.
     * @param errorText Error msg.
     */
    public static void logError(String prefix, String errorText) {
        String errorMsg = prefix + ": " + errorText;
        Skywiremob.printString(errorMsg);
    }

    /**
     * Shows a toast notification. Can be used from background threads.
     * @param text Text for the notification.
     * @param shortDuration If the duration of the notification must be short (true) or
     *                      long (false).
     */
    public static void showToast(String text, boolean shortDuration) {
        // Run in the UI thread.
        Handler handler = new Handler(Looper.getMainLooper());
        handler.post(() -> {
            // Close the previous notification.
            if (lastToast != null) {
                lastToast.cancel();
            }

            // Show the notification.
            lastToast = Toast.makeText(App.getContext(), text, shortDuration ? Toast.LENGTH_SHORT : Toast.LENGTH_LONG);
            lastToast.show();
        });
    }

    /**
     * Gets the list of the app launchers installed in the device. More than one entry may share
     * the same package name. The current app is ignored.
     */
    public static List<ResolveInfo> getDeviceAppsList() {
        Intent mainIntent = new Intent(Intent.ACTION_MAIN, null);
        mainIntent.addCategory(Intent.CATEGORY_LAUNCHER);

        String packageName = App.getContext().getPackageName();
        ArrayList<ResolveInfo> response = new ArrayList<>();

        // Get all the entries in the device which coincide with the intent.
        for (ResolveInfo app : App.getContext().getPackageManager().queryIntentActivities( mainIntent, 0)) {
            if (!app.activityInfo.packageName.equals(packageName)) {
                response.add(app);
            }
        }

        return response;
    }

    /**
     * Filters a list of package names and returns only the ones which are from launchers
     * currently installed in the device. The current app is ignored.
     * @param apps List to filter.
     * @return Filtered list.
     */
    public static HashSet<String> filterAvailableApps(HashSet<String> apps) {
        HashSet<String> availableApps = new HashSet<>();
        for (ResolveInfo app : getDeviceAppsList()) {
            availableApps.add(app.activityInfo.packageName);
        }

        HashSet<String> response = new HashSet<>();
        for (String app : apps) {
            if (availableApps.contains(app)) {
                response.add(app);
            }
        }

        return response;
    }

    /**
     * Closes the provided activity if the VPN service is running. If the activity is closed,
     * a toast is shown.
     * @param activity Activity to close.
     * @return True if the activity was closed, false if not.
     */
    public static boolean closeActivityIfServiceRunning(Activity activity) {
        if (VPNCoordinator.getInstance().isServiceRunning()) {
            HelperFunctions.showToast(App.getContext().getString(R.string.vpn_already_running_warning), true);
            activity.finish();

            return true;
        }

        return false;
    }

    /**
     * Checks if there is connection via internet to at least one of the testing URLs set in the
     * globals class.
     * @param logError If true and there is an error checking the connection, the error will
     *                 be logged.
     * @return Observable which emits if there is connection or not.
     */
    public static Observable<Boolean> checkInternetConnectivity(boolean logError) {
        return checkInternetConnectivity(0, logError);
    }

    /**
     * Internal function for checking if there is internet connectivity, recursively.
     * @param urlIndex Index of the testing URL to check.
     * @param logError If the error, if any, must be logged at the end of the operation.
     */
    private static Observable<Boolean> checkInternetConnectivity(int urlIndex, boolean logError) {
        return ApiClient.checkConnection(Globals.INTERNET_CHECKING_ADDRESSES[urlIndex])
            // If there is a valid response, return true.
            .map(response -> true)
            .onErrorResumeNext(err -> {
                // If there is an error and there are more testing URLs, continue to the next step.
                if (urlIndex < Globals.INTERNET_CHECKING_ADDRESSES.length - 1) {
                    return checkInternetConnectivity(urlIndex + 1, logError);
                }

                if (logError) {
                    HelperFunctions.logError("Checking network connectivity", err);
                }

                return Observable.just(false);
            });
    }

    /**
     * Returns an intent for opening the app.
     */
    public static PendingIntent getOpenAppPendingIntent() {
        final Intent openAppIntent = new Intent(App.getContext(), MainActivity.class);
        openAppIntent.setFlags(Intent.FLAG_ACTIVITY_NEW_TASK);
        openAppIntent.setAction(Intent.ACTION_MAIN);
        openAppIntent.addCategory(Intent.CATEGORY_LAUNCHER);

        return PendingIntent.getActivity(App.getContext(), 0, openAppIntent, PendingIntent.FLAG_UPDATE_CURRENT);
    }

    /**
     * Allows to convert a bytes value to KB, MB, GB, etc. It considers 1024, and not 1000, a K.
     * @param bytes Amount of data to process, in bytes.
     * @param calculatePerSecond If true, the result will have "/s" added at the end.
     * @param useBits If the data must be shown in bits (true) or bytes (false).
     */
    public static String computeDataAmountString(double bytes, boolean calculatePerSecond, boolean useBits) {
        double current = (double)bytes;

        // Set the correct units.
        String[] scales;
        if (calculatePerSecond) {
            if (useBits) {
                scales = new String[]{" b/s", " Kb/s", " Mb/s", " Gb/s", " Tb/s", "Pb/s", "Eb/s", "Zb/s", "Yb/s"};
            } else {
                scales = new String[]{" B/s", " KB/s", " MB/s", " GB/s", " TB/s", "PB/s", "EB/s", "ZB/s", "YB/s"};
            }
        } else {
            if (useBits) {
                scales = new String[]{" b", " Kb", " Mb", " Gb", " Tb", "Pb", "Eb", "Zb", "Yb"};
            } else {
                scales = new String[]{" B", " KB", " MB", " GB", " TB", "PB", "EB", "ZB", "YB"};
            }
        }

        // Convert to bits, if needed.
        if (useBits) {
            current *= 8;
        }

        // Divide the speed by 1024 until getting an appropriate scale to return.
        for (int i = 0; i < scales.length - 1; i++) {
            if (current < 1024) {
                // Return decimals depending on how long the number is.
                if (current < 10) {
                    return twoDecimalsFormatter.format(current) + scales[i];
                } else if (current < 100) {
                    return oneDecimalsFormatter.format(current) + scales[i];
                }

                return zeroDecimalsFormatter.format(current) + scales[i];
            }

            current /= 1024;
        }

        return current + scales[scales.length - 1];
    }

    public static String getLatencyValue(double latency) {
        String initialPart;
        String lastPart;

        if (latency >= 1000) {
            initialPart = oneDecimalsFormatter.format(latency / 1000);
            lastPart = App.getContext().getString(R.string.general_seconds_abbreviation);
        } else {
            initialPart = oneDecimalsFormatter.format(latency);
            lastPart = App.getContext().getString(R.string.general_milliseconds_abbreviation);
        }

        return initialPart + lastPart;
    }

    public static int getFlagResourceId(String countryCode) {
        if (countryCode.toLowerCase() != "do") {
            int flagResourceId = App.getContext().getResources().getIdentifier(
                    countryCode.toLowerCase(),
                    "drawable",
                    App.getContext().getPackageName()
            );

            if (flagResourceId != 0) {
                return flagResourceId;
            } else {
                return R.drawable.zz;
            }
        } else {
            return R.drawable.do_flag;
        }
    }

    public static int getCongestionNumberColor(int congestion) {
        if (congestion < 60) {
            return ContextCompat.getColor(App.getContext(), R.color.green);
        } else if (congestion < 90) {
            return ContextCompat.getColor(App.getContext(), R.color.yellow);
        }

        return ContextCompat.getColor(App.getContext(), R.color.red);
    }

    public static int getLatencyNumberColor(int latency) {
        if (latency < 200) {
            return ContextCompat.getColor(App.getContext(), R.color.green);
        } else if (latency < 350) {
            return ContextCompat.getColor(App.getContext(), R.color.yellow);
        }

        return ContextCompat.getColor(App.getContext(), R.color.red);
    }

    public static int getHopsNumberColor(int hops) {
        if (hops < 5) {
            return ContextCompat.getColor(App.getContext(), R.color.green);
        } else if (hops < 9) {
            return ContextCompat.getColor(App.getContext(), R.color.yellow);
        }

        return ContextCompat.getColor(App.getContext(), R.color.red);
    }

    public static void configureModalWindow(Dialog modal) {
        Window window = modal.getWindow();
        window.setBackgroundDrawable(new ColorDrawable(Color.TRANSPARENT));
        window.setSoftInputMode(WindowManager.LayoutParams.SOFT_INPUT_ADJUST_RESIZE);

        WidthTypes screenWidthType = getWidthType(modal.getContext());
        if (screenWidthType != WidthTypes.SMALL) {
            int width = (int)TypedValue.applyDimension(
                TypedValue.COMPLEX_UNIT_DIP,
                500,
                modal.getContext().getResources().getDisplayMetrics()
            );

            WindowManager.LayoutParams params = window.getAttributes();
            params.width = width;
            params.height = WindowManager.LayoutParams.WRAP_CONTENT;
            window.setAttributes(params);
        }
    }

    public static boolean showBackgroundForVerticalScreen() {
        double proportion = (double)Resources.getSystem().getDisplayMetrics().widthPixels / (double)Resources.getSystem().getDisplayMetrics().heightPixels;
        if (proportion > 1.1) {
            return false;
        }

        return true;
    }

    public static WidthTypes getWidthType(Context ctx) {
        int screenWidthInDP = (int)(Resources.getSystem().getDisplayMetrics().widthPixels / ctx.getResources().getDisplayMetrics().density);

        if (screenWidthInDP >= 1100) {
            return WidthTypes.BIGGER;
        } else if (screenWidthInDP >= 800) {
            return WidthTypes.BIG;
        }

        return WidthTypes.SMALL;
    }

    public static int getTabletExtraHorizontalPadding(Context ctx) {
        WidthTypes widthType = getWidthType(ctx);

        if (widthType == WidthTypes.BIGGER) {
            return (int) TypedValue.applyDimension(
                TypedValue.COMPLEX_UNIT_DIP,
                100,
                ctx.getResources().getDisplayMetrics()
            );
        } else if (widthType == WidthTypes.BIG) {
            return (int)TypedValue.applyDimension(
                TypedValue.COMPLEX_UNIT_DIP,
                40,
                ctx.getResources().getDisplayMetrics()
            );
        }

        return 0;
    }

    public static boolean prepareAndStartVpn(Activity requestingActivity, LocalServerData server) {
        if (server.flag == ServerFlags.Blocked) {
            HelperFunctions.showToast(requestingActivity.getString(R.string.general_starting_blocked_server_error) + server.pk, false);

            return false;
        }

        long err = Skywiremob.isPKValid(server.pk).getCode();
        if (err != Skywiremob.ErrCodeNoError) {
            HelperFunctions.showToast(requestingActivity.getString(R.string.vpn_coordinator_invalid_credentials_error) + server.pk, false);
            return false;
        } else {
            Skywiremob.printString("PK is correct");
        }

        Globals.AppFilteringModes selectedMode = VPNGeneralPersistentData.getAppsSelectionMode();
        if (selectedMode != Globals.AppFilteringModes.PROTECT_ALL) {
            HashSet<String> selectedApps = HelperFunctions.filterAvailableApps(VPNGeneralPersistentData.getAppList(new HashSet<>()));

            if (selectedApps.size() == 0) {
                if (selectedMode == Globals.AppFilteringModes.PROTECT_SELECTED) {
                    HelperFunctions.showToast(requestingActivity.getString(R.string.vpn_no_apps_to_protect_warning), false);
                } else {
                    HelperFunctions.showToast(requestingActivity.getString(R.string.vpn_no_apps_to_ignore_warning), false);
                }
            }
        }

        VPNCoordinator.getInstance().startVPN(
            requestingActivity,
            server
        );

        return true;
    }

    public static String getServerName(VpnServerForList server, String defaultName) {
        if ((server.name == null || server.name.trim().equals("")) && (server.customName == null || server.customName.trim().equals(""))) {
            return defaultName;
        } else if (server.name != null && !server.name.trim().equals("") && (server.customName == null || server.customName.trim().equals(""))) {
            return server.name;
        } else if (server.customName != null && !server.customName.trim().equals("") && (server.name == null || server.name.trim().equals(""))) {
            return server.customName;
        }

        return server.customName + " - " + server.name;
    }

    public static String getServerNote(LocalServerData server) {
        String note = "";
        if (server.note != null && !server.note.trim().equals("")) {
            note = server.note;
        }
        if (server.personalNote != null && !server.personalNote.trim().equals("")) {
            if (note.length() > 0) {
                note += " - ";
            }
            note += server.personalNote;
        }

        return note.length() > 0 ? note : null;
    }

    public static void showServerOptions(Context ctx, VpnServerForList server, ServerLists listType) {
        ArrayList<OptionsItem.SelectableOption> options = new ArrayList();
        ArrayList<Integer> optionCodes = new ArrayList();

        OptionsItem.SelectableOption option = new OptionsItem.SelectableOption();
        option.icon = "\ue88e";
        option.translatableLabelId = R.string.tmp_server_options_view_info;
        options.add(option);
        optionCodes.add(10);
        option = new OptionsItem.SelectableOption();
        option.icon = "\ue14d";
        option.translatableLabelId = R.string.tmp_server_options_copy_pk;
        options.add(option);
        optionCodes.add(11);
        option = new OptionsItem.SelectableOption();
        option.icon = "\ue3c9";
        option.translatableLabelId = R.string.tmp_server_options_name;
        options.add(option);
        optionCodes.add(101);
        option = new OptionsItem.SelectableOption();
        option.icon = "\ue8d2";
        option.translatableLabelId = R.string.tmp_server_options_note;
        options.add(option);
        optionCodes.add(102);

        if (server.hasPassword) {
            option = new OptionsItem.SelectableOption();
            option.icon = "\ue898";
            option.translatableLabelId = R.string.tmp_server_options_remove_password;
            options.add(option);
            optionCodes.add(201);

            option = new OptionsItem.SelectableOption();
            option.icon = "\ue899";
            option.translatableLabelId = R.string.tmp_server_options_change_password;
            options.add(option);
            optionCodes.add(202);
        } else {
            if (server.enteredManually) {
                option = new OptionsItem.SelectableOption();
                option.icon = "\ue899";
                option.translatableLabelId = R.string.tmp_server_options_add_password;
                options.add(option);
                optionCodes.add(202);
            }
        }

        if (server.flag != ServerFlags.Favorite) {
            option = new OptionsItem.SelectableOption();
            option.icon = "\ue838";
            option.translatableLabelId = R.string.tmp_server_options_make_favorite;
            options.add(option);
            optionCodes.add(1);
        }

        if (server.flag == ServerFlags.Favorite) {
            option = new OptionsItem.SelectableOption();
            option.icon = "\ue83a";
            option.translatableLabelId = R.string.tmp_server_options_remove_from_favorites;
            options.add(option);
            optionCodes.add(-1);
        }

        if (server.flag != ServerFlags.Blocked) {
            option = new OptionsItem.SelectableOption();
            option.icon = "\ue925";
            option.translatableLabelId = R.string.tmp_server_options_block;
            options.add(option);
            optionCodes.add(2);
        }

        if (server.flag == ServerFlags.Blocked) {
            option = new OptionsItem.SelectableOption();
            option.icon = "\ue8dc";
            option.translatableLabelId = R.string.tmp_server_options_unblock;
            options.add(option);
            optionCodes.add(-2);
        }

        if (server.inHistory) {
            option = new OptionsItem.SelectableOption();
            option.icon = "\ue872";
            option.translatableLabelId = R.string.tmp_server_options_remove_from_history;
            options.add(option);
            optionCodes.add(-3);
        }

        OptionsModalWindow modal = new OptionsModalWindow(ctx, null, options, (int selectedOption) -> {
            LocalServerData savedVersion_ = VPNServersPersistentData.getInstance().getSavedVersion(server.pk);
            if (savedVersion_ == null) {
                savedVersion_ = VPNServersPersistentData.getInstance().processFromList(server);
            }

            final LocalServerData savedVersion = savedVersion_;

            if (optionCodes.get(selectedOption) > 200) {
                if (VPNCoordinator.getInstance().isServiceRunning() && VPNServersPersistentData.getInstance().getCurrentServer().pk.equals(savedVersion.pk)) {
                    HelperFunctions.showToast(App.getContext().getText(R.string.general_server_running_error).toString(), true);
                    return;
                }

                if (optionCodes.get(selectedOption) == 201) {
                    ConfirmationModalWindow confirmationModal = new ConfirmationModalWindow(
                        ctx,
                        R.string.tmp_server_options_remove_password_confirmation,
                        R.string.tmp_confirmation_yes,
                        R.string.tmp_confirmation_no,
                        () -> {
                            VPNServersPersistentData.getInstance().removePassword(savedVersion.pk);
                            HelperFunctions.showToast(ctx.getString(R.string.tmp_server_options_remove_password_done), true);
                        }
                    );
                    confirmationModal.show();
                } else {
                    ServerPasswordModalWindow passwordModal = new ServerPasswordModalWindow(
                        ctx,
                        server
                    );
                    passwordModal.show();
                }
            } else if (optionCodes.get(selectedOption) > 100) {
                EditServerValueModalWindow valueModal = new EditServerValueModalWindow(
                    ctx,
                    optionCodes.get(selectedOption) == 101,
                    server
                );
                valueModal.show();
            } else if (optionCodes.get(selectedOption) == 10) {
                ServerInfoModalWindow infoModal = new ServerInfoModalWindow(ctx, server, listType);
                infoModal.show();
            } else if (optionCodes.get(selectedOption) == 11) {
                ClipboardManager clipboard = (ClipboardManager)ctx.getSystemService(Context.CLIPBOARD_SERVICE);
                ClipData clipData = ClipData.newPlainText("", server.pk);
                clipboard.setPrimaryClip(clipData);
                HelperFunctions.showToast(ctx.getString(R.string.general_copied), true);
            } else if (optionCodes.get(selectedOption) == 1) {
                if (server.flag != ServerFlags.Blocked) {
                    VPNServersPersistentData.getInstance().changeFlag(savedVersion, ServerFlags.Favorite);
                    HelperFunctions.showToast(ctx.getString(R.string.tmp_server_options_make_favorite_done), true);
                    return;
                }

                ConfirmationModalWindow confirmationModal = new ConfirmationModalWindow(
                    ctx,
                    R.string.tmp_server_options_make_favorite_from_blocked_confirmation,
                    R.string.tmp_confirmation_yes,
                    R.string.tmp_confirmation_no,
                    () -> {
                        VPNServersPersistentData.getInstance().changeFlag(savedVersion, ServerFlags.Favorite);
                        HelperFunctions.showToast(ctx.getString(R.string.tmp_server_options_make_favorite_done), true);
                    }
                );
                confirmationModal.show();
            } else if (optionCodes.get(selectedOption) == -1) {
                VPNServersPersistentData.getInstance().changeFlag(savedVersion, ServerFlags.None);
                HelperFunctions.showToast(ctx.getString(R.string.tmp_server_options_remove_from_favorites_done), true);
            } else if (optionCodes.get(selectedOption) == 2) {
                if (VPNServersPersistentData.getInstance().getCurrentServer() != null &&
                    VPNServersPersistentData.getInstance().getCurrentServer().pk.toLowerCase().equals(server.pk.toLowerCase())
                ) {
                    HelperFunctions.showToast(ctx.getString(R.string.tmp_server_options_block_error), true);
                    return;
                }

                if (server.flag != ServerFlags.Favorite) {
                    VPNServersPersistentData.getInstance().changeFlag(savedVersion, ServerFlags.Blocked);
                    HelperFunctions.showToast(ctx.getString(R.string.tmp_server_options_block_done), true);
                    return;
                }

                ConfirmationModalWindow confirmationModal = new ConfirmationModalWindow(
                    ctx,
                    R.string.tmp_server_options_block_favorite_confirmation,
                    R.string.tmp_confirmation_yes,
                    R.string.tmp_confirmation_no,
                    () -> {
                        VPNServersPersistentData.getInstance().changeFlag(savedVersion, ServerFlags.Blocked);
                        HelperFunctions.showToast(ctx.getString(R.string.tmp_server_options_block_done), true);
                    }
                );
                confirmationModal.show();
            } else if (optionCodes.get(selectedOption) == -2) {
                VPNServersPersistentData.getInstance().changeFlag(savedVersion, ServerFlags.None);
                HelperFunctions.showToast(ctx.getString(R.string.tmp_server_options_unblock_done), true);
            } else if (optionCodes.get(selectedOption) == -3) {
                ConfirmationModalWindow confirmationModal = new ConfirmationModalWindow(
                    ctx,
                    R.string.tmp_server_options_remove_from_history_confirmation,
                    R.string.tmp_confirmation_yes,
                    R.string.tmp_confirmation_no,
                    () -> {
                        VPNServersPersistentData.getInstance().removeFromHistory(savedVersion.pk);
                        HelperFunctions.showToast(ctx.getString(R.string.tmp_server_options_remove_from_history_done), true);
                    }
                );
                confirmationModal.show();
            }
        });
        modal.show();
    }
}
