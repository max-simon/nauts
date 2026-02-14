import { Injectable } from '@angular/core';
import { Policy, PolicyEntry } from '../models/policy.model';
import { KvStoreService } from './kv-store.service';
import { policyKey, parsePolicyKey } from './kv-keys';
import { BehaviorSubject, Observable } from 'rxjs';

@Injectable({ providedIn: 'root' })
export class PolicyService {
  private policiesMap = new Map<string, PolicyEntry>(); // key -> PolicyEntry
  private policiesSubject = new BehaviorSubject<PolicyEntry[]>([]);
  private initialized = false;
  private stopWatcher?: () => void;

  constructor(private kv: KvStoreService) {}

  async initialize(): Promise<void> {
    if (this.initialized) return;

    // Load all policies from KV
    const entries = await this.kv.listAll<Policy>();
    for (const entry of entries) {
      const parsed = parsePolicyKey(entry.key);
      if (parsed) {
        this.policiesMap.set(entry.key, { policy: entry.value, revision: entry.revision });
      }
    }
    this.emitPolicies();

    // Start watching for changes
    this.stopWatcher = await this.kv.watch<Policy>((entry, operation) => {
      if (operation === 'PUT' && entry) {
        const parsed = parsePolicyKey(entry.key);
        if (parsed) {
          this.policiesMap.set(entry.key, { policy: entry.value, revision: entry.revision });
          this.emitPolicies();
        }
      } else if (operation === 'DEL' && entry) {
        this.policiesMap.delete(entry.key);
        this.emitPolicies();
      }
    });

    this.initialized = true;
  }

  destroy(): void {
    if (this.stopWatcher) {
      this.stopWatcher();
    }
  }

  private emitPolicies(): void {
    this.policiesSubject.next(Array.from(this.policiesMap.values()));
  }

  getPolicies$(): Observable<PolicyEntry[]> {
    return this.policiesSubject.asObservable();
  }

  listPolicies(account: string): PolicyEntry[] {
    return Array.from(this.policiesMap.values()).filter(e => e.policy.account === account);
  }

  listGlobalPolicies(): PolicyEntry[] {
    return this.listPolicies('_global');
  }

  listAllPolicies(): PolicyEntry[] {
    return Array.from(this.policiesMap.values());
  }

  getPolicy(account: string, id: string): PolicyEntry | null {
    const key = policyKey(account, id);
    return this.policiesMap.get(key) || null;
  }

  async createPolicy(policy: Policy): Promise<number> {
    policy.id = crypto.randomUUID();
    const key = policyKey(policy.account, policy.id);
    return this.kv.create(key, policy);
  }

  async updatePolicy(account: string, id: string, policy: Policy, revision: number): Promise<number> {
    const key = policyKey(account, id);
    return this.kv.put(key, policy, revision);
  }

  async deletePolicy(account: string, id: string, revision: number): Promise<void> {
    const key = policyKey(account, id);
    await this.kv.delete(key, revision);
  }
}
