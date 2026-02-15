import { Component, Input } from '@angular/core';
import { RouterLink } from '@angular/router';
import { MatChipsModule } from '@angular/material/chips';
import { MatIconModule } from '@angular/material/icon';
import { MatDividerModule } from '@angular/material/divider';
import { MatTooltipModule } from '@angular/material/tooltip';
import { validateResource } from '../validators/resource.validator';

interface Statement {
  effect: string;
  actions: string[];
  resources: string[];
}

export interface PolicyInfo {
  id: string;
  name: string;
  account: string;
}

@Component({
  selector: 'app-statements-view',
  standalone: true,
  imports: [RouterLink, MatChipsModule, MatIconModule, MatDividerModule, MatTooltipModule],
  template: `
    @for (stmt of statements; track $index; let i = $index) {
      <div class="statement">
        <div class="statement-header">
          <div class="statement-label">Statement {{ i + 1 }}</div>
          @if (policyInfos && policyInfos[i]) {
            <a [routerLink]="getPolicyRoute(policyInfos[i])" class="policy-link">
              {{ policyInfos[i].name }}
              (<span [class.global-account]="policyInfos[i].account === '_global'">{{ policyInfos[i].account === '_global' ? 'global' : policyInfos[i].account }}</span>)
            </a>
          }
        </div>
        <div class="statement-content">
          <div class="chip-section">
            <span class="chip-label">Actions</span>
            <mat-chip-set>
              @for (action of stmt.actions; track action) {
                <mat-chip>{{ action }}</mat-chip>
              }
            </mat-chip-set>
          </div>
          <div class="chip-section">
            <span class="chip-label">Resources</span>
            <mat-chip-set>
              @for (resource of stmt.resources; track resource) {
                <mat-chip [class.invalid-resource]="getResourceError(resource)">
                  @if (getResourceError(resource)) {
                    <mat-icon matChipAvatar class="warning-icon"
                              [matTooltip]="getResourceError(resource)!">warning</mat-icon>
                  }
                  {{ resource }}
                </mat-chip>
              }
            </mat-chip-set>
          </div>
        </div>
      </div>
      @if (!$last) {
        <mat-divider></mat-divider>
      }
    }
  `,
  styles: [`
    .statement {
      padding: 12px 0;
    }
    .statement-header {
      display: flex;
      align-items: center;
      justify-content: space-between;
      margin-bottom: 12px;
      gap: 12px;
    }
    .statement-label {
      font-weight: 500;
      font-size: 14px;
      color: var(--mat-sys-on-surface-variant);
    }
    .policy-link {
      color: var(--mat-sys-primary);
      text-decoration: none;
      font-size: 13px;
      white-space: nowrap;
    }
    .policy-link:hover {
      text-decoration: underline;
    }
    .global-account {
      font-style: italic;
      color: var(--mat-sys-on-surface-variant);
    }
    .statement-content {
      display: grid;
      grid-template-columns: 1fr 1fr;
      gap: 16px;
    }
    .chip-section {
      min-width: 0;
    }
    .chip-label {
      font-size: 12px;
      color: var(--mat-sys-on-surface-variant);
      display: block;
      margin-bottom: 4px;
    }
    .invalid-resource {
      background: var(--mat-sys-error-container) !important;
      color: var(--mat-sys-on-error-container) !important;
    }
    .warning-icon {
      color: var(--mat-sys-error);
      font-size: 18px;
      width: 18px;
      height: 18px;
    }
  `],
})
export class StatementsViewComponent {
  @Input() statements: Statement[] = [];
  @Input() policyInfos?: PolicyInfo[];
  @Input() currentAccount?: string;

  getResourceError(resource: string): string | null {
    return validateResource(resource);
  }

  getPolicyRoute(policyInfo: PolicyInfo): string[] {
    // For global policies, use the current account in the route instead of '_global'
    const routeAccount = policyInfo.account === '_global' && this.currentAccount
      ? this.currentAccount
      : policyInfo.account;
    return ['/policies', routeAccount, policyInfo.id];
  }
}
