import { Component, Input, Output, EventEmitter } from '@angular/core';
import { CommonModule } from '@angular/common';
import { FormsModule } from '@angular/forms';
import { ApiService } from '../../services/api.service';

interface Message {
  role: 'user' | 'assistant';
  content: string;
  loading?: boolean;
}

@Component({
  selector: 'app-chat',
  standalone: true,
  imports: [CommonModule, FormsModule],
  templateUrl: './chat.component.html',
  styleUrls: ['./chat.component.css']
})
export class ChatComponent {
  @Input() projectId = '';
  @Input() currentPath = '';
  @Output() close = new EventEmitter<void>();

  messages: Message[] = [];
  input = '';
  sending = false;

  constructor(private api: ApiService) {
    this.messages = [
      {
        role: 'assistant',
        content: 'Hello! I can answer questions about this repository\'s architecture, modules, and dependencies. What would you like to know?'
      }
    ];
  }

  onEnter(event: Event) {
    const ke = event as KeyboardEvent;
    if (!ke.shiftKey) {
      ke.preventDefault();
      this.send();
    }
  }

  send() {
    const q = this.input.trim();
    if (!q || this.sending) return;
    this.input = '';
    this.messages.push({ role: 'user', content: q });
    this.messages.push({ role: 'assistant', content: '', loading: true });
    this.sending = true;

    this.api.chat(this.projectId, q, this.currentPath).subscribe({
      next: (res) => {
        const last = this.messages[this.messages.length - 1];
        last.content = res.answer;
        last.loading = false;
        this.sending = false;
        setTimeout(() => this.scrollToBottom(), 50);
      },
      error: (err) => {
        const last = this.messages[this.messages.length - 1];
        last.content = err.error?.detail || 'Failed to get response. Check ANTHROPIC_API_KEY.';
        last.loading = false;
        this.sending = false;
      }
    });

    setTimeout(() => this.scrollToBottom(), 50);
  }

  private scrollToBottom() {
    const el = document.querySelector('.messages');
    if (el) el.scrollTop = el.scrollHeight;
  }
}
