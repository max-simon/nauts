import { Component, inject } from '@angular/core';
import { Router } from '@angular/router';
import { FormsModule } from '@angular/forms';
import { MatCardModule } from '@angular/material/card';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatInputModule } from '@angular/material/input';
import { MatTabsModule } from '@angular/material/tabs';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';
import { MatProgressBarModule } from '@angular/material/progress-bar';
import { NatsService, ConnectParams, AuthMethod, describeNatsError } from '../../services/nats.service';
import { ConfigService } from '../../services/config.service';

@Component({
  selector: 'app-login',
  standalone: true,
  imports: [
    FormsModule,
    MatCardModule,
    MatFormFieldModule,
    MatInputModule,
    MatTabsModule,
    MatButtonModule,
    MatIconModule,
    MatProgressBarModule,
  ],
  template: `
    <div class="login-wrapper">
      <mat-card class="login-card">
        <mat-card-header>
          <mat-card-title>nauts Control Plane</mat-card-title>
          <mat-card-subtitle>Connect to NATS</mat-card-subtitle>
        </mat-card-header>

        <mat-card-content>
          @if (!bucketMissing) {
            <mat-form-field appearance="outline" class="full-width">
              <mat-label>NATS URL</mat-label>
              <input matInput [(ngModel)]="url" placeholder="ws://localhost:9222">
            </mat-form-field>

            <mat-form-field appearance="outline" class="full-width">
              <mat-label>KV Bucket</mat-label>
              <input matInput [(ngModel)]="bucket" placeholder="nauts-policies">
            </mat-form-field>

            <mat-tab-group [(selectedIndex)]="authTabIndex" class="auth-tabs">

              <mat-tab label="NKey">
                <div class="tab-content">
                  <mat-form-field appearance="outline" class="full-width">
                    <mat-label>NKey Seed</mat-label>
                    <textarea matInput [(ngModel)]="nkeySeed" rows="3"
                              placeholder="SUAE..."></textarea>
                  </mat-form-field>
                </div>
              </mat-tab>

              <mat-tab label="Credentials">
                <div class="tab-content">
                  <button mat-stroked-button (click)="fileInput.click()" class="file-btn">
                    <mat-icon>upload_file</mat-icon>
                    Load .creds file
                  </button>
                  <input #fileInput type="file" accept=".creds" hidden
                         (change)="onCredsFileSelected($event)">
                  <mat-form-field appearance="outline" class="full-width">
                    <mat-label>Credentials</mat-label>
                    <textarea matInput [(ngModel)]="credsContents" rows="4"
                              placeholder="-----BEGIN NATS USER JWT-----"></textarea>
                  </mat-form-field>
                </div>
              </mat-tab>
            </mat-tab-group>
          }

          @if (bucketMissing) {
            <div class="bucket-missing">
              <mat-icon class="bucket-icon">storage</mat-icon>
              <p>KV bucket <strong>{{ bucket }}</strong> does not exist.</p>
            </div>
          }

          @if (busy) {
            <mat-progress-bar mode="indeterminate"></mat-progress-bar>
          }

          @if (errorMessage) {
            <div class="error-message">{{ errorMessage }}</div>
          }
        </mat-card-content>

        <mat-card-actions align="end">
          @if (!bucketMissing) {
            <button mat-flat-button (click)="onConnect()" [disabled]="busy">
              Connect
            </button>
          } @else {
            <button mat-button (click)="onBack()">Back</button>
            <button mat-flat-button (click)="onInitialize()" [disabled]="busy">
              Initialize
            </button>
          }
        </mat-card-actions>
      </mat-card>
    </div>
  `,
  styles: [`
    .login-wrapper {
      display: flex;
      align-items: center;
      justify-content: center;
      height: 100vh;
      background: var(--mat-sys-surface-variant);
    }
    .login-card {
      width: 480px;
      max-width: 90vw;
      padding: 24px;
    }
    .full-width {
      width: 100%;
    }
    .auth-tabs {
      margin: 16px 0;
    }
    .tab-content {
      padding-top: 16px;
    }
    .file-btn {
      margin-bottom: 12px;
    }
    .error-message {
      color: var(--mat-sys-error);
      font-size: 14px;
      margin-top: 8px;
    }
    .bucket-missing {
      display: flex;
      flex-direction: column;
      align-items: center;
      padding: 24px 0;
      text-align: center;
    }
    .bucket-icon {
      font-size: 48px;
      width: 48px;
      height: 48px;
      color: var(--mat-sys-on-surface-variant);
      margin-bottom: 8px;
    }
  `],
})
export class LoginComponent {
  private nats = inject(NatsService);
  private config = inject(ConfigService);
  private router = inject(Router);

  url = this.config.nats.url;
  bucket = this.config.nats.bucket;
  authTabIndex = 0;

  // Auth fields
  token = '';
  username = '';
  password = '';
  nkeySeed = '';
  credsContents = '';

  busy = false;
  errorMessage = '';
  bucketMissing = false;

  constructor() {
    const saved = this.nats.getSavedParams();
    if (saved) {
      this.url = saved.url;
      this.bucket = saved.bucket;
      switch (saved.auth.type) {
        case 'nkey':
          this.authTabIndex = 1;
          this.nkeySeed = saved.auth.seed;
          break;
        case 'creds':
          this.authTabIndex = 2;
          this.credsContents = saved.auth.contents;
          break;
      }
    }
  }

  async onConnect(): Promise<void> {
    this.busy = true;
    this.errorMessage = '';

    const params: ConnectParams = {
      url: this.url,
      bucket: this.bucket,
      auth: this.buildAuthMethod(),
    };

    try {
      await this.nats.connect(params);
    } catch (err) {
      this.errorMessage = describeNatsError(err);
      this.busy = false;
      return;
    }

    try {
      const exists = await this.nats.checkBucketExists();
      if (exists) {
        this.router.navigate(['/policies']);
      } else {
        this.bucketMissing = true;
      }
    } catch (err) {
      await this.nats.disconnect();
      this.errorMessage = describeNatsError(err);
    } finally {
      this.busy = false;
    }
  }

  async onInitialize(): Promise<void> {
    this.busy = true;
    this.errorMessage = '';

    try {
      await this.nats.createBucket();
      this.router.navigate(['/policies']);
    } catch (err) {
      this.errorMessage = describeNatsError(err);
    } finally {
      this.busy = false;
    }
  }

  onBack(): void {
    this.bucketMissing = false;
    this.errorMessage = '';
    this.nats.disconnect();
  }

  onCredsFileSelected(event: Event): void {
    const input = event.target as HTMLInputElement;
    const file = input.files?.[0];
    if (!file) return;

    const reader = new FileReader();
    reader.onload = () => {
      this.credsContents = reader.result as string;
    };
    reader.readAsText(file);
  }

  private buildAuthMethod(): AuthMethod {
    switch (this.authTabIndex) {
      case 1:
        return { type: 'nkey', seed: this.nkeySeed };
      case 2:
        return { type: 'creds', contents: this.credsContents };
      default:
        return { type: 'none' };
    }
  }
}
