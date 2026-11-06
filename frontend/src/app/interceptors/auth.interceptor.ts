import { HttpInterceptorFn, HttpErrorResponse } from '@angular/common/http';
import { inject } from '@angular/core';
import { Router } from '@angular/router';
import { catchError, throwError } from 'rxjs';
import { AuthService } from '../services/auth.service';

const INTERNAL = ['/api', '/ai'];

function isInternal(url: string): boolean {
  return INTERNAL.some(prefix => url.startsWith(prefix));
}

export const authInterceptor: HttpInterceptorFn = (req, next) => {
  const auth = inject(AuthService);
  const router = inject(Router);

  const authReq = isInternal(req.url)
    ? req.clone({ setHeaders: { Authorization: `Bearer ${auth.getToken() ?? ''}` } })
    : req;

  return next(authReq).pipe(
    catchError((err: HttpErrorResponse) => {
      if (err.status === 401 && isInternal(req.url)) {
        auth.logout();
        router.navigate(['/login']);
      }
      return throwError(() => err);
    })
  );
};
