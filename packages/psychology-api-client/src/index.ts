import type {
  AdminMeResponse,
  AuthSessionResponse,
  ErrorResponse,
  LoginRequest,
  MessageResponse,
  PublicSiteResponse,
  SiteSettingsResponse,
  UpdateSiteSettingsRequest,
} from './types.js';

export * from './types.js';

export class ApiClientError extends Error {
  public readonly code: string;
  public readonly status: number;
  public readonly details?: Record<string, unknown>;

  constructor(status: number, errorResponse: ErrorResponse) {
    super(errorResponse.message);
    this.name = 'ApiClientError';
    this.status = status;
    this.code = errorResponse.code;
    this.details = errorResponse.details;
  }
}

export interface ClientConfig {
  baseUrl: string;
  headers?: Record<string, string>;
  fetch?: typeof fetch;
}

export class PsychologyApiClient {
  private baseUrl: string;
  private headers: Record<string, string>;
  private fetchImpl: typeof fetch;

  constructor(config: ClientConfig) {
    this.baseUrl = config.baseUrl.replace(/\/+$/, '');
    this.headers = config.headers || {};
    this.fetchImpl = config.fetch || globalThis.fetch;
  }

  private async request<T>(path: string, options: RequestInit = {}): Promise<T> {
    const url = `${this.baseUrl}${path}`;
    const headers = {
      'Content-Type': 'application/json',
      ...this.headers,
      ...(options.headers as Record<string, string>),
    };

    const response = await this.fetchImpl(url, {
      ...options,
      headers,
    });

    if (!response.ok) {
      let errorData: ErrorResponse;
      try {
        errorData = (await response.json()) as ErrorResponse;
      } catch {
        errorData = {
          code: 'HTTP_ERROR',
          message: `Request failed with status ${response.status}`,
        };
      }
      throw new ApiClientError(response.status, errorData);
    }

    return (await response.json()) as T;
  }

  // ---------------- Public API ----------------
  async getPublicSite(): Promise<PublicSiteResponse> {
    return this.request<PublicSiteResponse>('/v1/public/site');
  }

  // ---------------- Admin Auth API ----------------
  async loginAdmin(data: LoginRequest): Promise<AuthSessionResponse> {
    return this.request<AuthSessionResponse>('/v1/admin/auth/login', {
      method: 'POST',
      body: JSON.stringify(data),
    });
  }

  async logoutAdmin(): Promise<MessageResponse> {
    return this.request<MessageResponse>('/v1/admin/auth/logout', {
      method: 'POST',
    });
  }

  async getAdminMe(): Promise<AdminMeResponse> {
    return this.request<AdminMeResponse>('/v1/admin/auth/me');
  }

  // ---------------- Admin Site Settings ----------------
  async getAdminSite(): Promise<SiteSettingsResponse> {
    return this.request<SiteSettingsResponse>('/v1/admin/site');
  }

  async updateAdminSite(data: UpdateSiteSettingsRequest): Promise<SiteSettingsResponse> {
    return this.request<SiteSettingsResponse>('/v1/admin/site', {
      method: 'PUT',
      body: JSON.stringify(data),
    });
  }
}
