import { Component, OnDestroy, ViewChild } from '@angular/core';
import { Subscription } from 'rxjs';
import { ActivatedRoute } from '@angular/router';
import { MatDialog } from '@angular/material/dialog';

import { VpnHelpers } from '../../vpn-helpers';
import { BackendState, VpnClientService, VpnServiceStates } from 'src/app/services/vpn-client.service';
import { SnackbarService } from 'src/app/services/snackbar.service';
import { AppsService } from 'src/app/services/apps.service';
import { processServiceError } from 'src/app/utils/errors';
import { DataUnits, VpnSavedDataService } from 'src/app/services/vpn-saved-data.service';
import GeneralUtils from 'src/app/utils/generalUtils';
import { SelectableOption, SelectOptionComponent } from 'src/app/components/layout/select-option/select-option.component';
import { TopBarComponent } from 'src/app/components/layout/top-bar/top-bar.component';
import { RouterConfigComponent, RouterConfigParams } from 'src/app/components/pages/node/node-info/node-info-content/router-config/router-config.component';

/**
 * Options that VpnSettingsComponent might be changing asynchronously.
 */
enum WorkingOptions {
  None = 0,
  Killswitch = 1,
}

/**
 * Page for changing the configuration of the VPN client.
 */
@Component({
  selector: 'app-vpn-settings-list',
  templateUrl: './vpn-settings.component.html',
  styleUrls: ['./vpn-settings.component.scss'],
})
export class VpnSettingsComponent implements OnDestroy {
  @ViewChild('topBarLoading') topBarLoading: TopBarComponent;
  @ViewChild('topBarLoaded') topBarLoaded: TopBarComponent;

  // If the data is being loaded.
  loading = true;
  // Current state of the VPN client app in the backend.
  backendData: BackendState;
  // If the option for getting the browser IP is active.
  getIpOption: boolean;
  // Units that must be used for displaying the data stats.
  dataUnitsOption: DataUnits;
  // Data for populating the tabs of the top bar.
  tabsData = VpnHelpers.vpnTabsData;

  // Pk of the local visor.
  currentLocalPk: string;

  // Current option being changed asynchronously.
  working: WorkingOptions = WorkingOptions.None;
  workingOptions = WorkingOptions;

  private navigationsSubscription: Subscription;
  private dataSubscription: Subscription;
  private operationSubscription: Subscription;

  constructor(
    private vpnClientService: VpnClientService,
    private snackbarService: SnackbarService,
    private appsService: AppsService,
    private vpnSavedDataService: VpnSavedDataService,
    private dialog: MatDialog,
    route: ActivatedRoute,
  ) {
    this.navigationsSubscription = route.paramMap.subscribe(params => {
      // Get the PK of the current local visor.
      if (params.has('key')) {
        this.currentLocalPk = params.get('key');
        VpnHelpers.changeCurrentPk(this.currentLocalPk);
        this.tabsData = VpnHelpers.vpnTabsData;
      }
    });

    // Get the current state of the VPN client app in the backend.
    this.dataSubscription = this.vpnClientService.backendState.subscribe(data => {
      if (data && data.serviceState !== VpnServiceStates.PerformingInitialCheck) {
        this.backendData = data;

        this.loading = false;
      }
    });

    this.getIpOption = this.vpnSavedDataService.getCheckIpSetting();
    this.dataUnitsOption = this.vpnSavedDataService.getDataUnitsSetting();
  }

  ngOnDestroy() {
    this.navigationsSubscription.unsubscribe();
    this.dataSubscription.unsubscribe();

    if (this.operationSubscription) {
      this.operationSubscription.unsubscribe();
    }
  }

  /**
   * Returns the css class that must be used for the dot indicating the current state of an option.
   * @param active If the option is active or not.
   */
  getStatusClass(active: boolean): string {
    switch (active) {
      case true:
        return 'dot-green';
      default:
        return 'dot-red';
    }
  }

