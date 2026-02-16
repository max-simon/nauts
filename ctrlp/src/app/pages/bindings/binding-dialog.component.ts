import { Component, inject, OnInit } from '@angular/core';
import { FormBuilder, FormGroup, ReactiveFormsModule, Validators, FormsModule } from '@angular/forms';
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
    FormsModule,
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
      @if (jsonMode) {
        <div class="json-editor">
          <mat-form-field appearance="outline" class="full-width">
            <mat-label>JSON</mat-label>
            <textarea matInput
                      [(ngModel)]="jsonText"
                      rows="15"
                      class="json-textarea"
                      spellcheck="false"></textarea>
            @if (jsonError) {
              <mat-error>{{ jsonError }}</mat-error>
            }
          </mat-form-field>
        </div>
      } @else {
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
      }
    </mat-dialog-content>
    <mat-dialog-actions align="end">
      <button mat-button mat-dialog-close>Cancel</button>
      <button mat-button (click)="toggleJsonMode()">
        {{ jsonMode ? 'Edit as Form' : 'Edit as JSON' }}
      </button>
      <button mat-flat-button (click)="save()" [disabled]="jsonMode ? !jsonText.trim() : !form.valid">
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
    .json-editor {
      min-width: 480px;
    }
    .json-textarea {
      font-family: 'Courier New', monospace;
      font-size: 12px;
      line-height: 1.5;
    }
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

  // JSON editing mode
  jsonMode = false;
  jsonText = '';
  jsonError = '';

  ngOnInit(): void {
    const binding = this.data.binding;
    // Strip "_global:" prefix from policy IDs for form
    const policies = binding?.policies
      ? binding.policies.map(policyId => policyId.startsWith('_global:') ? policyId.substring(8) : policyId)
      : [];
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

  toggleJsonMode(): void {
    if (!this.jsonMode) {
      // Switching to JSON mode - convert form to JSON
      const binding = this.buildBindingFromForm();
      this.jsonText = JSON.stringify(binding, null, 2);
      this.jsonError = '';
    } else {
      // Switching back to form mode - validate and parse JSON
      try {
        const binding = JSON.parse(this.jsonText);
        this.loadBindingIntoForm(binding);
        this.jsonError = '';
      } catch (err) {
        this.jsonError = err instanceof Error ? err.message : 'Invalid JSON';
        return; // Don't switch modes if JSON is invalid
      }
    }
    this.jsonMode = !this.jsonMode;
  }

  private buildBindingFromForm(): Binding {
    const raw = this.form.getRawValue();

    // Add "_global:" prefix to global policy IDs
    const policies = (raw.policies || []).map((policyId: string) => {
      const policyEntry = this.data.allPolicyEntries.find(p => p.policy.id === policyId);
      if (policyEntry && policyEntry.policy.account === '_global') {
        return `_global:${policyId}`;
      }
      return policyId;
    });

    return {
      role: raw.role,
      account: raw.account,
      policies: policies,
    };
  }

  private loadBindingIntoForm(binding: Binding): void {
    // Strip "_global:" prefix from policy IDs for display in form
    const policies = (binding.policies || []).map(policyId =>
      policyId.startsWith('_global:') ? policyId.substring(8) : policyId
    );

    this.form.patchValue({
      role: binding.role,
      account: binding.account,
      policies: policies,
    });

    // Update available policies based on account
    if (binding.account) {
      this.updateAvailablePolicies(binding.account);
    }
  }

  save(): void {
    if (this.jsonMode) {
      // Save from JSON
      try {
        const binding = JSON.parse(this.jsonText);
        this.dialogRef.close(binding);
      } catch (err) {
        this.jsonError = err instanceof Error ? err.message : 'Invalid JSON';
      }
    } else {
      // Save from form
      if (!this.form.valid) return;
      const binding = this.buildBindingFromForm();
      this.dialogRef.close(binding);
    }
  }
}
