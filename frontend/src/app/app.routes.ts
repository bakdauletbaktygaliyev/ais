import { Routes } from '@angular/router';
import { HomeComponent } from './pages/home/home.component';
import { ProjectComponent } from './pages/project/project.component';
import { LoginComponent } from './pages/login/login.component';
import { RegisterComponent } from './pages/register/register.component';
import { authGuard } from './guards/auth.guard';

export const routes: Routes = [
  { path: 'login',    component: LoginComponent },
  { path: 'register', component: RegisterComponent },
  { path: '',         component: HomeComponent,    canActivate: [authGuard] },
  { path: 'project/:id', component: ProjectComponent, canActivate: [authGuard] },
  { path: '**', redirectTo: '' }
];
