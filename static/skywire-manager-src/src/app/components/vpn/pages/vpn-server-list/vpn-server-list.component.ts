import { Component, OnDestroy } from '@angular/core';
import { Subscription, Observable } from 'rxjs';
import { MatDialog } from '@angular/material/dialog';
import { Router, ActivatedRoute } from '@angular/router';
import { TranslateService } from '@ngx-translate/core';

import { SortingModes, SortingColumn, DataSorter } from 'src/app/utils/lists/data-sorter';
import { DataFilterer } from 'src/app/utils/lists/data-filterer';
import { FilterProperties, FilterFieldTypes, PrintableLabel } from 'src/app/utils/filters';
import { countriesList } from 'src/app/utils/countries-list';
import { VpnClientDiscoveryService, VpnServer } from 'src/app/services/vpn-client-discovery.service';
import { VpnHelpers } from '../../vpn-helpers';
import { VpnClientService } from 'src/app/services/vpn-client.service';
import { SnackbarService } from 'src/app/services/snackbar.service';
import { AddVpnServerComponent } from './add-vpn-server/add-vpn-server.component';
import { VpnSavedDataService, LocalServerData, ServerFlags } from 'src/app/services/vpn-saved-data.service';
import GeneralUtils from 'src/app/utils/generalUtils';
import { EnterVpnServerPasswordComponent } from './enter-vpn-server-password/enter-vpn-server-password.component';

/**
 * Server lists VpnServerListComponent can show.
 */
export enum Lists {
  /**
   * List of public servers obtained from the discovery service.
   */
  Public = 'public',
  /**
   * List of the last servers to which the app has been connected.
   */
  History = 'history',
  /**
   * List with the favorite servers.
   */
  Favorites = 'favorites',
  /**
   * List with the blocked servers.
   */
  Blocked = 'blocked',
}

/**
 * Data of a server for being shown in the list. Combines the data provided by the discovery
 * service and the data saved locally.
 */
interface VpnServerForList {
  /**
   * 2 letter code of the country the server is in.
   */
  countryCode: string;
  /**
   * Name of the country.
   */
  countryName: string;
  /**
   * Sever name, obtained from the discovery service.
   */
  name: string;
  /**
   * Custom name set by the user.
   */
  customName: string;
  /**
   * Location of the server, obtained from the discovery service.
   */
  location: string;
  /**
   * Public key.
   */
  pk: string;


  // TODO: for currently commented columns, must be deleted or reactivated depending on what
  // happens to the columns.
  /**
   * Current congestion of the server, obtained from the discovery service.
   */
  // congestion?: number;
  /**
   * Rating of the congestion the server normally has, obtained from the discovery service.
   */
  // congestionRating?: Ratings;
  /**
   * Latency of the server, obtained from the discovery service.
   */
  // latency?: number;
  /**
   * Rating of the latency the server normally has, obtained from the discovery service.
   */
  // latencyRating?: Ratings;
  /**
   * Hops needed for reaching the server.
   */
  // hops?: number;


  /**
   * Note with information about the server, obtained from the discovery service.
   */
  note: string;
  /**
   * Personal note added by the user.
   */
  personalNote: string;
  /**
   * Last moment in which the VPN was connected to the server.
   */
  lastUsed?: number;
  /**
   * If the server is in the history of recently used servers.
   */
  inHistory?: boolean;
  /**
   * Special condition the server may have.
   */
  flag?: ServerFlags;
  /**
   * If the last time the server was used it was used with a password.
   */
  usedWithPassword?: boolean;
  /**
   * If the server was entered manually, at least one time.
   */
  enteredManually?: boolean;

  /**
   * Original VpnServer instance used for creating this object. Is not set if the objet was not
   * created using a VpnServer instance.
   */
  originalDiscoveryData?: VpnServer;
  /**
   * Original LocalServerData instance used for creating this object. Is not set if the objet
   * was not created using a LocalServerData instance.
   */
  originalLocalData?: LocalServerData;
}

/**
 * Page for showing the vpn server lists.
 */
@Component({
  selector: 'app-vpn-server-list',
  templateUrl: './vpn-server-list.component.html',
  styleUrls: ['./vpn-server-list.component.scss'],
})
export class VpnServerListComponent implements OnDestroy {
  // Small text for identifying the list, needed for the helper objects.
  private listId: string;

  // How many elements can be shown per page.
  private readonly maxFullListElements = 50;

