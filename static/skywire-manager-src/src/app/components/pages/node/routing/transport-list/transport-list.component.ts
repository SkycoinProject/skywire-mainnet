import { Component, Input, OnDestroy } from '@angular/core';
import { MatDialog, MatDialogRef } from '@angular/material/dialog';
import { Observable, Subscription } from 'rxjs';
import { ActivatedRoute } from '@angular/router';

import { Transport } from '../../../../../app.datatypes';
import { CreateTransportComponent } from './create-transport/create-transport.component';
import { TransportService } from '../../../../../services/transport.service';
import { NodeComponent } from '../../node.component';
import { AppConfig } from '../../../../../app.config';
import { ConfirmationComponent } from '../../../../layout/confirmation/confirmation.component';
import GeneralUtils from '../../../../../utils/generalUtils';
import { TransportDetailsComponent } from './transport-details/transport-details.component';
import { SnackbarService } from '../../../../../services/snackbar.service';
import { SelectColumnComponent, SelectedColumn } from 'src/app/components/layout/select-column/select-column.component';
import { SelectableOption, SelectOptionComponent } from 'src/app/components/layout/select-option/select-option.component';
import { processServiceError } from 'src/app/utils/errors';
import { OperationError } from 'src/app/utils/operation-error';

/**
 * List of the columns that can be used to sort the data.
 */
enum SortableColumns {
  State = 'transports.state',
  Id = 'transports.id',
  RemotePk = 'transports.remote',
  Type = 'transports.type',
  Uploaded = 'common.uploaded',
  Downloaded = 'common.downloaded',
}

/**
 * Shows the list of transports of a node. I can be used to show a short preview, with just some
 * elements and a link for showing the rest: or the full list, with pagination controls.
 */
@Component({
  selector: 'app-transport-list',
  templateUrl: './transport-list.component.html',
  styleUrls: ['./transport-list.component.scss']
})
export class TransportListComponent implements OnDestroy {
  private static sortByInternal = SortableColumns.Id;
  private static sortReverseInternal = false;

  @Input() nodePK: string;

  // Vars for keeping track of the column used for sorting the data.
  sortableColumns = SortableColumns;
  get sortBy(): SortableColumns { return TransportListComponent.sortByInternal; }
  set sortBy(val: SortableColumns) { TransportListComponent.sortByInternal = val; }
  get sortReverse(): boolean { return TransportListComponent.sortReverseInternal; }
  set sortReverse(val: boolean) { TransportListComponent.sortReverseInternal = val; }
  get sortingArrow(): string {
    return this.sortReverse ? 'keyboard_arrow_up' : 'keyboard_arrow_down';
  }

  dataSource: Transport[];
  /**
   * Keeps track of the state of the check boxes of the elements.
   */
  selections = new Map<string, boolean>();

  /**
   * If true, the control can only show few elements and, if there are more elements, a link for
   * accessing the full list. If false, the full list is shown, with pagination
   * controls, if needed.
   */
  showShortList_: boolean;
  @Input() set showShortList(val: boolean) {
    this.showShortList_ = val;
    this.recalculateElementsToShow();
  }

  allTransports: Transport[];
  transportsToShow: Transport[];
  numberOfPages = 1;
  currentPage = 1;
  // Used as a helper var, as the URL is read asynchronously.
  currentPageInUrl = 1;
  @Input() set transports(val: Transport[]) {
    this.allTransports = val;
    this.recalculateElementsToShow();
  }

  private navigationsSubscription: Subscription;
  private operationSubscriptionsGroup: Subscription[] = [];

  constructor(
    private dialog: MatDialog,
    private transportService: TransportService,
    private route: ActivatedRoute,
    private snackbarService: SnackbarService,
  ) {
    this.navigationsSubscription = this.route.paramMap.subscribe(params => {
      if (params.has('page')) {
        let selectedPage = Number.parseInt(params.get('page'), 10);
        if (isNaN(selectedPage) || selectedPage < 1) {
          selectedPage = 1;
        }

        this.currentPageInUrl = selectedPage;

        this.recalculateElementsToShow();
      }
    });
  }

  ngOnDestroy() {
    this.navigationsSubscription.unsubscribe();
    this.operationSubscriptionsGroup.forEach(sub => sub.unsubscribe());
  }

  /**
   * Returns the scss class to be used to show the current status of the transport.
   * @param forDot If true, returns a class for creating a colored dot. If false,
   * returns a class for a colored text.
   */
  transportStatusClass(transport: Transport, forDot: boolean): string {
    switch (transport.is_up) {
      case true:
        return forDot ? 'dot-green' : 'green-text';
      default:
        return forDot ? 'dot-red' : 'red-text';
    }
  }

