import { Injectable } from '@angular/core';
import { Router } from '@angular/router';
import { Observable, Subscription, of, BehaviorSubject, concat, throwError } from 'rxjs';
import { mergeMap, delay, retryWhen, take, catchError, map } from 'rxjs/operators';
import { HttpClient } from '@angular/common/http';

import { ApiService } from './api.service';
import { AppsService } from './apps.service';
import { VpnServer } from './vpn-client-discovery.service';
import { ManualVpnServerData } from '../components/vpn/pages/vpn-server-list/add-vpn-server/add-vpn-server.component';
import { VpnSavedDataService, LocalServerData } from './vpn-saved-data.service';
import { AppConfig } from '../app.config';
import { environment } from 'src/environments/environment';
import { SnackbarService } from './snackbar.service';
import { processServiceError } from '../utils/errors';
import { OperationError } from '../utils/operation-error';
import { TransportService } from './transport.service';

/**
 * States in which the VPN client app of the local visor can be.
 */
export enum AppState {
  Stopped = 'stopped',
  Connecting = 'Connecting',
  Running = 'Running',
  ShuttingDown = 'Shutting down',
  Reconnecting = 'Connection failed, reconnecting',
}

/**
 * Extended information about the state of the VPN and the VPN service.
 */
export class BackendState {
  /**
   * Last moment in which the VPN state was updated from the local visor.
   */
  updateDate: number = Date.now();
  /**
   * Current state of the VPN service.
   */
  serviceState: VpnServiceStates;
  /**
   * If the VPN service is busy working and will not process changes until finishing.
   */
  busy: boolean;
  /**
   * State and properties of the VPN client app.
   */
  vpnClientAppData: VpnClientAppData;
}

/**
 * State and properties of the VPN client app on the local visor.
 */
export class VpnClientAppData {
  /**
   * If the app is running.
   */
  running: boolean;
  /**
   * Public key of the currently selected server.
   */
  serverPk: string;
  /**
   * if the killswitch option is active.
   */
  killswitch: boolean;
  /**
   * Current state of the app.
   */
  appState: AppState;
  /**
   * Data transmission stats, if the app is running.
   */
  connectionData: VpnClientConnectionsData;
  /**
   * Min hops the reoutes must have.
   */
  minHops: number;
}

/**
 * VPN data transmission stats
 */
export class VpnClientConnectionsData {
  latency = 0;
  uploadSpeed = 0;
  downloadSpeed = 0;
  totalUploaded = 0;
  totalDownloaded = 0;
  downloadSpeedHistory: number[];
  uploadSpeedHistory: number[];
  latencyHistory: number[];
}

/**
 * States in which VpnClientService can be.
 */
export enum VpnServiceStates {
  /**
   * Checking the VPN state for the first tine, so all VPN state data may be invalid.
   */
  PerformingInitialCheck = 1,
  Off = 10,
  Starting = 20,
  Running = 100,
  Disconnecting = 200,
}

/**
 * Results returned when checking if the currently selected server can be changed by another
 * public key.
 */
export enum CheckPkResults {
  /**
   * The service is busy, so the change can not be made right now.
   */
  Busy = 1,
  /**
   * The change can be made without problems.
   */
  Ok = 2,
  /**
   * The VPN is running, so it will be stopped if the server is changed.
   */
  MustStop = 3,
  /**
   * The VPN is running and the provided PK is already being used as server, so there is no need
   * for making any changes.
   */
  SamePkRunning = 4,
  /**
   * The provided PK is already being used as the selected server, so there is no need for
   * changes. However, the VPN is stopped and it may be started to connect with the server.
   */
  SamePkStopped = 5,
}

/**
 * Allows to get and modify the state of the VPN. The service was made for the VPN client.
 */
@Injectable({
  providedIn: 'root'
})
export class VpnClientService {
  /**
   * Name of the VPN client app in the Skywire visor.
   */
  readonly vpnClientAppName = 'vpn-client';

  // Standard time to wait for refresing the data or retrying operations.
  private readonly standardWaitTime = 2000;

