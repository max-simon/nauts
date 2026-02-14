import { Injectable } from '@angular/core';
import { Binding, BindingEntry } from '../models/binding.model';
import { KvStoreService } from './kv-store.service';
import { bindingKey, bindingPrefix, parseBindingKey } from './kv-keys';

@Injectable({ providedIn: 'root' })
export class BindingService {
  constructor(private kv: KvStoreService) {}

  async listBindings(account: string): Promise<BindingEntry[]> {
    const entries = await this.kv.list<Binding>(bindingPrefix(account));
    return entries.map(e => ({ binding: e.value, revision: e.revision }));
  }

  async listAllBindings(): Promise<BindingEntry[]> {
    const entries = await this.kv.listAll<Binding>();
    return entries
      .filter(e => parseBindingKey(e.key) !== null)
      .map(e => ({ binding: e.value, revision: e.revision }));
  }

  async getBinding(account: string, role: string): Promise<BindingEntry | null> {
    const entry = await this.kv.get<Binding>(bindingKey(account, role));
    if (!entry) return null;
    return { binding: entry.value, revision: entry.revision };
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
