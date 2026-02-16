import { Routes } from '@angular/router';
import { LoginComponent } from './pages/login/login.component';
import { PoliciesComponent } from './pages/policies/policies.component';
import { BindingsComponent } from './pages/bindings/bindings.component';
import { SimulatorComponent } from './pages/simulator/simulator.component';
import { authGuard } from './guards/auth.guard';

export const routes: Routes = [
  { path: 'login', component: LoginComponent },
  { path: '', redirectTo: 'policies', pathMatch: 'full' },
  { path: 'policies', component: PoliciesComponent, canActivate: [authGuard] },
  { path: 'policies/:account', component: PoliciesComponent, canActivate: [authGuard] },
  { path: 'policies/:account/:id', component: PoliciesComponent, canActivate: [authGuard] },
  { path: 'bindings', component: BindingsComponent, canActivate: [authGuard] },
  { path: 'bindings/:account', component: BindingsComponent, canActivate: [authGuard] },
  { path: 'bindings/:account/:role', component: BindingsComponent, canActivate: [authGuard] },
  { path: 'simulator', component: SimulatorComponent, canActivate: [authGuard] },
];