  // Public key of the local Skywire visor.
  private nodeKey: string;
  // Subject for sending updates about the state of the VPN.
  private stateSubject = new BehaviorSubject<BackendState>(null);
  // Object with the data about the current state of the VPN.
  private currentEventData: BackendState;
  // Last state of the service.
  private lastServiceState: VpnServiceStates;
  // If the service is currently working (busy).
  private working = true;
  // If has a value, the current server must be replaced by this one.
  private requestedServer: LocalServerData = null;
  // Password provided with requestedServer.
  private requestedPassword: string = null;

  // Data transmission history values.
  private downloadSpeedHistory: number[];
  private uploadSpeedHistory: number[];
  private latencyHistory: number[];
  // Pk of the server for which the last data transmission history values were obtained.
  private connectionHistoryPk: string;

  private dataSubscription: Subscription;
  private continuousUpdateSubscription: Subscription;

  constructor(
    private apiService: ApiService,
    private appsService: AppsService,
    private router: Router,
    private vpnSavedDataService: VpnSavedDataService,
    private http: HttpClient,
    private snackbarService: SnackbarService,
    private transportService: TransportService,
  ) {
    // Set the initial state. PerformingInitialCheck will be replaced when getting the state
    // for the first time. The busy state too, to start being able to perform other operations.
    this.currentEventData = new BackendState();
    this.currentEventData.busy = true;
    this.lastServiceState = VpnServiceStates.PerformingInitialCheck;
  }

  /**
   * Makes the initializations for the service to work. Can be called multiple times, provided
   * that the local visor PK is not changed.
   * @param nodeKey Local visor public key.
   */
  initialize(nodeKey: string) {
    if (nodeKey) {
      // Save the local node PK and perform the initializations.
      if (!this.nodeKey) {
        this.nodeKey = nodeKey;

        this.vpnSavedDataService.initialize();

        this.updateData();
      } else {
        // The service is made to get the data of one local visor only. If another local visor
        // PK is provided, go to an error page.
        if (nodeKey !== this.nodeKey) {
          this.router.navigate(['vpn', 'unavailable'], { queryParams: {problem: 'pkChange'} });
        }
      }
    }
  }

  /**
   * Observable which continually emits state updates.
   */
  get backendState(): Observable<BackendState> {
    return this.stateSubject.asObservable();
  }

  /**
   * Makes the service update the VPN state immediately and continue doing so periodically.
   */
  updateData() {
    this.continuallyUpdateData(0);
  }

  /**
   * Starts the VPN.
   * @returns If it was possible to start the process (true) or not (false).
   */
  start(): boolean {
    // Continue only if the service is not busy and the VPN is stopped.
    if (!this.working && this.lastServiceState < 20) {
      this.checkBeforeChangingAppState(true);

      return true;
    }

    return false;
  }

  /**
   * Stops the VPN.
   * @returns If it was possible to start the process (true) or not (false).
   */
  stop(): boolean {
    // Continue only if the service is not busy and the VPN is running.
    if (!this.working && this.lastServiceState >= 20 && this.lastServiceState < 200) {
      this.checkBeforeChangingAppState(false);

      return true;
    }

    return false;
  }

  /**
   * Gets the public IP of the machine running this app. If there is an error, it could
   * return null.
   */
  getIp(): Observable<string> {
    // Use a test value if in development mode.
    if (!environment.production && AppConfig.vpn.hardcodedIpWhileDeveloping) {
      return of('8.8.8.8 (testing)');
    }

    return this.http.request('GET', 'https://api.ipify.org?format=json').pipe(
      retryWhen(errors => concat(errors.pipe(delay(this.standardWaitTime), take(4)), throwError(''))),
      map(data => {
        if (data && data['ip']) {
          return  data['ip'];
        }

        return null;
      })
    );
  }

  /**
   * Gets the country of the public IP of the machine running this app. If there is an error,
   * it could return null.
   */
  getIpCountry(ip: string): Observable<string> {
    // Use a test value if in development mode.
    if (!environment.production && AppConfig.vpn.hardcodedIpWhileDeveloping) {
      return of('United States (testing)');
    }

    return this.http.request('GET', 'https://ip2c.org/' + ip, { responseType: 'text' }).pipe(
      retryWhen(errors => concat(errors.pipe(delay(2000), take(4)), throwError(''))),
      map(data => {
        let country: string = null;

        // The name must be the fourth element of the retrieved value.
        if (data) {
          const dataParts: string[] = data.split(';');

          if (dataParts.length === 4) {
            country = dataParts[3];
          }
        }

        return country;
      })
    );
  }