  // Vars with the data of the columns used for sorting the data.
  dateSortData = new SortingColumn(['lastUsed'], 'vpn.server-list.date-small-table-label', SortingModes.NumberReversed);
  countrySortData = new SortingColumn(['countryName'], 'vpn.server-list.country-small-table-label', SortingModes.Text);
  nameSortData = new SortingColumn(['name'], 'vpn.server-list.name-small-table-label', SortingModes.Text);
  locationSortData = new SortingColumn(['location'], 'vpn.server-list.location-small-table-label', SortingModes.Text);
  pkSortData = new SortingColumn(['pk'], 'vpn.server-list.public-key-small-table-label', SortingModes.Text);
  noteSortData = new SortingColumn(['note'], 'vpn.server-list.note-small-table-label', SortingModes.Text);
  /*
  // TODO: for currently commented columns, must be deleted or reactivated depending on what
  // happens to the columns.
  congestionSortData = new SortingColumn(['congestion'], 'vpn.server-list.congestion-small-table-label', SortingModes.Number);
  congestionRatingSortData = new SortingColumn(
    ['congestionRating'],
    'vpn.server-list.congestion-rating-small-table-label',
    SortingModes.Number
  );
  latencySortData = new SortingColumn(['latency'], 'vpn.server-list.latency-small-table-label', SortingModes.Number);
  latencyRatingSortData = new SortingColumn(['latencyRating'], 'vpn.server-list.latency-rating-small-table-label', SortingModes.Number);
  hopsSortData = new SortingColumn(['hops'], 'vpn.server-list.hops-small-table-label', SortingModes.Number);
  */

  private dataSortedSubscription: Subscription;
  private dataFiltererSubscription: Subscription;
  // Objects in charge of sorting and filtering the data.
  dataSorter: DataSorter;
  dataFilterer: DataFilterer;

  // If the server list is being loaded.
  loading = true;
  // If the app is still loading the data about the local visor. Must be true for showing the list.
  loadingBackendData = true;
  // Data for populating the list.
  dataSource: VpnServerForList[];
  // Data for populating the tabs of the top bar.
  tabsData = VpnHelpers.vpnTabsData;

  // Vars for the pagination functionality.
  allServers: VpnServerForList[];
  filteredServers: VpnServerForList[];
  serversToShow: VpnServerForList[];
  numberOfPages = 1;
  currentPage = 1;
  // Used as a helper var, as the URL is read asynchronously.
  currentPageInUrl = 1;

  // Pk of the local visor.
  currentLocalPk: string;
  // List currently being shown. It also means which tab is currently selected in the lower tab bar.
  currentList = Lists.Public;
  // Currently selected server.
  currentServer: LocalServerData;
  // If the VPN is currently running.
  vpnRunning = false;

  serverFlags = ServerFlags;
  lists = Lists;

  // Array with the properties of the columns that can be used for filtering the data.
  filterProperties: FilterProperties[];

  private initialLoadStarted = false;

  private navigationsSubscription: Subscription;
  private dataSubscription: Subscription;
  private currentServerSubscription: Subscription;
  private backendDataSubscription: Subscription;

  constructor(
    private dialog: MatDialog,
    private router: Router,
    private translateService: TranslateService,
    private route: ActivatedRoute,
    private vpnClientDiscoveryService: VpnClientDiscoveryService,
    private vpnClientService: VpnClientService,
    private vpnSavedDataService: VpnSavedDataService,
    private snackbarService: SnackbarService,
  ) {
    this.navigationsSubscription = route.paramMap.subscribe(params => {
      // Get which list must be shown.
      if (params.has('type')) {
        if (params.get('type') === Lists.Favorites) {
          this.currentList = Lists.Favorites;
          this.listId = 'vfs';
        } else if (params.get('type') === Lists.Blocked) {
          this.currentList = Lists.Blocked;
          this.listId = 'vbs';
        } else if (params.get('type') === Lists.History) {
          this.currentList = Lists.History;
          this.listId = 'vhs';
        } else {
          this.currentList = Lists.Public;
          this.listId = 'vps';
        }
      } else {
        this.currentList = Lists.Public;
        this.listId = 'vps';
      }

      // Ensure the currently selected tab is the one that will be openen when returning to
      // the server list.
      VpnHelpers.setDefaultTabForServerList(this.currentList);

      // Get the PK of the current local visor.
      if (params.has('key')) {
        this.currentLocalPk = params.get('key');
        VpnHelpers.changeCurrentPk(this.currentLocalPk);
        this.tabsData = VpnHelpers.vpnTabsData;
      }

      // Get the requested page.
      if (params.has('page')) {
        let selectedPage = Number.parseInt(params.get('page'), 10);
        if (isNaN(selectedPage) || selectedPage < 1) {
          selectedPage = 1;
        }

        this.currentPageInUrl = selectedPage;
        this.recalculateElementsToShow();
      }

      // Load the data, if needed.
      if (!this.initialLoadStarted) {
        this.initialLoadStarted = true;
        this.loadData();
      }
    });

    this.currentServerSubscription = this.vpnSavedDataService.currentServerObservable.subscribe(server => this.currentServer = server);

    this.backendDataSubscription = this.vpnClientService.backendState.subscribe(data => {
      if (data) {
        this.loadingBackendData = false;
        this.vpnRunning = data.vpnClientAppData.running;
      }
    });
  }

