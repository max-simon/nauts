import { Component, inject, OnInit } from '@angular/core';
import { FormBuilder, FormGroup, ReactiveFormsModule, Validators, FormsModule } from '@angular/forms';
import { MAT_DIALOG_DATA, MatDialogModule, MatDialogRef } from '@angular/material/dialog';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatInputModule } from '@angular/material/input';
import { MatSelectModule } from '@angular/material/select';
import { MatAutocompleteModule, MatAutocompleteSelectedEvent } from '@angular/material/autocomplete';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';
import { MatChipInputEvent, MatChipsModule } from '@angular/material/chips';
import { MatDividerModule } from '@angular/material/divider';
import { MatTooltipModule } from '@angular/material/tooltip';
import { MatCheckboxModule } from '@angular/material/checkbox';
import { COMMA, ENTER } from '@angular/cdk/keycodes';
import { Policy } from '../../models/policy.model';
import { validateResource } from '../../validators/resource.validator';

const VALID_ACTIONS = [
  'nats.pub', 'nats.sub', 'nats.service', 'nats.*',
  'js.manage', 'js.view', 'js.consume', 'js.*',
  'kv.read', 'kv.edit', 'kv.view', 'kv.manage', 'kv.*',
];

export interface PolicyDialogData {
  mode: 'create' | 'edit';
  policy?: Policy;
  accounts: string[];
  currentAccount: string;
}

