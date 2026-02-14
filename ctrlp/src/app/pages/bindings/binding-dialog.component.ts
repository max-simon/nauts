import { Component, inject, OnInit } from '@angular/core';
import { FormBuilder, FormGroup, ReactiveFormsModule, Validators } from '@angular/forms';
import { MAT_DIALOG_DATA, MatDialogModule, MatDialogRef } from '@angular/material/dialog';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatInputModule } from '@angular/material/input';
import { MatSelectModule } from '@angular/material/select';
import { MatAutocompleteModule } from '@angular/material/autocomplete';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';
import { MatChipInputEvent, MatChipsModule } from '@angular/material/chips';
import { COMMA, ENTER } from '@angular/cdk/keycodes';
import { Binding } from '../../models/binding.model';

export interface BindingDialogData {
  mode: 'create' | 'edit';
  binding?: Binding;
  accounts: string[];
  currentAccount: string;
  allPolicyEntries: import('../../models/policy.model').PolicyEntry[];
}

@Component({
  selector: 'app-binding-dialog',
  standalone: true,
  imports: [
    ReactiveFormsModule,
    MatDialogModule,
    MatFormFieldModule,
    MatInputModule,
    MatSelectModule,
    MatAutocompleteModule,
    MatButtonModule,
    MatIconModule,
    MatChipsModule,
  ],
  template: `
    <h2 mat-dialog-title>{{ data.mode === 'create' ? 'Create Binding' : 'Edit Binding' }}</h2>
    <mat-dialog-content>
      <form [formGroup]="form" class="binding-form">
        <mat-form-field appearance="outline" class="full-width">
          <mat-label>Role</mat-label>
          <input matInput formControlName="role" placeholder="Role name">
          @if (form.get('role')?.hasError('required') && form.get('role')?.touched) {
            <mat-error>Role is required</mat-error>
          }
        </mat-form-field>

        <mat-form-field appearance="outline" class="full-width">
          <mat-label>Account</mat-label>
          <input matInput formControlName="account" [matAutocomplete]="accountAuto">
          <mat-autocomplete #accountAuto="matAutocomplete">
            @for (account of filteredAccounts; track account) {
              <mat-option [value]="account">{{ account }}</mat-option>
            }
          </mat-autocomplete>
          @if (form.get('account')?.hasError('required') && form.get('account')?.touched) {
            <mat-error>Account is required</mat-error>
          }
        </mat-form-field>

        <mat-form-field appearance="outline" class="full-width">
          <mat-label>Policies</mat-label>
          <mat-chip-grid #policiesChipGrid>
            @for (policy of policies; track policy) {
              <mat-chip-row (removed)="removePolicy(policy)">
                {{ getPolicyDisplayName(policy) }}
                <button matChipRemove><mat-icon>cancel</mat-icon></button>
              </mat-chip-row>
            }
          </mat-chip-grid>
          <input placeholder="Type policy ID and press Enter"
                 [matChipInputFor]="policiesChipGrid"
                 [matChipInputSeparatorKeyCodes]="separatorKeyCodes"
                 (matChipInputTokenEnd)="addPolicy($event)">
          @if (policies.length === 0 && form.get('policies')?.touched) {
            <mat-error>At least one policy is required</mat-error>
          }
        </mat-form-field>

        @if (availablePolicies.length > 0) {
          <div class="available-policies">
            <span class="hint">Available policies:</span>
            @for (p of availablePolicies; track p.policy.id) {
              <button mat-button type="button" class="policy-suggestion" (click)="addPolicyById(p.policy.id)"
                      [disabled]="policies.includes(p.policy.id)">
                {{ p.policy.name }}
              </button>
            }
          </div>
        }
      </form>
    </mat-dialog-content>
    <mat-dialog-actions align="end">
      <button mat-button mat-dialog-close>Cancel</button>
      <button mat-flat-button (click)="save()" [disabled]="!isValid()">
        {{ data.mode === 'create' ? 'Create' : 'Save' }}
      </button>
    </mat-dialog-actions>
  `,
  styles: [`
    .binding-form {
      display: flex;
      flex-direction: column;
      min-width: 480px;
      gap: 4px;
    }
    .full-width { width: 100%; }
    .available-policies {
      display: flex;
      flex-wrap: wrap;
      gap: 4px;
      align-items: center;
    }
    .hint {
      font-size: 12px;
      color: var(--mat-sys-on-surface-variant);
    }
    .policy-suggestion {
      font-size: 12px;
    }
  `],
})
export class BindingDialogComponent implements OnInit {
  readonly data = inject<BindingDialogData>(MAT_DIALOG_DATA);
  private dialogRef = inject(MatDialogRef<BindingDialogComponent>);
  private fb = inject(FormBuilder);

