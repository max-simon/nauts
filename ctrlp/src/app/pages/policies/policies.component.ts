import { Component, inject, OnInit, OnDestroy } from '@angular/core';
import { Router, ActivatedRoute } from '@angular/router';
import { MatTableModule } from '@angular/material/table';
import { MatSortModule, Sort } from '@angular/material/sort';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatInputModule } from '@angular/material/input';
import { MatSelectModule } from '@angular/material/select';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';
import { MatProgressBarModule } from '@angular/material/progress-bar';
import { MatSnackBar, MatSnackBarModule } from '@angular/material/snack-bar';
import { MatDialog, MatDialogModule } from '@angular/material/dialog';
import { MatTooltipModule } from '@angular/material/tooltip';
import { MatChipsModule } from '@angular/material/chips';
import { MatSlideToggleModule } from '@angular/material/slide-toggle';
import { FormsModule } from '@angular/forms';
import { PolicyStoreService } from '../../services/policy-store.service';
import { AccountService } from '../../services/account.service';
import { ConfigService } from '../../services/config.service';
import { NavigationService } from '../../services/navigation.service';
import { PolicyEntry } from '../../models/policy.model';
import { PolicyDetailsComponent } from './policy-details.component';
import { PolicyDialogComponent, PolicyDialogData } from './policy-dialog.component';
import { ConfirmDialogComponent, ConfirmDialogData } from '../../shared/confirm-dialog.component';
import { EmptyStateComponent } from '../../shared/empty-state.component';
import { ConflictError } from '../../services/kv-store.service';
import { validatePolicyResources } from '../../validators/resource.validator';

@Component({
  selector: 'app-policies',
  standalone: true,
  imports: [
    FormsModule,
    MatTableModule,
    MatSortModule,
    MatFormFieldModule,
    MatInputModule,
    MatSelectModule,
    MatButtonModule,
    MatIconModule,
    MatProgressBarModule,
    MatSnackBarModule,
    MatDialogModule,
    MatTooltipModule,
    MatChipsModule,
    MatSlideToggleModule,
    PolicyDetailsComponent,
    EmptyStateComponent,
  ],
  template: `
    <div class="page-container">
      <div class="list-panel">
        <div class="toolbar">
          <mat-form-field appearance="outline" class="search-field">
            <mat-label>Search</mat-label>
            <mat-icon matPrefix>search</mat-icon>
            <input matInput [(ngModel)]="searchQuery" (ngModelChange)="applyFilter()" placeholder="Filter by name or ID">
          </mat-form-field>
        </div>
        <div class="filter-row">
          <mat-slide-toggle [(ngModel)]="showGlobalPolicies" (change)="loadPolicies()" class="global-toggle">
            Show Global Policies
          </mat-slide-toggle>
        </div>

        @if (loading) {
          <mat-progress-bar mode="indeterminate"></mat-progress-bar>
        }

        @if (!loading && filteredEntries.length === 0) {
          <app-empty-state icon="policy" message="No policies found"></app-empty-state>
        } @else if (!loading) {
          <table mat-table [dataSource]="filteredEntries" matSort (matSortChange)="sortData($event)" class="full-width">
            <ng-container matColumnDef="account">
              <th mat-header-cell *matHeaderCellDef mat-sort-header>Account</th>
              <td mat-cell *matCellDef="let entry" [class.global-account]="entry.policy.account === '_global'">
                {{ entry.policy.account === '_global' ? 'global' : entry.policy.account }}
              </td>
            </ng-container>

            <ng-container matColumnDef="name">
              <th mat-header-cell *matHeaderCellDef mat-sort-header>Name</th>
              <td mat-cell *matCellDef="let entry">{{ entry.policy.name }}</td>
            </ng-container>

            <ng-container matColumnDef="status">
              <th mat-header-cell *matHeaderCellDef>Status</th>
              <td mat-cell *matCellDef="let entry" class="status-cell">
                <span class="binding-count"
                      [class.valid]="!hasInvalidResources(entry) && !hasEmptyResources(entry)"
                      [class.invalid]="hasInvalidResources(entry) || hasEmptyResources(entry)"
                      [matTooltip]="getStatusTooltip(entry)">
                  {{ getBindingCount(entry) }}
                </span>
              </td>
            </ng-container>

            <ng-container matColumnDef="id">
              <th mat-header-cell *matHeaderCellDef mat-sort-header>ID</th>
              <td mat-cell *matCellDef="let entry" class="id-cell">{{ entry.policy.id }}</td>
            </ng-container>

            <tr mat-header-row *matHeaderRowDef="displayedColumns"></tr>
            <tr mat-row *matRowDef="let entry; columns: displayedColumns;"
                [class.selected]="selectedEntry !== null && selectedEntry.policy.id === entry.policy.id"
                (click)="selectEntry(entry)"></tr>
          </table>
        }

        <button mat-fab class="fab-create" (click)="openCreateDialog()">
          <mat-icon>add</mat-icon>
        </button>
      </div>

      <div class="details-panel">
        @if (selectedEntry) {
          <app-policy-details
            [entry]="selectedEntry"
            (edit)="openEditDialog()"
            (delete)="confirmDelete()">
          </app-policy-details>
        } @else {
          <app-empty-state icon="arrow_back" message="Select a policy to view details"></app-empty-state>
        }
      </div>
    </div>
  `,
  styles: [`
    .page-container {
      display: flex;
      height: 100%;
      gap: 16px;
      padding: 16px;
    }
    .list-panel {
      flex: 1;
      min-width: 0;
      max-width: 50%;
      position: relative;
      display: flex;
      flex-direction: column;
    }
    .details-panel {
      flex: 1;
      min-width: 0;
      max-width: 50%;
    }
    .toolbar {
      display: flex;
      align-items: center;
      margin-bottom: 8px;
    }
    .search-field {
      width: 100%;
    }
    .filter-row {
      margin-bottom: 12px;
    }
    .global-toggle {
      transform: scale(0.85);
      transform-origin: left center;
    }
    table {
      width: 100%;
    }
    .id-cell {
      font-family: monospace;
      font-size: 12px;
      max-width: 200px;
      overflow: hidden;
      text-overflow: ellipsis;
    }
    .status-cell {
      vertical-align: middle;
    }
    .global-account {
      font-style: italic;
      color: var(--mat-sys-on-surface-variant);
    }
    .binding-count {
      display: inline-block;
      min-width: 24px;
      padding: 4px 8px;
      border-radius: 12px;
      text-align: center;
      font-weight: 500;
      font-size: 13px;
    }
    .binding-count.valid {
      background: #81c784;
      color: white;
    }
    .binding-count.invalid {
      background: var(--mat-sys-error-container);
      color: var(--mat-sys-on-error-container);
    }
    tr.mat-mdc-row:hover {
      background: var(--mat-sys-surface-variant);
      cursor: pointer;
    }
    tr.selected {
      background: var(--mat-sys-secondary-container);
    }
    .fab-create {
      position: absolute;
      bottom: 16px;
      right: 16px;
    }
    .full-width { width: 100%; }
  `],
})
export class PoliciesComponent implements OnInit, OnDestroy {
  private policyService = inject(PolicyStoreService);
  private configService = inject(ConfigService);
  private navigationService = inject(NavigationService);
  private dialog = inject(MatDialog);
  private snackBar = inject(MatSnackBar);
  private router = inject(Router);
  private route = inject(ActivatedRoute);

