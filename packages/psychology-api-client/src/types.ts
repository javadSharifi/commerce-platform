/**
 * Typed models corresponding to packages/contracts/psychology/openapi.yaml
 */

export interface ErrorResponse {
  code: string;
  message: string;
  details?: Record<string, unknown>;
}

export interface MessageResponse {
  message: string;
}

export interface PaginationMeta {
  page: number;
  limit: number;
  total: number;
  total_pages: number;
}

export type ThemeRadius = 'sm' | 'md' | 'lg';
export type ThemeScale = 'sm' | 'md' | 'lg';

export interface HomeProfile {
  introduction?: string;
  specialties?: string[];
  credentials?: string[];
  office_hours?: string;
  portrait_media_id?: string;
}

export interface PublicSiteResponse {
  home_profile?: HomeProfile;
  name: string;
  title: string;
  description: string;
  phone?: string;
  address?: string;
  primary_color: string;
  logo_url?: string | null;
  template_id: string;
  template_version: string;
  theme_radius: ThemeRadius;
  theme_scale: ThemeScale;
  palette_id: string;
}

export interface LoginRequest {
  email: string;
  password: string;
}

export interface PanelUser {
  id: string;
  email: string;
  full_name: string;
}

export type MembershipRole = 'owner' | 'content_manager' | 'receptionist' | 'therapist';

export interface Membership {
  tenant_id: string;
  tenant_name: string;
  tenant_slug: string;
  role: MembershipRole;
}

export interface AuthSessionResponse {
  token: string;
  user: PanelUser;
}

export interface AdminMeResponse {
  user: PanelUser;
  memberships: Membership[];
}

export interface SiteSettingsResponse {
  home_profile?: HomeProfile;
  tenant_id: string;
  primary_domain?: string;
  title: string;
  description: string;
  phone: string;
  address: string;
  primary_color: string;
  logo_media_id?: string | null;
  template_id: string;
  template_version: string;
  theme_radius: ThemeRadius;
  theme_scale: ThemeScale;
  palette_id?: string;
}

export interface SiteAppearanceResponse {
  template_id: string;
  template_version: string;
  palette_id: string;
  theme_radius: ThemeRadius;
  theme_scale: ThemeScale;
}

export interface UpdateSiteAppearanceRequest {
  template_id: string;
  palette_id: string;
  theme_radius?: ThemeRadius;
  theme_scale?: ThemeScale;
}

export interface UpdateSiteSettingsRequest {
  home_profile?: HomeProfile;
  title: string;
  description: string;
  phone?: string;
  address?: string;
  logo_media_id?: string | null;
}

export interface BuyerProfileResponse {
  id: string;
  phone: string;
  full_name?: string | null;
}

// ---------------- Public Workshops ----------------
export interface PublicWorkshopItem {
  id: string;
  title: string;
  topic?: string;
  slug: string;
  instructor_name: string;
  starts_at: string;
  ends_at: string;
  location: string;
  price_rials: number;
  remaining_capacity: number;
  is_registration_open: boolean;
}

export interface PublicWorkshopDetail extends PublicWorkshopItem {
  description: string;
  registration_deadline: string;
}

export interface RegisterWorkshopRequest {
  guest_name: string;
  guest_phone: string;
}

export type RegistrationStatus = 'pending_payment' | 'confirmed' | 'cancelled' | 'expired';

export interface RegisterWorkshopResponse {
  registration_id: string;
  status: 'confirmed' | 'pending_payment';
  lookup_token: string;
  payment_url?: string | null;
  expires_at?: string | null;
}

export interface RegistrationStatusResponse {
  registration_id: string;
  workshop_id: string;
  workshop_title: string;
  guest_name: string;
  status: RegistrationStatus;
  expires_at?: string | null;
}

// ---------------- Public Blog & Media ----------------
export type ContentBlockType = 'paragraph' | 'heading' | 'image' | 'video' | 'audio';

export interface ContentBlock {
  type: ContentBlockType;
  data: Record<string, unknown>;
}

interface BlogMetadata {
  author_name?: string;
  category?: string;
  is_featured?: boolean;
  cover_media_id?: string | null;
}

export interface PublicBlogPostItem extends BlogMetadata {
  id: string;
  title: string;
  slug: string;
  seo_description: string;
  published_at?: string | null;
}

export interface PublicBlogPostDetail extends PublicBlogPostItem {
  content_blocks: ContentBlock[];
}

export interface StreamUrlResponse {
  stream_url: string;
  expires_in_seconds: number;
}

// ---------------- Admin Workshops ----------------
export type WorkshopStatus = 'draft' | 'published' | 'closed' | 'cancelled';

