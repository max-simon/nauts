import { Component, EventEmitter, Input, Output, inject } from '@angular/core';
import { RouterLink } from '@angular/router';
import { MatCardModule } from '@angular/material/card';
import { MatChipsModule } from '@angular/material/chips';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';
import { MatTooltipModule } from '@angular/material/tooltip';
import { BindingEntry } from '../../models/binding.model';
import { PolicyStoreService } from '../../services/policy-store.service';

@Component({
  selector: 'app-binding-details',
  standalone: true,
  imports: [RouterLink, MatCardModule, MatChipsModule, MatButtonModule, MatIconModule, MatTooltipModule],
  template: `
    @if (entry) {
      <mat-card>
        <mat-card-header>
          <mat-card-title>{{ entry.binding.role }}</mat-card-title>
          <mat-card-subtitle>
            <div class="metadata">
              <div class="metadata-item">
                <span class="metadata-label">Account:</span>
                <span class="metadata-value">{{ entry.binding.account }}</span>
              </div>
            </div>
          </mat-card-subtitle>
        </mat-card-header>
        <mat-card-content>
          <h3 class="section-title">Policies</h3>
          <div class="policies-list">
            @for (policyId of entry.binding.policies; track policyId) {
              @if (getPolicyForId(policyId); as policy) {
                <a [routerLink]="['/policies', policy.policy.account, policy.policy.id]" class="policy-link">
                  {{ policy.policy.name }} ({{ policy.policy.account }})
                </a>
              } @else {
                <div class="policy-link dangling">
                  <mat-icon class="warning-icon" matTooltip="Policy does not exist">warning</mat-icon>
                  {{ policyId }}
                </div>
              }
            }
          </div>
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
      color: var(--mat-sys-on-surface);
    }
    .section-title {
      font-size: 16px;
      font-weight: 500;
      margin: 16px 0 8px 0;
      color: var(--mat-sys-on-surface-variant);
    }
    .policies-list {
      display: flex;
      flex-direction: column;
      gap: 8px;
      margin-top: 8px;
    }
    .policy-link {
      color: var(--mat-sys-primary);
      text-decoration: none;
      font-size: 14px;
      padding: 4px 0;
      cursor: pointer;
      display: flex;
      align-items: center;
      gap: 8px;
    }
    .policy-link:not(.dangling):hover {
      text-decoration: underline;
    }
    .policy-link.dangling {
      color: var(--mat-sys-error);
      cursor: default;
    }
    .warning-icon {
      color: var(--mat-sys-error);
      font-size: 18px;
      width: 18px;
      height: 18px;
    }
  `],
})
export class BindingDetailsComponent {
  private policyStore = inject(PolicyStoreService);

  @Input() entry: BindingEntry | null = null;
  @Input() danglingPolicies = new Set<string>();
  @Input() policyMap = new Map<string, string>(); // id -> name
  @Output() edit = new EventEmitter<void>();
  @Output() delete = new EventEmitter<void>();

  getPolicyForId(policyId: string) {
    if (!this.entry) return null;

    const account = this.entry.binding.account;
    const allPolicies = this.policyStore.listAllPolicies();

    return allPolicies.find(p =>
      p.policy.id === policyId &&
      (p.policy.account === account || p.policy.account === '_global')
    ) || null;
  }
}
