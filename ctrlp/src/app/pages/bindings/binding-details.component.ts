import { Component, EventEmitter, Input, Output, OnChanges, inject } from '@angular/core';
import { RouterLink } from '@angular/router';
import { MatCardModule } from '@angular/material/card';
import { MatChipsModule } from '@angular/material/chips';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';
import { MatTooltipModule } from '@angular/material/tooltip';
import { MatExpansionModule } from '@angular/material/expansion';
import { StatementsViewComponent, PolicyInfo } from '../../shared/statements-view.component';
import { BindingEntry } from '../../models/binding.model';
import { PolicyStoreService } from '../../services/policy-store.service';

@Component({
  selector: 'app-binding-details',
  standalone: true,
  imports: [RouterLink, MatCardModule, MatChipsModule, MatButtonModule, MatIconModule, MatTooltipModule, MatExpansionModule, StatementsViewComponent],
  template: `
    @if (entry) {
      <mat-card>
        <mat-card-header>
          <mat-card-title>{{ entry.binding.role }}</mat-card-title>
          <mat-card-subtitle>
            <div class="metadata">
              <div class="metadata-item">
                <span class="metadata-label">Account:</span>
                <span class="metadata-value">{{ entry.binding.account }}</span>
              </div>
            </div>
          </mat-card-subtitle>
        </mat-card-header>
        <mat-card-content>
          <mat-accordion multi>
            <mat-expansion-panel [expanded]="true">
              <mat-expansion-panel-header>
                <mat-panel-title>Policies ({{ entry.binding.policies.length }})</mat-panel-title>
              </mat-expansion-panel-header>
              <div class="policies-list">
                @for (policyId of entry.binding.policies; track policyId) {
                  @if (getPolicyForId(policyId); as policy) {
                    <a [routerLink]="getPolicyRoute(policy.policy.id, policy.policy.account)" class="policy-link">
                      {{ policy.policy.name }}
                      (<span [class.global-account]="policy.policy.account === '_global'">{{ policy.policy.account === '_global' ? 'global' : policy.policy.account }}</span>)
                    </a>
                  } @else {
                    <div class="policy-link dangling">
                      <mat-icon class="warning-icon" matTooltip="Policy does not exist">warning</mat-icon>
                      {{ policyId }}
                    </div>
                  }
                }
              </div>
            </mat-expansion-panel>

            <mat-expansion-panel [expanded]="false">
              <mat-expansion-panel-header>
                <mat-panel-title>Compiled Statements ({{ compiledStatements.length }})</mat-panel-title>
              </mat-expansion-panel-header>
              @if (compiledStatements.length > 0) {
                <app-statements-view
                  [statements]="compiledStatements"
                  [policyInfos]="statementPolicyInfos"
                  [currentAccount]="entry.binding.account"></app-statements-view>
              } @else {
                <div class="empty-message">No statements to compile</div>
              }
            </mat-expansion-panel>
          </mat-accordion>
        </mat-card-content>
        <mat-card-actions align="end">
          <button mat-button (click)="edit.emit()">
            <mat-icon>edit</mat-icon> Edit
          </button>
          <button mat-button color="warn" (click)="delete.emit()">
            <mat-icon>delete</mat-icon> Delete
          </button>
        </mat-card-actions>
      </mat-card>
    }
  `,
  styles: [`
    mat-card {
      margin: 0;
    }
    .metadata {
      display: flex;
      flex-direction: column;
      gap: 4px;
      margin-top: 8px;
    }
    .metadata-item {
      display: flex;
      gap: 8px;
      font-size: 13px;
    }
    .metadata-label {
      font-weight: 500;
      color: var(--mat-sys-on-surface-variant);
    }
    .metadata-value {
      color: var(--mat-sys-on-surface);
    }
    mat-accordion {
      display: flex;
      flex-direction: column;
      gap: 8px;
    }
    mat-expansion-panel {
      box-shadow: none !important;
      border: 1px solid var(--mat-sys-outline-variant);
    }
    .policies-list {
      display: flex;
      flex-direction: column;
      gap: 8px;
      margin-top: 8px;
    }
    .policy-link {
      color: var(--mat-sys-primary);
      text-decoration: none;
      font-size: 14px;
      padding: 4px 0;
      cursor: pointer;
      display: flex;
      align-items: center;
      gap: 8px;
    }
    .policy-link:not(.dangling):hover {
      text-decoration: underline;
    }
    .policy-link.dangling {
      color: var(--mat-sys-error);
      cursor: default;
    }
    .global-account {
      font-style: italic;
      color: var(--mat-sys-on-surface-variant);
    }
    .warning-icon {
      color: var(--mat-sys-error);
      font-size: 18px;
      width: 18px;
      height: 18px;
    }
    .empty-message {
      padding: 16px 0;
      color: var(--mat-sys-on-surface-variant);
      font-size: 14px;
      text-align: center;
    }
  `],
})
export class BindingDetailsComponent implements OnChanges {
  private policyStore = inject(PolicyStoreService);

  @Input() entry: BindingEntry | null = null;
  @Input() danglingPolicies = new Set<string>();
  @Input() policyMap = new Map<string, string>(); // id -> name
  @Output() edit = new EventEmitter<void>();
  @Output() delete = new EventEmitter<void>();

  compiledStatements: Array<{ effect: string; actions: string[]; resources: string[] }> = [];
  statementPolicyInfos: PolicyInfo[] = [];

  ngOnChanges(): void {
    this.compileStatements();
  }

  getPolicyForId(policyId: string) {
    if (!this.entry) return null;

    const account = this.entry.binding.account;
    const allPolicies = this.policyStore.listAllPolicies();

    return allPolicies.find(p =>
      p.policy.id === policyId &&
      (p.policy.account === account || p.policy.account === '_global')
    ) || null;
  }

  getPolicyRoute(policyId: string, policyAccount: string): string[] {
    // For global policies, use the current binding account in the route
    const routeAccount = policyAccount === '_global' && this.entry
      ? this.entry.binding.account
      : policyAccount;
    return ['/policies', routeAccount, policyId];
  }

  private compileStatements(): void {
    this.compiledStatements = [];
    this.statementPolicyInfos = [];

    if (!this.entry) return;

    // Get all policies for this binding
    for (const policyId of this.entry.binding.policies) {
      const policyEntry = this.getPolicyForId(policyId);
      if (policyEntry) {
        // Add all statements from this policy
        const policyInfo: PolicyInfo = {
          id: policyEntry.policy.id,
          name: policyEntry.policy.name,
          account: policyEntry.policy.account,
        };

        // Add each statement along with its policy info
        for (const statement of policyEntry.policy.statements) {
          this.compiledStatements.push(statement);
          this.statementPolicyInfos.push(policyInfo);
        }
      }
    }
  }
}