  ngOnDestroy() {
    this.navigationsSubscription.unsubscribe();
    this.currentServerSubscription.unsubscribe();
    this.backendDataSubscription.unsubscribe();

    if (this.dataSortedSubscription) {
      this.dataSortedSubscription.unsubscribe();
    }
    if (this.dataFiltererSubscription) {
      this.dataFiltererSubscription.unsubscribe();
    }
    if (this.dataSubscription) {
      this.dataSubscription.unsubscribe();
    }

    if (this.dataFilterer) {
      this.dataFilterer.dispose();
    }
    if (this.dataSorter) {
      this.dataSorter.dispose();
    }
  }

  /**
   * Opens the modal window for entering the server data manually.
   */
  enterManually() {
    AddVpnServerComponent.openDialog(this.dialog, this.currentLocalPk);
  }

  /**
   * Returns the translatable var that must be used for showing the notes of a server.
   * If there is only one note, the note itself is returned.
   */
  getNoteVar(server: VpnServerForList): string {
    if (server.note && server.personalNote) {
      return 'vpn.server-list.notes-info';
    } else if (!server.note && server.personalNote) {
      return server.personalNote;
    }

    return server.note;
  }

  /**
   * Selects a server and starts the process for connecting to it.
   */
  selectServer(server: VpnServerForList) {
    const savedVersion = this.vpnSavedDataService.getSavedVersion(server.pk, true);

    // Close any previous temporary loading error msg.
    this.snackbarService.closeCurrentIfTemporaryError();

    if (!savedVersion || savedVersion.flag !== ServerFlags.Blocked) {
      // To prevent overriding any password, if the currently selected server is selected again,
      // the case is managed here.
      if (this.currentServer.pk === server.pk) {
        if (this.vpnRunning) {
          // Inform that the VPN is already connected to the server.
          this.snackbarService.showWarning('vpn.server-change.already-selected-warning');
        } else {
          // Ask for confirmation for starting the VPN.
          const confirmationDialog = GeneralUtils.createConfirmationDialog(this.dialog, 'vpn.server-change.start-same-server-confirmation');
          confirmationDialog.componentInstance.operationAccepted.subscribe(() => {
            confirmationDialog.componentInstance.closeModal();

            this.vpnClientService.start();
            VpnHelpers.redirectAfterServerChange(this.router, null, this.currentLocalPk);
          });
        }

        return;
      }

      // If the server was previously used with a password, ask for it.
      if (savedVersion && savedVersion.usedWithPassword) {
        EnterVpnServerPasswordComponent.openDialog(this.dialog, true).afterClosed().subscribe((password: string) => {
          // Continue only if the user did not cancel the operation.
          if (password) {
            this.makeServerChange(server, password === '-' ? null : password.substr(1));
          }
        });

        return;
      }

      this.makeServerChange(server, null);
    } else {
      this.snackbarService.showError('vpn.starting-blocked-server-error', {}, true);
    }
  }

  /**
   * Changes the currently selected server and connects to the new one after that.
   */
  private makeServerChange(server: VpnServerForList, password: string) {
    VpnHelpers.processServerChange(
      this.router,
      this.vpnClientService,
      this.vpnSavedDataService,
      this.snackbarService,
      this.dialog,
      null,
      this.currentLocalPk,
      server.originalLocalData,
      server.originalDiscoveryData,
      null,
      password,
    );
  }

