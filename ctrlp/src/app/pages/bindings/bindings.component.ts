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
import { FormsModule } from '@angular/forms';
import { PolicyStoreService } from '../../services/policy-store.service';
import { AccountService } from '../../services/account.service';
import { ConfigService } from '../../services/config.service';
import { NavigationService } from '../../services/navigation.service';
import { BindingEntry } from '../../models/binding.model';
import { BindingDetailsComponent } from './binding-details.component';
import { BindingDialogComponent, BindingDialogData } from './binding-dialog.component';
import { ConfirmDialogComponent, ConfirmDialogData } from '../../shared/confirm-dialog.component';
import { EmptyStateComponent } from '../../shared/empty-state.component';
import { ConflictError } from '../../services/kv-store.service';

@Component({
  selector: 'app-bindings',
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
    BindingDetailsComponent,
    EmptyStateComponent,
  ],
  template: `
    <div class="page-container">
      <div class="list-panel">
        <div class="toolbar">
          <mat-form-field appearance="outline" class="filter-field">
            <mat-label>Account</mat-label>
            <mat-select [(value)]="selectedAccount" (selectionChange)="onAccountChange()">
              <mat-option value="">All Accounts</mat-option>
              @for (account of accounts; track account) {
                <mat-option [value]="account">{{ account }}</mat-option>
              }
            </mat-select>
          </mat-form-field>

          <mat-form-field appearance="outline" class="filter-field search-field">
            <mat-label>Search role</mat-label>
            <mat-icon matPrefix>search</mat-icon>
            <input matInput [(ngModel)]="roleFilter" (ngModelChange)="applyFilter()" placeholder="Filter by role">
          </mat-form-field>

          <mat-form-field appearance="outline" class="filter-field">
            <mat-label>Policy</mat-label>
            <mat-select [(value)]="policyFilter" (selectionChange)="applyFilter()">
              <mat-option value="">All</mat-option>
              @for (p of availablePolicies; track p) {
                <mat-option [value]="p">{{ p }}</mat-option>
              }
            </mat-select>
          </mat-form-field>
        </div>

        @if (loading) {
          <mat-progress-bar mode="indeterminate"></mat-progress-bar>
        }

        @if (!loading && filteredEntries.length === 0) {
          <app-empty-state icon="link" message="No bindings found"></app-empty-state>
        } @else if (!loading) {
          <table mat-table [dataSource]="filteredEntries" matSort (matSortChange)="sortData($event)" class="full-width">
            <ng-container matColumnDef="role">
              <th mat-header-cell *matHeaderCellDef mat-sort-header>Role</th>
              <td mat-cell *matCellDef="let entry">{{ entry.binding.role }}</td>
            </ng-container>

            <ng-container matColumnDef="account">
              <th mat-header-cell *matHeaderCellDef mat-sort-header>Account</th>
              <td mat-cell *matCellDef="let entry">{{ entry.binding.account }}</td>
            </ng-container>

            <ng-container matColumnDef="policies">
              <th mat-header-cell *matHeaderCellDef>Policies</th>
              <td mat-cell *matCellDef="let entry">{{ entry.binding.policies.length }}</td>
            </ng-container>

            <tr mat-header-row *matHeaderRowDef="displayedColumns"></tr>
            <tr mat-row *matRowDef="let entry; columns: displayedColumns;"
                [class.selected]="selectedEntry !== null && selectedEntry.binding.role === entry.binding.role"
                (click)="selectEntry(entry)"></tr>
          </table>
        }

        <button mat-fab class="fab-create" (click)="openCreateDialog()">
          <mat-icon>add</mat-icon>
        </button>
      </div>

      <div class="details-panel">
        @if (selectedEntry) {
          <app-binding-details
            [entry]="selectedEntry"
            [danglingPolicies]="danglingPolicies"
            [policyMap]="policyMap"
            (edit)="openEditDialog()"
            (delete)="confirmDelete()">
          </app-binding-details>
        } @else {
          <app-empty-state icon="arrow_back" message="Select a binding to view details"></app-empty-state>
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
      position: relative;
      display: flex;
      flex-direction: column;
    }
    .details-panel {
      width: 400px;
      flex-shrink: 0;
    }
    .toolbar {
      display: flex;
      align-items: center;
      gap: 12px;
      margin-bottom: 8px;
      flex-wrap: wrap;
    }
    .filter-field {
      min-width: 150px;
    }
    .search-field {
      flex: 1;
    }
    table {
      width: 100%;
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
export class BindingsComponent implements OnInit, OnDestroy {
  private store = inject(PolicyStoreService);
  private accountService = inject(AccountService);
  private configService = inject(ConfigService);
  private navigationService = inject(NavigationService);
  private dialog = inject(MatDialog);
  private snackBar = inject(MatSnackBar);
  private router = inject(Router);
  private route = inject(ActivatedRoute);

  displayedColumns = ['role', 'account', 'policies'];

  accounts: string[] = [];
  selectedAccount = '';
  roleFilter = '';
  policyFilter = '';
  loading = false;

  entries: BindingEntry[] = [];
  filteredEntries: BindingEntry[] = [];
  selectedEntry: BindingEntry | null = null;
  availablePolicies: string[] = [];
  availablePolicyEntries: import('../../models/policy.model').PolicyEntry[] = [];
  policyMap = new Map<string, string>(); // id -> name
  danglingPolicies = new Set<string>();
  
  private allBindings: BindingEntry[] = [];
  private allPolicies: import('../../models/policy.model').PolicyEntry[] = [];
  private bindingSubscription?: ReturnType<typeof setTimeout>;
  private policySubscription?: ReturnType<typeof setTimeout>;

  async ngOnInit(): Promise<void> {
    this.loading = true;
    
    try {
      this.accounts = await this.accountService.discoverAccounts();

      // Initialize store
      await this.store.initialize();

      // Subscribe to route params to get account and role
      this.route.params.subscribe(params => {
        const accountParam = params['account'];
        const roleParam = params['role'];
        
        // Set selected account from route, default to empty (all accounts)
        this.selectedAccount = accountParam || '';
        
        this.loadBindings();
        
        // If role param is present, select that binding
        if (roleParam && accountParam) {
          const binding = this.store.getBinding(accountParam, roleParam);
          if (binding) {
            this.selectedEntry = binding;
            this.updateDanglingPolicies();
          }
        }
      });

      // Subscribe to binding updates
      this.bindingSubscription = this.store.getBindings$().subscribe(bindings => {
        this.allBindings = bindings;
        this.loadBindings();
      }) as unknown as ReturnType<typeof setTimeout>;

      // Subscribe to policy updates
      this.policySubscription = this.store.getPolicies$().subscribe(policies => {
        this.allPolicies = policies;
        this.availablePolicyEntries = policies;
        this.availablePolicies = policies.map(p => p.policy.id);
        this.policyMap = new Map(policies.map(p => [p.policy.id, p.policy.name]));
        this.loadBindings();
      }) as unknown as ReturnType<typeof setTimeout>;
      
    } catch (err) {
      this.handleError(err);
    } finally {
      this.loading = false;
    }
  }

  ngOnDestroy(): void {
    if (this.bindingSubscription) {
      (this.bindingSubscription as unknown as { unsubscribe: () => void }).unsubscribe();
    }
    if (this.policySubscription) {
      (this.policySubscription as unknown as { unsubscribe: () => void }).unsubscribe();
    }
  }

  loadBindings(): void {
    this.entries = this.selectedAccount
      ? this.store.listBindings(this.selectedAccount)
      : this.store.listAllBindings();
      
      this.applyFilter();
      this.updateDanglingPolicies();

      // Re-select if still exists
      if (this.selectedEntry) {
        const found = this.entries.find(e => e.binding.role === this.selectedEntry!.binding.role && e.binding.account === this.selectedEntry!.binding.account);
        this.selectedEntry = found || null;
        if (this.selectedEntry) {
          this.updateDanglingPolicies();
        }
      }
  }

  onAccountChange(): void {
    this.selectedEntry = null;
    this.navigationService.setCurrentAccount(this.selectedAccount);
    if (this.selectedAccount) {
      this.router.navigate(['/bindings', this.selectedAccount]);
    } else {
      this.router.navigate(['/bindings']);
    }
  }

  applyFilter(): void {
    const roleQ = this.roleFilter.toLowerCase();
    this.filteredEntries = this.entries.filter(e => {
      const matchesRole = e.binding.role.toLowerCase().includes(roleQ);
      const matchesPolicy = !this.policyFilter || e.binding.policies.includes(this.policyFilter);
      return matchesRole && matchesPolicy;
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
        case 'role': return compare(a.binding.role, b.binding.role, isAsc);
        case 'account': return compare(a.binding.account, b.binding.account, isAsc);
        default: return 0;
      }
    });
  }

  selectEntry(entry: BindingEntry): void {
    if (this.selectedEntry?.binding.role === entry.binding.role && this.selectedEntry?.binding.account === entry.binding.account) {
      this.selectedEntry = null;
      // Navigate back to account view
      if (entry.binding.account) {
        this.router.navigate(['/bindings', entry.binding.account]);
      } else {
        this.router.navigate(['/bindings']);
      }
    } else {
      this.selectedEntry = entry;
      this.updateDanglingPolicies();
      // Navigate to detail view
      this.router.navigate(['/bindings', entry.binding.account, entry.binding.role]);
    }
  }

  private updateDanglingPolicies(): void {
    this.danglingPolicies = new Set<string>();
    if (this.selectedEntry) {
      for (const policyId of this.selectedEntry.binding.policies) {
        if (!this.availablePolicies.includes(policyId)) {
          this.danglingPolicies.add(policyId);
        }
      }
    }
  }

  openCreateDialog(): void {
    const dialogRef = this.dialog.open(BindingDialogComponent, {
      width: '600px',
      data: {
        mode: 'create',
        accounts: this.accounts,
        currentAccount: this.selectedAccount,
        allPolicyEntries: this.availablePolicyEntries,
      } as BindingDialogData,
    });

    dialogRef.afterClosed().subscribe(async (result) => {
      if (result) {
        try {
          await this.store.createBinding(result);
          this.snackBar.open('Binding created', 'Dismiss', { duration: 3000 });
          // Data will be updated automatically via watcher
        } catch (err) {
          this.handleError(err);
        }
      }
    });
  }

  openEditDialog(): void {
    if (!this.selectedEntry) return;

    const dialogRef = this.dialog.open(BindingDialogComponent, {
      width: '600px',
      data: {
        mode: 'edit',
        binding: { ...this.selectedEntry.binding },
        accounts: this.accounts,
        currentAccount: this.selectedAccount,
        allPolicyEntries: this.availablePolicyEntries,
      } as BindingDialogData,
    });

    dialogRef.afterClosed().subscribe(async (result) => {
      if (result && this.selectedEntry) {
        try {
          await this.store.updateBinding(
            this.selectedEntry.binding.account,
            this.selectedEntry.binding.role,
            result,
            this.selectedEntry.revision,
          );
          this.snackBar.open('Binding updated', 'Dismiss', { duration: 3000 });
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
        title: 'Delete Binding',
        message: `Are you sure you want to delete binding for role "${this.selectedEntry.binding.role}"?`,
      } as ConfirmDialogData,
    });

    dialogRef.afterClosed().subscribe(async (confirmed) => {
      if (confirmed && this.selectedEntry) {
        try {
          await this.store.deleteBinding(
            this.selectedEntry.binding.account,
            this.selectedEntry.binding.role,
            this.selectedEntry.revision,
          );
          this.snackBar.open('Binding deleted', 'Dismiss', { duration: 3000 });
          this.selectedEntry = null;
          await this.loadBindings();
        } catch (err) {
          this.handleError(err);
        }
      }
    });
  }

  private handleError(err: unknown): void {
    if (err instanceof ConflictError) {
      this.snackBar.open('Conflict: item was modified by another user. Reloading...', 'Dismiss', { duration: 5000 });
      this.loadBindings();
    } else {
      const message = err instanceof Error ? err.message : 'An error occurred';
      this.snackBar.open(message, 'Dismiss', { duration: 5000 });
    }
  }
}

function compare(a: string, b: string, isAsc: boolean): number {
  return (a < b ? -1 : 1) * (isAsc ? 1 : -1);
}
