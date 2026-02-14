import { Injectable } from '@angular/core';
import { HttpClient } from '@angular/common/http';
import { firstValueFrom } from 'rxjs';
import { AppConfig } from '../models/config.model';

@Injectable({ providedIn: 'root' })
export class ConfigService {
  private config!: AppConfig;

  constructor(private http: HttpClient) {}

  async load(): Promise<void> {
    this.config = await firstValueFrom(
      this.http.get<AppConfig>('assets/config.json')
    );
  }

  get nats() {
    return this.config.nats;
  }

  get ui() {
    return this.config.ui;
  }

  get isLoaded(): boolean {
    return !!this.config;
  }
}
