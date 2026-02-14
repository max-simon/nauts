import { inject } from '@angular/core';
import { CanActivateFn, Router } from '@angular/router';
import { NatsService } from '../services/nats.service';

export const authGuard: CanActivateFn = async () => {
  const nats = inject(NatsService);
  const router = inject(Router);

  if (nats.connectionStatus$.value === 'connected') {
    return true;
  }

  // Try to restore session from saved params
  const saved = nats.getSavedParams();
  if (saved) {
    try {
      await nats.connect(saved);
      return true;
    } catch {
      // Saved params are stale — fall through to login
    }
  }

  return router.createUrlTree(['/login']);
};
