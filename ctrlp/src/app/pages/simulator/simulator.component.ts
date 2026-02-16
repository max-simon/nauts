import { Component, inject, OnInit } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { RouterLink } from '@angular/router';
import { MatCardModule } from '@angular/material/card';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatInputModule } from '@angular/material/input';
import { MatSelectModule } from '@angular/material/select';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';
import { MatProgressBarModule } from '@angular/material/progress-bar';
import { MatExpansionModule } from '@angular/material/expansion';
import { MatSnackBar, MatSnackBarModule } from '@angular/material/snack-bar';
import { NatsService } from '../../services/nats.service';
import { PolicyStoreService } from '../../services/policy-store.service';
import { NavigationService } from '../../services/navigation.service';

interface Role {
  account: string;
  name: string;
}

interface User {
  id: string;
  roles: Role[];
}

interface DebugRequest {
  user: User;
  account: string;
}

interface DebugResponse {
  request: DebugRequest;
  compilation_result?: {
    permissions: {
      allow?: {
        publish?: string[];
        subscribe?: string[];
      };
      deny?: {
        publish?: string[];
        subscribe?: string[];
      };
    };
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
    RouterLink,
    MatCardModule,
    MatFormFieldModule,
    MatInputModule,
    MatSelectModule,
    MatButtonModule,
    MatIconModule,
    MatProgressBarModule,
    MatExpansionModule,
    MatSnackBarModule,
  ],
  template: `
    <div class="page-container">
      <div class="content-grid">
        <!-- Left: Input Form -->
        <div class="form-panel">
          <mat-card>
            <mat-card-header>
              <mat-card-title>User Configuration</mat-card-title>
            </mat-card-header>
            <mat-card-content>
              <div class="form-fields">
                <mat-form-field appearance="outline" class="full-width">
                  <mat-label>User Name</mat-label>
                  <input matInput [(ngModel)]="userName" placeholder="alice">
                </mat-form-field>

                <mat-form-field appearance="outline" class="full-width">
                  <mat-label>Target Account</mat-label>
                  <mat-select [(ngModel)]="targetAccount">
                    @for (account of availableAccounts; track account) {
                      <mat-option [value]="account">{{ account }}</mat-option>
                    }
                  </mat-select>
                </mat-form-field>

                <mat-form-field appearance="outline" class="full-width">
                  <mat-label>Roles</mat-label>
                  <mat-select [(ngModel)]="selectedRoles" multiple>
                    @for (role of availableRoles; track role) {
                      <mat-option [value]="role">{{ role }}</mat-option>
                    }
                  </mat-select>
                </mat-form-field>
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
        </div>

        <!-- Right: Results -->
        <div class="result-panel">
          @if (loading) {
            <mat-card class="loading-card">
              <mat-card-content>
                <mat-progress-bar mode="indeterminate"></mat-progress-bar>
                <p class="loading-text">Compiling permissions...</p>
              </mat-card-content>
            </mat-card>
          }

          @if (response && !loading) {
            <mat-card>
              <mat-card-header>
                <mat-card-title>
                  @if (response.error) {
                    <span class="error-title">Simulation Failed</span>
                  } @else {
                    <span class="success-title">Compilation Result</span>
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
                    <!-- Permissions -->
                    <mat-expansion-panel [expanded]="true">
                      <mat-expansion-panel-header>
                        <mat-panel-title>
                          Permissions
                        </mat-panel-title>
                      </mat-expansion-panel-header>
                      <div class="permissions-grid">
                        <div class="permission-section">
                          <div class="permission-header">
                            <span>Publish</span>
                          </div>
                          <div class="subject-list">
                            @for (subject of getPublishSubjects(); track subject) {
                              <div class="subject-item">{{ subject }}</div>
                            } @empty {
                              <div class="empty-message">No publish permissions</div>
                            }
                          </div>
                        </div>
                        <div class="permission-section">
                          <div class="permission-header">
                            <span>Subscribe</span>
                          </div>
                          <div class="subject-list">
                            @for (subject of getSubscribeSubjects(); track subject) {
                              <div class="subject-item">{{ subject }}</div>
                            } @empty {
                              <div class="empty-message">No subscribe permissions</div>
                            }
                          </div>
                        </div>
                      </div>
                    </mat-expansion-panel>

                    <!-- Roles & Policies -->
                    <mat-expansion-panel>
                      <mat-expansion-panel-header>
                        <mat-panel-title>
                          Roles & Policies ({{ response.compilation_result.roles.length }})
                        </mat-panel-title>
                      </mat-expansion-panel-header>
                      <div class="roles-container">
                        @for (role of response.compilation_result.roles; track role) {
                          <div class="role-card">
                            <div class="role-header">
                              <span class="role-name">{{ role.account }}.{{ role.name }}</span>
                            </div>
                            <div class="role-details">
                              <div class="detail-section">
                                <span class="detail-label">Binding:</span>
                                <a [routerLink]="['/bindings', role.account, role.name]" class="detail-link">
                                  {{ role.name }} ({{ role.account }})
                                </a>
                              </div>
                              @if (getPoliciesForRole(role); as policies) {
                                @if (policies.length > 0) {
                                  <div class="detail-section">
                                    <span class="detail-label">Policies:</span>
                                    <div class="policy-links">
                                      @for (policy of policies; track policy.id) {
                                        <a [routerLink]="getPolicyRoute(policy)" class="detail-link">
                                          {{ policy.name }}
                                          (<span [class.global-text]="policy.account === '_global'">{{ policy.account === '_global' ? 'global' : policy.account }}</span>)
                                        </a>
                                      }
                                    </div>
                                  </div>
                                }
                              }
                            </div>
                          </div>
                        }
                      </div>
                    </mat-expansion-panel>

                    <!-- Raw Response -->
                    <mat-expansion-panel>
                      <mat-expansion-panel-header>
                        <mat-panel-title>
                          Raw Response
                        </mat-panel-title>
                      </mat-expansion-panel-header>
                      <div class="raw-response">
                        <pre>{{ getRawResponse() }}</pre>
                      </div>
                    </mat-expansion-panel>
                  </mat-accordion>
                }
              </mat-card-content>
            </mat-card>
          }

          @if (!response && !loading) {
            <mat-card class="placeholder-card">
              <mat-card-content>
                <mat-icon class="placeholder-icon">play_circle_outline</mat-icon>
                <p>Configure a user and click Simulate to see results</p>
              </mat-card-content>
            </mat-card>
          }
        </div>
      </div>
    </div>
  `,
  styles: [`
    .page-container {
      padding: 24px;
      max-width: 1400px;
      margin: 0 auto;
      height: calc(100vh - 112px);
      display: flex;
      flex-direction: column;
    }
    .page-header {
      margin-bottom: 24px;
      text-align: center;
    }
    .page-header h1 {
      margin: 0 0 8px;
      font-size: 32px;
      font-weight: 500;
    }
    .subtitle {
      margin: 0;
      color: var(--mat-sys-on-surface-variant);
      font-size: 16px;
    }
    .content-grid {
      display: grid;
      grid-template-columns: 1fr 1fr;
      gap: 24px;
      flex: 1;
      min-height: 0;
    }
    .form-panel, .result-panel {
      display: flex;
      flex-direction: column;
    }
    .form-panel mat-card {
      height: fit-content;
    }
    .result-panel {
      overflow-y: auto;
    }
    .form-fields {
      display: flex;
      flex-direction: column;
      gap: 16px;
      padding: 8px 0;
    }
    .full-width {
      width: 100%;
    }
    .loading-card, .placeholder-card {
      display: flex;
      align-items: center;
      justify-content: center;
      min-height: 200px;
    }
    .loading-text {
      text-align: center;
      margin-top: 16px;
      color: var(--mat-sys-on-surface-variant);
    }
    .placeholder-card mat-card-content {
      display: flex;
      flex-direction: column;
      align-items: center;
      gap: 16px;
      padding: 48px;
      color: var(--mat-sys-on-surface-variant);
    }
    .placeholder-icon {
      font-size: 64px;
      width: 64px;
      height: 64px;
      opacity: 0.5;
    }
    .success-title {
      color: #4caf50;
    }
    .error-title {
      color: var(--mat-sys-error);
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
    mat-accordion {
      display: flex;
      flex-direction: column;
      gap: 8px;
    }
    mat-expansion-panel {
      box-shadow: none !important;
      border: 1px solid var(--mat-sys-outline-variant);
    }
    mat-panel-title {
      font-weight: 500;
    }
    .permissions-grid {
      display: grid;
      grid-template-columns: 1fr 1fr;
      gap: 16px;
      padding: 16px 0;
    }
    .permission-section {
      display: flex;
      flex-direction: column;
      gap: 12px;
    }
    .permission-header {
      font-weight: 500;
      font-size: 14px;
      color: var(--mat-sys-on-surface-variant);
    }
    .subject-list {
      display: flex;
      flex-direction: column;
      gap: 4px;
    }
    .subject-item {
      padding: 6px 12px;
      background: var(--mat-sys-surface-variant);
      border-radius: 4px;
      font-family: monospace;
      font-size: 12px;
    }
    .empty-message {
      padding: 12px;
      color: var(--mat-sys-on-surface-variant);
      font-size: 14px;
      font-style: italic;
      text-align: center;
    }
    .roles-container {
      display: flex;
      flex-direction: column;
      gap: 16px;
      padding: 16px 0;
    }
    .role-card {
      border: 1px solid var(--mat-sys-outline-variant);
      border-radius: 4px;
      padding: 16px;
      background: var(--mat-sys-surface-container-low);
    }
    .role-header {
      margin-bottom: 12px;
      padding-bottom: 12px;
      border-bottom: 1px solid var(--mat-sys-outline-variant);
    }
    .role-name {
      font-weight: 500;
      font-size: 16px;
      font-family: monospace;
      color: var(--mat-sys-primary);
    }
    .role-details {
      display: flex;
      flex-direction: column;
      gap: 8px;
    }
    .detail-section {
      display: flex;
      flex-direction: column;
      gap: 4px;
    }
    .detail-label {
      font-size: 12px;
      font-weight: 500;
      color: var(--mat-sys-on-surface-variant);
      text-transform: uppercase;
    }
    .detail-link {
      color: var(--mat-sys-primary);
      text-decoration: none;
      font-size: 14px;
    }
    .detail-link:hover {
      text-decoration: underline;
    }
    .policy-links {
      display: flex;
      flex-direction: column;
      gap: 4px;
    }
    .global-text {
      font-style: italic;
      color: var(--mat-sys-on-surface-variant);
    }
    .raw-response {
      padding: 16px 0;
    }
    .raw-response pre {
      margin: 0;
      padding: 16px;
      background: var(--mat-sys-surface-variant);
      border-radius: 4px;
      overflow-x: auto;
      font-family: 'Courier New', monospace;
      font-size: 12px;
      line-height: 1.5;
      color: var(--mat-sys-on-surface);
    }
  `],
})
export class SimulatorComponent implements OnInit {
  private nats = inject(NatsService);
  private snackBar = inject(MatSnackBar);
  private policyStore = inject(PolicyStoreService);
  private navigationService = inject(NavigationService);