export interface AdminWorkshopItem {
  topic?: string;
  id: string;
  title: string;
  slug: string;
  instructor_name: string;
  starts_at: string;
  ends_at: string;
  capacity: number;
  confirmed_count: number;
  pending_count: number;
  price_rials: number;
  status: WorkshopStatus;
  created_at: string;
}

export interface AdminWorkshopDetail extends AdminWorkshopItem {
  description: string;
  location: string;
  registration_deadline: string;
  updated_at: string;
}

export interface CreateWorkshopRequest {
  topic?: string;
  title: string;
  slug: string;
  description?: string;
  instructor_name: string;
  starts_at: string;
  ends_at: string;
  location: string;
  registration_deadline: string;
  capacity: number;
  price_rials: number;
}

export interface UpdateWorkshopRequest {
  topic?: string;
  title?: string;
  slug?: string;
  description?: string;
  instructor_name?: string;
  starts_at?: string;
  ends_at?: string;
  location?: string;
  registration_deadline?: string;
  capacity?: number;
  price_rials?: number;
}

export interface AdminRegistrationItem {
  id: string;
  guest_name: string;
  guest_phone: string;
  status: RegistrationStatus;
  purchase_id?: string | null;
  expires_at?: string | null;
  created_at: string;
}

// ---------------- Admin Payments & Refunds ----------------
export type PurchaseItemType = 'workshop' | 'course';
export type PurchaseStatus = 'pending' | 'completed' | 'refund_required' | 'refunded' | 'failed';

export interface AdminPurchaseItem {
  id: string;
  item_type: PurchaseItemType;
  item_id: string;
  title_snapshot: string;
  amount_rials: number;
  status: PurchaseStatus;
  tracking_code?: string | null;
  created_at: string;
}

export interface RecordRefundRequest {
  reference_number: string;
  reason: string;
  amount_rials: number;
}

export interface RefundResponse {
  refund_id: string;
  purchase_id: string;
  reference_number: string;
  amount_rials: number;
  registered_at: string;
}

// ---------------- Admin Blog ----------------
export type BlogPostStatus = 'draft' | 'published';

export interface AdminBlogPostItem extends BlogMetadata {
  id: string;
  title: string;
  slug: string;
  status: BlogPostStatus;
  published_at?: string | null;
  created_at: string;
}

export interface AdminBlogPostDetail extends BlogMetadata {
  id: string;
  title: string;
  slug: string;
  seo_description: string;
  content_blocks: ContentBlock[];
  status: BlogPostStatus;
  published_at?: string | null;
  created_at: string;
  updated_at: string;
}

export interface CreateBlogPostRequest extends BlogMetadata {
  title: string;
  slug: string;
  seo_description: string;
  content_blocks: ContentBlock[];
}

export interface UpdateBlogPostRequest extends BlogMetadata {
  title?: string;
  slug?: string;
  seo_description?: string;
  content_blocks?: ContentBlock[];
}

// ---------------- Admin Media ----------------
export type MediaAccessClass = 'public' | 'course_private';
export type MediaValidationStatus = 'pending' | 'verified' | 'rejected';

export interface RequestMediaUploadRequest {
  original_name: string;
  mime_type: string;
  size_bytes: number;
  access_class: MediaAccessClass;
}

export interface RequestMediaUploadResponse {
  upload_id: string;
  upload_url: string;
  staging_key: string;
  expires_at: string;
}

export interface FinalizeMediaUploadRequest {
  upload_id: string;
}

export interface MediaAssetResponse {
  id: string;
  storage_key: string;
  original_name: string;
  mime_type: string;
  size_bytes: number;
  access_class: MediaAccessClass;
  validation_status: MediaValidationStatus;
  created_at: string;
}

export interface MediaListResponse {
  items: MediaAssetResponse[];
  quota_used_bytes: number;
  quota_total_bytes: number;
}

// ---------------- Phase 2: Booking & Therapists ----------------
export type AppointmentStatus = 'pending' | 'confirmed' | 'cancelled' | 'expired' | 'completed';
export type SlotStatus = 'available' | 'reserved' | 'booked' | 'blocked';
export type StatusHistoryAction = 'created' | 'confirmed' | 'cancelled' | 'rescheduled' | 'expired' | 'completed';
export type StatusHistoryActorType = 'guest' | 'staff' | 'system';

export interface PublicTherapistItem {
  id: string;
  name: string;
  slug: string;
  title: string;
  avatar_asset_id?: string | null;
  session_duration_minutes: number;
}

export interface PublicTherapistDetail {
  id: string;
  name: string;
  slug: string;
  title: string;
  bio: string;
  avatar_asset_id?: string | null;
  session_duration_minutes: number;
}

