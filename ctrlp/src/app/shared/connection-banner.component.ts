import { Component, inject } from '@angular/core';
import { AsyncPipe } from '@angular/common';
import { MatToolbarModule } from '@angular/material/toolbar';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';
import { MatProgressBarModule } from '@angular/material/progress-bar';
import { NatsService, ConnectionStatus } from '../services/nats.service';
import { map } from 'rxjs';

@Component({
  selector: 'app-connection-banner',
  standalone: true,
  imports: [AsyncPipe, MatToolbarModule, MatButtonModule, MatIconModule, MatProgressBarModule],
  template: `
    @if (showBanner$ | async) {
      <div class="connection-banner">
        @if ((status$ | async) === 'reconnecting') {
          <mat-progress-bar mode="indeterminate"></mat-progress-bar>
          <div class="banner-content">
            <mat-icon>sync</mat-icon>
            <span>Reconnecting to NATS...</span>
          </div>
        } @else {
          <div class="banner-content">
            <mat-icon>cloud_off</mat-icon>
            <span>Disconnected from NATS</span>
            <button mat-button (click)="reconnect()">Retry</button>
          </div>
        }
      </div>
    }
  `,
  styles: [`
    .connection-banner {
      background: var(--mat-sys-error-container);
      color: var(--mat-sys-on-error-container);
    }
    .banner-content {
      display: flex;
      align-items: center;
      gap: 8px;
      padding: 8px 16px;
      font-size: 14px;
    }
  `],
})
export class ConnectionBannerComponent {
  private nats = inject(NatsService);

  status$ = this.nats.connectionStatus$;
  showBanner$ = this.nats.connectionStatus$.pipe(
    map(s => s === 'disconnected' || s === 'reconnecting' || s === 'error')
  );

  reconnect(): void {
    this.nats.reconnect();
  }
}
