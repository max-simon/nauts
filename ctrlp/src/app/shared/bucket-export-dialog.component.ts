import { Component, inject, OnInit } from '@angular/core';
import { MatDialogModule, MatDialogRef } from '@angular/material/dialog';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatInputModule } from '@angular/material/input';
import { MatButtonModule } from '@angular/material/button';
import { MatProgressBarModule } from '@angular/material/progress-bar';
import { MatSnackBar, MatSnackBarModule } from '@angular/material/snack-bar';
import { FormsModule } from '@angular/forms';
import { KvStoreService } from '../services/kv-store.service';

@Component({
  selector: 'app-bucket-export-dialog',
  standalone: true,
  imports: [
    FormsModule,
    MatDialogModule,
    MatFormFieldModule,
    MatInputModule,
    MatButtonModule,
    MatProgressBarModule,
    MatSnackBarModule,
  ],
  template: `
    <h2 mat-dialog-title>Import/Export Bucket</h2>
    <mat-dialog-content>
      @if (loading) {
        <mat-progress-bar mode="indeterminate"></mat-progress-bar>
      }

      <div class="export-info">
        <p>Export the entire bucket as JSON, or paste JSON to import.</p>
        <p class="warning">⚠️ Importing will overwrite existing data with matching keys.</p>
      </div>

      <mat-form-field appearance="outline" class="full-width">
        <mat-label>Bucket Data (JSON)</mat-label>
        <textarea matInput
                  [(ngModel)]="jsonText"
                  rows="25"
                  class="json-textarea"
                  spellcheck="false"
                  [disabled]="loading"></textarea>
        @if (jsonError) {
          <mat-error>{{ jsonError }}</mat-error>
        }
      </mat-form-field>
    </mat-dialog-content>
    <mat-dialog-actions align="end">
      <button mat-button mat-dialog-close [disabled]="loading">Cancel</button>
      <button mat-button (click)="exportToFile()" [disabled]="loading || !jsonText.trim()">
        Export to File
      </button>
      <button mat-flat-button (click)="importData()" [disabled]="loading || !jsonText.trim()">
        Import
      </button>
    </mat-dialog-actions>
  `,
  styles: [`
    mat-dialog-content {
      min-width: 600px;
      max-width: 800px;
    }
    .export-info {
      margin-bottom: 16px;
      padding: 12px;
      background: var(--mat-sys-surface-variant);
      border-radius: 4px;
    }
    .export-info p {
      margin: 4px 0;
      font-size: 14px;
    }
    .warning {
      color: var(--mat-sys-error);
      font-weight: 500;
    }
    .full-width {
      width: 100%;
    }
    .json-textarea {
      font-family: 'Courier New', monospace;
      font-size: 12px;
      line-height: 1.5;
    }
  `],
})
export class BucketExportDialogComponent implements OnInit {
  private dialogRef = inject(MatDialogRef<BucketExportDialogComponent>);
  private kvStore = inject(KvStoreService);
  private snackBar = inject(MatSnackBar);

  loading = false;
  jsonText = '';
  jsonError = '';

  async ngOnInit(): Promise<void> {
    await this.loadBucketData();
  }

  private async loadBucketData(): Promise<void> {
    this.loading = true;
    this.jsonError = '';

    try {
      const entries = await this.kvStore.listAll<unknown>();
      const bucketData: Record<string, unknown> = {};

      for (const entry of entries) {
        bucketData[entry.key] = entry.value;
      }

      this.jsonText = JSON.stringify(bucketData, null, 2);
    } catch (err) {
      this.jsonError = err instanceof Error ? err.message : 'Failed to load bucket data';
      this.snackBar.open(this.jsonError, 'Dismiss', { duration: 5000 });
    } finally {
      this.loading = false;
    }
  }

  exportToFile(): void {
    try {
      // Validate JSON first
      JSON.parse(this.jsonText);

      // Create blob and download
      const blob = new Blob([this.jsonText], { type: 'application/json' });
      const url = window.URL.createObjectURL(blob);
      const link = document.createElement('a');
      link.href = url;
      link.download = `nauts-bucket-${new Date().toISOString().split('T')[0]}.json`;
      link.click();
      window.URL.revokeObjectURL(url);

      this.snackBar.open('Bucket data exported', 'Dismiss', { duration: 3000 });
    } catch (err) {
      this.jsonError = err instanceof Error ? err.message : 'Invalid JSON';
      this.snackBar.open('Export failed: ' + this.jsonError, 'Dismiss', { duration: 5000 });
    }
  }

  async importData(): Promise<void> {
    this.loading = true;
    this.jsonError = '';

    try {
      const data = JSON.parse(this.jsonText);

      if (typeof data !== 'object' || data === null || Array.isArray(data)) {
        throw new Error('JSON must be an object with key-value pairs');
      }

      let importedCount = 0;
      let errorCount = 0;

      for (const [key, value] of Object.entries(data)) {
        try {
          await this.kvStore.put(key, value);
          importedCount++;
        } catch (err) {
          console.error(`Failed to import key "${key}":`, err);
          errorCount++;
        }
      }

      if (errorCount > 0) {
        this.snackBar.open(
          `Imported ${importedCount} entries with ${errorCount} errors`,
          'Dismiss',
          { duration: 5000 }
        );
      } else {
        this.snackBar.open(
          `Successfully imported ${importedCount} entries`,
          'Dismiss',
          { duration: 3000 }
        );
      }

      this.dialogRef.close({ imported: true });
    } catch (err) {
      this.jsonError = err instanceof Error ? err.message : 'Import failed';
      this.snackBar.open('Import failed: ' + this.jsonError, 'Dismiss', { duration: 5000 });
    } finally {
      this.loading = false;
    }
  }
}
