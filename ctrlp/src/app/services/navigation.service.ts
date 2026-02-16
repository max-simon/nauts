import { Injectable } from '@angular/core';
import { BehaviorSubject, Observable } from 'rxjs';

@Injectable({ providedIn: 'root' })
export class NavigationService {
  private currentAccountSubject = new BehaviorSubject<string>('');
  
  getCurrentAccount$(): Observable<string> {
    return this.currentAccountSubject.asObservable();
  }
  
  getCurrentAccount(): string {
    return this.currentAccountSubject.value;
  }
  
  setCurrentAccount(account: string): void {
    this.currentAccountSubject.next(account);
  }
}