  displayedColumns = ['account', 'name', 'status', 'id'];

  selectedAccount = '';
  searchQuery = '';
  showGlobalPolicies = true;
  loading = false;

  entries: PolicyEntry[] = [];
  filteredEntries: PolicyEntry[] = [];
  selectedEntry: PolicyEntry | null = null;
  
  private allPolicies: PolicyEntry[] = [];
  private policySubscription?: ReturnType<typeof setTimeout>;

  async ngOnInit(): Promise<void> {
    this.loading = true;

    try {
      // Initialize store
      await this.policyService.initialize();

      // Subscribe to route params to get account and policy ID
      this.route.params.subscribe(params => {
        const accountParam = params['account'];
        const idParam = params['id'];

        // Set selected account from route, default to empty (all accounts)
        this.selectedAccount = accountParam || '';

        // Sync with navigation service
        this.navigationService.setCurrentAccount(this.selectedAccount);

        this.loadPolicies();

        // If ID param is present, select that policy
        if (idParam && accountParam) {
          const policy = this.policyService.getPolicy(accountParam, idParam);
          if (policy) {
            this.selectedEntry = policy;
          }
        }
      });

      // Subscribe to policy updates
      this.policySubscription = this.policyService.getPolicies$().subscribe(policies => {
        this.allPolicies = policies;
        this.loadPolicies();
      }) as unknown as ReturnType<typeof setTimeout>;

    } catch (err) {
      this.handleError(err);
    } finally {
      this.loading = false;
    }
  }

  ngOnDestroy(): void {
    if (this.policySubscription) {
      (this.policySubscription as unknown as { unsubscribe: () => void }).unsubscribe();
    }
  }

  loadPolicies(): void {
    let policies = this.selectedAccount
      ? this.policyService.listPolicies(this.selectedAccount)
      : this.policyService.listAllPolicies();

    // Filter out global policies if toggle is off
    if (!this.showGlobalPolicies) {
      policies = policies.filter(p => p.policy.account !== '_global');
    } else if (this.selectedAccount) {
      // If showing specific account, also include global policies
      const globalPolicies = this.policyService.listGlobalPolicies();
      policies = [...policies, ...globalPolicies];
    }

    this.entries = policies;
    this.applyFilter();

    // Re-select if still exists
    if (this.selectedEntry) {
      const found = this.entries.find(e => e.policy.id === this.selectedEntry!.policy.id && e.policy.account === this.selectedEntry!.policy.account);
      this.selectedEntry = found || null;
    }
  }

  applyFilter(): void {
    const q = this.searchQuery.toLowerCase();
    this.filteredEntries = this.entries.filter(e => {
      return e.policy.name.toLowerCase().includes(q) || e.policy.id.toLowerCase().includes(q);
    });
  }

