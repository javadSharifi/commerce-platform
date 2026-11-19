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

export interface PublicSiteResponse {
  name: string;
  title: string;
  description: string;
  phone?: string;
  address?: string;
  primary_color: string;
  logo_url?: string | null;
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
  tenant_id: string;
  title: string;
  description: string;
  phone: string;
  address: string;
  primary_color: string;
  logo_media_id?: string | null;
}

export interface UpdateSiteSettingsRequest {
  title: string;
  description: string;
  phone?: string;
  address?: string;
  primary_color: string;
  logo_media_id?: string | null;
}

export interface BuyerProfileResponse {
  id: string;
  phone: string;
  full_name?: string | null;
}
