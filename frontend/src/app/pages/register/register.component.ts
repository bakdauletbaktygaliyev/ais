import { Component } from '@angular/core';
import { CommonModule } from '@angular/common';
import { FormsModule } from '@angular/forms';
import { Router, RouterLink } from '@angular/router';
import { AuthService } from '../../services/auth.service';

type Step = 'form' | 'verify';

@Component({
  selector: 'app-register',
  standalone: true,
  imports: [CommonModule, FormsModule, RouterLink],
  templateUrl: './register.component.html',
  styleUrls: ['./register.component.css']
})
export class RegisterComponent {
  step: Step = 'form';

  email = '';
  password = '';
  confirm = '';
  code = '';

  loading = false;
  error = '';

  constructor(private auth: AuthService, private router: Router) {}

  get passwordMismatch(): boolean {
    return this.confirm.length > 0 && this.confirm !== this.password;
  }

  submitForm() {
    if (!this.email || !this.password || this.passwordMismatch) return;
    if (this.password.length < 8) {
      this.error = 'Password must be at least 8 characters';
      return;
    }
    this.loading = true;
    this.error = '';

    this.auth.register(this.email, this.password).subscribe({
      next: () => {
        this.loading = false;
        this.step = 'verify';
      },
      error: (err) => {
        this.loading = false;
        this.error = err.error?.error || 'Registration failed';
      }
    });
  }

  submitCode() {
    if (!this.code || this.code.length !== 6) return;
    this.loading = true;
    this.error = '';

    this.auth.verify(this.email, this.code).subscribe({
      next: () => this.router.navigate(['/']),
      error: (err) => {
        this.loading = false;
        this.error = err.error?.error || 'Verification failed';
      }
    });
  }

  resend() {
    this.loading = true;
    this.error = '';
    this.code = '';
    this.auth.register(this.email, this.password).subscribe({
      next: () => { this.loading = false; },
      error: (err) => {
        this.loading = false;
        this.error = err.error?.error || 'Could not resend code';
      }
    });
  }
}
