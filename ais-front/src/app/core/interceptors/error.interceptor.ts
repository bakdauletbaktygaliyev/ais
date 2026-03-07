import { HttpInterceptorFn, HttpErrorResponse } from '@angular/common/http';
import { inject } from '@angular/core';
import { catchError, throwError } from 'rxjs';

export const errorInterceptor: HttpInterceptorFn = (req, next) => {
    return next(req).pipe(
        catchError((error: HttpErrorResponse) => {
            let message = 'An unexpected error occurred';

            if (error.status === 0) {
                message = 'Network error — check your connection';
            } else if (error.error?.message) {
                message = error.error.message;
            } else if (error.message) {
                message = error.message;
            }

            console.error('[HTTP Error]', error.status, message, error.url);
            return throwError(() => ({ status: error.status, message, original: error }));
        })
    );
};
