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
  SiteAppearanceResponse,
  SiteSettingsResponse,
  StreamUrlResponse,
  UpdateBlogPostRequest,
  UpdateSiteAppearanceRequest,
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
  CourseStatus,
  PublicCourseItem,
  PublicCourseDetail,
  PublicLessonItem,
  LessonStreamResponse,
  CourseCheckoutRequest,
  CourseCheckoutResponse,
  SendOTPRequest,
  SendOTPResponse,
  VerifyOTPRequest,
  VerifyOTPResponse,
  BuyerAccountMe,
  BuyerCourseEntitlementItem,
  AdminCourseItem,
  CreateCourseRequest,
  UpdateCourseRequest,
  AdminLessonItem,
  CreateLessonRequest,
  UpdateLessonRequest,
  AdminCourseEntitlementItem,
  TenantMetricsReport,
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
    const normalizedPath = path.startsWith('/v1/')
      ? path
      : `/v1${path.startsWith('/') ? path : '/' + path}`;
    const url = `${this.baseUrl}${normalizedPath}`;
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

  async getAdminAppearance(): Promise<SiteAppearanceResponse> {
    return this.request<SiteAppearanceResponse>('/v1/admin/site/appearance');
  }

  async updateAdminAppearance(data: UpdateSiteAppearanceRequest): Promise<SiteAppearanceResponse> {
    return this.request<SiteAppearanceResponse>('/v1/admin/site/appearance', {
      method: 'PUT',
      body: JSON.stringify(data),
    });
  }

  // ---------------- Public Workshops ----------------
  async listPublicWorkshops(): Promise<PublicWorkshopItem[]> {
    return this.request<PublicWorkshopItem[]>('/v1/public/workshops');
  }

  async getPublicWorkshop(id: string): Promise<PublicWorkshopDetail> {
    return this.request<PublicWorkshopDetail>(`/v1/public/workshops/${id}`);
  }

  async registerPublicWorkshop(id: string, data: RegisterWorkshopRequest): Promise<RegisterWorkshopResponse> {
    return this.request<RegisterWorkshopResponse>(`/v1/public/workshops/${id}/register`, {
      method: 'POST',
      body: JSON.stringify(data),
    });
  }

  async getPublicRegistrationStatus(id: string, lookupToken: string): Promise<RegistrationStatusResponse> {
    return this.request<RegistrationStatusResponse>(`/v1/public/workshops/registrations/${id}?lookup_token=${encodeURIComponent(lookupToken)}`);
  }

  // ---------------- Admin Workshops ----------------
  async listAdminWorkshops(status?: WorkshopStatus): Promise<AdminWorkshopItem[]> {
    const query = status ? `?status=${encodeURIComponent(status)}` : '';
    return this.request<AdminWorkshopItem[]>(`/v1/admin/workshops${query}`);
  }

  async getAdminWorkshop(id: string): Promise<AdminWorkshopDetail> {
    return this.request<AdminWorkshopDetail>(`/v1/admin/workshops/${id}`);
  }

  async createAdminWorkshop(data: CreateWorkshopRequest): Promise<AdminWorkshopDetail> {
    return this.request<AdminWorkshopDetail>('/v1/admin/workshops', {
      method: 'POST',
      body: JSON.stringify(data),
    });
  }

  async updateAdminWorkshop(id: string, data: UpdateWorkshopRequest): Promise<AdminWorkshopDetail> {
    return this.request<AdminWorkshopDetail>(`/v1/admin/workshops/${id}`, {
      method: 'PUT',
      body: JSON.stringify(data),
    });
  }

  async publishAdminWorkshop(id: string): Promise<AdminWorkshopDetail> {
    return this.request<AdminWorkshopDetail>(`/v1/admin/workshops/${id}/publish`, {
      method: 'POST',
    });
  }

  async closeAdminWorkshop(id: string): Promise<AdminWorkshopDetail> {
    return this.request<AdminWorkshopDetail>(`/v1/admin/workshops/${id}/close`, {
      method: 'POST',
    });
  }

  async cancelAdminWorkshop(id: string): Promise<AdminWorkshopDetail> {
    return this.request<AdminWorkshopDetail>(`/v1/admin/workshops/${id}/cancel`, {
      method: 'POST',
    });
  }

  async listAdminRegistrations(workshopId: string): Promise<AdminRegistrationItem[]> {
    return this.request<AdminRegistrationItem[]>(`/v1/admin/workshops/${workshopId}/registrations`);
  }

  async cancelAdminRegistration(workshopId: string, registrationId: string): Promise<MessageResponse> {
    return this.request<MessageResponse>(`/v1/admin/workshops/${workshopId}/registrations/${registrationId}/cancel`, {
      method: 'POST',
    });
  }

  // ---------------- Admin Payments & Refunds ----------------
  async listAdminPayments(status?: string): Promise<AdminPurchaseItem[]> {
    const query = status ? `?status=${encodeURIComponent(status)}` : '';
    return this.request<AdminPurchaseItem[]>(`/v1/admin/payments${query}`);
  }

  async recordAdminRefund(purchaseId: string, data: RecordRefundRequest): Promise<RefundResponse> {
    return this.request<RefundResponse>(`/v1/admin/payments/${purchaseId}/refund`, {
      method: 'POST',
      body: JSON.stringify(data),
    });
  }

  // ---------------- Public Blog & Media ----------------
  async listPublicBlogPosts(): Promise<PublicBlogPostItem[]> {
    return this.request<PublicBlogPostItem[]>('/v1/public/blog');
  }

  async getPublicBlogPost(slug: string): Promise<PublicBlogPostDetail> {
    return this.request<PublicBlogPostDetail>(`/v1/public/blog/${encodeURIComponent(slug)}`);
  }

  async getPublicMediaStreamUrl(mediaId: string): Promise<StreamUrlResponse> {
    return this.request<StreamUrlResponse>(`/v1/public/media/${encodeURIComponent(mediaId)}/stream-url`);
  }

  // ---------------- Admin Blog ----------------
  async listAdminBlogPosts(): Promise<AdminBlogPostItem[]> {
    return this.request<AdminBlogPostItem[]>('/v1/admin/blog');
  }

  async getAdminBlogPost(id: string): Promise<AdminBlogPostDetail> {
    return this.request<AdminBlogPostDetail>(`/v1/admin/blog/${id}`);
  }

  async createAdminBlogPost(data: CreateBlogPostRequest): Promise<AdminBlogPostDetail> {
    return this.request<AdminBlogPostDetail>('/v1/admin/blog', {
      method: 'POST',
      body: JSON.stringify(data),
    });
  }

  async updateAdminBlogPost(id: string, data: UpdateBlogPostRequest): Promise<AdminBlogPostDetail> {
    return this.request<AdminBlogPostDetail>(`/v1/admin/blog/${id}`, {
      method: 'PUT',
      body: JSON.stringify(data),
    });
  }

  async publishAdminBlogPost(id: string): Promise<AdminBlogPostDetail> {
    return this.request<AdminBlogPostDetail>(`/v1/admin/blog/${id}/publish`, {
      method: 'POST',
    });
  }

  async unpublishAdminBlogPost(id: string): Promise<AdminBlogPostDetail> {
    return this.request<AdminBlogPostDetail>(`/v1/admin/blog/${id}/unpublish`, {
      method: 'POST',
    });
  }

  async deleteAdminBlogPost(id: string): Promise<MessageResponse> {
    return this.request<MessageResponse>(`/v1/admin/blog/${id}`, {
      method: 'DELETE',
    });
  }

  // ---------------- Admin Media ----------------
  async requestMediaUploadUrl(data: RequestMediaUploadRequest): Promise<RequestMediaUploadResponse> {
    return this.request<RequestMediaUploadResponse>('/v1/admin/media/upload-url', {
      method: 'POST',
      body: JSON.stringify(data),
    });
  }

  async finalizeMediaUpload(data: FinalizeMediaUploadRequest): Promise<MediaAssetResponse> {
    return this.request<MediaAssetResponse>('/v1/admin/media/finalize', {
      method: 'POST',
      body: JSON.stringify(data),
    });
  }

  async listAdminMedia(): Promise<MediaListResponse> {
    return this.request<MediaListResponse>('/v1/admin/media');
  }

  // ---------------- Public Booking & Therapists (Phase 2) ----------------
  async listPublicTherapists(): Promise<PublicTherapistItem[]> {
    return this.request<PublicTherapistItem[]>('/v1/public/therapists');
  }

  async getPublicTherapistDetail(id: string): Promise<PublicTherapistDetail> {
    return this.request<PublicTherapistDetail>(`/v1/public/therapists/${id}`);
  }

  async listPublicTherapistSlots(id: string, fromDate?: string): Promise<PublicAppointmentSlot[]> {
    const query = fromDate ? `?from_date=${encodeURIComponent(fromDate)}` : '';
    return this.request<PublicAppointmentSlot[]>(`/v1/public/therapists/${id}/slots${query}`);
  }

  async createPublicAppointment(data: CreateAppointmentRequest): Promise<PublicAppointmentResponse> {
    return this.request<PublicAppointmentResponse>('/v1/public/appointments', {
      method: 'POST',
      body: JSON.stringify(data),
    });
  }

  async getPublicAppointmentStatus(id: string, lookupHash: string): Promise<PublicAppointmentResponse> {
    return this.request<PublicAppointmentResponse>(`/v1/public/appointments/${id}`, {
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
    return this.request<PublicAppointmentResponse>(`/v1/public/appointments/${id}/cancel`, {
      method: 'POST',
      headers: {
        'X-Guest-Lookup-Hash': lookupHash,
      },
      body: JSON.stringify(data || {}),
    });
  }

  // ---------------- Admin Booking & Therapists (Phase 2) ----------------
  async listAdminTherapists(): Promise<AdminTherapistResponse[]> {
    return this.request<AdminTherapistResponse[]>('/v1/admin/therapists');
  }

  async createAdminTherapist(data: CreateTherapistRequest): Promise<AdminTherapistResponse> {
    return this.request<AdminTherapistResponse>('/v1/admin/therapists', {
      method: 'POST',
      body: JSON.stringify(data),
    });
  }

  async getAdminTherapist(id: string): Promise<AdminTherapistResponse> {
    return this.request<AdminTherapistResponse>(`/v1/admin/therapists/${id}`);
  }

  async updateAdminTherapist(id: string, data: UpdateTherapistRequest): Promise<AdminTherapistResponse> {
    return this.request<AdminTherapistResponse>(`/v1/admin/therapists/${id}`, {
      method: 'PUT',
      body: JSON.stringify(data),
    });
  }

  async deleteAdminTherapist(id: string): Promise<void> {
    await this.request<void>(`/v1/admin/therapists/${id}`, {
      method: 'DELETE',
    });
  }

  async getAdminWorkingHours(therapistId: string): Promise<WorkingHourItem[]> {
    return this.request<WorkingHourItem[]>(`/v1/admin/therapists/${therapistId}/working-hours`);
  }

  async updateAdminWorkingHours(
    therapistId: string,
    data: SetWorkingHoursRequest
  ): Promise<WorkingHourItem[]> {
    return this.request<WorkingHourItem[]>(`/v1/admin/therapists/${therapistId}/working-hours`, {
      method: 'PUT',
      body: JSON.stringify(data),
    });
  }

  async getAdminScheduleExceptions(therapistId: string): Promise<ScheduleExceptionItem[]> {
    return this.request<ScheduleExceptionItem[]>(`/v1/admin/therapists/${therapistId}/exceptions`);
  }

  async createAdminScheduleException(
    therapistId: string,
    data: CreateScheduleExceptionRequest
  ): Promise<ScheduleExceptionItem> {
    return this.request<ScheduleExceptionItem>(`/v1/admin/therapists/${therapistId}/exceptions`, {
      method: 'POST',
      body: JSON.stringify(data),
    });
  }

  async deleteAdminScheduleException(therapistId: string, exceptionId: string): Promise<void> {
    await this.request<void>(`/v1/admin/therapists/${therapistId}/exceptions/${exceptionId}`, {
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
    return this.request<AdminAppointmentSlot[]>(`/v1/admin/therapists/${therapistId}/slots${query}`);
  }

  async generateAdminAppointmentSlots(
    therapistId: string,
    data: GenerateSlotsRequest
  ): Promise<{ generated_count: number; skipped_count: number }> {
    return this.request<{ generated_count: number; skipped_count: number }>(
      `/v1/admin/therapists/${therapistId}/slots/generate`,
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
    return this.request<AdminAppointmentListItem[]>(`/v1/admin/appointments${query}`);
  }

  async getAdminAppointment(id: string): Promise<AdminAppointmentDetail> {
    return this.request<AdminAppointmentDetail>(`/v1/admin/appointments/${id}`);
  }

  async confirmAdminAppointment(id: string): Promise<AdminAppointmentDetail> {
    return this.request<AdminAppointmentDetail>(`/v1/admin/appointments/${id}/confirm`, {
      method: 'POST',
    });
  }

  async cancelAdminAppointment(
    id: string,
    data?: CancelAppointmentRequest
  ): Promise<AdminAppointmentDetail> {
    return this.request<AdminAppointmentDetail>(`/v1/admin/appointments/${id}/cancel`, {
      method: 'POST',
      body: JSON.stringify(data || {}),
    });
  }

  async rescheduleAdminAppointment(
    id: string,
    data: RescheduleAppointmentRequest
  ): Promise<AdminAppointmentDetail> {
    return this.request<AdminAppointmentDetail>(`/v1/admin/appointments/${id}/reschedule`, {
      method: 'POST',
      body: JSON.stringify(data),
    });
  }

  async getAdminAppointmentHistory(id: string): Promise<AppointmentStatusHistoryItem[]> {
    return this.request<AppointmentStatusHistoryItem[]>(`/v1/admin/appointments/${id}/history`);
  }

  // ---------------- Public Courses (Phase 3 - PSY-3.1) ----------------
  async listPublicCourses(): Promise<PublicCourseItem[]> {
    return this.request<PublicCourseItem[]>('/v1/public/courses');
  }

  async getPublicCourse(slug: string): Promise<PublicCourseDetail> {
    return this.request<PublicCourseDetail>(`/v1/public/courses/${slug}`);
  }

  async getPublicLessonPreviewStream(slug: string, lessonId: string): Promise<LessonStreamResponse> {
    return this.request<LessonStreamResponse>(`/v1/public/courses/${slug}/lessons/${lessonId}/preview-stream`);
  }

  async initiateCourseCheckout(slug: string, data: CourseCheckoutRequest): Promise<CourseCheckoutResponse> {
    return this.request<CourseCheckoutResponse>(`/v1/public/courses/${slug}/checkout`, {
      method: 'POST',
      body: JSON.stringify(data),
    });
  }

  // ---------------- Buyer Auth & Account (Phase 3 - PSY-3.1) ----------------
  async sendBuyerOTP(data: SendOTPRequest): Promise<SendOTPResponse> {
    return this.request<SendOTPResponse>('/v1/account/auth/otp/send', {
      method: 'POST',
      body: JSON.stringify(data),
    });
  }

  async verifyBuyerOTP(data: VerifyOTPRequest): Promise<VerifyOTPResponse> {
    return this.request<VerifyOTPResponse>('/v1/account/auth/otp/verify', {
      method: 'POST',
      body: JSON.stringify(data),
    });
  }

  async logoutBuyer(): Promise<MessageResponse> {
    return this.request<MessageResponse>('/v1/account/auth/logout', {
      method: 'POST',
    });
  }

  async getBuyerMe(): Promise<BuyerAccountMe> {
    return this.request<BuyerAccountMe>('/v1/account/auth/me');
  }

  async listMyCourses(): Promise<BuyerCourseEntitlementItem[]> {
    return this.request<BuyerCourseEntitlementItem[]>('/v1/account/courses');
  }

  async getCourseLessonStreamUrl(courseId: string, lessonId: string): Promise<LessonStreamResponse> {
    return this.request<LessonStreamResponse>(`/v1/account/courses/${courseId}/lessons/${lessonId}/stream`);
  }

  // ---------------- Admin Courses & Lessons (Phase 3 - PSY-3.1) ----------------
  async listAdminCourses(status?: CourseStatus): Promise<AdminCourseItem[]> {
    const query = status ? `?status=${encodeURIComponent(status)}` : '';
    return this.request<AdminCourseItem[]>(`/v1/admin/courses${query}`);
  }

  async createAdminCourse(data: CreateCourseRequest): Promise<AdminCourseItem> {
    return this.request<AdminCourseItem>('/v1/admin/courses', {
      method: 'POST',
      body: JSON.stringify(data),
    });
  }

  async getAdminCourse(id: string): Promise<AdminCourseItem> {
    return this.request<AdminCourseItem>(`/v1/admin/courses/${id}`);
  }

  async updateAdminCourse(id: string, data: UpdateCourseRequest): Promise<AdminCourseItem> {
    return this.request<AdminCourseItem>(`/v1/admin/courses/${id}`, {
      method: 'PUT',
      body: JSON.stringify(data),
    });
  }

  async deleteAdminCourse(id: string): Promise<MessageResponse> {
    return this.request<MessageResponse>(`/v1/admin/courses/${id}`, {
      method: 'DELETE',
    });
  }

  async publishAdminCourse(id: string): Promise<AdminCourseItem> {
    return this.request<AdminCourseItem>(`/v1/admin/courses/${id}/publish`, {
      method: 'POST',
    });
  }

  async archiveAdminCourse(id: string): Promise<AdminCourseItem> {
    return this.request<AdminCourseItem>(`/v1/admin/courses/${id}/archive`, {
      method: 'POST',
    });
  }

  async listAdminCourseLessons(courseId: string): Promise<AdminLessonItem[]> {
    return this.request<AdminLessonItem[]>(`/v1/admin/courses/${courseId}/lessons`);
  }

  async createAdminCourseLesson(courseId: string, data: CreateLessonRequest): Promise<AdminLessonItem> {
    return this.request<AdminLessonItem>(`/v1/admin/courses/${courseId}/lessons`, {
      method: 'POST',
      body: JSON.stringify(data),
    });
  }

  async getAdminCourseLesson(courseId: string, lessonId: string): Promise<AdminLessonItem> {
    return this.request<AdminLessonItem>(`/v1/admin/courses/${courseId}/lessons/${lessonId}`);
  }

  async updateAdminCourseLesson(courseId: string, lessonId: string, data: UpdateLessonRequest): Promise<AdminLessonItem> {
    return this.request<AdminLessonItem>(`/v1/admin/courses/${courseId}/lessons/${lessonId}`, {
      method: 'PUT',
      body: JSON.stringify(data),
    });
  }

  async deleteAdminCourseLesson(courseId: string, lessonId: string): Promise<MessageResponse> {
    return this.request<MessageResponse>(`/v1/admin/courses/${courseId}/lessons/${lessonId}`, {
      method: 'DELETE',
    });
  }

  async listAdminCourseEntitlements(courseId: string): Promise<AdminCourseEntitlementItem[]> {
    return this.request<AdminCourseEntitlementItem[]>(`/v1/admin/courses/${courseId}/entitlements`);
  }

  // ---------------- Admin Reports & Analytics (Phase 4 - PSY-4.1) ----------------
  async getAdminReportsMetrics(): Promise<TenantMetricsReport> {
    return this.request<TenantMetricsReport>('/v1/admin/reports/metrics');
  }
}
