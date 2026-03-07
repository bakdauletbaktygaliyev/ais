import {
    Component, OnInit, OnDestroy, Input, Output,
    EventEmitter, ViewChild, ElementRef, inject,
    ChangeDetectorRef, ChangeDetectionStrategy
} from '@angular/core';
import { CommonModule } from '@angular/common';
import { FormsModule } from '@angular/forms';
import { Subject } from 'rxjs';
import { takeUntil, filter } from 'rxjs/operators';

import { WebSocketService } from '../../core/services/websocket.service';
import { ChatMessage, FileRef } from '../../core/models';

function uuidv4(): string {
    return 'xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx'.replace(/[xy]/g, (c) => {
        const r = Math.random() * 16 | 0;
        return (c === 'x' ? r : (r & 0x3 | 0x8)).toString(16);
    });
}

@Component({
    selector: 'app-chat',
    standalone: true,
    imports: [CommonModule, FormsModule],
    changeDetection: ChangeDetectionStrategy.OnPush,
    template: `
    <div class="chat-container">
      <div class="chat-header">
        <div class="chat-header-left">
          <span class="chat-icon">🤖</span>
          <span class="chat-title">AI Assistant</span>
          <span class="chat-subtitle" *ngIf="nodeContext">· {{ nodeContext }}</span>
        </div>
        <button class="close-btn" (click)="close.emit()">✕</button>
      </div>

      <div class="messages-area" #messagesArea>
        <div class="welcome-msg" *ngIf="messages.length === 0">
          <p>Ask anything about this codebase.</p>
          <div class="suggestion-chips">
            <button
              *ngFor="let s of suggestions"
              class="suggestion-chip"
              (click)="useSuggestion(s)"
            >{{ s }}</button>
          </div>
        </div>

        <div
          *ngFor="let msg of messages; trackBy: trackById"
          class="message"
          [class.user]="msg.role === 'user'"
          [class.assistant]="msg.role === 'assistant'"
        >
          <div class="message-avatar">{{ msg.role === 'user' ? '👤' : '🤖' }}</div>
          <div class="message-body">
            <div class="message-content" [class.streaming]="msg.isStreaming">
              {{ msg.content }}<span *ngIf="msg.isStreaming" class="cursor">▋</span>
            </div>
            <div class="message-refs" *ngIf="msg.references && msg.references.length">
              <span class="refs-label">Sources:</span>
              <button
                *ngFor="let ref of msg.references"
                class="ref-chip"
                (click)="openRef(ref)"
              >{{ getFileName(ref.filePath) }}:{{ ref.startLine }}</button>
            </div>
          </div>
        </div>
      </div>

      <div class="chat-input-area">
        <div class="input-wrapper">
          <textarea
            #inputArea
            class="chat-textarea"
            [(ngModel)]="inputText"
            placeholder="Ask about this codebase..."
            [disabled]="isStreaming"
            (keydown.enter)="onEnterKey($event)"
            rows="1"
          ></textarea>
          <button
            class="send-btn"
            [disabled]="!inputText.trim() || isStreaming"
            (click)="sendMessage()"
          >
            <svg *ngIf="!isStreaming" viewBox="0 0 24 24" fill="currentColor">
              <path d="M2.01 21L23 12 2.01 3 2 10l15 2-15 2z"/>
            </svg>
            <span *ngIf="isStreaming" class="stop-spinner"></span>
          </button>
        </div>
        <p class="input-hint">Enter to send · Shift+Enter for new line</p>
      </div>
    </div>
  `,
    styles: [`
    .chat-container { display:flex; flex-direction:column; height:100%; background:#1e1e2e; }
    .chat-header { display:flex; align-items:center; justify-content:space-between; padding:1rem 1.25rem; border-bottom:1px solid #1e293b; flex-shrink:0; }
    .chat-header-left { display:flex; align-items:center; gap:0.5rem; }
    .chat-icon { font-size:1.1rem; }
    .chat-title { font-size:0.9rem; font-weight:600; color:#e2e8f0; }
    .chat-subtitle { font-size:0.78rem; color:#64748b; }
    .close-btn { background:none; border:none; color:#64748b; cursor:pointer; font-size:1rem; }
    .messages-area { flex:1; overflow-y:auto; padding:1rem; display:flex; flex-direction:column; gap:1rem; scroll-behavior:smooth; }
    .welcome-msg { text-align:center; color:#64748b; padding:2rem 1rem; font-size:0.875rem; }
    .suggestion-chips { display:flex; flex-wrap:wrap; justify-content:center; gap:0.5rem; margin-top:1rem; }
    .suggestion-chip { background:#0f172a; border:1px solid #334155; color:#94a3b8; border-radius:20px; padding:0.4rem 0.85rem; font-size:0.78rem; cursor:pointer; transition:all 0.15s; }
    .suggestion-chip:hover { border-color:#6366f1; color:#c4b5fd; }
    .message { display:flex; gap:0.75rem; align-items:flex-start; }
    .message.user { flex-direction:row-reverse; }
    .message-avatar { font-size:1.25rem; flex-shrink:0; padding-top:0.1rem; }
    .message-body { flex:1; min-width:0; }
    .message.user .message-body { align-items:flex-end; display:flex; flex-direction:column; }
    .message-content { background:#0f172a; border-radius:12px; padding:0.75rem 1rem; font-size:0.875rem; line-height:1.6; color:#e2e8f0; white-space:pre-wrap; word-break:break-word; max-width:100%; }
    .message.user .message-content { background:#312e81; color:#e0e7ff; }
    .cursor { display:inline-block; animation:blink 1s step-end infinite; color:#a78bfa; }
    @keyframes blink { 0%,100%{opacity:1}50%{opacity:0} }
    .message-refs { margin-top:0.5rem; display:flex; flex-wrap:wrap; gap:0.4rem; align-items:center; }
    .refs-label { font-size:0.72rem; color:#64748b; }
    .ref-chip { background:#1e293b; border:1px solid #334155; border-radius:4px; padding:0.15rem 0.5rem; font-size:0.72rem; color:#94a3b8; cursor:pointer; font-family:monospace; }
    .ref-chip:hover { border-color:#6366f1; color:#c4b5fd; }
    .chat-input-area { border-top:1px solid #1e293b; padding:0.75rem 1rem; flex-shrink:0; }
    .input-wrapper { display:flex; gap:0.5rem; align-items:flex-end; background:#0f172a; border:1px solid #334155; border-radius:10px; padding:0.4rem 0.4rem 0.4rem 0.75rem; transition:border-color 0.2s; }
    .input-wrapper:focus-within { border-color:#6366f1; }
    .chat-textarea { flex:1; background:transparent; border:none; outline:none; color:#e2e8f0; font-size:0.875rem; resize:none; min-height:20px; max-height:120px; line-height:1.5; font-family:inherit; }
    .chat-textarea::placeholder { color:#475569; }
    .send-btn { width:32px; height:32px; border-radius:8px; background:#6366f1; border:none; color:white; cursor:pointer; display:flex; align-items:center; justify-content:center; flex-shrink:0; transition:opacity 0.2s; }
    .send-btn:disabled { opacity:0.4; cursor:not-allowed; }
    .send-btn svg { width:16px; height:16px; }
    .stop-spinner { width:14px; height:14px; border:2px solid rgba(255,255,255,0.3); border-top-color:white; border-radius:50%; animation:spin 0.7s linear infinite; display:block; }
    @keyframes spin { to{transform:rotate(360deg)} }
    .input-hint { font-size:0.7rem; color:#334155; margin-top:0.4rem; text-align:center; }
  `]
})
export class ChatComponent implements OnInit, OnDestroy {
    @Input() repoId!: string;
    @Input() nodeId?: string;
    @Input() nodeContext?: string;
    @Output() close = new EventEmitter<void>();

