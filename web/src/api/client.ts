export interface ApiError {
  error: string;
}

export class ApiClient {
  private static async request<T>(path: string, options: RequestInit = {}): Promise<T> {
    const defaultHeaders: Record<string, string> = {
      Accept: 'application/json',
    };

    if (options.body && typeof options.body === 'string') {
      defaultHeaders['Content-Type'] = 'application/json';
    }

    const res = await fetch(path, {
      ...options,
      credentials: 'same-origin',
      headers: {
        ...defaultHeaders,
        ...options.headers,
      },
    });

    if (res.status === 401) {
      if (!window.location.pathname.startsWith('/login')
        && !window.location.pathname.startsWith('/register')
        && !window.location.pathname.startsWith('/invite')) {
        window.location.href = '/login';
      }
      throw new Error('Unauthorized');
    }

    if (!res.ok) {
      let errMsg = `Request failed: ${res.statusText}`;
      try {
        const errJson: ApiError = await res.json();
        if (errJson.error) {
          errMsg = errJson.error;
        }
      } catch {
        // Fallback
      }
      throw new Error(errMsg);
    }

    // Check if response is JSON
    const contentType = res.headers.get('content-type');
    if (contentType && contentType.includes('application/json')) {
      return (await res.json()) as T;
    }

    return (await res.text()) as unknown as T;
  }

  static get<T>(path: string): Promise<T> {
    return this.request<T>(path, { method: 'GET' });
  }

  static post<T>(path: string, body?: unknown): Promise<T> {
    return this.request<T>(path, {
      method: 'POST',
      body: body ? JSON.stringify(body) : undefined,
    });
  }

  static patch<T>(path: string, body?: unknown): Promise<T> {
    return this.request<T>(path, {
      method: 'PATCH',
      body: body ? JSON.stringify(body) : undefined,
    });
  }

  static delete<T>(path: string): Promise<T> {
    return this.request<T>(path, { method: 'DELETE' });
  }
}