  /**
   * Returns the text to be used to indicate the current status of a transport.
   * @param forTooltip If true, returns a text for a tooltip. If false, returns a
   * text for the transport list shown on small screens.
   */
  transportStatusText(transport: Transport, forTooltip: boolean): string {
    switch (transport.is_up) {
      case true:
        return 'transports.statuses.online' + (forTooltip ? '-tooltip' : '');
      default:
        return 'transports.statuses.offline' + (forTooltip ? '-tooltip' : '');
    }
  }

  /**
   * Changes the selection state of an entry (modifies the state of its checkbox).
   */
  changeSelection(transport: Transport) {
    if (this.selections.get(transport.id)) {
      this.selections.set(transport.id, false);
    } else {
      this.selections.set(transport.id, true);
    }
  }

  /**
   * Check if at lest one entry has been selected via its checkbox.
   */
  hasSelectedElements(): boolean {
    if (!this.selections) {
      return false;
    }

    let found = false;
    this.selections.forEach((val) => {
      if (val) {
        found = true;
      }
    });

    return found;
  }

  /**
   * Selects or deselects all items.
   */
  changeAllSelections(setSelected: boolean) {
    this.selections.forEach((val, key) => {
      this.selections.set(key, setSelected);
    });
  }

  /**
   * Deletes the selected elements.
   */
  deleteSelected() {
    // Ask for confirmation.
    const confirmationDialog = GeneralUtils.createConfirmationDialog(this.dialog, 'transports.delete-selected-confirmation');

    confirmationDialog.componentInstance.operationAccepted.subscribe(() => {
      confirmationDialog.componentInstance.showProcessing();

      const elementsToRemove: string[] = [];
      this.selections.forEach((val, key) => {
        if (val) {
          elementsToRemove.push(key);
        }
      });

      this.deleteRecursively(elementsToRemove, confirmationDialog);
    });
  }

  /**
   * Shows the transport creation modal window.
   */
  create() {
    CreateTransportComponent.openDialog(this.dialog);
  }

  /**
   * Opens the modal window used on small screens with the options of an element.
   */
  showOptionsDialog(transport: Transport) {
    const options: SelectableOption[] = [
      {
        icon: 'visibility',
        label: 'transports.details.title',
      },
      {
        icon: 'close',
        label: 'transports.delete',
      }
    ];

    SelectOptionComponent.openDialog(this.dialog, options).afterClosed().subscribe((selectedOption: number) => {
      if (selectedOption === 1) {
        this.details(transport);
      } else if (selectedOption === 2) {
        this.delete(transport.id);
      }
    });
  }

  /**
   * Shows a modal window with the details of a transport.
   */
  details(transport: Transport) {
    TransportDetailsComponent.openDialog(this.dialog, transport);
  }

  /**
   * Deletes a specific element.
   */
  delete(id: string) {
    const confirmationDialog = GeneralUtils.createConfirmationDialog(this.dialog, 'transports.delete-confirmation');

    confirmationDialog.componentInstance.operationAccepted.subscribe(() => {
      confirmationDialog.componentInstance.showProcessing();

      // Start the operation and save it for posible cancellation.
      this.operationSubscriptionsGroup.push(this.startDeleting(id).subscribe(() => {
        confirmationDialog.close();
        // Make the parent page reload the data.
        NodeComponent.refreshCurrentDisplayedData();
        this.snackbarService.showDone('transports.deleted');
      }, (err: OperationError) => {
        err = processServiceError(err);
        confirmationDialog.componentInstance.showDone('confirmation.error-header-text', err.translatableErrorMsg);
      }));
    });
  }

  /**
   * Changes the column and/or order used for sorting the data.
   */
  changeSortingOrder(column: SortableColumns) {
    if (this.sortBy !== column) {
      this.sortBy = column;
      this.sortReverse = false;
    } else {
      this.sortReverse = !this.sortReverse;
    }

    this.recalculateElementsToShow();
  }

  /**
   * Opens the modal window used on small screens for selecting how to sort the data.
   */
  openSortingOrderModal() {
    // Get the list of sortable columns.
    const enumKeys = Object.keys(SortableColumns);
    const columnsMap = new Map<string, SortableColumns>();
    const columns = enumKeys.map(key => {
      const val = SortableColumns[key as any];
      columnsMap.set(val, SortableColumns[key]);

      return val;
    });

    SelectColumnComponent.openDialog(this.dialog, columns).afterClosed().subscribe((result: SelectedColumn) => {
      if (result) {
        if (columnsMap.has(result.label) && (result.sortReverse !== this.sortReverse || columnsMap.get(result.label) !== this.sortBy)) {
          this.sortBy = columnsMap.get(result.label);
          this.sortReverse = result.sortReverse;

          this.recalculateElementsToShow();
        }
      }
    });
  }

