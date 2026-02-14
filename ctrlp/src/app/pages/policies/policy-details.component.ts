import { Component, EventEmitter, Input, Output, OnChanges } from '@angular/core';
import { MatCardModule } from '@angular/material/card';
import { MatChipsModule } from '@angular/material/chips';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';
import { MatDividerModule } from '@angular/material/divider';
import { MatTooltipModule } from '@angular/material/tooltip';
import { PolicyEntry } from '../../models/policy.model';
import { validatePolicyResources } from '../../validators/resource.validator';

@Component({
  selector: 'app-policy-details',
  standalone: true,
  imports: [MatCardModule, MatChipsModule, MatButtonModule, MatIconModule, MatDividerModule, MatTooltipModule],
  template: `
    @if (entry) {
      <mat-card>
        <mat-card-header>
          <mat-card-title>{{ entry.policy.name }}</mat-card-title>
          <mat-card-subtitle>
            <mat-chip-set>
              <mat-chip>{{ entry.policy.id }}</mat-chip>
              <mat-chip>{{ entry.policy.account }}</mat-chip>
            </mat-chip-set>
          </mat-card-subtitle>
        </mat-card-header>
        <mat-card-content>
          @for (stmt of entry.policy.statements; track $index; let i = $index) {
            <div class="statement">
              <div class="statement-label">Statement {{ i + 1 }}</div>
              <div class="chip-section">
                <span class="chip-label">Effect</span>
                <mat-chip-set>
                  <mat-chip class="effect-chip">{{ stmt.effect }}</mat-chip>
                </mat-chip-set>
              </div>
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
            @if (!$last) {
              <mat-divider></mat-divider>
            }
          }
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
    .statement {
      padding: 12px 0;
    }
    .statement-label {
      font-weight: 500;
      font-size: 14px;
      margin-bottom: 8px;
      color: var(--mat-sys-on-surface-variant);
    }
    .chip-section {
      margin: 8px 0;
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
  `],
})
export class PolicyDetailsComponent implements OnChanges {
  @Input() entry: PolicyEntry | null = null;
  @Output() edit = new EventEmitter<void>();
  @Output() delete = new EventEmitter<void>();
  
  resourceErrors = new Map<string, string>();

  ngOnChanges(): void {
    if (this.entry) {
      this.resourceErrors = validatePolicyResources(this.entry.policy);
    } else {
      this.resourceErrors.clear();
    }
  }

  getResourceError(resource: string): string | null {
    return this.resourceErrors.get(resource) || null;
  }
}
