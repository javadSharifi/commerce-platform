import type {
  AdminBlogPostDetail,
  AdminBlogPostItem,
  AdminMeResponse,
  AdminPurchaseItem,
  AdminRegistrationItem,
  AdminWorkshopDetail,
  AdminWorkshopItem,
  AuthSessionResponse,
  BlogPostStatus,
  CreateBlogPostRequest,
  CreateWorkshopRequest,
  ErrorResponse,
  FinalizeMediaUploadRequest,
  LoginRequest,
  MediaAssetResponse,
  MediaListResponse,
  MessageResponse,
  PublicBlogPostDetail,
  PublicBlogPostItem,
  PublicSiteResponse,
  PublicWorkshopDetail,
  PublicWorkshopItem,
  RecordRefundRequest,
  RefundResponse,
  RegisterWorkshopRequest,
  RegisterWorkshopResponse,
  RegistrationStatusResponse,
  RequestMediaUploadRequest,
  RequestMediaUploadResponse,
  SiteSettingsResponse,
  StreamUrlResponse,
  UpdateBlogPostRequest,
  UpdateSiteSettingsRequest,
  UpdateWorkshopRequest,
  WorkshopStatus,
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

  // ---------------- Public Workshops ----------------
  async listPublicWorkshops(): Promise<PublicWorkshopItem[]> {
    return this.request<PublicWorkshopItem[]>('/public/workshops');
  }

  async getPublicWorkshop(id: string): Promise<PublicWorkshopDetail> {
    return this.request<PublicWorkshopDetail>(`/public/workshops/${id}`);
  }

  async registerPublicWorkshop(id: string, data: RegisterWorkshopRequest): Promise<RegisterWorkshopResponse> {
    return this.request<RegisterWorkshopResponse>(`/public/workshops/${id}/register`, {
      method: 'POST',
      body: JSON.stringify(data),
    });
  }

  async getPublicRegistrationStatus(id: string, lookupToken: string): Promise<RegistrationStatusResponse> {
    return this.request<RegistrationStatusResponse>(`/public/workshops/registrations/${id}?lookup_token=${encodeURIComponent(lookupToken)}`);
  }

  // ---------------- Admin Workshops ----------------
  async listAdminWorkshops(status?: WorkshopStatus): Promise<AdminWorkshopItem[]> {
    const query = status ? `?status=${encodeURIComponent(status)}` : '';
    return this.request<AdminWorkshopItem[]>(`/admin/workshops${query}`);
  }

  async getAdminWorkshop(id: string): Promise<AdminWorkshopDetail> {
    return this.request<AdminWorkshopDetail>(`/admin/workshops/${id}`);
  }

  async createAdminWorkshop(data: CreateWorkshopRequest): Promise<AdminWorkshopDetail> {
    return this.request<AdminWorkshopDetail>('/admin/workshops', {
      method: 'POST',
      body: JSON.stringify(data),
    });
  }

  async updateAdminWorkshop(id: string, data: UpdateWorkshopRequest): Promise<AdminWorkshopDetail> {
    return this.request<AdminWorkshopDetail>(`/admin/workshops/${id}`, {
      method: 'PUT',
      body: JSON.stringify(data),
    });
  }

  async publishAdminWorkshop(id: string): Promise<AdminWorkshopDetail> {
    return this.request<AdminWorkshopDetail>(`/admin/workshops/${id}/publish`, {
      method: 'POST',
    });
  }

  async closeAdminWorkshop(id: string): Promise<AdminWorkshopDetail> {
    return this.request<AdminWorkshopDetail>(`/admin/workshops/${id}/close`, {
      method: 'POST',
    });
  }

  async cancelAdminWorkshop(id: string): Promise<AdminWorkshopDetail> {
    return this.request<AdminWorkshopDetail>(`/admin/workshops/${id}/cancel`, {
      method: 'POST',
    });
  }

  async listAdminRegistrations(workshopId: string): Promise<AdminRegistrationItem[]> {
    return this.request<AdminRegistrationItem[]>(`/admin/workshops/${workshopId}/registrations`);
  }

  async cancelAdminRegistration(workshopId: string, registrationId: string): Promise<MessageResponse> {
    return this.request<MessageResponse>(`/admin/workshops/${workshopId}/registrations/${registrationId}/cancel`, {
      method: 'POST',
    });
  }

  // ---------------- Admin Payments & Refunds ----------------
  async listAdminPayments(status?: string): Promise<AdminPurchaseItem[]> {
    const query = status ? `?status=${encodeURIComponent(status)}` : '';
    return this.request<AdminPurchaseItem[]>(`/admin/payments${query}`);
  }

  async recordAdminRefund(purchaseId: string, data: RecordRefundRequest): Promise<RefundResponse> {
    return this.request<RefundResponse>(`/admin/payments/${purchaseId}/refund`, {
      method: 'POST',
      body: JSON.stringify(data),
    });
  }

  // ---------------- Public Blog & Media ----------------
  async listPublicBlogPosts(): Promise<PublicBlogPostItem[]> {
    return this.request<PublicBlogPostItem[]>('/public/blog');
  }

  async getPublicBlogPost(slug: string): Promise<PublicBlogPostDetail> {
    return this.request<PublicBlogPostDetail>(`/public/blog/${encodeURIComponent(slug)}`);
  }

  async getPublicMediaStreamUrl(mediaId: string): Promise<StreamUrlResponse> {
    return this.request<StreamUrlResponse>(`/public/media/${encodeURIComponent(mediaId)}/stream-url`);
  }

  // ---------------- Admin Blog ----------------
  async listAdminBlogPosts(): Promise<AdminBlogPostItem[]> {
    return this.request<AdminBlogPostItem[]>('/admin/blog');
  }

  async getAdminBlogPost(id: string): Promise<AdminBlogPostDetail> {
    return this.request<AdminBlogPostDetail>(`/admin/blog/${id}`);
  }

  async createAdminBlogPost(data: CreateBlogPostRequest): Promise<AdminBlogPostDetail> {
    return this.request<AdminBlogPostDetail>('/admin/blog', {
      method: 'POST',
      body: JSON.stringify(data),
    });
  }

  async updateAdminBlogPost(id: string, data: UpdateBlogPostRequest): Promise<AdminBlogPostDetail> {
    return this.request<AdminBlogPostDetail>(`/admin/blog/${id}`, {
      method: 'PUT',
      body: JSON.stringify(data),
    });
  }

  async publishAdminBlogPost(id: string): Promise<AdminBlogPostDetail> {
    return this.request<AdminBlogPostDetail>(`/admin/blog/${id}/publish`, {
      method: 'POST',
    });
  }

  async unpublishAdminBlogPost(id: string): Promise<AdminBlogPostDetail> {
    return this.request<AdminBlogPostDetail>(`/admin/blog/${id}/unpublish`, {
      method: 'POST',
    });
  }

  async deleteAdminBlogPost(id: string): Promise<MessageResponse> {
    return this.request<MessageResponse>(`/admin/blog/${id}`, {
      method: 'DELETE',
    });
  }

  // ---------------- Admin Media ----------------
  async requestMediaUploadUrl(data: RequestMediaUploadRequest): Promise<RequestMediaUploadResponse> {
    return this.request<RequestMediaUploadResponse>('/admin/media/upload-url', {
      method: 'POST',
      body: JSON.stringify(data),
    });
  }

  async finalizeMediaUpload(data: FinalizeMediaUploadRequest): Promise<MediaAssetResponse> {
    return this.request<MediaAssetResponse>('/admin/media/finalize', {
      method: 'POST',
      body: JSON.stringify(data),
    });
  }

  async listAdminMedia(): Promise<MediaListResponse> {
    return this.request<MediaListResponse>('/admin/media');
  }
}