@Component({
  selector: 'app-policy-dialog',
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
    MatChipsModule,
    MatDividerModule,
    MatTooltipModule,
    MatCheckboxModule,
  ],
  template: `
    <h2 mat-dialog-title>{{ data.mode === 'create' ? 'Create Policy' : 'Edit Policy' }}</h2>
    <mat-dialog-content>
      @if (jsonMode) {
        <div class="json-editor">
          <mat-form-field appearance="outline" class="full-width">
            <mat-label>JSON</mat-label>
            <textarea matInput
                      [(ngModel)]="jsonText"
                      rows="20"
                      class="json-textarea"
                      spellcheck="false"></textarea>
            @if (jsonError) {
              <mat-error>{{ jsonError }}</mat-error>
            }
          </mat-form-field>
        </div>
      } @else {
        <form [formGroup]="form" class="policy-form">
        <mat-form-field appearance="outline" class="full-width">
          <mat-label>Name</mat-label>
          <input matInput formControlName="name" placeholder="Policy name">
          @if (form.get('name')?.hasError('required') && form.get('name')?.touched) {
            <mat-error>Name is required</mat-error>
          }
        </mat-form-field>

        <mat-checkbox formControlName="isGlobal" (change)="onGlobalChange()" [disabled]="data.mode === 'edit'">
          Global Policy
        </mat-checkbox>

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

        <div class="statements-header">
          <h3>Statements</h3>
          <button mat-button type="button" (click)="addStatement()">
            <mat-icon>add</mat-icon> Add Statement
          </button>
        </div>

        @for (actions of stmtActions; track $index; let i = $index) {
          <div class="statement-card">
            <div class="statement-body">
              <div class="statement-header">
                <span class="statement-label">Statement {{ i + 1 }}</span>
                @if (stmtActions.length > 1) {
                  <button mat-icon-button type="button" (click)="removeStatement(i)">
                    <mat-icon>delete</mat-icon>
                  </button>
                }
              </div>

              <mat-form-field appearance="outline" class="full-width">
                <mat-label>Actions</mat-label>
                <mat-chip-grid #actionsChipGrid>
                  @for (action of stmtActions[i]; track action) {
                    <mat-chip-row (removed)="removeAction(i, action)">
                      {{ action }}
                      <button matChipRemove><mat-icon>cancel</mat-icon></button>
                    </mat-chip-row>
                  }
                </mat-chip-grid>
                <input #actionInput
                       placeholder="Type action..."
                       [matChipInputFor]="actionsChipGrid"
                       [matChipInputSeparatorKeyCodes]="separatorKeyCodes"
                       (matChipInputTokenEnd)="addAction(i, $event)"
                       [matAutocomplete]="actionAuto"
                       (input)="actionInputValue = $any($event.target).value">
                <mat-autocomplete #actionAuto="matAutocomplete"
                                  (optionSelected)="onActionSelected(i, $event, actionInput)">
                  @for (action of getFilteredActions(i); track action) {
                    <mat-option [value]="action">{{ action }}</mat-option>
                  }
                </mat-autocomplete>
              </mat-form-field>

              <mat-form-field appearance="outline" class="full-width">
                <mat-label>Resources</mat-label>
                <mat-chip-grid #resourcesChipGrid>
                  @for (resource of stmtResources[i]; track resource) {
                    <mat-chip-row (removed)="removeResource(i, resource)"
                                  [class.invalid-resource]="getResourceError(resource)">
                      @if (getResourceError(resource)) {
                        <mat-icon matChipAvatar class="warning-icon" 
                                  [matTooltip]="getResourceError(resource)!">warning</mat-icon>
                      }
                      {{ resource }}
                      <button matChipRemove><mat-icon>cancel</mat-icon></button>
                    </mat-chip-row>
                  }
                </mat-chip-grid>
                <input placeholder="Type resource and press Enter"
                       [matChipInputFor]="resourcesChipGrid"
                       [matChipInputSeparatorKeyCodes]="separatorKeyCodes"
                       (matChipInputTokenEnd)="addResource(i, $event)">
                @if (hasInvalidResources(i)) {
                  <mat-hint class="warning-hint">
                    <mat-icon class="hint-icon">warning</mat-icon>
                    Some resources have invalid formats
                  </mat-hint>
                }
              </mat-form-field>
            </div>
          </div>
          @if (!$last) {
            <mat-divider></mat-divider>
          }
        }
        </form>
      }
    </mat-dialog-content>
    <mat-dialog-actions align="end">
      <button mat-button mat-dialog-close>Cancel</button>
      <button mat-button (click)="toggleJsonMode()">
        {{ jsonMode ? 'Edit as Form' : 'Edit as JSON' }}
      </button>
      <button mat-flat-button (click)="save()" [disabled]="jsonMode ? !jsonText.trim() : !isValid()">
        {{ data.mode === 'create' ? 'Create' : 'Save' }}
      </button>
    </mat-dialog-actions>
  `,
  styles: [`
    .policy-form {
      display: flex;
      flex-direction: column;
      min-width: 480px;
      gap: 4px;
    }
    .full-width { width: 100%; }
    .statements-header {
      display: flex;
      align-items: center;
      justify-content: space-between;
      margin: 8px 0 4px;
    }
    .statements-header h3 { margin: 0; }
    .statement-card {
      padding: 8px 0;
    }
    .statement-body {
      display: flex;
      flex-direction: column;
      gap: 4px;
    }
    .statement-header {
      display: flex;
      align-items: center;
      justify-content: space-between;
    }
    .statement-label {
      font-weight: 500;
      font-size: 14px;
      color: var(--mat-sys-on-surface-variant);
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
    .warning-hint {
      color: var(--mat-sys-error);
      display: flex;
      align-items: center;
      gap: 4px;
      font-size: 12px;
    }
    .hint-icon {
      font-size: 16px;
      width: 16px;
      height: 16px;
    }
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
export class PolicyDialogComponent implements OnInit {
  readonly data = inject<PolicyDialogData>(MAT_DIALOG_DATA);
  private dialogRef = inject(MatDialogRef<PolicyDialogComponent>);
  private fb = inject(FormBuilder);

  readonly separatorKeyCodes = [ENTER, COMMA] as const;

  form!: FormGroup;
  filteredAccounts: string[] = [];
  actionInputValue = '';
  stmtActions: string[][] = [];
  stmtResources: string[][] = [];

  // JSON editing mode
  jsonMode = false;
  jsonText = '';
  jsonError = '';

  ngOnInit(): void {
    const policy = this.data.policy;
    this.filteredAccounts = this.data.accounts;

    const stmts = policy?.statements || [];
    this.stmtActions = stmts.map(s => [...(s.actions || [])]);
    this.stmtResources = stmts.map(s => [...(s.resources || [])]);
    if (stmts.length === 0) {
      this.stmtActions = [[]];
      this.stmtResources = [[]];
    }

    const isGlobal = policy?.account === '_global';
    const accountDisabled = this.data.mode === 'edit' || isGlobal;

    this.form = this.fb.group({
      name: [policy?.name || '', Validators.required],
      isGlobal: [isGlobal],
      account: [{ value: policy?.account || this.data.currentAccount, disabled: accountDisabled }, Validators.required],
    });

    this.form.get('account')?.valueChanges.subscribe(val => {
      const q = (val || '').toLowerCase();
      this.filteredAccounts = this.data.accounts.filter(a => a.toLowerCase().includes(q));
    });
  }

  onGlobalChange(): void {
    const isGlobal = this.form.get('isGlobal')?.value;
    const accountControl = this.form.get('account');

    if (isGlobal) {
      accountControl?.setValue('_global');
      accountControl?.disable();
    } else {
      if (accountControl?.value === '_global') {
        accountControl?.setValue(this.data.currentAccount);
      }
      accountControl?.enable();
    }
  }

  isValid(): boolean {
    const raw = this.form.getRawValue();
    if (!raw.name?.trim() || !raw.account?.trim()) return false;
    return this.stmtActions.length > 0 &&
           this.stmtActions.every(a => a.length > 0) &&
           this.stmtResources.every(r => r.length > 0);
  }

  addStatement(): void {
    this.stmtActions.push([]);
    this.stmtResources.push([]);
  }

  removeStatement(index: number): void {
    this.stmtActions.splice(index, 1);
    this.stmtResources.splice(index, 1);
  }

  getFilteredActions(stmtIndex: number): string[] {
    const selected = this.stmtActions[stmtIndex] || [];
    const q = (this.actionInputValue || '').toLowerCase();
    return VALID_ACTIONS.filter(a => !selected.includes(a) && a.toLowerCase().includes(q));
  }

  addAction(stmtIndex: number, event: MatChipInputEvent): void {
    const value = (event.value || '').trim();
    if (value && VALID_ACTIONS.includes(value) && !this.stmtActions[stmtIndex].includes(value)) {
      this.stmtActions[stmtIndex].push(value);
    }
    event.chipInput.clear();
    this.actionInputValue = '';
  }

  onActionSelected(stmtIndex: number, event: MatAutocompleteSelectedEvent, input: HTMLInputElement): void {
    const value = event.option.viewValue;
    if (!this.stmtActions[stmtIndex].includes(value)) {
      this.stmtActions[stmtIndex].push(value);
    }
    input.value = '';
    this.actionInputValue = '';
  }

  removeAction(stmtIndex: number, action: string): void {
    this.stmtActions[stmtIndex] = this.stmtActions[stmtIndex].filter(a => a !== action);
  }

  addResource(stmtIndex: number, event: MatChipInputEvent): void {
    const value = (event.value || '').trim();
    if (value && !this.stmtResources[stmtIndex].includes(value)) {
      this.stmtResources[stmtIndex].push(value);
    }
    event.chipInput.clear();
  }

  removeResource(stmtIndex: number, resource: string): void {
    this.stmtResources[stmtIndex] = this.stmtResources[stmtIndex].filter(r => r !== resource);
  }

  getResourceError(resource: string): string | null {
    return validateResource(resource);
  }

  hasInvalidResources(stmtIndex: number): boolean {
    return this.stmtResources[stmtIndex].some(r => validateResource(r) !== null);
  }

  toggleJsonMode(): void {
    if (!this.jsonMode) {
      // Switching to JSON mode - convert form to JSON
      const policy = this.buildPolicyFromForm();
      this.jsonText = JSON.stringify(policy, null, 2);
      this.jsonError = '';
    } else {
      // Switching back to form mode - validate and parse JSON
      try {
        const policy = JSON.parse(this.jsonText);
        this.loadPolicyIntoForm(policy);
        this.jsonError = '';
      } catch (err) {
        this.jsonError = err instanceof Error ? err.message : 'Invalid JSON';
        return; // Don't switch modes if JSON is invalid
      }
    }
    this.jsonMode = !this.jsonMode;
  }

  private buildPolicyFromForm(): Policy {
    const raw = this.form.getRawValue();
    return {
      id: this.data.policy?.id || '',
      account: raw.account,
      name: raw.name,
      statements: this.stmtActions.map((actions, i) => ({
        effect: 'allow' as const,
        actions,
        resources: this.stmtResources[i],
      })),
    };
  }

  private loadPolicyIntoForm(policy: Policy): void {
    this.form.patchValue({
      name: policy.name,
      isGlobal: policy.account === '_global',
      account: policy.account,
    });

    this.stmtActions = policy.statements.map(s => [...(s.actions || [])]);
    this.stmtResources = policy.statements.map(s => [...(s.resources || [])]);
  }

  save(): void {
    if (this.jsonMode) {
      // Save from JSON
      try {
        const policy = JSON.parse(this.jsonText);
        // Preserve the original ID if editing
        if (this.data.policy?.id) {
          policy.id = this.data.policy.id;
        }
        this.dialogRef.close(policy);
      } catch (err) {
        this.jsonError = err instanceof Error ? err.message : 'Invalid JSON';
      }
    } else {
      // Save from form
      if (!this.isValid()) return;
      const policy = this.buildPolicyFromForm();
      this.dialogRef.close(policy);
    }
  }
}