  userName = '';
  targetAccount = '';
  selectedRoles: string[] = [];

  availableAccounts: string[] = [];
  availableRoles: string[] = [];

  loading = false;
  response: DebugResponse | null = null;

  async ngOnInit(): Promise<void> {
    // Load available accounts and roles
    await this.loadAccountsAndRoles();

    // Try to restore state from localStorage
    const savedState = this.loadStateFromStorage();
    if (savedState) {
      this.userName = savedState.userName || '';
      this.targetAccount = savedState.targetAccount || '';
      this.selectedRoles = savedState.selectedRoles || [];
      this.response = savedState.response || null;
    } else {
      // Set default account from navigation service if no saved state
      this.navigationService.getCurrentAccount$().subscribe(account => {
        if (account && this.availableAccounts.includes(account)) {
          this.targetAccount = account;
        } else if (this.availableAccounts.length > 0) {
          this.targetAccount = this.availableAccounts[0];
        }
      });

      // Load some example data
      this.userName = 'alice';
    }
  }

  async loadAccountsAndRoles(): Promise<void> {
    await this.policyStore.initialize();

    // Get all bindings to extract accounts and roles
    const allBindings = this.policyStore.listAllBindings();

    const accountSet = new Set<string>();
    const roleSet = new Set<string>();

    allBindings.forEach(entry => {
      accountSet.add(entry.binding.account);
      roleSet.add(`${entry.binding.account}.${entry.binding.role}`);
    });

    this.availableAccounts = Array.from(accountSet).sort();
    this.availableRoles = Array.from(roleSet).sort();
  }