export interface PublicAppointmentSlot {
  id: string;
  therapist_id: string;
  starts_at: string;
  ends_at: string;
  status: SlotStatus;
}

export interface CreateAppointmentRequest {
  therapist_id: string;
  slot_id: string;
  guest_name: string;
  guest_phone: string;
  /** @deprecated The API ignores this legacy field. */
  notes?: string;
}

export interface PublicAppointmentResponse {
  id: string;
  therapist_id: string;
  slot_id: string;
  guest_name: string;
  guest_lookup_hash: string;
  status: AppointmentStatus;
  expires_at: string;
  starts_at: string;
  ends_at: string;
}

export interface CancelAppointmentRequest {
  reason?: string;
}

export interface AdminTherapistResponse {
  id: string;
  user_id?: string | null;
  name: string;
  slug: string;
  title: string;
  bio: string;
  avatar_asset_id?: string | null;
  session_duration_minutes: number;
  is_active: boolean;
  created_at: string;
  updated_at: string;
}

export interface CreateTherapistRequest {
  user_id?: string | null;
  name: string;
  slug: string;
  title?: string;
  bio?: string;
  avatar_asset_id?: string | null;
  session_duration_minutes?: number;
  is_active?: boolean;
}

export interface UpdateTherapistRequest {
  user_id?: string | null;
  name?: string;
  slug?: string;
  title?: string;
  bio?: string;
  avatar_asset_id?: string | null;
  session_duration_minutes?: number;
  is_active?: boolean;
}

export interface WorkingHourItem {
  id?: string;
  day_of_week: number;
  start_time: string;
  end_time: string;
  is_active: boolean;
}

export interface SetWorkingHoursRequest {
  items: WorkingHourItem[];
}

export interface ScheduleExceptionItem {
  id: string;
  date: string;
  is_unavailable: boolean;
  start_time?: string | null;
  end_time?: string | null;
  reason?: string;
}

export interface CreateScheduleExceptionRequest {
  date: string;
  is_unavailable?: boolean;
  start_time?: string | null;
  end_time?: string | null;
  reason?: string;
}

export interface GenerateSlotsRequest {
  from_date: string;
  to_date: string;
}

export interface AdminAppointmentSlot {
  id: string;
  therapist_id: string;
  starts_at: string;
  ends_at: string;
  status: SlotStatus;
}

export interface AdminAppointmentListItem {
  id: string;
  therapist_id: string;
  therapist_name: string;
  slot_id: string;
  starts_at: string;
  ends_at: string;
  guest_name: string;
  guest_phone: string;
  status: AppointmentStatus;
  created_at: string;
}

export interface AdminAppointmentDetail {
  id: string;
  therapist_id: string;
  therapist_name: string;
  slot_id: string;
  starts_at: string;
  ends_at: string;
  guest_name: string;
  guest_phone: string;
  notes: string;
  status: AppointmentStatus;
  expires_at: string;
  confirmed_at?: string | null;
  cancelled_at?: string | null;
  cancellation_reason: string;
  created_at: string;
  updated_at: string;
}

export interface RescheduleAppointmentRequest {
  new_slot_id: string;
  reason?: string;
}

export interface AppointmentStatusHistoryItem {
  id: string;
  appointment_id: string;
  from_status?: string | null;
  to_status: string;
  action: StatusHistoryAction;
  actor_type: StatusHistoryActorType;
  actor_id?: string | null;
  reason: string;
  metadata?: Record<string, unknown>;
  created_at: string;
}

// ------------------------------------------------------------------------------------------------
// Phase 3: Recorded Courses & Lessons Types (PSY-3.1)
// ------------------------------------------------------------------------------------------------

export type CourseStatus = 'draft' | 'published' | 'archived';

export interface PublicCourseItem {
  id: string;
  title: string;
  topic?: string;
  slug: string;
  description: string;
  instructor_name: string;
  price_rials: number;
  cover_media_id?: string | null;
  status: 'published';
  published_at?: string | null;
  total_lessons_count?: number;
  lessons_count?: number;
  total_duration_seconds?: number;
}

export interface PublicLessonItem {
  id: string;
  title: string;
  description: string;
  display_order: number;
  duration_seconds: number;
  is_free_preview: boolean;
}

export interface PublicCourseDetail {
  id: string;
  title: string;
  topic?: string;
  slug: string;
  description: string;
  instructor_name: string;
  price_rials: number;
  cover_media_id?: string | null;
  status: CourseStatus;
  published_at?: string | null;
  lessons: PublicLessonItem[];
}

export interface LessonStreamResponse {
  stream_url: string;
  expires_at: string;
  expires_in_seconds?: number;
  duration_seconds?: number;
  mime_type?: string;
  size_bytes?: number;
}

