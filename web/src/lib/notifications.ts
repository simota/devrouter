// Desktop notification utilities for DevRouter

export type NotificationType = 'error' | 'warning' | 'info' | 'success';

interface NotificationOptions {
  title: string;
  body: string;
  type?: NotificationType;
  tag?: string;
  requireInteraction?: boolean;
}

const NOTIFICATION_ICONS: Record<NotificationType, string> = {
  error: 'data:image/svg+xml,<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="%23f85149"><circle cx="12" cy="12" r="10"/><line x1="15" y1="9" x2="9" y2="15" stroke="white" stroke-width="2"/><line x1="9" y1="9" x2="15" y2="15" stroke="white" stroke-width="2"/></svg>',
  warning: 'data:image/svg+xml,<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="%23d29922"><path d="M12 2L2 22h20L12 2z"/><line x1="12" y1="9" x2="12" y2="15" stroke="white" stroke-width="2"/><circle cx="12" cy="18" r="1" fill="white"/></svg>',
  info: 'data:image/svg+xml,<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="%2300e5ff"><circle cx="12" cy="12" r="10"/><line x1="12" y1="16" x2="12" y2="12" stroke="white" stroke-width="2"/><circle cx="12" cy="8" r="1" fill="white"/></svg>',
  success: 'data:image/svg+xml,<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="%233fb950"><circle cx="12" cy="12" r="10"/><polyline points="16,8 10,16 8,12" stroke="white" stroke-width="2" fill="none"/></svg>',
};

class NotificationManager {
  private permission: NotificationPermission = 'default';
  private enabled: boolean = true;
  private recentNotifications: Map<string, number> = new Map();
  private cooldownMs: number = 30000; // 30 seconds between duplicate notifications

  constructor() {
    this.loadSettings();
    this.checkPermission();
  }

  private loadSettings(): void {
    try {
      const saved = localStorage.getItem('devrouter:notifications');
      if (saved) {
        const settings = JSON.parse(saved);
        this.enabled = settings.enabled ?? true;
      }
    } catch {
      // Ignore errors
    }
  }

  private saveSettings(): void {
    try {
      localStorage.setItem('devrouter:notifications', JSON.stringify({
        enabled: this.enabled,
      }));
    } catch {
      // Ignore errors
    }
  }

  async checkPermission(): Promise<NotificationPermission> {
    if (!('Notification' in window)) {
      this.permission = 'denied';
      return this.permission;
    }
    this.permission = Notification.permission;
    return this.permission;
  }

  async requestPermission(): Promise<NotificationPermission> {
    if (!('Notification' in window)) {
      return 'denied';
    }

    this.permission = await Notification.requestPermission();
    return this.permission;
  }

  isSupported(): boolean {
    return 'Notification' in window;
  }

  isEnabled(): boolean {
    return this.enabled;
  }

  setEnabled(enabled: boolean): void {
    this.enabled = enabled;
    this.saveSettings();
  }

  getPermission(): NotificationPermission {
    return this.permission;
  }

  private shouldThrottle(tag: string): boolean {
    const now = Date.now();
    const lastTime = this.recentNotifications.get(tag);

    if (lastTime && now - lastTime < this.cooldownMs) {
      return true;
    }

    this.recentNotifications.set(tag, now);

    // Clean up old entries
    for (const [key, time] of this.recentNotifications.entries()) {
      if (now - time > this.cooldownMs * 2) {
        this.recentNotifications.delete(key);
      }
    }

    return false;
  }

  async notify(options: NotificationOptions): Promise<Notification | null> {
    if (!this.enabled || !this.isSupported() || this.permission !== 'granted') {
      return null;
    }

    const tag = options.tag || `${options.title}:${options.body}`;

    if (this.shouldThrottle(tag)) {
      return null;
    }

    try {
      const notification = new Notification(options.title, {
        body: options.body,
        icon: NOTIFICATION_ICONS[options.type || 'info'],
        tag,
        requireInteraction: options.requireInteraction ?? false,
      });

      // Auto-close after 5 seconds unless requireInteraction is true
      if (!options.requireInteraction) {
        setTimeout(() => notification.close(), 5000);
      }

      return notification;
    } catch (error) {
      console.error('Failed to show notification:', error);
      return null;
    }
  }

  // Convenience methods for common notification types
  error(title: string, body: string, tag?: string): Promise<Notification | null> {
    return this.notify({ title, body, type: 'error', tag, requireInteraction: true });
  }

  warning(title: string, body: string, tag?: string): Promise<Notification | null> {
    return this.notify({ title, body, type: 'warning', tag });
  }

  info(title: string, body: string, tag?: string): Promise<Notification | null> {
    return this.notify({ title, body, type: 'info', tag });
  }

  success(title: string, body: string, tag?: string): Promise<Notification | null> {
    return this.notify({ title, body, type: 'success', tag });
  }
}

// Singleton instance
export const notifications = new NotificationManager();

// Health check specific notifications
export function notifyServiceDown(stackName: string, serviceName: string): void {
  notifications.error(
    'Service Down',
    `${serviceName} in ${stackName} is not responding`,
    `service-down:${stackName}:${serviceName}`
  );
}

export function notifyServiceRecovered(stackName: string, serviceName: string): void {
  notifications.success(
    'Service Recovered',
    `${serviceName} in ${stackName} is now healthy`,
    `service-up:${stackName}:${serviceName}`
  );
}

export function notifyContainerStopped(stackName: string, serviceName: string): void {
  notifications.warning(
    'Container Stopped',
    `${serviceName} in ${stackName} has stopped`,
    `container-stopped:${stackName}:${serviceName}`
  );
}

export function notifyHighResourceUsage(
  stackName: string,
  serviceName: string,
  resource: 'cpu' | 'memory',
  percent: number
): void {
  notifications.warning(
    `High ${resource.toUpperCase()} Usage`,
    `${serviceName} in ${stackName} is using ${percent.toFixed(1)}% ${resource}`,
    `high-${resource}:${stackName}:${serviceName}`
  );
}