  readonly separatorKeyCodes = [ENTER, COMMA] as const;

  form!: FormGroup;
  filteredAccounts: string[] = [];
  policies: string[] = [];
  availablePolicies: import('../../models/policy.model').PolicyEntry[] = [];
  policyMap = new Map<string, string>(); // id -> name

  ngOnInit(): void {
    const binding = this.data.binding;
    this.policies = binding?.policies ? [...binding.policies] : [];
    this.filteredAccounts = [...this.data.accounts]; // Initialize with all accounts

    // Build policy map for all policies
    this.policyMap = new Map(this.data.allPolicyEntries.map(p => [p.policy.id, p.policy.name]));

    this.form = this.fb.group({
      role: [{ value: binding?.role || '', disabled: this.data.mode === 'edit' }, Validators.required],
      account: [{ value: binding?.account || this.data.currentAccount, disabled: this.data.mode === 'edit' }, Validators.required],
      policies: [this.policies],
    });

    // Filter available policies based on initial account
    this.updateAvailablePolicies(binding?.account || this.data.currentAccount);

    // Subscribe to account changes to update policy proposals and filtered accounts
    this.form.get('account')?.valueChanges.subscribe(val => {
      const accountValue = (val || '').trim();
      
      // Update available policies when a valid account is selected
      if (accountValue && this.data.accounts.includes(accountValue)) {
        this.updateAvailablePolicies(accountValue);
        // Reset to show all accounts after selection so dropdown works properly
        this.filteredAccounts = [...this.data.accounts];
      } else {
        // Filter account dropdown based on typed value (partial match)
        const q = accountValue.toLowerCase();
        this.filteredAccounts = this.data.accounts.filter(a => a.toLowerCase().includes(q));
      }
    });
  }

  private updateAvailablePolicies(account: string): void {
    this.availablePolicies = this.data.allPolicyEntries.filter(p => 
      p.policy.account === account || p.policy.account === '_global'
    );
  }

  addPolicy(event: MatChipInputEvent): void {
    const value = (event.value || '').trim();
    if (value && !this.policies.includes(value)) {
      this.policies.push(value);
      this.form.get('policies')?.setValue(this.policies);
    }
    event.chipInput.clear();
  }

  addPolicyById(id: string): void {
    if (!this.policies.includes(id)) {
      this.policies.push(id);
      this.form.get('policies')?.setValue(this.policies);
    }
  }

  removePolicy(policy: string): void {
    this.policies = this.policies.filter(p => p !== policy);
    this.form.get('policies')?.setValue(this.policies);
  }

  getPolicyDisplayName(policyId: string): string {
    return this.policyMap.get(policyId) || policyId;
  }

  isValid(): boolean {
    return this.form.valid && this.policies.length > 0;
  }

  save(): void {
    if (!this.isValid()) return;

    const raw = this.form.getRawValue();
    const binding: Binding = {
      role: raw.role,
      account: raw.account,
      policies: this.policies,
    };

    this.dialogRef.close(binding);
  }
}