export interface CourseCheckoutRequest {
  idempotency_key: string;
  callback_url: string;
}

export interface CourseCheckoutResponse {
  payment_attempt_id: string;
  purchase_id: string;
  authority: string;
  payment_url: string;
  amount_rials: number;
  status: 'initiated';
  expires_at?: string | null;
}

export interface SendOTPRequest {
  phone: string;
}

export interface SendOTPResponse {
  message: string;
  expires_in_seconds: number;
}

export interface VerifyOTPRequest {
  phone: string;
  code: string;
}

export interface BuyerAccountMe {
  id: string;
  phone: string;
  full_name?: string | null;
  status: 'active' | 'blocked';
}

export interface VerifyOTPResponse {
  buyer: BuyerAccountMe;
}

export interface BuyerCourseEntitlementItem {
  id: string;
  course_id: string;
  course_slug?: string;
  course_title_snapshot: string;
  price_rials_snapshot: number;
  status: 'active' | 'revoked';
  granted_at: string;
}

export interface AdminCourseItem {
  topic?: string;
  id: string;
  title: string;
  slug: string;
  description: string;
  instructor_name: string;
  price_rials: number;
  cover_media_id?: string | null;
  status: CourseStatus;
  lessons_count?: number;
  published_at?: string | null;
  created_at: string;
  updated_at: string;
}

export interface CreateCourseRequest {
  topic?: string;
  title: string;
  slug: string;
  description?: string;
  instructor_name: string;
  price_rials: number;
  cover_media_id?: string | null;
}

export interface UpdateCourseRequest {
  topic?: string;
  title?: string;
  slug?: string;
  description?: string;
  instructor_name?: string;
  price_rials?: number;
  cover_media_id?: string | null;
}

export interface AdminLessonItem {
  id: string;
  course_id: string;
  title: string;
  description: string;
  display_order: number;
  media_asset_id: string;
  duration_seconds: number;
  is_free_preview: boolean;
  created_at: string;
  updated_at?: string;
}

export interface CreateLessonRequest {
  title: string;
  description?: string;
  display_order: number;
  media_asset_id: string;
  duration_seconds?: number;
  is_free_preview?: boolean;
}

export interface UpdateLessonRequest {
  title?: string;
  description?: string;
  display_order?: number;
  media_asset_id?: string;
  duration_seconds?: number;
  is_free_preview?: boolean;
}

export interface AdminCourseEntitlementItem {
  id: string;
  buyer_id: string;
  buyer_phone: string;
  buyer_full_name: string;
  status: 'active' | 'revoked';
  course_title_snapshot: string;
  price_rials_snapshot: number;
  granted_at: string;
  revoked_at?: string | null;
  revocation_reason?: string | null;
}

export interface WorkshopMetrics {
  total_workshops: number;
  published_workshops: number;
  draft_workshops: number;
  closed_workshops: number;
  cancelled_workshops: number;
  total_registrations: number;
  confirmed_registrations: number;
  pending_registrations: number;
  cancelled_registrations: number;
}

export interface BookingMetrics {
  active_therapists: number;
  total_slots: number;
  available_slots: number;
  booked_slots: number;
  held_slots: number;
  total_appointments: number;
  confirmed_appointments: number;
  pending_appointments: number;
  completed_appointments: number;
  cancelled_appointments: number;
}

export interface CourseMetrics {
  total_courses: number;
  published_courses: number;
  draft_courses: number;
  archived_courses: number;
  total_lessons: number;
  total_entitlements: number;
  active_entitlements: number;
  revoked_entitlements: number;
  pending_entitlements: number;
}

export interface FinancialMetrics {
  total_purchases: number;
  completed_purchases: number;
  pending_purchases: number;
  failed_purchases: number;
  refund_required_purchases: number;
  refunded_purchases: number;
  gross_revenue_rials: number;
  total_refunded_rials: number;
  net_revenue_rials: number;
  workshop_revenue_rials: number;
  course_revenue_rials: number;
}

export interface PaymentAttemptMetrics {
  total_attempts: number;
  initiated_attempts: number;
  verified_attempts: number;
  failed_attempts: number;
}

export interface MediaMetrics {
  total_assets: number;
  total_bytes: number;
  storage_quota_bytes: number;
  usage_percentage_permille: number;
  usage_percentage: number;
  public_bytes: number;
  course_private_bytes: number;
  video_count: number;
  audio_count: number;
  image_count: number;
}

export interface TenantMetricsReport {
  tenant_id: string;
  generated_at: string;
  workshops: WorkshopMetrics;
  booking: BookingMetrics;
  courses: CourseMetrics;
  financial: FinancialMetrics;
  payment_attempts: PaymentAttemptMetrics;
  media: MediaMetrics;
}



