import { Injectable } from '@angular/core';
import { BehaviorSubject } from 'rxjs';
import { NatsService } from './nats.service';
import { accountFromKeyPrefix } from './kv-keys';

@Injectable({ providedIn: 'root' })
export class AccountService {
  private accountsSubject = new BehaviorSubject<string[]>([]);
  readonly accounts$ = this.accountsSubject.asObservable();

  constructor(private nats: NatsService) {}

  async discoverAccounts(): Promise<string[]> {
    const kv = await this.nats.getKvBucket();
    const accountSet = new Set<string>();

    const keys = await kv.keys();
    for await (const key of keys) {
      const dotIndex = key.indexOf('.');
      if (dotIndex > 0) {
        const prefix = key.substring(0, dotIndex);
        const account = accountFromKeyPrefix(prefix);
        if (account !== '*') {
          accountSet.add(account);
        }
      }
    }

    const accounts = Array.from(accountSet).sort();
    this.accountsSubject.next(accounts);
    return accounts;
  }

  get accounts(): string[] {
    return this.accountsSubject.value;
  }
}
