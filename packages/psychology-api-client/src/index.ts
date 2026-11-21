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
  PublicTherapistItem,
  PublicTherapistDetail,
  PublicAppointmentSlot,
  CreateAppointmentRequest,
  PublicAppointmentResponse,
  CancelAppointmentRequest,
  AdminTherapistResponse,
  CreateTherapistRequest,
  UpdateTherapistRequest,
  WorkingHourItem,
  SetWorkingHoursRequest,
  ScheduleExceptionItem,
  CreateScheduleExceptionRequest,
  GenerateSlotsRequest,
  AdminAppointmentSlot,
  AdminAppointmentListItem,
  AdminAppointmentDetail,
  RescheduleAppointmentRequest,
  AppointmentStatusHistoryItem,
  AppointmentStatus,
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

  // ---------------- Public Booking & Therapists (Phase 2) ----------------
  async listPublicTherapists(): Promise<PublicTherapistItem[]> {
    return this.request<PublicTherapistItem[]>('/public/therapists');
  }

  async getPublicTherapistDetail(id: string): Promise<PublicTherapistDetail> {
    return this.request<PublicTherapistDetail>(`/public/therapists/${id}`);
  }

  async listPublicTherapistSlots(id: string, fromDate?: string): Promise<PublicAppointmentSlot[]> {
    const query = fromDate ? `?from_date=${encodeURIComponent(fromDate)}` : '';
    return this.request<PublicAppointmentSlot[]>(`/public/therapists/${id}/slots${query}`);
  }

  async createPublicAppointment(data: CreateAppointmentRequest): Promise<PublicAppointmentResponse> {
    return this.request<PublicAppointmentResponse>('/public/appointments', {
      method: 'POST',
      body: JSON.stringify(data),
    });
  }

  async getPublicAppointmentStatus(id: string, lookupHash: string): Promise<PublicAppointmentResponse> {
    return this.request<PublicAppointmentResponse>(`/public/appointments/${id}`, {
      headers: {
        'X-Guest-Lookup-Hash': lookupHash,
      },
    });
  }

  async cancelPublicAppointment(
    id: string,
    lookupHash: string,
    data?: CancelAppointmentRequest
  ): Promise<PublicAppointmentResponse> {
    return this.request<PublicAppointmentResponse>(`/public/appointments/${id}/cancel`, {
      method: 'POST',
      headers: {
        'X-Guest-Lookup-Hash': lookupHash,
      },
      body: JSON.stringify(data || {}),
    });
  }

  // ---------------- Admin Booking & Therapists (Phase 2) ----------------
  async listAdminTherapists(): Promise<AdminTherapistResponse[]> {
    return this.request<AdminTherapistResponse[]>('/admin/therapists');
  }

  async createAdminTherapist(data: CreateTherapistRequest): Promise<AdminTherapistResponse> {
    return this.request<AdminTherapistResponse>('/admin/therapists', {
      method: 'POST',
      body: JSON.stringify(data),
    });
  }

  async getAdminTherapist(id: string): Promise<AdminTherapistResponse> {
    return this.request<AdminTherapistResponse>(`/admin/therapists/${id}`);
  }

  async updateAdminTherapist(id: string, data: UpdateTherapistRequest): Promise<AdminTherapistResponse> {
    return this.request<AdminTherapistResponse>(`/admin/therapists/${id}`, {
      method: 'PUT',
      body: JSON.stringify(data),
    });
  }

  async deleteAdminTherapist(id: string): Promise<void> {
    await this.request<void>(`/admin/therapists/${id}`, {
      method: 'DELETE',
    });
  }

  async getAdminWorkingHours(therapistId: string): Promise<WorkingHourItem[]> {
    return this.request<WorkingHourItem[]>(`/admin/therapists/${therapistId}/working-hours`);
  }

  async updateAdminWorkingHours(
    therapistId: string,
    data: SetWorkingHoursRequest
  ): Promise<WorkingHourItem[]> {
    return this.request<WorkingHourItem[]>(`/admin/therapists/${therapistId}/working-hours`, {
      method: 'PUT',
      body: JSON.stringify(data),
    });
  }

  async getAdminScheduleExceptions(therapistId: string): Promise<ScheduleExceptionItem[]> {
    return this.request<ScheduleExceptionItem[]>(`/admin/therapists/${therapistId}/exceptions`);
  }

  async createAdminScheduleException(
    therapistId: string,
    data: CreateScheduleExceptionRequest
  ): Promise<ScheduleExceptionItem> {
    return this.request<ScheduleExceptionItem>(`/admin/therapists/${therapistId}/exceptions`, {
      method: 'POST',
      body: JSON.stringify(data),
    });
  }

  async deleteAdminScheduleException(therapistId: string, exceptionId: string): Promise<void> {
    await this.request<void>(`/admin/therapists/${therapistId}/exceptions/${exceptionId}`, {
      method: 'DELETE',
    });
  }

  async listAdminAppointmentSlots(
    therapistId: string,
    from?: string,
    to?: string
  ): Promise<AdminAppointmentSlot[]> {
    const params = new URLSearchParams();
    if (from) params.set('from', from);
    if (to) params.set('to', to);
    const query = params.toString() ? `?${params.toString()}` : '';
    return this.request<AdminAppointmentSlot[]>(`/admin/therapists/${therapistId}/slots${query}`);
  }

  async generateAdminAppointmentSlots(
    therapistId: string,
    data: GenerateSlotsRequest
  ): Promise<{ generated_count: number; skipped_count: number }> {
    return this.request<{ generated_count: number; skipped_count: number }>(
      `/admin/therapists/${therapistId}/slots/generate`,
      {
        method: 'POST',
        body: JSON.stringify(data),
      }
    );
  }

  async listAdminAppointments(
    therapistId?: string,
    status?: AppointmentStatus
  ): Promise<AdminAppointmentListItem[]> {
    const params = new URLSearchParams();
    if (therapistId) params.set('therapist_id', therapistId);
    if (status) params.set('status', status);
    const query = params.toString() ? `?${params.toString()}` : '';
    return this.request<AdminAppointmentListItem[]>(`/admin/appointments${query}`);
  }

  async getAdminAppointment(id: string): Promise<AdminAppointmentDetail> {
    return this.request<AdminAppointmentDetail>(`/admin/appointments/${id}`);
  }

  async confirmAdminAppointment(id: string): Promise<AdminAppointmentDetail> {
    return this.request<AdminAppointmentDetail>(`/admin/appointments/${id}/confirm`, {
      method: 'POST',
    });
  }

  async cancelAdminAppointment(
    id: string,
    data?: CancelAppointmentRequest
  ): Promise<AdminAppointmentDetail> {
    return this.request<AdminAppointmentDetail>(`/admin/appointments/${id}/cancel`, {
      method: 'POST',
      body: JSON.stringify(data || {}),
    });
  }

  async rescheduleAdminAppointment(
    id: string,
    data: RescheduleAppointmentRequest
  ): Promise<AdminAppointmentDetail> {
    return this.request<AdminAppointmentDetail>(`/admin/appointments/${id}/reschedule`, {
      method: 'POST',
      body: JSON.stringify(data),
    });
  }

  async getAdminAppointmentHistory(id: string): Promise<AppointmentStatusHistoryItem[]> {
    return this.request<AppointmentStatusHistoryItem[]>(`/admin/appointments/${id}/history`);
  }
}