  /**
   * Returns the translatable var that must be used for indicating the current state of an option.
   * @param active If the option is active or not.
   */
  getStatusText(active: boolean): string {
    switch (active) {
      case true:
        return 'vpn.settings-page.setting-on';
      default:
        return 'vpn.settings-page.setting-off';
    }
  }

  /**
   * Gets the translatable string for a data units selection.
   */
  getUnitsOptionText(units: DataUnits): string {
    switch (units) {
      case DataUnits.OnlyBits:
        return 'vpn.settings-page.data-units-modal.only-bits';
      case DataUnits.OnlyBytes:
        return 'vpn.settings-page.data-units-modal.only-bytes';
      default:
        return 'vpn.settings-page.data-units-modal.bits-speed-and-bytes-volume';
    }
  }

  /**
   * Starts changing the killswitch option.
   */
  changeKillswitchOption() {
    // Do not continue if another option is being changed.
    if (this.working !== WorkingOptions.None) {
      this.snackbarService.showWarning('vpn.settings-page.working-warning');

      return;
    }

    // If the VPN is running, ask for confirmation.
    if (this.backendData.vpnClientAppData.running) {
      const confirmationDialog =
        GeneralUtils.createConfirmationDialog(this.dialog, 'vpn.settings-page.change-while-connected-confirmation');

      confirmationDialog.componentInstance.operationAccepted.subscribe(() => {
        confirmationDialog.componentInstance.closeModal();

        this.finishChangingKillswitchOption();
      });
    } else {
      this.finishChangingKillswitchOption();
    }
  }

  /**
   * Finishes the procedure for changing the killswitch option.
   */
  private finishChangingKillswitchOption() {
    this.working = WorkingOptions.Killswitch;

    this.operationSubscription = this.appsService.changeAppSettings(
      this.currentLocalPk,
      this.vpnClientService.vpnClientAppName,
      { killswitch: !this.backendData.vpnClientAppData.killswitch },
    ).subscribe(
      () => {
        this.working = WorkingOptions.None;
        this.vpnClientService.updateData();
      },
      err => {
        this.working = WorkingOptions.None;

        err = processServiceError(err);
        this.snackbarService.showError(err);
      },
    );
  }

  /**
   * Changes the option for getting the browser IP.
   */
  changeGetIpOption() {
    this.getIpOption = !this.getIpOption;

    this.vpnSavedDataService.setCheckIpSetting(this.getIpOption);
  }

  /**
   * Opens the UI for changing the data units option.
   */
  changeDataUnits() {
    const options: SelectableOption[] = [];
    const optionValues: DataUnits[] = [];

    // Get all the available options and mark the currently selected one.
    Object.keys(DataUnits).forEach(key => {
      const option: SelectableOption = { label: this.getUnitsOptionText(DataUnits[key]) };

      if (this.dataUnitsOption === DataUnits[key]) {
        option.icon = 'done';
      }

      options.push(option);
      optionValues.push(DataUnits[key]);
    });

    // Open the option selection modal window.
    SelectOptionComponent.openDialog(this.dialog, options, 'vpn.settings-page.data-units-modal.title').afterClosed()
      .subscribe((result: number) => {
        if (result) {
          // Save the new value.
          this.dataUnitsOption = optionValues[result - 1];
          this.vpnSavedDataService.setDataUnitsSetting(this.dataUnitsOption);

          // Make the top bar use the new value.
          if (this.topBarLoading) {
            this.topBarLoading.updateVpnDataStatsUnit();
          }
          if (this.topBarLoaded) {
            this.topBarLoaded.updateVpnDataStatsUnit();
          }
        }
      });
  }

  /**
   * Opens the modal window for changing the hops configuration.
   */
  changeHops() {
    const params: RouterConfigParams = {nodePk: this.currentLocalPk, minHops: this.backendData.vpnClientAppData.minHops};
    RouterConfigComponent.openDialog(this.dialog, params).afterClosed().subscribe();
  }
}
