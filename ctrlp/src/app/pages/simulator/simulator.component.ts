import { Component, inject, OnInit } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { MatCardModule } from '@angular/material/card';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatInputModule } from '@angular/material/input';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';
import { MatChipsModule } from '@angular/material/chips';
import { MatProgressBarModule } from '@angular/material/progress-bar';
import { MatExpansionModule } from '@angular/material/expansion';
import { MatSnackBar, MatSnackBarModule } from '@angular/material/snack-bar';
import { COMMA, ENTER } from '@angular/cdk/keycodes';
import { MatChipInputEvent } from '@angular/material/chips';
import { NatsService } from '../../services/nats.service';

interface Role {
  account: string;
  name: string;
}

interface User {
  id: string;
  roles: Role[];
  attributes?: Record<string, string>;
}

interface DebugRequest {
  user: User;
  account: string;
}

interface DebugResponse {
  request: DebugRequest;
  compilation_result?: {
    user: any;
    permissions: any;
    permissionsRaw: any;
    warnings: string[];
    roles: Role[];
    policies: Record<string, any[]>;
  };
  error?: {
    code: string;
    message: string;
  };
}

@Component({
  selector: 'app-simulator',
  standalone: true,
  imports: [
    FormsModule,
    MatCardModule,
    MatFormFieldModule,
    MatInputModule,
    MatButtonModule,
    MatIconModule,
    MatChipsModule,
    MatProgressBarModule,
    MatExpansionModule,
    MatSnackBarModule,
  ],
  template: `
    <div class="page-container">
      <div class="simulator-panel">
        <h1>Permission Simulator</h1>
        <p class="subtitle">Test permission compilation for users and accounts</p>

        <mat-card class="input-card">
          <mat-card-header>
            <mat-card-title>Request Configuration</mat-card-title>
          </mat-card-header>
          <mat-card-content>
            <div class="form-grid">
              <mat-form-field appearance="outline" class="full-width">
                <mat-label>User ID</mat-label>
                <input matInput [(ngModel)]="userId" placeholder="alice">
              </mat-form-field>

              <mat-form-field appearance="outline" class="full-width">
                <mat-label>Target Account</mat-label>
                <input matInput [(ngModel)]="targetAccount" placeholder="APP">
              </mat-form-field>

              <div class="chip-input-section">
                <label class="chip-label">Roles (format: account.role)</label>
                <mat-chip-grid #rolesChipGrid>
                  @for (role of roles; track role) {
                    <mat-chip-row (removed)="removeRole(role)">
                      {{ role }}
                      <button matChipRemove><mat-icon>cancel</mat-icon></button>
                    </mat-chip-row>
                  }
                </mat-chip-grid>
                <input
                  placeholder="Type role and press Enter (e.g., APP.admin)"
                  [matChipInputFor]="rolesChipGrid"
                  [matChipInputSeparatorKeyCodes]="separatorKeyCodes"
                  (matChipInputTokenEnd)="addRole($event)">
              </div>

              <div class="chip-input-section">
                <label class="chip-label">Attributes (format: key=value)</label>
                <mat-chip-grid #attrsChipGrid>
                  @for (attr of getAttributesArray(); track attr) {
                    <mat-chip-row (removed)="removeAttribute(attr)">
                      {{ attr }}
                      <button matChipRemove><mat-icon>cancel</mat-icon></button>
                    </mat-chip-row>
                  }
                </mat-chip-grid>
                <input
                  placeholder="Type attribute and press Enter (e.g., department=engineering)"
                  [matChipInputFor]="attrsChipGrid"
                  [matChipInputSeparatorKeyCodes]="separatorKeyCodes"
                  (matChipInputTokenEnd)="addAttribute($event)">
              </div>
            </div>
          </mat-card-content>
          <mat-card-actions align="end">
            <button mat-button (click)="clearForm()">
              <mat-icon>clear</mat-icon> Clear
            </button>
            <button mat-flat-button color="primary" (click)="simulate()" [disabled]="loading || !canSimulate()">
              <mat-icon>play_arrow</mat-icon> Simulate
            </button>
          </mat-card-actions>
        </mat-card>

        @if (loading) {
          <mat-progress-bar mode="indeterminate"></mat-progress-bar>
        }

        @if (response) {
          <mat-card class="result-card">
            <mat-card-header>
              <mat-card-title>
                @if (response.error) {
                  <mat-icon class="error-icon">error</mat-icon>
                  Simulation Failed
                } @else {
                  <mat-icon class="success-icon">check_circle</mat-icon>
                  Simulation Result
                }
              </mat-card-title>
            </mat-card-header>
            <mat-card-content>
              @if (response.error) {
                <div class="error-message">
                  <strong>{{ response.error.code }}</strong>
                  <p>{{ response.error.message }}</p>
                </div>
              } @else if (response.compilation_result) {
                <mat-accordion multi>
                  <mat-expansion-panel [expanded]="true">
                    <mat-expansion-panel-header>
                      <mat-panel-title>Permissions</mat-panel-title>
                    </mat-expansion-panel-header>
                    <pre class="json-output">{{ formatJson(response.compilation_result.permissions) }}</pre>
                  </mat-expansion-panel>

                  <mat-expansion-panel>
                    <mat-expansion-panel-header>
                      <mat-panel-title>Roles ({{ response.compilation_result.roles.length }})</mat-panel-title>
                    </mat-expansion-panel-header>
                    <div class="roles-list">
                      @for (role of response.compilation_result.roles; track role) {
                        <div class="role-item">{{ role.account }}.{{ role.name }}</div>
                      }
                    </div>
                  </mat-expansion-panel>

                  <mat-expansion-panel>
                    <mat-expansion-panel-header>
                      <mat-panel-title>Policies</mat-panel-title>
                    </mat-expansion-panel-header>
                    <pre class="json-output">{{ formatJson(response.compilation_result.policies) }}</pre>
                  </mat-expansion-panel>

                  @if (response.compilation_result.warnings.length > 0) {
                    <mat-expansion-panel>
                      <mat-expansion-panel-header>
                        <mat-panel-title>
                          <mat-icon class="warning-icon">warning</mat-icon>
                          Warnings ({{ response.compilation_result.warnings.length }})
                        </mat-panel-title>
                      </mat-expansion-panel-header>
                      <div class="warnings-list">
                        @for (warning of response.compilation_result.warnings; track warning) {
                          <div class="warning-item">{{ warning }}</div>
                        }
                      </div>
                    </mat-expansion-panel>
                  }

                  <mat-expansion-panel>
                    <mat-expansion-panel-header>
                      <mat-panel-title>Raw Response</mat-panel-title>
                    </mat-expansion-panel-header>
                    <pre class="json-output">{{ formatJson(response) }}</pre>
                  </mat-expansion-panel>
                </mat-accordion>
              }
            </mat-card-content>
          </mat-card>
        }
      </div>
    </div>
  `,
  styles: [`
    .page-container {
      padding: 24px;
      max-width: 1200px;
      margin: 0 auto;
    }
    .simulator-panel h1 {
      margin: 0 0 8px;
      font-size: 32px;
      font-weight: 500;
    }
    .subtitle {
      margin: 0 0 24px;
      color: var(--mat-sys-on-surface-variant);
      font-size: 16px;
    }
    .input-card {
      margin-bottom: 16px;
    }
    .form-grid {
      display: flex;
      flex-direction: column;
      gap: 16px;
    }
    .full-width {
      width: 100%;
    }
    .chip-input-section {
      display: flex;
      flex-direction: column;
      gap: 8px;
    }
    .chip-label {
      font-size: 14px;
      color: var(--mat-sys-on-surface-variant);
      font-weight: 500;
    }
    .result-card {
      margin-top: 16px;
    }
    .error-message {
      padding: 16px;
      background: var(--mat-sys-error-container);
      color: var(--mat-sys-on-error-container);
      border-radius: 4px;
    }
    .error-message strong {
      display: block;
      margin-bottom: 8px;
      font-size: 16px;
    }
    .error-icon {
      color: var(--mat-sys-error);
      margin-right: 8px;
    }
    .success-icon {
      color: #4caf50;
      margin-right: 8px;
    }
    .warning-icon {
      color: var(--mat-sys-error);
      margin-right: 8px;
      font-size: 20px;
      width: 20px;
      height: 20px;
    }
    .json-output {
      background: var(--mat-sys-surface-variant);
      padding: 16px;
      border-radius: 4px;
      overflow-x: auto;
      font-family: 'Courier New', monospace;
      font-size: 12px;
      line-height: 1.5;
      margin: 0;
    }
    .roles-list {
      display: flex;
      flex-direction: column;
      gap: 8px;
      padding: 8px 0;
    }
    .role-item {
      padding: 8px 12px;
      background: var(--mat-sys-surface-variant);
      border-radius: 4px;
      font-family: monospace;
    }
    .warnings-list {
      display: flex;
      flex-direction: column;
      gap: 8px;
      padding: 8px 0;
    }
    .warning-item {
      padding: 12px;
      background: var(--mat-sys-error-container);
      color: var(--mat-sys-on-error-container);
      border-radius: 4px;
      font-size: 14px;
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
  `],
})
export class SimulatorComponent implements OnInit {
  private nats = inject(NatsService);
  private snackBar = inject(MatSnackBar);

