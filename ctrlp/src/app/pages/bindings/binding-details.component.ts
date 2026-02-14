import { Component, EventEmitter, Input, Output } from '@angular/core';
import { MatCardModule } from '@angular/material/card';
import { MatChipsModule } from '@angular/material/chips';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';
import { BindingEntry } from '../../models/binding.model';

@Component({
  selector: 'app-binding-details',
  standalone: true,
  imports: [MatCardModule, MatChipsModule, MatButtonModule, MatIconModule],
  template: `
    @if (entry) {
      <mat-card>
        <mat-card-header>
          <mat-card-title>{{ entry.binding.role }}</mat-card-title>
          <mat-card-subtitle>{{ entry.binding.account }}</mat-card-subtitle>
        </mat-card-header>
        <mat-card-content>
          <div class="section">
            <span class="section-label">Policies</span>
            <mat-chip-set>
              @for (policyId of entry.binding.policies; track policyId) {
                <mat-chip>
                  @if (danglingPolicies.has(policyId)) {
                    <mat-icon matChipAvatar class="warning-icon">warning</mat-icon>
                  }
                  {{ policyId }}
                </mat-chip>
              }
            </mat-chip-set>
            @if (danglingPolicies.size > 0) {
              <p class="warning-text">
                <mat-icon class="warning-icon-inline">warning</mat-icon>
                Some policies do not exist in the current account
              </p>
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
    .section {
      padding: 12px 0;
    }
    .section-label {
      font-size: 12px;
      color: var(--mat-sys-on-surface-variant);
      display: block;
      margin-bottom: 8px;
    }
    .warning-text {
      display: flex;
      align-items: center;
      gap: 4px;
      font-size: 12px;
      color: var(--mat-sys-error);
      margin-top: 8px;
    }
    .warning-icon, .warning-icon-inline {
      color: var(--mat-sys-error);
      font-size: 18px;
      width: 18px;
      height: 18px;
    }
  `],
})
export class BindingDetailsComponent {
  @Input() entry: BindingEntry | null = null;
  @Input() danglingPolicies = new Set<string>();
  @Output() edit = new EventEmitter<void>();
  @Output() delete = new EventEmitter<void>();
}