  /**
   * Changes the currently selected server and connects to it.
   * @returns If it was possible to start the process (true) or not (false).
   */
  changeServerUsingHistory(newServer: LocalServerData, password: string): boolean {
    this.requestedServer = newServer;
    this.requestedPassword = password;
    this.updateRequestedServerPasswordSetting();

    return this.changeServer();
  }

  /**
   * Changes the currently selected server and connects to it.
   * @returns If it was possible to start the process (true) or not (false).
   */
  changeServerUsingDiscovery(newServer: VpnServer, password: string): boolean {
    this.requestedServer = this.vpnSavedDataService.processFromDiscovery(newServer);
    this.requestedPassword = password;
    this.updateRequestedServerPasswordSetting();

    return this.changeServer();
  }

  /**
   * Changes the currently selected server and connects to it.
   * @returns If it was possible to start the process (true) or not (false).
   */
  changeServerManually(newServer: ManualVpnServerData, password: string): boolean {
    this.requestedServer = this.vpnSavedDataService.processFromManual(newServer);
    this.requestedPassword = password;
    this.updateRequestedServerPasswordSetting();

    return this.changeServer();
  }

  /**
   * Updates the "usedWithPassword" property of the server in the requestedServer var, locally
   * and in in persistent storage.
   */
  private updateRequestedServerPasswordSetting() {
    this.requestedServer.usedWithPassword = !!this.requestedPassword && this.requestedPassword !== '';

    const alreadySavedVersion = this.vpnSavedDataService.getSavedVersion(this.requestedServer.pk, true);
    if (alreadySavedVersion) {
      alreadySavedVersion.usedWithPassword = this.requestedServer.usedWithPassword;
      this.vpnSavedDataService.updateServer(alreadySavedVersion);
    }
  }

  /**
   * Starts the process for changing the selected server to the one in the requestedServer var.
   * @returns If it was possible to start the process (true) or not (false).
   */
  private changeServer(): boolean {
    if (!this.working) {
      // If the VPN is active, the this.stop() call will stop it and then continue with the
      // process for changing the server. If not, the this.processServerChange() call will
      // continue with the operation.
      if (!this.stop()) {
        this.processServerChange();
      }

      return true;
    }

    return false;
  }

  /**
   * Checks if at this moment it is possible to change the selected server to the provided PK and
   * what must be done for the change to work.
   */
  checkNewPk(newPk): CheckPkResults {
    if (this.working) {
      return CheckPkResults.Busy;
    } else if (this.lastServiceState !== VpnServiceStates.Off) {
      if (newPk === this.vpnSavedDataService.currentServer.pk) {
        return CheckPkResults.SamePkRunning;
      } else {
        return CheckPkResults.MustStop;
      }
    } else if (this.vpnSavedDataService.currentServer && newPk === this.vpnSavedDataService.currentServer.pk) {
      return CheckPkResults.SamePkStopped;
    }

    return CheckPkResults.Ok;
  }

  /**
   * Changes the currently selected server to the one set in this.requestedServer. After that,
   * the VPN is started.
   */
  private processServerChange() {
    if (this.dataSubscription) {
      this.dataSubscription.unsubscribe();
    }

    const data = { pk: this.requestedServer.pk };
    if (this.requestedPassword) {
      data['passcode'] = this.requestedPassword;
    } else {
      data['passcode'] = '';
    }

    // Mark the service as busy, stop updating the VPN state and inform about the changes.
    this.stopContinuallyUpdatingData();
    this.working = true;
    this.sendUpdate();

    // Make the changes in the local visor.
    this.dataSubscription = this.appsService.changeAppSettings(
      this.nodeKey,
      this.vpnClientAppName,
      data,
    ).subscribe(
      () => {
        // Save the changes locally.
        this.vpnSavedDataService.modifyCurrentServer(this.requestedServer);

        // Make the service work normally again.
        this.requestedServer = null;
        this.requestedPassword = null;
        this.working = false;

        // Start the VPN.
        this.start();
      }, (err: OperationError) => {
        // Inform about the error.
        err = processServiceError(err);
        this.snackbarService.showError('vpn.server-change.backend-error', null, false, err.originalServerErrorMsg);

        // Make the service work normally again.
        this.working = false;
        this.requestedServer = null;
        this.requestedPassword = null;
        this.sendUpdate();
        this.updateData();
      }
    );
  }