  readonly separatorKeyCodes = [ENTER, COMMA] as const;

  userId = '';
  targetAccount = '';
  roles: string[] = [];
  attributes: Record<string, string> = {};

  loading = false;
  response: DebugResponse | null = null;

  ngOnInit(): void {
    // Load some example data
    this.userId = 'alice';
    this.targetAccount = 'APP';
    this.roles = ['APP.admin'];
  }

  canSimulate(): boolean {
    return this.userId.trim() !== '' && this.targetAccount.trim() !== '';
  }

  addRole(event: MatChipInputEvent): void {
    const value = (event.value || '').trim();
    if (value && !this.roles.includes(value)) {
      // Validate format: account.role
      if (!value.includes('.')) {
        this.snackBar.open('Role must be in format: account.role', 'Dismiss', { duration: 3000 });
        event.chipInput.clear();
        return;
      }
      this.roles.push(value);
    }
    event.chipInput.clear();
  }

  removeRole(role: string): void {
    this.roles = this.roles.filter(r => r !== role);
  }

  addAttribute(event: MatChipInputEvent): void {
    const value = (event.value || '').trim();
    if (value) {
      // Parse key=value format
      const [key, ...valueParts] = value.split('=');
      if (!key || valueParts.length === 0) {
        this.snackBar.open('Attribute must be in format: key=value', 'Dismiss', { duration: 3000 });
        event.chipInput.clear();
        return;
      }
      this.attributes[key] = valueParts.join('=');
    }
    event.chipInput.clear();
  }

