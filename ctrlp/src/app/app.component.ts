import { Component, inject, OnInit, OnDestroy } from '@angular/core';
import { Router, RouterOutlet, RouterLink, RouterLinkActive, NavigationEnd } from '@angular/router';
import { MatToolbarModule } from '@angular/material/toolbar';
import { MatSidenavModule } from '@angular/material/sidenav';
import { MatListModule } from '@angular/material/list';
import { MatIconModule } from '@angular/material/icon';
import { MatButtonModule } from '@angular/material/button';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatSelectModule } from '@angular/material/select';
import { FormsModule } from '@angular/forms';
import { ConnectionBannerComponent } from './shared/connection-banner.component';
import { NatsService } from './services/nats.service';
import { NavigationService } from './services/navigation.service';
import { AccountService } from './services/account.service';
import { PolicyStoreService } from './services/policy-store.service';
import { Subscription } from 'rxjs';
import { filter, map, skip, debounceTime } from 'rxjs/operators';

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
    MatFormFieldModule,
    MatSelectModule,
    FormsModule,
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
            <div class="account-selector">
              <mat-form-field appearance="outline" class="account-field">
                <mat-label>Account</mat-label>
                <mat-select [(value)]="currentAccount" (selectionChange)="onAccountChange()">
                  <mat-option value="">All Accounts</mat-option>
                  @for (account of accounts; track account) {
                    <mat-option [value]="account">{{ account }}</mat-option>
                  }
                </mat-select>
              </mat-form-field>
            </div>
            <mat-nav-list>
              <a mat-list-item [routerLink]="getPoliciesLink()" routerLinkActive="active-link">
                <mat-icon matListItemIcon>policy</mat-icon>
                <span matListItemTitle>Policies</span>
              </a>
              <a mat-list-item [routerLink]="getBindingsLink()" routerLinkActive="active-link">
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
    .account-selector {
      padding: 16px 12px 8px;
    }
    .account-field {
      width: 100%;
      font-size: 14px;
    }
  `],
})
export class AppComponent implements OnInit, OnDestroy {
  private router = inject(Router);
  private nats = inject(NatsService);
  private navigationService = inject(NavigationService);
  private accountService = inject(AccountService);
  private policyStore = inject(PolicyStoreService);
  private subs: Subscription[] = [];

  showShell = false;
  currentAccount = '';
  accounts: string[] = [];

  ngOnInit(): void {
    // Track current account
    this.subs.push(
      this.navigationService.getCurrentAccount$().subscribe(account => {
        this.currentAccount = account;
      })
    );

    // Track current route to show/hide shell and load accounts when navigating to protected routes
    this.subs.push(
      this.router.events.pipe(
        filter((e): e is NavigationEnd => e instanceof NavigationEnd),
      ).subscribe(async (e) => {
        const showShell = !e.urlAfterRedirects.startsWith('/login');
        this.showShell = showShell;

        // Load accounts when navigating to protected routes (if not already loaded)
        if (showShell && this.accounts.length === 0) {
          await this.loadAccounts();
        }
      }),
    );

    // Reload accounts when policies or bindings change (debounced to avoid multiple rapid calls)
    this.subs.push(
      this.policyStore.getPolicies$().pipe(
        debounceTime(100),
      ).subscribe(() => {
        if (this.showShell) {
          this.loadAccounts();
        }
      })
    );

    this.subs.push(
      this.policyStore.getBindings$().pipe(
        debounceTime(100),
      ).subscribe(() => {
        if (this.showShell) {
          this.loadAccounts();
        }
      })
    );

    // Auto-redirect to login on disconnect (skip initial 'disconnected' state)
    this.subs.push(
      this.nats.connectionStatus$.pipe(
        skip(1),
        filter(s => s === 'disconnected'),
      ).subscribe(() => this.router.navigate(['/login'])),
    );
  }

  private async loadAccounts(): Promise<void> {
    try {
      this.accounts = await this.accountService.discoverAccounts();
    } catch (err) {
      console.error('Failed to load accounts:', err);
    }
  }

  ngOnDestroy(): void {
    this.subs.forEach(s => s.unsubscribe());
  }

  onAccountChange(): void {
    this.navigationService.setCurrentAccount(this.currentAccount);
    // Navigate to the current page with the new account parameter
    const currentUrl = this.router.url;
    if (currentUrl.startsWith('/policies')) {
      this.router.navigate(this.getPoliciesLink());
    } else if (currentUrl.startsWith('/bindings')) {
      this.router.navigate(this.getBindingsLink());
    }
  }

  getPoliciesLink(): string[] {
    return this.currentAccount ? ['/policies', this.currentAccount] : ['/policies'];
  }

  getBindingsLink(): string[] {
    return this.currentAccount ? ['/bindings', this.currentAccount] : ['/bindings'];
  }

  logout(): void {
    this.nats.disconnect();
    this.router.navigate(['/login']);
  }
}