  /**
   * Checks and configures the local visor to make it posible to start or stop the VPN and then
   * starts or stops it.
   * @param startApp If the VPN must be started or stopped.
   */
  private checkBeforeChangingAppState(startApp: boolean) {
    // Cancel if the service is busy.
    if (this.working) {
      return;
    }

    // Mark the service as busy, stop updating the VPN state and inform about the changes.
    this.stopContinuallyUpdatingData();
    this.working = true;
    this.sendUpdate();

    // If the VPN is going to be stopped, just continue with the process, as no config is needed.
    if (!startApp) {
      this.changeAppState(startApp);

      return;
    }

    if (this.dataSubscription) {
      this.dataSubscription.unsubscribe();
    }

    // Get the current general state of the local visor.
    this.dataSubscription = this.apiService.get(`visors/${this.nodeKey}`).pipe(
      mergeMap(nodeInfo => {
        // Check if the local visor already has a transport for connecting with the server.
        let transportFound = false;
        if (nodeInfo.transports && nodeInfo.transports.length > 0) {
          (nodeInfo.transports as any[]).forEach(transport => {
            if (transport.remote_pk === this.vpnSavedDataService.currentServer.pk) {
              transportFound = true;
            }
          });
        }

        // If the transport was found, do nothing.
        if (transportFound) {
          return of(null);
        }

        // If the transport was not found, create one.
        return this.transportService.create(
          this.nodeKey,
          this.vpnSavedDataService.currentServer.pk,
          'dmsg',
        );
      }), retryWhen(errors =>
        concat(errors.pipe(delay(this.standardWaitTime), take(3)), errors.pipe(mergeMap(err => throwError(err))))
      ),
    ).subscribe(() => {
      // Continue with the process.
      this.changeAppState(startApp);
    }, (err: OperationError) => {
      // Inform about the error.
      err = processServiceError(err);
      this.snackbarService.showError('vpn.status-page.problem-connecting-error', null, false, err.originalServerErrorMsg);

      // Make the service work normally again.
      this.working = false;
      this.sendUpdate();
      this.updateData();
    });
  }

  /**
   * Starts or stops the VPN client app in the local visor, which starts or stops the VPN
   * protection. Must be called only by checkBeforeChangingAppState.
   * @param startApp If the app must be started or stopped.
   */
  private changeAppState(startApp: boolean) {
    const data = { status: 1 };

    if (startApp) {
      this.lastServiceState = VpnServiceStates.Starting;
      // This will remove any previously saved data transmission history.
      this.connectionHistoryPk = null;
    } else {
      this.lastServiceState = VpnServiceStates.Disconnecting;
      data.status = 0;
    }

    if (this.dataSubscription) {
      this.dataSubscription.unsubscribe();
    }

    this.dataSubscription = this.appsService.changeAppSettings(
      this.nodeKey,
      this.vpnClientAppName,
      data,
    ).pipe(
      catchError(err => {
        // If the response was an error, check the state of the backend, to know if the change
        // was made. There are some cases in which this may happen.
        return this.getVpnClientState().pipe(mergeMap(appData => {
          if (appData) {
            if (startApp && appData.running) {
              return of(true);
            } else if (!startApp && !appData.running) {
              return of(true);
            }
          }

          return throwError(err);
        }));
      }),
      retryWhen(errors =>
        concat(errors.pipe(delay(this.standardWaitTime), take(3)), errors.pipe(mergeMap(err => throwError(err))))
      ),
    ).subscribe(() => {
      this.working = false;

      // Update the local values.
      if (startApp) {
        this.currentEventData.vpnClientAppData.running = true;
        this.lastServiceState = VpnServiceStates.Running;

        this.vpnSavedDataService.updateHistory();
      } else {
        this.currentEventData.vpnClientAppData.running = false;
        this.lastServiceState = VpnServiceStates.Off;
      }

      // Make the service work normally again.
      this.sendUpdate();
      this.updateData();

      // If the app was stopped but a request for changing the server was saved, start with
      // the process for changing it.
      if (!startApp && this.requestedServer) {
        this.processServerChange();
      }
    }, (err: OperationError) => {
      err = processServiceError(err);

      // Inform about the error.
      if (this.lastServiceState === VpnServiceStates.Starting) {
        this.snackbarService.showError('vpn.status-page.problem-starting-error', null, false, err.originalServerErrorMsg);
      } else if (this.lastServiceState === VpnServiceStates.Disconnecting) {
        this.snackbarService.showError('vpn.status-page.problem-stopping-error', null, false, err.originalServerErrorMsg);
      } else {
        this.snackbarService.showError('vpn.status-page.generic-problem-error', null, false, err.originalServerErrorMsg);
      }

      // Make the service work normally again.
      this.working = false;
      this.sendUpdate();
      this.updateData();
    });
  }

