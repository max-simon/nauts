import { Component, EventEmitter, Input, Output, OnChanges, inject } from '@angular/core';
import { RouterLink } from '@angular/router';
import { MatCardModule } from '@angular/material/card';
import { MatChipsModule } from '@angular/material/chips';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';
import { MatDividerModule } from '@angular/material/divider';
import { MatTooltipModule } from '@angular/material/tooltip';
import { MatExpansionModule } from '@angular/material/expansion';
import { PolicyEntry } from '../../models/policy.model';
import { BindingEntry } from '../../models/binding.model';
import { PolicyStoreService } from '../../services/policy-store.service';
import { validatePolicyResources } from '../../validators/resource.validator';

@Component({
  selector: 'app-policy-details',
  standalone: true,
  imports: [RouterLink, MatCardModule, MatChipsModule, MatButtonModule, MatIconModule, MatDividerModule, MatTooltipModule, MatExpansionModule],
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

          <mat-accordion multi>
            <mat-expansion-panel [expanded]="true">
              <mat-expansion-panel-header>
                <mat-panel-title>Statements</mat-panel-title>
              </mat-expansion-panel-header>
              @for (stmt of entry.policy.statements; track $index; let i = $index) {
                <div class="statement">
                  <div class="statement-label">Statement {{ i + 1 }}</div>
                  <div class="statement-content">
                    <div class="chip-section">
                      <span class="chip-label">Actions</span>
                      <mat-chip-set>
                        @for (action of stmt.actions; track action) {
                          <mat-chip>{{ action }}</mat-chip>
                        }
                      </mat-chip-set>
                    </div>
                    <div class="chip-section">
                      <span class="chip-label">Resources</span>
                      <mat-chip-set>
                        @for (resource of stmt.resources; track resource) {
                          <mat-chip [class.invalid-resource]="getResourceError(resource)">
                            @if (getResourceError(resource)) {
                              <mat-icon matChipAvatar class="warning-icon"
                                        [matTooltip]="getResourceError(resource)!">warning</mat-icon>
                            }
                            {{ resource }}
                          </mat-chip>
                        }
                      </mat-chip-set>
                    </div>
                  </div>
                </div>
                @if (!$last) {
                  <mat-divider></mat-divider>
                }
              }
            </mat-expansion-panel>

            @if (relatedBindings.length > 0) {
              <mat-expansion-panel [expanded]="false">
                <mat-expansion-panel-header>
                  <mat-panel-title>Bindings ({{ relatedBindings.length }})</mat-panel-title>
                </mat-expansion-panel-header>
                <div class="bindings-list">
                  @for (binding of relatedBindings; track binding.binding.role) {
                    <a [routerLink]="['/bindings', binding.binding.account, binding.binding.role]" class="binding-link">
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
    .statement {
      padding: 12px 0;
    }
    .statement-label {
      font-weight: 500;
      font-size: 14px;
      margin-bottom: 12px;
      color: var(--mat-sys-on-surface-variant);
    }
    .statement-content {
      display: grid;
      grid-template-columns: 1fr 1fr;
      gap: 16px;
    }
    .chip-section {
      min-width: 0;
    }
    .chip-label {
      font-size: 12px;
      color: var(--mat-sys-on-surface-variant);
      display: block;
      margin-bottom: 4px;
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
  `],
})
export class PolicyDetailsComponent implements OnChanges {
  private policyStore = inject(PolicyStoreService);

  @Input() entry: PolicyEntry | null = null;
  @Output() edit = new EventEmitter<void>();
  @Output() delete = new EventEmitter<void>();

  resourceErrors = new Map<string, string>();
  relatedBindings: BindingEntry[] = [];

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

    this.relatedBindings = this.policyStore.getBindingsForPolicy(this.entry.policy.id);
  }

  getResourceError(resource: string): string | null {
    return this.resourceErrors.get(resource) || null;
  }
}
