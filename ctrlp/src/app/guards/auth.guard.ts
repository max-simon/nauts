import { inject } from '@angular/core';
import { CanActivateFn, Router } from '@angular/router';
import { NatsService } from '../services/nats.service';

export const authGuard: CanActivateFn = async () => {
  const nats = inject(NatsService);
  const router = inject(Router);

  // If not connected, try to restore session
  if (nats.connectionStatus$.value !== 'connected') {
    const saved = nats.getSavedParams();
    if (!saved) {
      return router.createUrlTree(['/login']);
    }
    try {
      await nats.connect(saved);
    } catch {
      return router.createUrlTree(['/login']);
    }
  }

  // Always verify the KV bucket exists (even if already connected)
  try {
    const exists = await nats.checkBucketExists();
    if (!exists) {
      return router.createUrlTree(['/login'], { queryParams: { bucketMissing: 'true' } });
    }
  } catch {
    return router.createUrlTree(['/login']);
  }

  return true;
};