  sortData(sort: Sort): void {
    if (!sort.active || sort.direction === '') {
      this.applyFilter();
      return;
    }

    this.filteredEntries.sort((a, b) => {
      const isAsc = sort.direction === 'asc';
      switch (sort.active) {
        case 'account': return compare(a.policy.account, b.policy.account, isAsc);
        case 'name': return compare(a.policy.name, b.policy.name, isAsc);
        case 'id': return compare(a.policy.id, b.policy.id, isAsc);
        default: return 0;
      }
    });
  }

  selectEntry(entry: PolicyEntry): void {
    if (this.selectedEntry?.policy.id === entry.policy.id && this.selectedEntry?.policy.account === entry.policy.account) {
      this.selectedEntry = null;
      // Navigate back to account view
      if (this.selectedAccount) {
        this.router.navigate(['/policies', this.selectedAccount]);
      } else {
        this.router.navigate(['/policies']);
      }
    } else {
      this.selectedEntry = entry;
      // Navigate to detail view - use current selectedAccount for global policies
      const accountForRoute = entry.policy.account === '_global' && this.selectedAccount
        ? this.selectedAccount
        : entry.policy.account;
      this.router.navigate(['/policies', accountForRoute, entry.policy.id]);
    }
  }

  hasInvalidResources(entry: PolicyEntry): boolean {
    const errors = validatePolicyResources(entry.policy);
    return errors.size > 0;
  }

  hasEmptyResources(entry: PolicyEntry): boolean {
    return entry.policy.statements.some(stmt => !stmt.resources || stmt.resources.length === 0);
  }

  getBindingCount(entry: PolicyEntry): number {
    return this.policyService.getBindingsForPolicy(entry.policy.id).length;
  }

  getStatusTooltip(entry: PolicyEntry): string {
    const count = this.getBindingCount(entry);
    const bindingText = count === 1 ? 'binding' : 'bindings';

    if (this.hasInvalidResources(entry)) {
      return `Policy contains invalid resources. Used by ${count} ${bindingText}.`;
    } else if (this.hasEmptyResources(entry)) {
      return `Policy contains statements with no resources. Used by ${count} ${bindingText}.`;
    } else {
      return `Validation succeeded. Used by ${count} ${bindingText}.`;
    }
  }

  openCreateDialog(): void {
    const dialogRef = this.dialog.open(PolicyDialogComponent, {
      width: '600px',
      data: {
        mode: 'create',
        accounts: [],
        currentAccount: this.selectedAccount,
      } as PolicyDialogData,
    });

    dialogRef.afterClosed().subscribe(async (result) => {
      if (result) {
        try {
          await this.policyService.createPolicy(result);
          this.snackBar.open('Policy created', 'Dismiss', { duration: 3000 });
          // Data will be updated automatically via watcher
        } catch (err) {
          this.handleError(err);
        }
      }
    });
  }

  openEditDialog(): void {
    if (!this.selectedEntry) return;

    const dialogRef = this.dialog.open(PolicyDialogComponent, {
      width: '600px',
      data: {
        mode: 'edit',
        policy: { ...this.selectedEntry.policy },
        accounts: [],
        currentAccount: this.selectedAccount,
      } as PolicyDialogData,
    });

    dialogRef.afterClosed().subscribe(async (result) => {
      if (result && this.selectedEntry) {
        try {
          await this.policyService.updatePolicy(
            this.selectedEntry.policy.account,
            this.selectedEntry.policy.id,
            result,
            this.selectedEntry.revision,
          );
          this.snackBar.open('Policy updated', 'Dismiss', { duration: 3000 });
          // Data will be updated automatically via watcher
        } catch (err) {
          this.handleError(err);
        }
      }
    });
  }

  confirmDelete(): void {
    if (!this.selectedEntry) return;

    const dialogRef = this.dialog.open(ConfirmDialogComponent, {
      data: {
        title: 'Delete Policy',
        message: `Are you sure you want to delete "${this.selectedEntry.policy.name}"?`,
      } as ConfirmDialogData,
    });

    dialogRef.afterClosed().subscribe(async (confirmed) => {
      if (confirmed && this.selectedEntry) {
        try {
          await this.policyService.deletePolicy(
            this.selectedEntry.policy.account,
            this.selectedEntry.policy.id,
            this.selectedEntry.revision,
          );
          this.snackBar.open('Policy deleted', 'Dismiss', { duration: 3000 });
          this.selectedEntry = null;
          // Data will be updated automatically via watcher
        } catch (err) {
          this.handleError(err);
        }
      }
    });
  }

  private handleError(err: unknown): void {
    if (err instanceof ConflictError) {
      this.snackBar.open('Conflict: item was modified by another user. Reloading...', 'Dismiss', { duration: 5000 });
      this.loadPolicies();
    } else {
      const message = err instanceof Error ? err.message : 'An error occurred';
      this.snackBar.open(message, 'Dismiss', { duration: 5000 });
    }
  }
}

function compare(a: string, b: string, isAsc: boolean): number {
  return (a < b ? -1 : 1) * (isAsc ? 1 : -1);
}
