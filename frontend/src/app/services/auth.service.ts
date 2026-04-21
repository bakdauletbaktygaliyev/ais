import { Injectable } from '@angular/core';
import { HttpClient } from '@angular/common/http';
import { Observable, tap } from 'rxjs';
import { AuthResponse } from '../models/auth.model';

@Injectable({ providedIn: 'root' })
export class AuthService {
  private readonly api = '/api/auth';
  private readonly TOKEN_KEY = 'ais_token';
  private readonly EMAIL_KEY = 'ais_email';

  constructor(private http: HttpClient) {}

  register(email: string, password: string): Observable<{ pending: boolean; email: string }> {
    return this.http.post<{ pending: boolean; email: string }>(`${this.api}/register`, { email, password });
  }

  verify(email: string, code: string): Observable<AuthResponse> {
    return this.http.post<AuthResponse>(`${this.api}/verify`, { email, code }).pipe(
      tap(res => this.persist(res))
    );
  }

  login(email: string, password: string): Observable<AuthResponse> {
    return this.http.post<AuthResponse>(`${this.api}/login`, { email, password }).pipe(
      tap(res => this.persist(res))
    );
  }

  logout(): void {
    localStorage.removeItem(this.TOKEN_KEY);
    localStorage.removeItem(this.EMAIL_KEY);
  }

  getToken(): string | null {
    return localStorage.getItem(this.TOKEN_KEY);
  }

  getEmail(): string | null {
    return localStorage.getItem(this.EMAIL_KEY);
  }

  isLoggedIn(): boolean {
    return !!this.getToken();
  }

  private persist(res: AuthResponse): void {
    localStorage.setItem(this.TOKEN_KEY, res.token);
    localStorage.setItem(this.EMAIL_KEY, res.email);
  }
}