  /**
   * Sorts the data and recalculates which elements should be shown on the UI.
   */
  private recalculateElementsToShow() {
    // Needed to prevent racing conditions.
    this.currentPage = this.currentPageInUrl;

    // Needed to prevent racing conditions.
    if (this.allTransports) {
      // Sort all the data.
      this.allTransports.sort((a, b) => {
        const defaultOrder = a.id.localeCompare(b.id);

        let response: number;
        if (this.sortBy === SortableColumns.Id) {
          response = !this.sortReverse ? a.id.localeCompare(b.id) : b.id.localeCompare(a.id);
        } else if (this.sortBy === SortableColumns.State) {
          if (a.is_up && !b.is_up) {
            response = -1;
          } else if (!a.is_up && b.is_up) {
            response = 1;
          }
          response = response * (this.sortReverse ? -1 : 1);
        } else if (this.sortBy === SortableColumns.RemotePk) {
          response = !this.sortReverse ? a.remote_pk.localeCompare(b.remote_pk) : b.remote_pk.localeCompare(a.remote_pk);
        } else if (this.sortBy === SortableColumns.Type) {
          response = !this.sortReverse ? a.type.localeCompare(b.type) : b.type.localeCompare(a.type);
        } else if (this.sortBy === SortableColumns.Uploaded) {
          response = !this.sortReverse ? b.log.sent - a.log.sent : a.log.sent - b.log.sent;
        } else if (this.sortBy === SortableColumns.Downloaded) {
          response = !this.sortReverse ? b.log.recv - a.log.recv : a.log.recv - b.log.recv;
        } else {
          response = defaultOrder;
        }

        return response !== 0 ? response : defaultOrder;
      });

      // Calculate the pagination values.
      const maxElements = this.showShortList_ ? AppConfig.maxShortListElements : AppConfig.maxFullListElements;
      this.numberOfPages = Math.ceil(this.allTransports.length / maxElements);
      if (this.currentPage > this.numberOfPages) {
        this.currentPage = this.numberOfPages;
      }

      // Limit the elements to show.
      const start = maxElements * (this.currentPage - 1);
      const end = start + maxElements;
      this.transportsToShow = this.allTransports.slice(start, end);

      // Create a map with the elements to show, as a helper.
      const currentElementsMap = new Map<string, boolean>();
      this.transportsToShow.forEach(transport => {
        currentElementsMap.set(transport.id, true);

        // Add to the selections map the elements that are going to be shown.
        if (!this.selections.has(transport.id)) {
          this.selections.set(transport.id, false);
        }
      });

      // Remove from the selections map the elements that are not going to be shown.
      const keysToRemove: string[] = [];
      this.selections.forEach((value, key) => {
        if (!currentElementsMap.has(key)) {
          keysToRemove.push(key);
        }
      });
      keysToRemove.forEach(key => {
        this.selections.delete(key);
      });

    } else {
      this.transportsToShow = null;
      this.selections = new Map<string, boolean>();
    }

    this.dataSource = this.transportsToShow;
  }

  /**
   * Prepares the operation for deteling an element, but does not start it. To start the operation,
   * subscribe to the response.
   */
  private startDeleting(id: string): Observable<any> {
    return this.transportService.delete(NodeComponent.getCurrentNodeKey(), id);
  }

  /**
   * Recursively deletes a list of elements.
   * @param ids List with the IDs of the elements to delete.
   * @param confirmationDialog Dialog used for requesting confirmation from the user.
   */
  deleteRecursively(ids: string[], confirmationDialog: MatDialogRef<ConfirmationComponent, any>) {
    this.operationSubscriptionsGroup.push(this.startDeleting(ids[ids.length - 1]).subscribe(() => {
      ids.pop();
      if (ids.length === 0) {
        confirmationDialog.close();
        // Make the parent page reload the data.
        NodeComponent.refreshCurrentDisplayedData();
        this.snackbarService.showDone('transports.deleted');
      } else {
        this.deleteRecursively(ids, confirmationDialog);
      }
    }, (err: OperationError) => {
      NodeComponent.refreshCurrentDisplayedData();

      err = processServiceError(err);
      confirmationDialog.componentInstance.showDone('confirmation.error-header-text', err.translatableErrorMsg);
    }));
  }
}