  /**
   * Opens the options modal window for a specific server.
   */
  openOptions(server: VpnServerForList) {
    let savedVersion = this.vpnSavedDataService.getSavedVersion(server.pk, true);
    if (!savedVersion) {
      savedVersion = this.vpnSavedDataService.processFromDiscovery(server.originalDiscoveryData);
    }
    if (!savedVersion) {
      // This should not happen.
      this.snackbarService.showError('vpn.unexpedted-error');

      return;
    }

    VpnHelpers.openServerOptions(
      savedVersion,
      this.router,
      this.vpnSavedDataService,
      this.vpnClientService,
      this.snackbarService,
      this.dialog
    ).subscribe(changesMade => {
      if (changesMade) {
        // Update the data shown in the UI.
        this.processAllServers();
      }
    });
  }

  /**
   * Loads the server list.
   */
  private loadData() {
    if (this.currentList === Lists.Public) {
      // Get the vpn servers from the discovery service.
      this.dataSubscription = this.vpnClientDiscoveryService.getServers().subscribe(response => {
        // Process the result.
        this.allServers = response.map(server => {
          return {
            countryCode: server.countryCode,
            countryName: this.getCountryName(server.countryCode),
            name: server.name,
            customName: null,
            location: server.location,
            pk: server.pk,
            note: server.note,
            personalNote: null,

            /*
            // TODO: for currently commented columns, must be deleted or reactivated depending on
            // what happens to the columns.
            congestion: server.congestion,
            congestionRating: server.congestionRating,
            latency: server.latency,
            latencyRating: server.latencyRating,
            hops: server.hops,
            */

            originalDiscoveryData: server,
          };
        });

        // Update the data in the saved versions of the servers.
        this.vpnSavedDataService.updateFromDiscovery(response);

        this.loading = false;
        this.processAllServers();
      });
    } else {
      let dataObservable: Observable<LocalServerData[]>;

      // Get the requested data.
      if (this.currentList === Lists.History) {
        dataObservable = this.vpnSavedDataService.history;
      } else if (this.currentList === Lists.Favorites) {
        dataObservable = this.vpnSavedDataService.favorites;
      } else {
        dataObservable = this.vpnSavedDataService.blocked;
      }

      this.dataSubscription = dataObservable.subscribe(response => {
        // Process the result.
        const processedList: VpnServerForList[] = [];
        response.forEach(server => {
          processedList.push({
            countryCode: server.countryCode,
            countryName: this.getCountryName(server.countryCode),
            name: server.name,
            customName: null,
            location: server.location,
            pk: server.pk,
            note: server.note,
            personalNote: null,
            lastUsed: server.lastUsed,
            inHistory: server.inHistory,
            flag: server.flag,

            originalLocalData: server,
          });
        });

        this.allServers = processedList;

        this.loading = false;
        this.processAllServers();
      });
    }
  }