  removeAttribute(attr: string): void {
    const [key] = attr.split('=');
    delete this.attributes[key];
  }

  getAttributesArray(): string[] {
    return Object.entries(this.attributes).map(([key, value]) => `${key}=${value}`);
  }

  clearForm(): void {
    this.userId = '';
    this.targetAccount = '';
    this.roles = [];
    this.attributes = {};
    this.response = null;
  }

  async simulate(): Promise<void> {
    if (!this.canSimulate()) return;

    this.loading = true;
    this.response = null;

    try {
      const nc = this.nats.getConnection();

      // Build user object with roles
      const userRoles: Role[] = this.roles.map(roleStr => {
        const [account, name] = roleStr.split('.');
        return { account, name };
      });

      const user: User = {
        id: this.userId,
        roles: userRoles,
      };

      if (Object.keys(this.attributes).length > 0) {
        user.attributes = this.attributes;
      }

      const request: DebugRequest = {
        user,
        account: this.targetAccount,
      };

      // Send request to nauts.debug
      const response = await nc.request(
        'nauts.debug',
        new TextEncoder().encode(JSON.stringify(request)),
        { timeout: 5000 }
      );

      const responseData = JSON.parse(new TextDecoder().decode(response.data));
      this.response = responseData;

      if (responseData.error) {
        this.snackBar.open(`Simulation failed: ${responseData.error.message}`, 'Dismiss', { duration: 5000 });
      } else {
        this.snackBar.open('Simulation completed successfully', 'Dismiss', { duration: 3000 });
      }
    } catch (err) {
      console.error('Simulation error:', err);
      this.snackBar.open(
        err instanceof Error ? err.message : 'Simulation request failed',
        'Dismiss',
        { duration: 5000 }
      );
    } finally {
      this.loading = false;
    }
  }

  formatJson(obj: any): string {
    return JSON.stringify(obj, null, 2);
  }
}