  /**
   * Starts continually getting the state of the VPN and sending updates.
   * @param delayMs Delay, in ms, before starting to get the data.
   */
  private continuallyUpdateData(delayMs: number) {
    // Cancel if the service is busy, but not if the initial check is being performed.
    if (this.working && this.lastServiceState !== VpnServiceStates.PerformingInitialCheck) {
      return;
    }

    if (this.continuousUpdateSubscription) {
      this.continuousUpdateSubscription.unsubscribe();
    }

    this.continuousUpdateSubscription = of(0).pipe(
      delay(delayMs),
      mergeMap(() => this.getVpnClientState()),
      retryWhen(errors => concat(
        // During the initial check, retry only a few times.
        errors.pipe(delay(this.standardWaitTime), take(this.lastServiceState === VpnServiceStates.PerformingInitialCheck ? 5 : 1000000000)),
        throwError('')
      )),
    ).subscribe(appData => {
      if (appData) {
        // Remove the busy state of the initial check.
        if (this.lastServiceState === VpnServiceStates.PerformingInitialCheck) {
          this.working = false;
        }

        // Check if the server PK was changed externally.
        this.vpnSavedDataService.compareCurrentServer(appData.serverPk);

        // Update the data and send the event.
        if (appData.running) {
          this.lastServiceState = VpnServiceStates.Running;
        } else {
          this.lastServiceState = VpnServiceStates.Off;
        }
        this.currentEventData.vpnClientAppData = appData;
        this.currentEventData.updateDate = Date.now();
        this.sendUpdate();
      } else if (this.lastServiceState === VpnServiceStates.PerformingInitialCheck) {
        // Go to the error page, as it was not possible to connect with the local visor.
        this.router.navigate(['vpn', 'unavailable']);
        this.nodeKey = null;
      }

      // Program the next update.
      this.continuallyUpdateData(this.standardWaitTime);
    }, () => {
      // Go to the error page, as it was not possible to connect with the local visor.
      this.router.navigate(['vpn', 'unavailable']);
      this.nodeKey = null;
    });
  }

  /**
   * Makes the service stop continually updating the VPN state.
   */
  stopContinuallyUpdatingData() {
    if (this.continuousUpdateSubscription) {
      this.continuousUpdateSubscription.unsubscribe();
    }
  }