  /**
   * Makes preparations for the page to work well with the obtained server list.
   */
  private processAllServers() {
    this.fillFilterPropertiesArray();

    // Create a list with the countries on the server list. Also, add all saved data to each
    // server.
    const countriesSet = new Set<string>();
    this.allServers.forEach((server, i) => {
      countriesSet.add(server.countryCode);

      // Add the saved data, if any.
      const saveddata = this.vpnSavedDataService.getSavedVersion(server.pk, i === 0);
      server.customName = saveddata ? saveddata.customName : null;
      server.personalNote = saveddata ? saveddata.personalNote : null;
      server.inHistory = saveddata ? saveddata.inHistory : false;
      server.flag = saveddata ? saveddata.flag : ServerFlags.None;
      server.enteredManually = saveddata ? saveddata.enteredManually : false;
      server.usedWithPassword = saveddata ? saveddata.usedWithPassword : false;
    });

    // Create a filter option for each country.
    let countriesFilteringLabels: PrintableLabel[] = [];
    countriesSet.forEach(v => {
      countriesFilteringLabels.push({
        label: this.getCountryName(v),
        value: v,
        image: '/assets/img/big-flags/' + v.toLowerCase() + '.png',
      });
    });

    // Sort the countries list and add an empty option at the top.
    countriesFilteringLabels.sort((a, b) => a.label.localeCompare(b.label));
    countriesFilteringLabels = [{
      label: 'vpn.server-list.filter-dialog.country-options.any',
      value: ''
    }].concat(countriesFilteringLabels);

    // Include the option for filtering by country.
    const countryFilter: FilterProperties = {
      filterName: 'vpn.server-list.filter-dialog.country',
      keyNameInElementsArray: 'countryCode',
      type: FilterFieldTypes.Select,
      printableLabelsForValues: countriesFilteringLabels,
      printableLabelGeneralSettings: {
        defaultImage: '/assets/img/big-flags/unknown.png',
        imageWidth: 20,
        imageHeight: 15,
      }
    };
    this.filterProperties = [countryFilter].concat(this.filterProperties);

    // Initialize the data sorter.
    const sortableColumns: SortingColumn[] = [];
    let defaultColumn: number;
    let tieBreakerColumn: number;
    if (this.currentList === Lists.Public) {
      sortableColumns.push(this.countrySortData);
      sortableColumns.push(this.nameSortData);
      sortableColumns.push(this.locationSortData);
      sortableColumns.push(this.pkSortData);
      sortableColumns.push(this.noteSortData);
      /*
      // TODO: for currently commented columns, must be deleted or reactivated depending on
      // what happens to the columns.
      sortableColumns.push(this.congestionSortData);
      sortableColumns.push(this.congestionRatingSortData);
      sortableColumns.push(this.latencySortData);
      sortableColumns.push(this.latencyRatingSortData);
      sortableColumns.push(this.hopsSortData);
      */

      defaultColumn = 0;
      tieBreakerColumn = 1;
    } else {
      if (this.currentList === Lists.History) {
        sortableColumns.push(this.dateSortData);
      }

      sortableColumns.push(this.countrySortData);
      sortableColumns.push(this.nameSortData);
      sortableColumns.push(this.locationSortData);
      sortableColumns.push(this.pkSortData);
      sortableColumns.push(this.noteSortData);

      defaultColumn = this.currentList === Lists.History ? 0 : 1;
      tieBreakerColumn = this.currentList === Lists.History ? 2 : 3;
    }
    this.dataSorter = new DataSorter(this.dialog, this.translateService, sortableColumns, defaultColumn, this.listId);
    this.dataSorter.setTieBreakerColumnIndex(tieBreakerColumn);
    this.dataSortedSubscription = this.dataSorter.dataSorted.subscribe(() => {
      // When this happens, the data in allServers has already been sorted.
      this.recalculateElementsToShow();
    });

    // Initialize the data filterer.
    this.dataFilterer = new DataFilterer(this.dialog, this.route, this.router, this.filterProperties, this.listId);
    this.dataFiltererSubscription = this.dataFilterer.dataFiltered.subscribe(data => {
      this.filteredServers = data;
      this.dataSorter.setData(this.filteredServers);
    });

    // Remove the blocked servers, if needed.
    let serversToUse: VpnServerForList[];
    if (this.currentList === Lists.Public) {
      serversToUse = this.allServers.filter(server => server.flag !== ServerFlags.Blocked);
    } else {
      serversToUse = this.allServers;
    }

    this.dataFilterer.setData(serversToUse);
  }

  /**
   * Fills the array with the properties that the data filterer must use, depending on the
   * list that will be shown.
   */
  private fillFilterPropertiesArray() {
    this.filterProperties = [
      {
        filterName: 'vpn.server-list.filter-dialog.name',
        keyNameInElementsArray: 'name',
        secondaryKeyNameInElementsArray: 'customName',
        type: FilterFieldTypes.TextInput,
        maxlength: 100,
      },
      {
        filterName: 'vpn.server-list.filter-dialog.location',
        keyNameInElementsArray: 'location',
        type: FilterFieldTypes.TextInput,
        maxlength: 100,
      },
      {
        filterName: 'vpn.server-list.filter-dialog.public-key',
        keyNameInElementsArray: 'pk',
        type: FilterFieldTypes.TextInput,
        maxlength: 100,
      }
    ];
    /*
    // TODO: for currently commented columns, must be deleted or reactivated depending on
    // what happens to the columns.
    if (this.currentList === Lists.Public) {
      this.filterProperties.push({
        filterName: 'vpn.server-list.filter-dialog.congestion-rating',
        keyNameInElementsArray: 'congestionRating',
        type: FilterFieldTypes.Select,
        printableLabelsForValues: [
          {
            value: '',
            label: 'vpn.server-list.filter-dialog.rating-options.any',
          },
          {
            value: Ratings.Gold + '',
            label: 'vpn.server-list.filter-dialog.rating-options.gold',
          },
          {
            value: Ratings.Silver + '',
            label: 'vpn.server-list.filter-dialog.rating-options.silver',
          },
          {
            value: Ratings.Bronze + '',
            label: 'vpn.server-list.filter-dialog.rating-options.bronze',
          }
        ],
      });

      this.filterProperties.push({
        filterName: 'vpn.server-list.filter-dialog.latency-rating',
        keyNameInElementsArray: 'latencyRating',
        type: FilterFieldTypes.Select,
        printableLabelsForValues: [
          {
            value: '',
            label: 'vpn.server-list.filter-dialog.rating-options.any',
          },
          {
            value: Ratings.Gold + '',
            label: 'vpn.server-list.filter-dialog.rating-options.gold',
          },
          {
            value: Ratings.Silver + '',
            label: 'vpn.server-list.filter-dialog.rating-options.silver',
          },
          {
            value: Ratings.Bronze + '',
            label: 'vpn.server-list.filter-dialog.rating-options.bronze',
          }
        ],
      });
    }
    */
  }

