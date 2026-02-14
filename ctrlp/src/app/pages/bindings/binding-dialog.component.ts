import { Component, inject, OnInit } from '@angular/core';
import { FormBuilder, FormGroup, ReactiveFormsModule, Validators } from '@angular/forms';
import { MAT_DIALOG_DATA, MatDialogModule, MatDialogRef } from '@angular/material/dialog';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatInputModule } from '@angular/material/input';
import { MatSelectModule } from '@angular/material/select';
import { MatAutocompleteModule } from '@angular/material/autocomplete';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';
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
          <mat-select formControlName="policies" multiple>
            @if (accountSpecificPolicies.length > 0) {
              <mat-optgroup label="Account Policies">
                @for (p of accountSpecificPolicies; track p.policy.id) {
                  <mat-option [value]="p.policy.id">{{ p.policy.name }}</mat-option>
                }
              </mat-optgroup>
            }
            @if (globalPolicies.length > 0) {
              <mat-optgroup label="Global Policies">
                @for (p of globalPolicies; track p.policy.id) {
                  <mat-option [value]="p.policy.id">{{ p.policy.name }}</mat-option>
                }
              </mat-optgroup>
            }
          </mat-select>
          @if (form.get('policies')?.hasError('required') && form.get('policies')?.touched) {
            <mat-error>At least one policy is required</mat-error>
          }
        </mat-form-field>
      </form>
    </mat-dialog-content>
    <mat-dialog-actions align="end">
      <button mat-button mat-dialog-close>Cancel</button>
      <button mat-flat-button (click)="save()" [disabled]="!form.valid">
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
  `],
})
export class BindingDialogComponent implements OnInit {
  readonly data = inject<BindingDialogData>(MAT_DIALOG_DATA);
  private dialogRef = inject(MatDialogRef<BindingDialogComponent>);
  private fb = inject(FormBuilder);

  form!: FormGroup;
  filteredAccounts: string[] = [];
  accountSpecificPolicies: import('../../models/policy.model').PolicyEntry[] = [];
  globalPolicies: import('../../models/policy.model').PolicyEntry[] = [];

  ngOnInit(): void {
    const binding = this.data.binding;
    const policies = binding?.policies ? [...binding.policies] : [];
    this.filteredAccounts = [...this.data.accounts]; // Initialize with all accounts

    this.form = this.fb.group({
      role: [binding?.role || '', Validators.required],
      account: [{ value: binding?.account || this.data.currentAccount, disabled: this.data.mode === 'edit' }, Validators.required],
      policies: [policies, Validators.required],
    });

    // Filter available policies based on initial account
    this.updateAvailablePolicies(binding?.account || this.data.currentAccount);

    // Subscribe to account changes to update policy proposals and filtered accounts
    this.form.get('account')?.valueChanges.subscribe(val => {
      const accountValue = (val || '').trim();

      // Update available policies when a valid account is selected
      if (accountValue && this.data.accounts.includes(accountValue)) {
        this.updateAvailablePolicies(accountValue);
        // Reset selected policies when account changes
        this.form.get('policies')?.setValue([]);
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
    this.accountSpecificPolicies = this.data.allPolicyEntries.filter(p => p.policy.account === account);
    this.globalPolicies = this.data.allPolicyEntries.filter(p => p.policy.account === '_global');
  }

  save(): void {
    if (!this.form.valid) return;

    const raw = this.form.getRawValue();
    const binding: Binding = {
      role: raw.role,
      account: raw.account,
      policies: raw.policies,
    };

    this.dialogRef.close(binding);
  }
}
