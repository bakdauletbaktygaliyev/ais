import { Injectable, OnDestroy } from '@angular/core';
import {
    Subject, Observable, interval, EMPTY,
    BehaviorSubject, fromEvent, merge
} from 'rxjs';
import {
    webSocket, WebSocketSubject
} from 'rxjs/webSocket';
import {
    switchMap, retryWhen, delay, tap,
    shareReplay, filter, takeUntil
} from 'rxjs/operators';
import { environment } from '@env/environment';
import { WsIncoming, WsOutgoing } from '../models';

export type WsConnectionState = 'connecting' | 'connected' | 'disconnected' | 'error';

@Injectable({ providedIn: 'root' })
export class WebSocketService implements OnDestroy {
    private socket$: WebSocketSubject<WsOutgoing> | null = null;
    private messages$ = new Subject<WsOutgoing>();
    private destroy$ = new Subject<void>();
    private reconnectDelay = 2000;
    private maxReconnectDelay = 30000;
    private currentReconnectDelay = this.reconnectDelay;

    readonly connectionState$ = new BehaviorSubject<WsConnectionState>('disconnected');

    connect(): void {
        if (this.socket$ && !this.socket$.closed) {
            return;
        }

        this.connectionState$.next('connecting');

        this.socket$ = webSocket<WsOutgoing>({
            url: environment.wsUrl,
            openObserver: {
                next: () => {
                    this.connectionState$.next('connected');
                    this.currentReconnectDelay = this.reconnectDelay;
                    console.log('[WS] Connected to', environment.wsUrl);
                },
            },
            closeObserver: {
                next: () => {
                    this.connectionState$.next('disconnected');
                    console.log('[WS] Connection closed');
                },
            },
        });

        this.socket$.pipe(
            tap({
                error: (err) => {
                    console.error('[WS] Error:', err);
                    this.connectionState$.next('error');
                },
            }),
            retryWhen((errors$) =>
                errors$.pipe(
                    tap(() => {
                        this.connectionState$.next('disconnected');
                        console.log(`[WS] Reconnecting in ${this.currentReconnectDelay}ms...`);
                    }),
                    delay(this.currentReconnectDelay),
                    tap(() => {
                        this.currentReconnectDelay = Math.min(
                            this.currentReconnectDelay * 1.5,
                            this.maxReconnectDelay
                        );
                    }),
                )
            ),
            takeUntil(this.destroy$),
        ).subscribe({
            next: (msg) => this.messages$.next(msg),
            error: (err) => console.error('[WS] Fatal error:', err),
        });

        // Keepalive ping every 30 seconds
        interval(30_000).pipe(
            takeUntil(this.destroy$),
            filter(() => this.connectionState$.value === 'connected'),
        ).subscribe(() => this.send({ type: 'ping' }));
    }

    disconnect(): void {
        this.socket$?.complete();
        this.socket$ = null;
        this.connectionState$.next('disconnected');
    }

    send(message: WsIncoming): void {
        if (this.socket$ && !this.socket$.closed) {
            this.socket$.next(message as unknown as WsOutgoing);
        } else {
            console.warn('[WS] Cannot send, not connected:', message);
        }
    }

    messages(): Observable<WsOutgoing> {
        return this.messages$.asObservable();
    }

    messagesOfType(type: string): Observable<WsOutgoing> {
        return this.messages$.pipe(
            filter((msg) => msg.type === type),
        );
    }

    subscribeToRepo(repoId: string): void {
        this.send({ type: 'subscribe', repoId });
    }

    unsubscribeFromRepo(repoId: string): void {
        this.send({ type: 'unsubscribe', repoId });
    }

    sendChatMessage(
        repoId: string,
        message: string,
        nodeId?: string,
        history?: { role: string; content: string }[]
    ): void {
        this.send({
            type: 'chat',
            repoId,
            message,
            nodeId,
            history: history || [],
        });
    }

    ngOnDestroy(): void {
        this.destroy$.next();
        this.destroy$.complete();
        this.disconnect();
    }
}