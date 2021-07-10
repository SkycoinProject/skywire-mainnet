import { Component, Input } from '@angular/core';
import { MatDialog } from '@angular/material/dialog';

import { Node } from '../../../../../app.datatypes';
import { EditLabelComponent } from 'src/app/components/layout/edit-label/edit-label.component';
import { NodeComponent } from '../../node.component';
import TimeUtils, { ElapsedTime } from 'src/app/utils/timeUtils';
import { LabeledElementTypes, StorageService } from 'src/app/services/storage.service';
import { NodeService, HealthStatus } from 'src/app/services/node.service';
import { RouterConfigComponent, RouterConfigParams } from './router-config/router-config.component';

/**
 * Shows the basic info of a node.
 */
@Component({
  selector: 'app-node-info-content',
  templateUrl: './node-info-content.component.html',
  styleUrls: ['./node-info-content.component.scss']
})
export class NodeInfoContentComponent {
  @Input() set nodeInfo(val: Node) {
    this.node = val;
    this.nodeHealthInfo = this.nodeService.getHealthStatus(val);
    this.timeOnline = TimeUtils.getElapsedTime(val.secondsOnline);
  }

  node: Node;
  nodeHealthInfo: HealthStatus;
  timeOnline: ElapsedTime;

  constructor(
    private dialog: MatDialog,
    public storageService: StorageService,
    private nodeService: NodeService,
  ) { }

  showEditLabelDialog() {
    let labelInfo =  this.storageService.getLabelInfo(this.node.localPk);
    if (!labelInfo) {
      labelInfo = {
        id: this.node.localPk,
        label: '',
        identifiedElementType: LabeledElementTypes.Node,
      };
    }

    EditLabelComponent.openDialog(this.dialog, labelInfo).afterClosed().subscribe((changed: boolean) => {
      if (changed) {
        NodeComponent.refreshCurrentDisplayedData();
      }
    });
  }

  changeRouterConfig() {
    const params: RouterConfigParams = {nodePk: this.node.localPk, minHops: this.node.minHops};
    RouterConfigComponent.openDialog(this.dialog, params).afterClosed().subscribe((changed: boolean) => {
      if (changed) {
        NodeComponent.refreshCurrentDisplayedData();
      }
    });
  }
}
