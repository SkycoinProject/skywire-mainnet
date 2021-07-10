import { Component, Inject, ViewChild, ElementRef, OnInit, OnDestroy } from '@angular/core';
import { MAT_DIALOG_DATA, MatDialogRef, MatDialogConfig, MatDialog } from '@angular/material/dialog';
import { FormGroup, FormBuilder, Validators } from '@angular/forms';
import { Subscription } from 'rxjs';

import { SnackbarService } from '../../../../../../services/snackbar.service';
import { AppConfig } from 'src/app/app.config';
import { Node } from '../../../../../../app.datatypes';
import { ButtonComponent } from 'src/app/components/layout/button/button.component';
import { OperationError } from 'src/app/utils/operation-error';
import { processServiceError } from 'src/app/utils/errors';
import { RouteService } from 'src/app/services/route.service';

/**
 * Params for RouterConfigComponent.
 */
export interface RouterConfigParams {
  /**
   * PK of the node.
   */
  nodePk: string;
  /**
   * Current value of the min hops property in the node.
   */
   minHops: number;
}

/**
 * Modal window for changing the configuration related to the router. It changes the values
 * and shows a confirmation msg by itself.
 */
@Component({
  selector: 'app-router-config',
  templateUrl: './router-config.component.html',
  styleUrls: ['./router-config.component.scss']
})
export class RouterConfigComponent implements OnInit, OnDestroy {
  @ViewChild('button') button: ButtonComponent;
  @ViewChild('firstInput') firstInput: ElementRef;

  form: FormGroup;

  private operationSubscription: Subscription;

  /**
   * Opens the modal window. Please use this function instead of opening the window "by hand".
   */
  public static openDialog(dialog: MatDialog, node: RouterConfigParams): MatDialogRef<RouterConfigComponent, any> {
    const config = new MatDialogConfig();
    config.data = node;
    config.autoFocus = false;
    config.width = AppConfig.smallModalWidth;

    return dialog.open(RouterConfigComponent, config);
  }

  constructor(
    private dialogRef: MatDialogRef<RouterConfigComponent>,
    @Inject(MAT_DIALOG_DATA) private data: RouterConfigParams,
    private formBuilder: FormBuilder,
    private snackbarService: SnackbarService,
    private routeService: RouteService,
  ) { }

  ngOnInit() {
    this.form = this.formBuilder.group({
      'min': [this.data.minHops, Validators.compose([
        Validators.required,
        Validators.maxLength(3),
        Validators.pattern('^[0-9]+$'),
      ])],
    });

    setTimeout(() => (this.firstInput.nativeElement as HTMLElement).focus());
  }

  ngOnDestroy() {
    if (this.operationSubscription) {
      this.operationSubscription.unsubscribe();
    }
  }

  save() {
    if (!this.form.valid || this.operationSubscription) {
      return;
    }

    this.button.showLoading();

    this.operationSubscription = this.routeService.setMinHops(
      this.data.nodePk,
      Number.parseInt(this.form.get('min').value, 10)
    ).subscribe({
      next: this.onSuccess.bind(this),
      error: this.onError.bind(this)
    });
  }

  private onSuccess(response: any) {
    this.dialogRef.close(true);
    this.snackbarService.showDone('router-config.done');
  }

  private onError(err: OperationError) {
    this.button.showError();
    this.operationSubscription = null;
    err = processServiceError(err);

    this.snackbarService.showError(err);
  }
}
