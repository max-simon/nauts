import { Injectable } from '@angular/core';
import { BehaviorSubject, Observable } from 'rxjs';
import { Policy, PolicyEntry } from '../models/policy.model';
import { Binding, BindingEntry } from '../models/binding.model';
import { KvStoreService } from './kv-store.service';
import { policyKey, parsePolicyKey, bindingKey, parseBindingKey } from './kv-keys';

@Injectable({ providedIn: 'root' })
export class PolicyStoreService {
  private policiesMap = new Map<string, PolicyEntry>();
  private bindingsMap = new Map<string, BindingEntry>();
  private policiesSubject = new BehaviorSubject<PolicyEntry[]>([]);
  private bindingsSubject = new BehaviorSubject<BindingEntry[]>([]);
  private initialized = false;
  private stopWatcher?: () => void;

  constructor(private kv: KvStoreService) {}

  async initialize(): Promise<void> {
    if (this.initialized) return;

    // Load all entries from KV
    const entries = await this.kv.listAll<Policy | Binding>();
    for (const entry of entries) {
      this.handlePut(entry.key, entry.value, entry.revision);
    }
    this.emitPolicies();
    this.emitBindings();

    // Start a single watcher for all changes
    this.stopWatcher = await this.kv.watch<Policy | Binding>((entry, operation) => {
      if (operation === 'PUT' && entry) {
        if (this.handlePut(entry.key, entry.value, entry.revision)) {
          this.emitAll(entry.key);
        }
      } else if (operation === 'DEL' && entry) {
        if (this.handleDel(entry.key)) {
          this.emitAll(entry.key);
        }
      }
    });

    this.initialized = true;
  }

  destroy(): void {
    if (this.stopWatcher) {
      this.stopWatcher();
    }
  }

  private handlePut(key: string, value: unknown, revision: number): boolean {
    if (parsePolicyKey(key)) {
      this.policiesMap.set(key, { policy: value as Policy, revision });
      return true;
    }
    if (parseBindingKey(key)) {
      this.bindingsMap.set(key, { binding: value as Binding, revision });
      return true;
    }
    return false;
  }

  private handleDel(key: string): boolean {
    if (parsePolicyKey(key)) {
      this.policiesMap.delete(key);
      return true;
    }
    if (parseBindingKey(key)) {
      this.bindingsMap.delete(key);
      return true;
    }
    return false;
  }

  private emitAll(key: string): void {
    if (parsePolicyKey(key)) {
      this.emitPolicies();
    } else if (parseBindingKey(key)) {
      this.emitBindings();
    }
  }

  private emitPolicies(): void {
    this.policiesSubject.next(Array.from(this.policiesMap.values()));
  }

  private emitBindings(): void {
    this.bindingsSubject.next(Array.from(this.bindingsMap.values()));
  }

  // --- Policy methods ---

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

  // --- Binding methods ---

  getBindings$(): Observable<BindingEntry[]> {
    return this.bindingsSubject.asObservable();
  }

  listBindings(account: string): BindingEntry[] {
    return Array.from(this.bindingsMap.values()).filter(e => e.binding.account === account);
  }

  listAllBindings(): BindingEntry[] {
    return Array.from(this.bindingsMap.values());
  }

  getBinding(account: string, role: string): BindingEntry | null {
    const key = bindingKey(account, role);
    return this.bindingsMap.get(key) || null;
  }

  async createBinding(binding: Binding): Promise<number> {
    const key = bindingKey(binding.account, binding.role);
    return this.kv.create(key, binding);
  }

  async updateBinding(account: string, role: string, binding: Binding, revision: number): Promise<number> {
    const key = bindingKey(account, role);
    return this.kv.put(key, binding, revision);
  }

  async deleteBinding(account: string, role: string, revision: number): Promise<void> {
    const key = bindingKey(account, role);
    await this.kv.delete(key, revision);
  }
}
