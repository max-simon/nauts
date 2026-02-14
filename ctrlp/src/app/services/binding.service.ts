import { Injectable } from '@angular/core';
import { Binding, BindingEntry } from '../models/binding.model';
import { KvStoreService } from './kv-store.service';
import { bindingKey, parseBindingKey } from './kv-keys';
import { BehaviorSubject, Observable } from 'rxjs';

@Injectable({ providedIn: 'root' })
export class BindingService {
  private bindingsMap = new Map<string, BindingEntry>(); // key -> BindingEntry
  private bindingsSubject = new BehaviorSubject<BindingEntry[]>([]);
  private initialized = false;
  private stopWatcher?: () => void;

  constructor(private kv: KvStoreService) {}

  async initialize(): Promise<void> {
    if (this.initialized) return;

    // Load all bindings from KV
    const entries = await this.kv.listAll<Binding>();
    for (const entry of entries) {
      const parsed = parseBindingKey(entry.key);
      if (parsed) {
        this.bindingsMap.set(entry.key, { binding: entry.value, revision: entry.revision });
      }
    }
    this.emitBindings();

    // Start watching for changes
    this.stopWatcher = await this.kv.watch<Binding>((entry, operation) => {
      if (operation === 'PUT' && entry) {
        const parsed = parseBindingKey(entry.key);
        if (parsed) {
          this.bindingsMap.set(entry.key, { binding: entry.value, revision: entry.revision });
          this.emitBindings();
        }
      } else if (operation === 'DEL' && entry) {
        this.bindingsMap.delete(entry.key);
        this.emitBindings();
      }
    });

    this.initialized = true;
  }

  destroy(): void {
    if (this.stopWatcher) {
      this.stopWatcher();
    }
  }

  private emitBindings(): void {
    this.bindingsSubject.next(Array.from(this.bindingsMap.values()));
  }

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