  canSimulate(): boolean {
    return this.userName.trim() !== '' && this.targetAccount.trim() !== '' && this.selectedRoles.length > 0;
  }

  clearForm(): void {
    this.userName = '';
    this.selectedRoles = [];
    this.response = null;
    this.clearStateFromStorage();
  }

  async simulate(): Promise<void> {
    if (!this.canSimulate()) return;

    this.loading = true;
    this.response = null;

    try {
      const nc = this.nats.getConnection();

      // Parse selected roles into Role objects
      const userRoles: Role[] = this.selectedRoles.map(roleStr => {
        const [account, name] = roleStr.split('.');
        return { account, name };
      });

      const user: User = {
        id: this.userName,
        roles: userRoles,
      };

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

      // Save state to localStorage
      this.saveStateToStorage();

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

  getPublishSubjects(): string[] {
    if (!this.response?.compilation_result) return [];

    const perms = this.response.compilation_result as any;
    const pubAllow = perms?.permissions?.pub?.allow || [];

    // Extract subject field from each permission object
    return pubAllow.map((p: any) => p.subject || p.Subject).filter(Boolean);
  }

  getSubscribeSubjects(): string[] {
    if (!this.response?.compilation_result) return [];

    const perms = this.response.compilation_result as any;
    const subAllow = perms?.permissions?.sub?.allow || [];

    // Extract subject field from each permission object
    return subAllow.map((p: any) => p.subject || p.Subject).filter(Boolean);
  }

  getPoliciesForRole(role: Role): Array<{ id: string; name: string; account: string }> {
    const binding = this.policyStore.getBinding(role.account, role.name);
    if (!binding) return [];

    const policies: Array<{ id: string; name: string; account: string }> = [];

    for (const policyId of binding.binding.policies) {
      // Strip "_global:" prefix if present
      const cleanPolicyId = policyId.startsWith('_global:') ? policyId.substring(8) : policyId;

      const allPolicies = this.policyStore.listAllPolicies();
      const policyEntry = allPolicies.find(p =>
        p.policy.id === cleanPolicyId &&
        (p.policy.account === role.account || p.policy.account === '_global')
      );

      if (policyEntry) {
        policies.push({
          id: policyEntry.policy.id,
          name: policyEntry.policy.name,
          account: policyEntry.policy.account,
        });
      }
    }

    return policies;
  }

  getPolicyRoute(policy: { id: string; account: string }): string[] {
    // For global policies, use the current target account in the route
    const routeAccount = policy.account === '_global' ? this.targetAccount : policy.account;
    return ['/policies', routeAccount, policy.id];
  }

  getRawResponse(): string {
    if (!this.response) return '';
    return JSON.stringify(this.response, null, 2);
  }

  private saveStateToStorage(): void {
    try {
      const state = {
        userName: this.userName,
        targetAccount: this.targetAccount,
        selectedRoles: this.selectedRoles,
        response: this.response,
      };
      localStorage.setItem('simulator-state', JSON.stringify(state));
    } catch (err) {
      console.error('Failed to save simulator state:', err);
    }
  }

  private loadStateFromStorage(): any {
    try {
      const stored = localStorage.getItem('simulator-state');
      if (stored) {
        return JSON.parse(stored);
      }
    } catch (err) {
      console.error('Failed to load simulator state:', err);
    }
    return null;
  }

  private clearStateFromStorage(): void {
    try {
      localStorage.removeItem('simulator-state');
    } catch (err) {
      console.error('Failed to clear simulator state:', err);
    }
  }
}
