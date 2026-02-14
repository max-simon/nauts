import { Component, inject, OnInit, OnDestroy } from '@angular/core';
import { Router, RouterOutlet, RouterLink, RouterLinkActive, NavigationEnd } from '@angular/router';
import { MatToolbarModule } from '@angular/material/toolbar';
import { MatSidenavModule } from '@angular/material/sidenav';
import { MatListModule } from '@angular/material/list';
import { MatIconModule } from '@angular/material/icon';
import { MatButtonModule } from '@angular/material/button';
import { ConnectionBannerComponent } from './shared/connection-banner.component';
import { NatsService } from './services/nats.service';
import { Subscription } from 'rxjs';
import { filter, map, skip } from 'rxjs/operators';

@Component({
  selector: 'app-root',
  standalone: true,
  imports: [
    RouterOutlet,
    RouterLink,
    RouterLinkActive,
    MatToolbarModule,
    MatSidenavModule,
    MatListModule,
    MatIconModule,
    MatButtonModule,
    ConnectionBannerComponent,
  ],
  template: `
    <div class="app-container">
      @if (showShell) {
        <mat-toolbar color="primary">
          <button mat-icon-button (click)="sidenav.toggle()">
            <mat-icon>menu</mat-icon>
          </button>
          <span class="app-title">nauts Control Plane</span>
          <span class="spacer"></span>
          <button mat-icon-button (click)="logout()">
            <mat-icon>logout</mat-icon>
          </button>
        </mat-toolbar>

        <app-connection-banner></app-connection-banner>

        <mat-sidenav-container class="sidenav-container">
          <mat-sidenav #sidenav mode="side" opened>
            <mat-nav-list>
              <a mat-list-item routerLink="/policies" routerLinkActive="active-link">
                <mat-icon matListItemIcon>policy</mat-icon>
                <span matListItemTitle>Policies</span>
              </a>
              <a mat-list-item routerLink="/bindings" routerLinkActive="active-link">
                <mat-icon matListItemIcon>link</mat-icon>
                <span matListItemTitle>Bindings</span>
              </a>
            </mat-nav-list>
          </mat-sidenav>

          <mat-sidenav-content>
            <router-outlet></router-outlet>
          </mat-sidenav-content>
        </mat-sidenav-container>
      } @else {
        <router-outlet></router-outlet>
      }
    </div>
  `,
  styles: [`
    .app-container {
      display: flex;
      flex-direction: column;
      height: 100vh;
    }
    .app-title {
      margin-left: 8px;
      font-weight: 500;
    }
    .spacer {
      flex: 1;
    }
    .sidenav-container {
      flex: 1;
    }
    mat-sidenav {
      width: 200px;
    }
    .active-link {
      background: var(--mat-sys-secondary-container);
    }
  `],
})
export class AppComponent implements OnInit, OnDestroy {
  private router = inject(Router);
  private nats = inject(NatsService);
  private subs: Subscription[] = [];

  showShell = false;

  ngOnInit(): void {
    // Track current route to show/hide shell
    this.subs.push(
      this.router.events.pipe(
        filter((e): e is NavigationEnd => e instanceof NavigationEnd),
        map(e => !e.urlAfterRedirects.startsWith('/login')),
      ).subscribe(show => this.showShell = show),
    );

    // Auto-redirect to login on disconnect (skip initial 'disconnected' state)
    this.subs.push(
      this.nats.connectionStatus$.pipe(
        skip(1),
        filter(s => s === 'disconnected'),
      ).subscribe(() => this.router.navigate(['/login'])),
    );
  }

  ngOnDestroy(): void {
    this.subs.forEach(s => s.unsubscribe());
  }

  logout(): void {
    this.nats.disconnect();
    this.router.navigate(['/login']);
  }
}