  /**
   * Recalculates which elements should be shown on the list, mainly related to the pagination.
   */
  private recalculateElementsToShow() {
    // Needed to prevent race conditions.
    this.currentPage = this.currentPageInUrl;

    // Needed to prevent race conditions.
    if (this.filteredServers) {
      // Calculate the pagination values.
      const maxElements = this.maxFullListElements;
      this.numberOfPages = Math.ceil(this.filteredServers.length / maxElements);
      if (this.currentPage > this.numberOfPages) {
        this.currentPage = this.numberOfPages;
      }

      // Limit the elements to show.
      const start = maxElements * (this.currentPage - 1);
      const end = start + maxElements;
      this.serversToShow = this.filteredServers.slice(start, end);
    } else {
      this.serversToShow = null;
    }

    this.dataSource = this.serversToShow;
  }

  /**
   * Gets the full name of a country.
   * @param countryCode 2 letter code of the country.
   */
  getCountryName(countryCode: string): string {
    return countriesList[countryCode.toUpperCase()] ? countriesList[countryCode.toUpperCase()] : countryCode;
  }


  // TODO: the functions below are for currently commented columns, must be deleted or reactivated
  // depending on what happens to the columns.

  /**
   * Gets the name of the translatable var that must be used for showing a latency value. This
   * allows to add the correct measure suffix.
   */
  /*
  getLatencyValueString(latency: number): string {
    return VpnHelpers.getLatencyValueString(latency);
  }
  */

  /**
   * Gets the string value to show in the UI a latency value with an adecuate number of decimals.
   * This function converts the value from ms to segs, if appropriate, so the value must be shown
   * using the var returned by getLatencyValueString.
   */
  /*
  getPrintableLatency(latency: number): string {
    return VpnHelpers.getPrintableLatency(latency);
  }
  */

  /**
   * Gets the class that must be used for showing the color of a congestion value.
   */
  /*
  getCongestionTextColorClass(congestion: number): string {
    if (congestion < 60) {
      return 'green-value';
    } else if (congestion < 90) {
      return 'yellow-value';
    }

    return 'red-value';
  }
  */

  /**
   * Gets the class that must be used for showing the color of a latency value.
   */
  /*
  getLatencyTextColorClass(latency: number): string {
    if (latency < 200) {
      return 'green-value';
    } else if (latency < 350) {
      return 'yellow-value';
    }

    return 'red-value';
  }
  */

  /**
   * Gets the class that must be used for showing the color of a hops value.
   */
  /*
  getHopsTextColorClass(hops: number): string {
    if (hops < 5) {
      return 'green-value';
    } else if (hops < 9) {
      return 'yellow-value';
    }

    return 'red-value';
  }
  */

  /**
   * Returns the name of the image that must be shown for a rating value.
   */
  /*
  getRatingIcon(rating: Ratings): string {
    if (rating === Ratings.Gold) {
      return 'gold-rating';
    } else if (rating === Ratings.Silver) {
      return 'silver-rating';
    }

    return 'bronze-rating';
  }
  */

  /**
   * Returns the translatable var for describing a rating value.
   */
  /*
  getRatingText(rating: Ratings): string {
    if (rating === Ratings.Gold) {
      return 'vpn.server-list.gold-rating-info';
    } else if (rating === Ratings.Silver) {
      return 'vpn.server-list.silver-rating-info';
    }

    return 'vpn.server-list.bronze-rating-info';
  }
  */
}