  /**
   * Gets the current state of the VPN.
   */
  private getVpnClientState(): Observable<VpnClientAppData> {
    let vpnClientData: VpnClientAppData;

    // Get the basic info about the local visor.
    return this.apiService.get(`visors/${this.nodeKey}/summary`).pipe(mergeMap(nodeInfo => {
      let appData: any;

      // Get the data of the VPN client app.
      if (nodeInfo && nodeInfo.overview && nodeInfo.overview.apps && (nodeInfo.overview.apps as any[]).length > 0) {
        (nodeInfo.overview.apps as any[]).forEach(value => {
          if (value.name === this.vpnClientAppName) {
            appData = value;
          }
        });
      }

      // Get the required data from the app properties.
      if (appData) {
        vpnClientData = new VpnClientAppData();
        vpnClientData.running = appData.status !== 0;

        vpnClientData.appState = AppState.Stopped;
        if (appData.detailed_status === AppState.Connecting) {
          vpnClientData.appState = AppState.Connecting;
        } else if (appData.detailed_status === AppState.Running) {
          vpnClientData.appState = AppState.Running;
        } else if (appData.detailed_status === AppState.ShuttingDown) {
          vpnClientData.appState = AppState.ShuttingDown;
        } else if (appData.detailed_status === AppState.Reconnecting) {
          vpnClientData.appState = AppState.Reconnecting;
        }

        vpnClientData.killswitch = false;

        if (appData.args && appData.args.length > 0) {
          for (let i = 0; i < appData.args.length; i++) {
            if (appData.args[i] === '-srv' && i + 1 < appData.args.length) {
              vpnClientData.serverPk = appData.args[i + 1];
            }

            if (appData.args[i].toLowerCase().includes('-killswitch')) {
              vpnClientData.killswitch = (appData.args[i] as string).toLowerCase().includes('true');
            }
          }
        }
      }

      // Get the min hops value.
      vpnClientData.minHops = nodeInfo.min_hops ? nodeInfo.min_hops : 0;

      // Get the data transmission data, is the app is running.
      if (vpnClientData && vpnClientData.running) {
        return this.apiService.get(`visors/${this.nodeKey}/apps/${this.vpnClientAppName}/connections`);
      }

      return of(null);
    }), map((connectionsInfo: any[]) => {
      // If data transmission data was received, process it.
      if (connectionsInfo && connectionsInfo.length > 0) {
        const vpnClientConnectionsData = new VpnClientConnectionsData();
        // Get the data from each connection. some data are averaged and some are added.
        connectionsInfo.forEach(connection => {
          vpnClientConnectionsData.latency += connection.latency / connectionsInfo.length;
          vpnClientConnectionsData.uploadSpeed += connection.upload_speed / connectionsInfo.length;
          vpnClientConnectionsData.downloadSpeed += connection.download_speed / connectionsInfo.length;
          vpnClientConnectionsData.totalUploaded += connection.bandwidth_sent;
          vpnClientConnectionsData.totalDownloaded += connection.bandwidth_received;
        });

        // If the server was changed, reset the history data.
        if (!this.connectionHistoryPk || this.connectionHistoryPk !== vpnClientData.serverPk) {
          this.connectionHistoryPk = vpnClientData.serverPk;

          this.uploadSpeedHistory = [0, 0, 0, 0, 0, 0, 0, 0, 0, 0];
          this.downloadSpeedHistory = [0, 0, 0, 0, 0, 0, 0, 0, 0, 0];
          this.latencyHistory = [0, 0, 0, 0, 0, 0, 0, 0, 0, 0];
        }

        // Clean the data.
        vpnClientConnectionsData.latency = Math.round(vpnClientConnectionsData.latency);
        vpnClientConnectionsData.uploadSpeed = Math.round(vpnClientConnectionsData.uploadSpeed);
        vpnClientConnectionsData.downloadSpeed = Math.round(vpnClientConnectionsData.downloadSpeed);
        vpnClientConnectionsData.totalUploaded = Math.round(vpnClientConnectionsData.totalUploaded);
        vpnClientConnectionsData.totalDownloaded = Math.round(vpnClientConnectionsData.totalDownloaded);

        // Update the data history arrays with the recent values.

        this.uploadSpeedHistory.splice(0, 1);
        this.uploadSpeedHistory.push(vpnClientConnectionsData.uploadSpeed);
        vpnClientConnectionsData.uploadSpeedHistory = this.uploadSpeedHistory;

        this.downloadSpeedHistory.splice(0, 1);
        this.downloadSpeedHistory.push(vpnClientConnectionsData.downloadSpeed);
        vpnClientConnectionsData.downloadSpeedHistory = this.downloadSpeedHistory;

        this.latencyHistory.splice(0, 1);
        this.latencyHistory.push(vpnClientConnectionsData.latency);
        vpnClientConnectionsData.latencyHistory = this.latencyHistory;

        vpnClientData.connectionData = vpnClientConnectionsData;
      }

      return vpnClientData;
    }));
  }

  /**
   * Sends an update about the state of the VPN. It automatically sets the serviceState and
   * busy properties of currentEventData.
   */
  private sendUpdate() {
    this.currentEventData.serviceState = this.lastServiceState;
    this.currentEventData.busy = this.working;
    this.stateSubject.next(this.currentEventData);
  }
}