    @ViewChild('messagesArea') messagesAreaRef!: ElementRef<HTMLDivElement>;

    private ws = inject(WebSocketService);
    private cdr = inject(ChangeDetectorRef);
    private destroy$ = new Subject<void>();

    messages: ChatMessage[] = [];
    inputText = '';
    isStreaming = false;
    private streamingMsgId = '';

    suggestions = [
        'What are the main architectural layers?',
        'Which files have the most dependencies?',
        'Are there any circular dependencies?',
        'What does the main entry point do?',
    ];

    ngOnInit(): void {
        this.ws.messagesOfType('chat_token').pipe(
            takeUntil(this.destroy$),
            filter((msg) => msg.repoId === this.repoId),
        ).subscribe((msg) => {
            if (msg.token) {
                this.appendToken(msg.token, msg.references as FileRef[] | undefined);
            }
            if (msg.done) {
                this.finalizeStream();
            }
            this.cdr.markForCheck();
        });

        this.ws.messagesOfType('error').pipe(
            takeUntil(this.destroy$),
        ).subscribe((msg) => {
            this.finalizeStream();
            this.appendAssistantMessage(`Error: ${msg.error || 'Unknown error'}`, []);
            this.cdr.markForCheck();
        });
    }

    sendMessage(): void {
        const text = this.inputText.trim();
        if (!text || this.isStreaming) return;

        this.inputText = '';

        this.messages.push({
            id: uuidv4(),
            role: 'user',
            content: text,
            timestamp: new Date(),
        });

        this.streamingMsgId = uuidv4();
        this.messages.push({
            id: this.streamingMsgId,
            role: 'assistant',
            content: '',
            timestamp: new Date(),
            isStreaming: true,
        });

        this.isStreaming = true;

        const history = this.messages
            .filter((m) => !m.isStreaming && m.id !== this.streamingMsgId)
            .slice(-10)
            .map((m) => ({ role: m.role, content: m.content }));

        this.ws.sendChatMessage(this.repoId, text, this.nodeId, history);
        this.scrollToBottom();
        this.cdr.markForCheck();
    }

    private appendToken(token: string, refs?: FileRef[]): void {
        const msg = this.messages.find((m) => m.id === this.streamingMsgId);
        if (msg) {
            msg.content += token;
            if (refs?.length) msg.references = refs;
        }
        this.scrollToBottom();
    }

    private finalizeStream(): void {
        const msg = this.messages.find((m) => m.id === this.streamingMsgId);
        if (msg) msg.isStreaming = false;
        this.isStreaming = false;
        this.streamingMsgId = '';
        this.scrollToBottom();
    }

    private appendAssistantMessage(content: string, refs: FileRef[]): void {
        this.messages.push({ id: uuidv4(), role: 'assistant', content, timestamp: new Date(), references: refs });
        this.scrollToBottom();
    }

    onEnterKey(event: Event): void {
        const ke = event as KeyboardEvent;
        if (!ke.shiftKey) { event.preventDefault(); this.sendMessage(); }
    }

    useSuggestion(text: string): void { this.inputText = text; this.sendMessage(); }

    openRef(ref: FileRef): void { console.log('Open ref:', ref); }

    getFileName(path: string): string { return path.split('/').pop() || path; }

    trackById(_: number, msg: ChatMessage): string { return msg.id; }

    private scrollToBottom(): void {
        setTimeout(() => {
            const el = this.messagesAreaRef?.nativeElement;
            if (el) el.scrollTop = el.scrollHeight;
        }, 20);
    }

    ngOnDestroy(): void {
        this.destroy$.next();
        this.destroy$.complete();
    }
}