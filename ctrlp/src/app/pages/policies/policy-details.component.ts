import { Component, EventEmitter, Input, Output, OnChanges, OnInit, OnDestroy, inject } from '@angular/core';
import { RouterLink, ActivatedRoute } from '@angular/router';
import { MatCardModule } from '@angular/material/card';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';
import { MatDividerModule } from '@angular/material/divider';
import { MatExpansionModule } from '@angular/material/expansion';
import { StatementsViewComponent } from '../../shared/statements-view.component';
import { PolicyEntry } from '../../models/policy.model';
import { BindingEntry } from '../../models/binding.model';
import { PolicyStoreService } from '../../services/policy-store.service';
import { validatePolicyResources } from '../../validators/resource.validator';
import { Subscription } from 'rxjs';

@Component({
  selector: 'app-policy-details',
  standalone: true,
  imports: [RouterLink, MatCardModule, MatButtonModule, MatIconModule, MatDividerModule, MatExpansionModule, StatementsViewComponent],
  template: `
    @if (entry) {
      <mat-card>
        <mat-card-header>
          <mat-card-title>{{ entry.policy.name }}</mat-card-title>
          <mat-card-subtitle>
            <div class="metadata">
              <div class="metadata-item">
                <span class="metadata-label">ID:</span>
                <span class="metadata-value">{{ entry.policy.id }}</span>
              </div>
              @if (entry.policy.account !== '_global') {
                <div class="metadata-item">
                  <span class="metadata-label">Account:</span>
                  <span class="metadata-value">{{ entry.policy.account }}</span>
                </div>
              }
            </div>
          </mat-card-subtitle>
        </mat-card-header>
        <mat-card-content>
          @if (entry.policy.account === '_global') {
            <div class="global-warning">
              <mat-icon>info</mat-icon>
              <span>This is a global policy. Changes will affect all accounts.</span>
            </div>
          }

          @if (hasKeyMismatch()) {
            <div class="key-mismatch-warning">
              <mat-icon>warning</mat-icon>
              <span>{{ getKeyMismatchMessage() }}</span>
            </div>
          }

          <mat-accordion multi>
            <mat-expansion-panel [expanded]="true">
              <mat-expansion-panel-header>
                <mat-panel-title>Statements</mat-panel-title>
              </mat-expansion-panel-header>
              <app-statements-view [statements]="entry.policy.statements"></app-statements-view>
            </mat-expansion-panel>

            @if (relatedBindings.length > 0) {
              <mat-expansion-panel [expanded]="false">
                <mat-expansion-panel-header>
                  <mat-panel-title>Bindings ({{ getCurrentAccountBindingCount() }})</mat-panel-title>
                </mat-expansion-panel-header>
                <div class="bindings-list">
                  @for (binding of relatedBindings; track binding.binding.role) {
                    <a [routerLink]="['/bindings', binding.binding.account, binding.binding.role]"
                       class="binding-link"
                       [class.other-account]="isBindingFromOtherAccount(binding)">
                      {{ binding.binding.role }} ({{ binding.binding.account }})
                    </a>
                  }
                </div>
              </mat-expansion-panel>
            }
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
    .global-warning {
      display: flex;
      align-items: center;
      gap: 8px;
      padding: 12px 16px;
      margin-bottom: 16px;
      background: var(--mat-sys-primary-container);
      color: var(--mat-sys-on-primary-container);
      border-radius: 4px;
      font-size: 14px;
    }
    .global-warning mat-icon {
      font-size: 20px;
      width: 20px;
      height: 20px;
    }
    .key-mismatch-warning {
      display: flex;
      align-items: center;
      gap: 8px;
      padding: 12px 16px;
      margin-bottom: 16px;
      background: var(--mat-sys-error-container);
      color: var(--mat-sys-on-error-container);
      border-radius: 4px;
      font-size: 14px;
    }
    .key-mismatch-warning mat-icon {
      font-size: 20px;
      width: 20px;
      height: 20px;
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
      font-family: monospace;
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
    .bindings-list {
      display: flex;
      flex-direction: column;
      gap: 8px;
      margin-top: 8px;
    }
    .binding-link {
      color: var(--mat-sys-primary);
      text-decoration: none;
      font-size: 14px;
      padding: 4px 0;
      cursor: pointer;
    }
    .binding-link:hover {
      text-decoration: underline;
    }
    .binding-link.other-account {
      color: var(--mat-sys-on-surface-variant);
      font-style: italic;
    }
  `],
})
export class PolicyDetailsComponent implements OnChanges, OnInit, OnDestroy {
  private policyStore = inject(PolicyStoreService);
  private route = inject(ActivatedRoute);

  @Input() entry: PolicyEntry | null = null;
  @Output() edit = new EventEmitter<void>();
  @Output() delete = new EventEmitter<void>();

  resourceErrors = new Map<string, string>();
  relatedBindings: BindingEntry[] = [];
  currentAccount = '';
  currentPolicyId = '';

  private routeSubscription?: Subscription;

  ngOnInit(): void {
    // Subscribe to route params to get current account and policy ID
    this.routeSubscription = this.route.params.subscribe(params => {
      this.currentAccount = params['account'] || '';
      this.currentPolicyId = params['id'] || '';
      this.loadRelatedBindings();
    });
  }

  ngOnDestroy(): void {
    this.routeSubscription?.unsubscribe();
  }

  ngOnChanges(): void {
    if (this.entry) {
      this.resourceErrors = validatePolicyResources(this.entry.policy);
      this.loadRelatedBindings();
    } else {
      this.resourceErrors.clear();
      this.relatedBindings = [];
    }
  }

  private loadRelatedBindings(): void {
    if (!this.entry) {
      this.relatedBindings = [];
      return;
    }

    // Get all bindings for this policy
    const allBindings = this.policyStore.getBindingsForPolicy(this.entry.policy.id);

    // Sort: current account bindings first, then other accounts
    this.relatedBindings = allBindings.sort((a, b) => {
      const aIsOther = this.isBindingFromOtherAccount(a);
      const bIsOther = this.isBindingFromOtherAccount(b);

      if (aIsOther === bIsOther) return 0;
      return aIsOther ? 1 : -1;
    });
  }

  isBindingFromOtherAccount(binding: BindingEntry): boolean {
    return !!this.currentAccount && binding.binding.account !== this.currentAccount;
  }

  getCurrentAccountBindingCount(): number {
    return this.relatedBindings.filter(b => !this.isBindingFromOtherAccount(b)).length;
  }

  hasKeyMismatch(): boolean {
    if (!this.entry || !this.currentAccount || !this.currentPolicyId) return false;

    // Check if the policy's account matches the account in the route (KV key)
    const accountMismatch = this.entry.policy.account !== this.currentAccount && this.entry.policy.account !== "_global";

    // Check if the policy's ID matches the ID in the route (KV key)
    const idMismatch = this.entry.policy.id !== this.currentPolicyId;

    return accountMismatch || idMismatch;
  }

  getKeyMismatchMessage(): string {
    if (!this.entry || !this.currentAccount || !this.currentPolicyId) return '';

    const messages: string[] = [];

    if (this.entry.policy.account !== this.currentAccount) {
      messages.push(`Account mismatch: policy has "${this.entry.policy.account}" but KV key uses "${this.currentAccount}"`);
    }

    if (this.entry.policy.id !== this.currentPolicyId) {
      messages.push(`ID mismatch: policy has "${this.entry.policy.id}" but KV key uses "${this.currentPolicyId}"`);
    }

    return messages.join('. ');
  }

  getResourceError(resource: string): string | null {
    return this.resourceErrors.get(resource) || null;
  }
}
